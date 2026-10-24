package server

import "time"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// minEatInterval 是两次进食的最短间隔。原版吃一口要 1.6 秒（32 tick），
// 这里放宽到 1 秒，吸收网络抖动；只用来挡「一秒几十次进食事件」的刷包，不是精确计时。
const minEatInterval time.Duration = time.Second

// eatOutcome 是一次进食的结果。
type eatOutcome struct {
	Accepted bool
	Slot     int
	Held     protocol.Item
	// Overflow 是背包放不下的残留物（比如碗），要丢在地上。
	Overflow protocol.Item
}

// eatRejectReason 返回拒绝进食的原因，空串表示可以吃。调用方必须已持有 player.mu。
func (player *Player) eatRejectReason(info foodInfo, now time.Time) string {
	if player.combat.Dead {
		return "已死亡"
	}
	if player.GameMode == int32(protocol.GameModeSpectator) {
		return "旁观模式"
	}
	if !info.AlwaysEdible && player.food.Food >= maxFoodLevel {
		return "不饿"
	}
	if !player.food.LastEat.IsZero() && now.Sub(player.food.LastEat) < minEatInterval {
		return "吃得太快"
	}
	return ""
}

// consumeOne 扣掉一个；只剩一个时变成空气。
func consumeOne(item protocol.Item) protocol.Item {
	if item.Count > 1 {
		item.Count--
		return item
	}
	return protocol.Air()
}

// eatHeld 吃手里的东西：校验、扣一个、回饥饿值与饱和度、处理残留物，整个过程在同一把锁里。
// 手持物以服务端记录为准，不信客户端。创造模式照常回饥饿但不扣物品。
func (player *Player) eatHeld(now time.Time) (eatOutcome, string) {
	player.mu.Lock()
	defer player.mu.Unlock()
	var slot int = int(player.HeldSlot)
	if slot < 0 || slot >= len(player.Inventory) {
		return eatOutcome{}, "手持槽位无效"
	}
	var held protocol.Item = player.Inventory[slot]
	var info, isFood = foodFor(held)
	if !isFood {
		return eatOutcome{}, "手里不是食物"
	}
	var reason string = player.eatRejectReason(info, now)
	if reason != "" {
		return eatOutcome{}, reason
	}
	player.food.LastEat = now
	player.food.eat(info.Restore, info.Saturation)
	var outcome eatOutcome = eatOutcome{Accepted: true, Slot: slot, Held: held}
	if player.GameMode == int32(protocol.GameModeCreative) {
		return outcome, ""
	}
	held = consumeOne(held)
	player.Inventory[slot] = held
	player.HeldItem = held
	outcome.Held = held
	if info.Residue.IsAir() {
		return outcome, ""
	}
	if held.IsAir() {
		player.Inventory[slot] = info.Residue
		player.HeldItem = info.Residue
		outcome.Held = info.Residue
		return outcome, ""
	}
	var leftover, _ = player.addItemLocked(info.Residue)
	outcome.Overflow = leftover
	return outcome, ""
}

// handleEntityEvent 处理客户端上报的实体事件：吃完（USE_ITEM）和吃的过程中的食物碎屑（EATING_ITEM）。
// 事件里带的实体 id 一律不信，以会话对应的玩家为准。
func (srv *Server) handleEntityEvent(sess *network.Session, pkt *packets.EntityEventPacket) {
	var player *Player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	if pkt.Event == packets.EntityEventUseItem {
		srv.finishEating(player)
		return
	}
	if pkt.Event == packets.EntityEventEatingItem {
		srv.echoEatingParticles(player, pkt.Data)
	}
}

// finishEating 客户端说吃完了：服务端自己再校验一遍，通过才真的扣物品回饥饿。
func (srv *Server) finishEating(player *Player) {
	var outcome, reason = player.eatHeld(time.Now())
	if !outcome.Accepted {
		// 客户端已经按「吃掉了」在本地画了，把真实背包推回去。
		srv.sendPlayerInventory(player)
		srv.logEatReject(player, reason)
		return
	}
	srv.sendPlayerInventory(player)
	srv.sendHunger(player)
	srv.broadcastInWorld(
		srv.playerWorldName(player),
		&packets.EntityEventPacket{
			EntityRuntimeID: player.EntityRuntimeID,
			Event:           packets.EntityEventUseItem,
		},
		"",
	)
	srv.broadcastHeldItem(player)
	if !outcome.Overflow.IsAir() {
		var posX, posY, posZ = player.Position()
		srv.spawnDroppedItem(outcome.Overflow, posX, posY, posZ, 0, 0.1, 0, 10, srv.playerWorldName(player))
	}
}

// logEatReject 拒绝原因只进调试日志：正常客户端偶尔会因为延迟触发，告诉玩家没意义；
// 作弊客户端则不该被告知阈值。
func (srv *Server) logEatReject(player *Player, reason string) {
	logger.Debug("server", "%s 进食被拒: %s", player.Username, reason)
}

// echoEatingParticles 把进食碎屑事件转给同世界的人。只在手里真是食物时转发，
// 否则任何人都能对着别人刷粒子事件。
func (srv *Server) echoEatingParticles(player *Player, data int32) {
	if data == 0 || player.gameMode() == int32(protocol.GameModeSpectator) {
		return
	}
	var _, isFood = foodFor(player.heldSnapshot())
	if !isFood {
		return
	}
	srv.broadcastInWorld(
		srv.playerWorldName(player),
		&packets.EntityEventPacket{
			EntityRuntimeID: player.EntityRuntimeID,
			Event:           packets.EntityEventEatingItem,
			Data:            data,
		},
		"",
	)
}
