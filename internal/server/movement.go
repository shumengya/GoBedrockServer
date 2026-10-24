package server

import "math"
import "time"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// 移动校验参数。
//
// 客户端每 tick 上报一次位置，理论上限就是原版的移动速度。这里给的是「宽容上限」：
// 只要不是明显的瞬移/飞行就放行，宁可漏判也不误伤正常玩家（网络抖动、骑乘、
// 被击退、服务器卡顿后攒了几帧一起发，都会让单包位移突然变大）。
// maxMoveSpeedPerTick 是水平方向每 tick 允许的格数，原版最快约 0.3，
// 这里放 10 倍余量专门抓瞬移，不去抓「跑得快了一点」。
const maxMoveSpeedPerTick float64 = 3

// maxFallSpeedPerTick 是垂直方向每 tick 允许的格数，留出自由落体的余量。
const maxFallSpeedPerTick float64 = 4

// moveTolerance 是固定容差，吸收浮点误差和小幅回弹。
const moveTolerance float64 = 1.5

// moveGraceTicks 是两次校验之间允许的最小间隔（tick）。
// 客户端偶尔会在一帧里连发几个包，间隔太短就跳过校验，不累积误判。
const moveGraceTicks int32 = 2

// moveRejectReason 返回拒绝这次移动的原因，空串表示放行。
// 判断依据是「从上一次合法位置到这次位置的距离」，按经过的 tick 数放大阈值。
func (srv *Server) moveRejectReason(player *Player, posX float32, posY float32, posZ float32, now time.Time) string {
	var last moveRecord = player.movementSnapshot()
	if !last.Known || last.At.IsZero() {
		return ""
	}
	var elapsed time.Duration = now.Sub(last.At)
	if elapsed <= 0 {
		return ""
	}
	var ticks float64 = float64(elapsed) / float64(50*time.Millisecond)
	if ticks < float64(moveGraceTicks) {
		return ""
	}
	if player.IsDead() {
		return ""
	}
	var diffX float64 = math.Abs(float64(posX) - float64(last.PosX))
	var diffY float64 = math.Abs(float64(posY) - float64(last.PosY))
	var diffZ float64 = math.Abs(float64(posZ) - float64(last.PosZ))
	var horizontal float64 = math.Sqrt(diffX*diffX + diffZ*diffZ)
	var limitHorizontal float64 = maxMoveSpeedPerTick*ticks + moveTolerance
	var limitVertical float64 = maxFallSpeedPerTick*ticks + moveTolerance
	if horizontal > limitHorizontal && diffY > limitVertical {
		return "水平与垂直同时超出"
	}
	if horizontal > limitHorizontal {
		return "水平移动过快"
	}
	if diffY > limitVertical {
		return "垂直移动过快"
	}
	if srv.clippedThroughTerrain(srv.playerWorld(player), last, posX, posY, posZ) {
		return "穿墙"
	}
	return ""
}

// clippedThroughTerrain 粗查穿墙：目标位置是否卡在实体方块里。
// 只查头脚两格，不做真实射线检测。玩家已经被卡住时不判，免得卡墙里出不来。
func (srv *Server) clippedThroughTerrain(blockWorld *world.World, last moveRecord, posX float32, posY float32, posZ float32) bool {
	if blockWorld == nil {
		return false
	}
	var feetY float64 = float64(posY - playerEyeHeight)
	var headY float64 = float64(posY)
	var newX int32 = int32(math.Floor(float64(posX)))
	var newZ int32 = int32(math.Floor(float64(posZ)))
	var newFeet byte = blockIDAt(blockWorld, newX, int32(math.Floor(feetY)), newZ)
	var newHead byte = blockIDAt(blockWorld, newX, int32(math.Floor(headY)), newZ)
	if !isSuffocating(newFeet) && !isSuffocating(newHead) {
		return false
	}
	// 原来就在实体方块里，说明是被卡住的，不算穿墙犯规。
	var oldX int32 = int32(math.Floor(float64(last.PosX)))
	var oldZ int32 = int32(math.Floor(float64(last.PosZ)))
	var oldFeet byte = blockIDAt(blockWorld, oldX, int32(math.Floor(feetY)), oldZ)
	var oldHead byte = blockIDAt(blockWorld, oldX, int32(math.Floor(headY)), oldZ)
	return !isSuffocating(oldFeet) && !isSuffocating(oldHead)
}

// pullBack 把玩家拉回上一次合法位置，并告诉他客户端别自己接着插值。
func (srv *Server) pullBack(player *Player, reason string) {
	var last moveRecord = player.movementSnapshot()
	if !last.Known {
		return
	}
	var yaw, pitch = player.Look()
	player.UpdatePosition(last.PosX, last.PosY, last.PosZ, yaw, pitch, player.onGroundSnapshot())
	if player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.MovePlayerPacket{
		EntityRuntimeID: player.EntityRuntimeID,
		X:               last.PosX,
		Y:               last.PosY,
		Z:               last.PosZ,
		Yaw:             yaw,
		Pitch:           pitch,
		BodyYaw:         yaw,
		Mode:            protocol.MovePlayerModeReset,
		OnGround:        player.onGroundSnapshot(),
	})
	logger.Debug("server", "%s 移动回拉: %s", player.Username, reason)
}

// handleEntityFall 接收客户端上报的落地，换算成摔落伤害。
// 客户端只在落地时发一次，所以这里不能再叠无敌时间（bypassCooldown）。
func (srv *Server) handleEntityFall(sess *network.Session, pkt *packets.EntityFallPacket) {
	var player *Player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	if player.gameMode() != int32(protocol.GameModeSurvival) {
		return
	}
	var damage float32 = fallDamageFor(pkt.FallDistance)
	if damage <= 0 {
		return
	}
	logger.Debug("server", "%s 摔落 %.2f 格", player.Username, pkt.FallDistance)
	srv.applyDamage(player, damage, DamageCauseFall, nil, true)
}
