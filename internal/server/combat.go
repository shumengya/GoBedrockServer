package server

import "math"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// playerList 返回当前在线玩家的快照，供 tick 期间遍历。
// 直接遍历 srv.players 会和广播路径抢锁，先拷贝出来再处理。
func (srv *Server) playerList() []*Player {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var list []*Player = make([]*Player, 0, len(srv.players))
	var uuid string = ""
	for uuid = range srv.players {
		list = append(list, srv.players[uuid])
	}
	return list
}

// handleInteract 只处理左键攻击玩家；右键、悬停、下载具都忽略。
// 生物实体还没有血量系统，打了也不扣耐久，免得空挥白掉耐久。
func (srv *Server) handleInteract(sess *network.Session, pkt *packets.InteractPacket) {
	if pkt.Action != packets.InteractLeftClick {
		return
	}
	var attacker *Player = srv.playerBySession(sess)
	if attacker == nil || sess.State() != network.StatePlaying || attacker.IsDead() {
		return
	}
	var target *Player = srv.playerByRuntimeID(pkt.Target)
	if target == nil || target == attacker {
		return
	}
	var reason string = srv.attackRejectReason(attacker, target)
	if reason != "" {
		logger.Debug("server", "%s 攻击 %s 被拒: %s", attacker.Username, target.Username, reason)
		// 其余拒绝原因（距离、旁观）是正常客户端不会触发的防作弊判定，不提示；
		// PVP 关闭是玩家会撞上的规则，不说一声他只会觉得是 bug。
		if reason == pvpDisabledReason {
			srv.notifyBlocked(attacker, "§c这个服务器没有开启 PVP")
		}
		return
	}
	var held protocol.Item = attacker.heldSnapshot()
	var damage float32 = toolInfoFor(held).AttackDamage
	var applied bool = srv.applyDamage(target, damage, DamageCausePlayer, attacker, false)
	if !applied {
		return
	}
	logger.Info("server", "%s 攻击 %s 伤害=%.1f 剩余=%.1f", attacker.Username, target.Username, damage, target.Health())
	srv.exhaustPlayer(attacker, exhaustAttack)
	// 击退：把目标朝攻击者反方向推出去，同时稍微抬一点，和原版手感一致。
	srv.knockback(attacker, target)
	srv.useHeldTool(attacker, attackWear(held))
}

// knockback 把目标沿「攻击者 → 目标」方向推出去。
// 水平速度固定 0.4（原版玩家互殴的击退量），Y 给 0.4 让目标微微弹起。
func (srv *Server) knockback(attacker *Player, target *Player) {
	// 击退只看水平方向，垂直分量不影响推的方向。
	var attackerX, attackerZ = attacker.horizontalPosition()
	var targetX, targetZ = target.horizontalPosition()
	var diffX float64 = float64(targetX) - float64(attackerX)
	var diffZ float64 = float64(targetZ) - float64(attackerZ)
	var length float64 = math.Sqrt(diffX*diffX + diffZ*diffZ)
	var speedX float32 = 0
	var speedZ float32 = 0
	if length > 0 {
		var ratio float64 = knockbackHorizontal / length
		speedX = float32(diffX * ratio)
		speedZ = float32(diffZ * ratio)
	}
	if target.Session == nil {
		return
	}
	target.Session.SendPacket(&packets.SetEntityMotionPacket{
		EntityRuntimeID: target.EntityRuntimeID,
		SpeedX:          speedX,
		SpeedY:          knockbackVertical,
		SpeedZ:          speedZ,
	})
}

func (srv *Server) playerByRuntimeID(runtimeID int64) *Player {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var uuid string = ""
	for uuid = range srv.players {
		var player *Player = srv.players[uuid]
		if player.EntityRuntimeID == runtimeID {
			return player
		}
	}
	return nil
}

// pvpDisabledReason 是 PVP 关闭时的拒绝原因；handleInteract 靠它区分「该提示玩家」的情况。
const pvpDisabledReason string = "PVP 已关闭"

func (srv *Server) attackRejectReason(attacker *Player, target *Player) string {
	if srv.cfg != nil && !srv.cfg.PVP {
		return pvpDisabledReason
	}
	var attackerMode int32 = attacker.gameMode()
	if attackerMode == int32(protocol.GameModeSpectator) {
		return "旁观模式不能攻击"
	}
	if target.Session == nil || target.Session.State() != network.StatePlaying || target.IsDead() {
		return "目标不可攻击"
	}
	if !sameWorldName(srv.playerWorldName(attacker), srv.playerWorldName(target), srv.world.Name) {
		return "不在同一世界"
	}
	var attackerX, attackerY, attackerZ = attacker.Position()
	var targetX, targetY, targetZ = target.Position()
	var diffX float64 = float64(attackerX - targetX)
	var diffY float64 = float64(attackerY - targetY)
	var diffZ float64 = float64(attackerZ - targetZ)
	if math.Sqrt(diffX*diffX+diffY*diffY+diffZ*diffZ) > float64(maxAttackDistance) {
		return "距离过远"
	}
	return ""
}

func (srv *Server) sendHealth(player *Player, health float32) {
	if player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.SetHealthPacket{Health: int32(math.Ceil(float64(health)))})
}

// killPlayer 死亡流程：广播死亡动画与死亡消息、掉落背包、发 Respawn 让客户端弹重生界面。
// 真正的传送和回血在 respawnPlayer 里等客户端点了重生才做。
func (srv *Server) killPlayer(victim *Player, killer *Player, cause DamageCause) {
	var worldName string = srv.playerWorldName(victim)
	srv.broadcastInWorld(
		worldName,
		&packets.EntityEventPacket{
			EntityRuntimeID: victim.EntityRuntimeID,
			Event:           packets.EntityEventDeathAnimation,
		},
		"",
	)
	var parameters []string = []string{victim.Username}
	if cause == DamageCausePlayer && killer != nil {
		parameters = append(parameters, killer.Username)
	}
	srv.broadcast(
		&packets.TextPacket{
			Type:       protocol.TextTypeTranslation,
			Message:    deathTranslationKey(cause),
			Parameters: parameters,
		},
		"",
	)
	logger.Info("server", "%s 死亡 原因=%s", victim.Username, damageLabel(cause))
	// 生存模式掉落背包，创造模式不掉（跟原版一致）。
	if victim.gameMode() != int32(protocol.GameModeCreative) {
		srv.dropPlayerInventory(victim)
	}
	var spawnWorld *world.World = srv.playerWorld(victim)
	if victim.Session == nil || spawnWorld == nil {
		return
	}
	victim.Session.SendPacket(&packets.RespawnPacket{
		PositionX: float32(spawnWorld.SpawnX) + 0.5,
		PositionY: float32(spawnWorld.SpawnY) + 1,
		PositionZ: float32(spawnWorld.SpawnZ) + 0.5,
	})
}

// dropPlayerInventory 把死亡玩家的背包全部撒在脚下，并清空背包。
func (srv *Server) dropPlayerInventory(victim *Player) {
	var posX, posY, posZ = victim.Position()
	var worldName string = srv.playerWorldName(victim)
	var items []protocol.Item = victim.takeAllItems()
	var index int = 0
	for index = 0; index < len(items); index++ {
		var item protocol.Item = items[index]
		if item.IsAir() || item.Count == 0 {
			continue
		}
		srv.spawnDroppedItem(item, posX, posY, posZ, 0, 0.1, 0, 10, worldName)
	}
	victim.SyncInventory()
	srv.syncArmor(victim)
}

// respawnPlayer 处理客户端点了“重生”：回出生点、回满血、通知其他玩家。
func (srv *Server) respawnPlayer(player *Player) {
	if !player.IsDead() {
		return
	}
	var spawnWorld *world.World = srv.playerWorld(player)
	if spawnWorld == nil {
		return
	}
	var spawnX float32 = float32(spawnWorld.SpawnX) + 0.5
	var spawnY float32 = float32(spawnWorld.SpawnY) + 1
	var spawnZ float32 = float32(spawnWorld.SpawnZ) + 0.5
	var yaw, pitch = player.Look()
	player.UpdatePosition(spawnX, spawnY, spawnZ, yaw, pitch, true)
	player.Revive()
	player.ResetMovement()
	if player.Session != nil {
		player.Session.SendPacket(&packets.MovePlayerPacket{
			EntityRuntimeID: player.EntityRuntimeID,
			X:               spawnX,
			Y:               spawnY,
			Z:               spawnZ,
			Yaw:             yaw,
			Pitch:           pitch,
			Mode:            protocol.MovePlayerModeReset,
			OnGround:        true,
		})
		srv.sendVitals(player)
		srv.sendHealth(player, maxPlayerHealth)
		srv.sendPlayerInventory(player)
		srv.sendOwnArmor(player)
	}
	srv.broadcastInWorld(
		srv.playerWorldName(player),
		&packets.EntityEventPacket{
			EntityRuntimeID: player.EntityRuntimeID,
			Event:           packets.EntityEventRespawn,
		},
		player.UUID,
	)
	srv.broadcastPlayerMove(player)
	logger.Info("server", "%s 重生", player.Username)
}
