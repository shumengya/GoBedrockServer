package server

import "math"
import "time"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// blockPos 方块坐标，挖掘校验和距离判断共用。
type blockPos struct {
	PosX int32
	PosY int32
	PosZ int32
}

func (srv *Server) playerBySession(sess *network.Session) *Player {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	return srv.players[sess.ClientUUID]
}

func (srv *Server) handlePlayerAction(sess *network.Session, pkt *packets.PlayerActionPacket) {
	logger.Debug("server", "%s 动作 %s (%d,%d,%d)", sess.Username, packets.ActionName(pkt.Action), pkt.X, pkt.Y, pkt.Z)
	var player *Player = srv.playerBySession(sess)
	if player == nil {
		return
	}
	switch pkt.Action {
	case packets.ActionReleaseItem:
		if sess.State() == network.StatePlaying {
			srv.drinkMilk(player)
		}
	case packets.ActionStartBreak:
		srv.startBreak(player, pkt)
	case packets.ActionAbortBreak, packets.ActionStopBreak:
		if pkt.Action == packets.ActionAbortBreak {
			player.stopMining()
		}
		srv.broadcastLevelEvent(
			srv.playerWorldName(player),
			packets.LevelEventBlockStopBreak,
			float32(pkt.X),
			float32(pkt.Y),
			float32(pkt.Z),
			0,
		)
	case packets.ActionContinueBreak:
		var blockWorld *world.World = srv.playerWorld(player)
		var blockID, blockMeta = blockWorld.GetBlock(pkt.X, pkt.Y, pkt.Z)
		if blockID == world.BlockAir {
			return
		}
		var data int32 = int32(blockID) | int32(blockMeta)<<8 | pkt.Face<<16
		srv.broadcastLevelEvent(
			srv.playerWorldName(player),
			packets.LevelEventParticlePunchBlock,
			float32(pkt.X),
			float32(pkt.Y),
			float32(pkt.Z),
			data,
		)
	case packets.ActionSpawnSameDimension, packets.ActionSpawnOverworld:
		srv.respawnPlayer(player)
	case packets.ActionStartSprint:
		player.setSprinting(true)
	case packets.ActionStopSprint:
		player.setSprinting(false)
	case packets.ActionStartSneak:
		player.setSneaking(true)
	case packets.ActionStopSneak:
		player.setSneaking(false)
	case packets.ActionJump:
		srv.onJump(player)
	}
}

// 拒绝提示的节流间隔。玩家按住不放会持续触发拒绝，不节流聊天栏会被刷爆。
const noticeInterval time.Duration = 2 * time.Second

// notifyBlocked 告诉玩家这次操作被服务端拒了，并说明原因。
// 不给提示的话玩家只会看到「按了没反应」，根本不知道是距离、保护还是别的问题。
func (srv *Server) notifyBlocked(player *Player, reason string) {
	if player == nil || player.Session == nil || reason == "" {
		return
	}
	if !player.claimNotice(reason, time.Now(), noticeInterval) {
		return
	}
	player.Session.SendPacket(&packets.TextPacket{
		Type:    protocol.TextTypeRaw,
		Message: reason,
	})
}

// breakRejectMessage 把内部拒绝原因翻成给玩家看的话。
// 「未开始挖掘」「挖掘时间不足」是防作弊判定，正常客户端几乎不会触发，
// 真触发了也不该提示——告诉作弊者「你挖太快了」等于教他怎么调。
func breakRejectMessage(reason string) string {
	switch reason {
	case "旁观模式":
		return "§c旁观模式不能破坏方块"
	case "不可破坏":
		return "§c这个方块破坏不了"
	case "距离过远":
		return "§c目标方块太远，够不着"
	case "出生点保护":
		return "§c这里在出生点保护范围内，不能破坏方块"
	default:
		return ""
	}
}

// startBreak 记录挖掘起点，并按服务端手持物广播裂纹速度。
func (srv *Server) startBreak(player *Player, pkt *packets.PlayerActionPacket) {
	if player.IsDead() {
		return
	}
	// 距离和出生点保护在开始挖的时候就拦掉，免得客户端对着保护区里的方块
	// 一直放裂纹动画，最后发 RemoveBlock 又被打回。
	var posX, posY, posZ = player.Position()
	var target blockPos = blockPos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	if !blockInReach(posX, posY, posZ, target, srv.breakDistance(player)) {
		logger.Debug("server", "%s 开始挖掘被拒: (%d,%d,%d) 距离过远", player.Username, pkt.X, pkt.Y, pkt.Z)
		srv.notifyBlocked(player, "§c目标方块太远，够不着")
		return
	}
	if srv.spawnProtected(player, pkt.X, pkt.Z) {
		logger.Debug("server", "%s 开始挖掘被拒: (%d,%d,%d) 在出生点保护范围内", player.Username, pkt.X, pkt.Y, pkt.Z)
		srv.notifyBlocked(player, "§c这里在出生点保护范围内，不能破坏方块")
		return
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var blockID byte = 0
	blockID, _ = blockWorld.GetBlock(pkt.X, pkt.Y, pkt.Z)
	var ticks int32 = breakTimeTicks(blockID, player.heldSnapshot().ID)
	player.startMining(miningState{
		Active:  true,
		BlockX:  pkt.X,
		BlockY:  pkt.Y,
		BlockZ:  pkt.Z,
		BlockID: blockID,
		Started: time.Now(),
		Ticks:   ticks,
	})
	if ticks <= 0 || player.gameMode() == int32(protocol.GameModeCreative) {
		return
	}
	srv.broadcastLevelEvent(
		srv.playerWorldName(player),
		packets.LevelEventBlockStartBreak,
		float32(pkt.X),
		float32(pkt.Y),
		float32(pkt.Z),
		65535/ticks,
	)
}

// breakRejectReason 返回拒绝破坏的原因，空串表示放行。
// 生存模式要求挖够时间（留容差），防止客户端直接发 RemoveBlock 秒破。
func breakRejectReason(
	gameMode int32,
	info blockInfo,
	state miningState,
	target blockPos,
	blockID byte,
	now time.Time,
) string {
	if gameMode == int32(protocol.GameModeSpectator) {
		return "旁观模式"
	}
	if gameMode == int32(protocol.GameModeCreative) {
		return ""
	}
	if blockUnbreakable(info) {
		return "不可破坏"
	}
	if info.Hardness == 0 {
		return ""
	}
	var sameTarget bool = state.BlockX == target.PosX && state.BlockY == target.PosY && state.BlockZ == target.PosZ
	if !state.Active || !sameTarget || state.BlockID != blockID {
		return "未开始挖掘"
	}
	var need time.Duration = time.Duration(float64(state.Ticks)*breakTimeTolerance) * 50 * time.Millisecond
	if now.Sub(state.Started) < need {
		return "挖掘时间不足"
	}
	return ""
}

// blockInReach 从眼睛位置算到方块中心。
// 传入的 Y 已经是眼睛高度（Player.Y 存的就是眼睛位置），不要再加眼高。
func blockInReach(eyeX float32, eyeY float32, eyeZ float32, target blockPos, limit float32) bool {
	var diffX float64 = float64(eyeX) - (float64(target.PosX) + 0.5)
	var diffY float64 = float64(eyeY) - (float64(target.PosY) + 0.5)
	var diffZ float64 = float64(eyeZ) - (float64(target.PosZ) + 0.5)
	return math.Sqrt(diffX*diffX+diffY*diffY+diffZ*diffZ) <= float64(limit)
}

func (srv *Server) handleRemoveBlock(sess *network.Session, pkt *packets.RemoveBlockPacket) {
	var player *Player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	// 破坏和放置共用一把锁，免得一边挖一边有人往同一格放。
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld *world.World = srv.playerWorld(player)
	var worldName string = srv.playerWorldName(player)
	var blockID, blockMeta = blockWorld.GetBlock(pkt.X, pkt.Y, pkt.Z)
	if blockID == world.BlockAir {
		return
	}
	var target blockPos = blockPos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	var gameMode int32 = player.gameMode()
	var limit float32 = maxBreakDistanceSurvival
	if gameMode == int32(protocol.GameModeCreative) {
		limit = maxBreakDistanceCreative
	}
	var posX, posY, posZ = player.Position()
	var info blockInfo = miningInfoFor(blockID)
	var reason string = breakRejectReason(gameMode, info, player.miningSnapshot(), target, blockID, time.Now())
	if reason == "" && !blockInReach(posX, posY, posZ, target, limit) {
		reason = "距离过远"
	}
	if reason == "" && srv.spawnProtected(player, pkt.X, pkt.Z) {
		reason = "出生点保护"
	}
	if reason != "" {
		logger.Debug("server", "%s 破坏 id=%d (%d,%d,%d) 被拒: %s", sess.Username, blockID, pkt.X, pkt.Y, pkt.Z, reason)
		srv.notifyBlocked(player, breakRejectMessage(reason))
		// 客户端已经本地删掉方块，要把原方块推回去
		sess.SendPacket(&packets.UpdateBlockPacket{
			X:         pkt.X,
			Y:         pkt.Y,
			Z:         pkt.Z,
			BlockID:   uint32(blockID),
			BlockData: uint32(blockMeta),
			Flags:     protocol.UpdateBlockFlagAllPriority,
		})
		return
	}
	player.stopMining()
	// 先取手持物再改方块：耐久和掉落都按挖掉这一刻的工具算
	var held protocol.Item = player.heldSnapshot()
	// 门、床、双层植物要把另一半一起拆掉，得在这一格变成空气之前取，之后就查不到了。
	var partner, hasPartner = blockrule.PairedHalf(
		loadingView{target: blockWorld},
		blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z},
	)
	if !blockWorld.SetBlock(pkt.X, pkt.Y, pkt.Z, world.BlockAir, 0) {
		return
	}
	logger.Info("server", "%s 破坏方块 id=%d 于 (%d,%d,%d)", sess.Username, blockID, pkt.X, pkt.Y, pkt.Z)
	var tile, hasTile = blockWorld.RemoveTile(pkt.X, pkt.Y, pkt.Z)
	if hasTile {
		srv.releaseTile(worldName, tile)
	}
	srv.broadcastLevelEvent(
		worldName,
		packets.LevelEventParticleDestroy,
		float32(pkt.X),
		float32(pkt.Y),
		float32(pkt.Z),
		int32(blockID)|int32(blockMeta)<<8,
	)
	srv.broadcastBlock(worldName, pkt.X, pkt.Y, pkt.Z, 0, 0)
	var origins []blockrule.Pos = []blockrule.Pos{{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}}
	var partnerDrops []protocol.Item = nil
	if hasPartner {
		partnerDrops = srv.removePartnerLocked(worldName, blockWorld, partner, held)
		origins = append(origins, partner)
	}
	// 挖掉一个方块，贴着它的火把、立在它上面的花都要重新评估；创造模式也一样。
	srv.settleBlocksLocked(worldName, blockWorld, origins)
	if gameMode == int32(protocol.GameModeCreative) {
		return
	}
	srv.exhaustPlayer(player, exhaustMining)
	var drops []protocol.Item = blockDrops(blockID, blockMeta, held, defaultDropRandom{})
	if hasTile && tile.ID == world.TileBed {
		drops = withBedColor(drops, tile.Color)
	}
	if blockID == world.BlockSkull && tile.ID == world.TileSkull {
		drops = []protocol.Item{{ID: 397, Meta: int32(tile.SkullType), Count: 1}}
	}
	drops = append(drops, partnerDrops...)
	var index int = 0
	for index = 0; index < len(drops); index++ {
		srv.spawnDroppedItem(
			drops[index],
			float32(pkt.X)+0.5,
			float32(pkt.Y)+0.5,
			float32(pkt.Z)+0.5,
			0,
			0.1,
			0,
			10,
			worldName,
		)
	}
	srv.useHeldTool(player, miningWear(held, info))
}

// miningWear 对齐 PocketMine Tool::useOn：挖硬度 > 0 的方块时，挖掘工具扣 1，剑扣 2。
func miningWear(held protocol.Item, info blockInfo) int32 {
	if info.Hardness <= 0 {
		return 0
	}
	var tool toolInfo = toolInfoFor(held)
	if tool.Kind == toolSword {
		return 2
	}
	if tool.Kind == toolPickaxe || tool.Kind == toolAxe || tool.Kind == toolShovel {
		return 1
	}
	return 0
}

// attackWear 对齐 PocketMine Tool::useOn：打实体时剑扣 1，镐斧铲扣 2。
func attackWear(held protocol.Item) int32 {
	var tool toolInfo = toolInfoFor(held)
	if tool.Kind == toolSword {
		return 1
	}
	if tool.Kind == toolPickaxe || tool.Kind == toolAxe || tool.Kind == toolShovel {
		return 2
	}
	return 0
}

// useHeldTool 扣手持工具耐久并同步给自己和其他玩家；工具用坏时手里变空。
func (srv *Server) useHeldTool(player *Player, wear int32) {
	var slot, item, changed = player.DamageHeldTool(wear)
	if !changed {
		return
	}
	srv.sendInventorySlot(player, slot, item)
	if item.IsAir() {
		logger.Info("server", "%s 的工具用坏了", player.Username)
	}
	srv.broadcastHeldItem(player)
}

// breakDistance 返回该玩家允许的破坏距离：创造模式放宽。
func (srv *Server) breakDistance(player *Player) float32 {
	if player.gameMode() == int32(protocol.GameModeCreative) {
		return maxBreakDistanceCreative
	}
	return maxBreakDistanceSurvival
}
