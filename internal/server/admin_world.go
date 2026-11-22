package server

import "fmt"
import "maps"
import "slices"

import "go-mc-server/internal/command"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// PlayerPosition 返回玩家脚下的坐标。存的 Y 是眼睛高度，/tp 的坐标按脚算，所以要减掉。
// 找不到（或还在加载）统一返回 false，具体原因由随后的传送调用报出。
func (admin serverAdmin) PlayerPosition(name string) (float32, float32, float32, bool) {
	var player, err = admin.lookupPlaying(name)
	if err != nil {
		return 0, 0, 0, false
	}
	var posX, posY, posZ = player.Position()
	return posX, posY - playerEyeHeight, posZ, true
}

// teleportPlayer 把玩家传到 worldName 里的指定位置，posY 是眼睛高度。
// 跨世界先走正常的换世界流程（换维度画面、重发区块），再覆盖成目标坐标。
func (srv *Server) teleportPlayer(player *Player, worldName string, posX float32, posY float32, posZ float32) {
	var crossWorld bool = !sameWorldName(srv.playerWorldName(player), worldName, srv.world.Name)
	if crossWorld {
		srv.transferPlayer(player, worldName)
	}
	var yaw, pitch = player.Look()
	player.UpdatePosition(posX, posY, posZ, yaw, pitch, false)
	// 传送距离可能几百格，不清掉移动基准会被校验当成瞬移拉回来。
	player.ResetMovement()
	// 目的地周围的区块还没发过，按新位置补上。
	srv.enqueueAroundMove(player)
	if player.Session != nil {
		player.Session.SendPacket(&packets.MovePlayerPacket{
			EntityRuntimeID: player.EntityRuntimeID,
			X:               posX,
			Y:               posY,
			Z:               posZ,
			Yaw:             yaw,
			Pitch:           pitch,
			BodyYaw:         yaw,
			Mode:            protocol.MovePlayerModeTeleport,
			OnGround:        false,
		})
	}
	srv.broadcastPlayerMove(player)
}

func (admin serverAdmin) TeleportToPlayer(subject string, target string) (string, string, error) {
	var mover, err = admin.lookupPlaying(subject)
	if err != nil {
		return "", "", err
	}
	var destination *Player = nil
	destination, err = admin.lookupPlaying(target)
	if err != nil {
		return "", "", err
	}
	var posX, posY, posZ = destination.Position()
	admin.srv.teleportPlayer(mover, admin.srv.playerWorldName(destination), posX, posY, posZ)
	return mover.Username, destination.Username, nil
}

// TeleportToPosition 传到当前世界的指定坐标。这里是信任边界，坐标再校验一遍，
// 不假设调用方（命令层）已经查过。
func (admin serverAdmin) TeleportToPosition(subject string, posX float32, posY float32, posZ float32) (string, error) {
	var err error = command.ValidateTeleportTarget(posX, posY, posZ)
	if err != nil {
		return "", fmt.Errorf("传送坐标不合法: %w", err)
	}
	var mover *Player = nil
	mover, err = admin.lookupPlaying(subject)
	if err != nil {
		return "", err
	}
	admin.srv.teleportPlayer(mover, admin.srv.playerWorldName(mover), posX, posY+playerEyeHeight, posZ)
	return mover.Username, nil
}

// Broadcast 对应 /say，颜色对齐参考端（淡紫色）。
func (admin serverAdmin) Broadcast(sender string, message string) {
	admin.srv.broadcast(
		&packets.TextPacket{
			Type:    protocol.TextTypeRaw,
			Message: fmt.Sprintf("§d[%s] %s", sender, message),
		},
		"",
	)
	logger.Info("chat", "[%s] %s", sender, message)
}

// Tell 私聊。允许发给自己；目标必须已进入游戏。
func (admin serverAdmin) Tell(sender string, target string, message string) (string, error) {
	var player, err = admin.lookupPlaying(target)
	if err != nil {
		return "", err
	}
	player.Session.SendPacket(&packets.TextPacket{
		Type:    protocol.TextTypeRaw,
		Message: fmt.Sprintf("§7%s 悄悄地对你说: %s", sender, message),
	})
	logger.Info("chat", "[%s -> %s] %s", sender, player.Username, message)
	return player.Username, nil
}

func (admin serverAdmin) SaveAll() {
	admin.srv.saveAll("command")
}

func (admin serverAdmin) RequestStop() {
	admin.srv.RequestStop()
}

func (admin serverAdmin) WorldNames() []string {
	return slices.Sorted(maps.Keys(admin.srv.worlds))
}

func (admin serverAdmin) WorldTime(worldName string) (int32, bool) {
	var target, found = admin.srv.worlds[worldName]
	if !found {
		return 0, false
	}
	return target.TimeTicks(), true
}

// 改时间后立刻同步给该世界的玩家，不等下一个同步周期。
func (admin serverAdmin) SetWorldTime(worldName string, ticks int32) {
	var target, found = admin.srv.worlds[worldName]
	if !found {
		return
	}
	target.SetTimeTicks(ticks)
	admin.srv.broadcastWorldTime(target)
}

func (admin serverAdmin) AddWorldTime(worldName string, delta int32) {
	var target, found = admin.srv.worlds[worldName]
	if !found {
		return
	}
	target.AddTimeTicks(delta)
	admin.srv.broadcastWorldTime(target)
}

func (admin serverAdmin) SetWorldTimeStopped(worldName string, stopped bool) {
	var target, found = admin.srv.worlds[worldName]
	if !found {
		return
	}
	target.SetTimeStopped(stopped)
	admin.srv.broadcastWorldTime(target)
}
