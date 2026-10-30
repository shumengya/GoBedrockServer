package server

import "math"
import "time"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// DamageCause 是伤害来源。死亡消息和减伤规则都按它区分。
type DamageCause int

const DamageCauseGeneric DamageCause = 0
const DamageCauseFall DamageCause = 1
const DamageCauseVoid DamageCause = 2
const DamageCauseSuffocation DamageCause = 3
const DamageCauseDrowning DamageCause = 4
const DamageCauseLava DamageCause = 5
const DamageCauseFire DamageCause = 6
const DamageCauseCactus DamageCause = 7
const DamageCausePlayer DamageCause = 8
const DamageCauseStarvation DamageCause = 9

// lifeTick 区分每种环境伤害各自的节拍器，互不干扰。
type lifeTick int

const lifeTickVoid lifeTick = 0
const lifeTickLava lifeTick = 1
const lifeTickFire lifeTick = 2
const lifeTickCactus lifeTick = 3
const lifeTickSuffocation lifeTick = 4
const lifeTickDrown lifeTick = 5
const lifeTickCount lifeTick = 6

// 环境伤害的固定数值与触发间隔，对齐原版 / PocketMine。
const lavaDamage float32 = 4
const fireDamage float32 = 1
const suffocationDmg float32 = 1
const drowningDamage float32 = 1
const voidDamage float32 = 4
const cactusDamage float32 = 1

const voidDamageTick int32 = 10
const drowningTick int32 = 10
const fireDamageTick int32 = 10
const lavaDamageTick int32 = 5

// airMax 是满氧气 tick：300 tick = 15 秒，对齐参考端 Player::initEntity。
const airMax int32 = 300

// 摔落：落差不超过 3 格不掉血，超出部分每格 1 点。
const fallSafeBlocks float64 = 3

// 窒息：头和脚都在实体方块里时每 10 tick 掉 1 点。
const suffocationTick int32 = 10

func damageLabel(cause DamageCause) string {
	switch cause {
	case DamageCauseFall:
		return "摔落"
	case DamageCauseVoid:
		return "虚空"
	case DamageCauseSuffocation:
		return "窒息"
	case DamageCauseDrowning:
		return "溺水"
	case DamageCauseLava:
		return "岩浆"
	case DamageCauseFire:
		return "火焰"
	case DamageCauseCactus:
		return "仙人掌"
	case DamageCausePlayer:
		return "玩家攻击"
	case DamageCauseStarvation:
		return "饥饿"
	default:
		return "未知"
	}
}

// deathTranslationKey 返回 1.1 客户端内置的死亡翻译键，键名取自参考端 eng.ini。
func deathTranslationKey(cause DamageCause) string {
	switch cause {
	case DamageCauseFall:
		return "death.attack.fall"
	case DamageCauseVoid:
		return "death.attack.outOfWorld"
	case DamageCauseSuffocation:
		return "death.attack.inWall"
	case DamageCauseDrowning:
		return "death.attack.drown"
	case DamageCauseLava:
		return "death.attack.lava"
	case DamageCauseFire:
		return "death.attack.onFire"
	case DamageCauseCactus:
		return "death.attack.cactus"
	case DamageCausePlayer:
		return "death.attack.player"
	default:
		return "death.attack.generic"
	}
}

// applyDamage 是玩家扣血的唯一入口：扣血 → SetHealth → 受伤动画 → 血尽则死亡。
// cause 用来选死亡消息；killer 为空表示不是玩家打的。
// bypassCooldown 给环境伤害用：它们自己有触发间隔，再叠一层无敌时间就永远打不死人。
func (srv *Server) applyDamage(target *Player, amount float32, cause DamageCause, killer *Player, bypassCooldown bool) bool {
	if target == nil || amount <= 0 {
		return false
	}
	// 护甲先减伤。减到 0 等于这一下被挡住了：不扣血、不进无敌时间、不击退、不磨损盔甲。
	if armorReducesDamage(cause) {
		amount = reduceByArmor(amount, target.armorPoints())
		if amount <= 0 {
			return false
		}
	}
	var applied, health, died = target.takeDamage(amount, cause, time.Now(), bypassCooldown)
	if !applied {
		return false
	}
	srv.sendHealth(target, health)
	// 受伤本身也耗疲劳（对齐参考端 CAUSE_DAMAGE）；死亡那一击玩家已是 Dead，会被忽略。
	srv.exhaustPlayer(target, exhaustDamage)
	srv.broadcastInWorld(
		srv.playerWorldName(target),
		&packets.EntityEventPacket{
			EntityRuntimeID: target.EntityRuntimeID,
			Event:           packets.EntityEventHurtAnimation,
		},
		"",
	)
	if !died {
		if armorWearsOnHit(cause) {
			srv.wearArmorAfterHit(target)
		}
		return true
	}
	srv.killPlayer(target, killer, cause)
	return true
}

// tickLife 每 tick 跑一次环境伤害、溺水与自然回血。
func (srv *Server) tickLife() {
	var list []*Player = srv.playerList()
	var index int = 0
	for index = 0; index < len(list); index++ {
		var player *Player = list[index]
		if player.Session == nil || player.Session.State() != network.StatePlaying {
			continue
		}
		if player.IsDead() {
			continue
		}
		srv.tickDrowning(player)
		srv.applyEnvironment(player)
		srv.tickFood(player)
	}
}

// applyEnvironment 按所在位置的方块与高度判定各种环境伤害。
func (srv *Server) applyEnvironment(player *Player) {
	var gameMode int32 = player.gameMode()
	if gameMode == int32(protocol.GameModeCreative) || gameMode == int32(protocol.GameModeSpectator) {
		return
	}
	var posX, posY, posZ = player.Position()
	var blockWorld *world.World = srv.playerWorld(player)
	// Player.Y 存的是眼睛高度，脚底要减 playerEyeHeight 才是站立方块。
	var feetY float64 = float64(posY - playerEyeHeight)
	var feetBlockY int32 = int32(math.Floor(feetY))
	var headBlockY int32 = int32(math.Floor(float64(posY)))
	var blockX int32 = int32(math.Floor(float64(posX)))
	var blockZ int32 = int32(math.Floor(float64(posZ)))

	// 虚空：掉到 y=-64 以下持续掉血，直到死亡重生。
	if feetY < -64 {
		if player.dueForTick(lifeTickVoid, voidDamageTick) {
			srv.applyDamage(player, voidDamage, DamageCauseVoid, nil, true)
		}
		return
	}

	var feetID byte = blockIDAt(blockWorld, blockX, feetBlockY, blockZ)
	var headID byte = blockIDAt(blockWorld, blockX, headBlockY, blockZ)

	// 岩浆最优先：泡进去大额伤害，还会点燃。
	if isLavaBlock(feetID) || isLavaBlock(headID) {
		player.setFireTicks(fireTicksOnLava)
		if player.dueForTick(lifeTickLava, lavaDamageTick) {
			srv.applyDamage(player, lavaDamage, DamageCauseLava, nil, true)
		}
	}
	// 仙人掌贴着就扎。
	if feetID == world.BlockCactus || headID == world.BlockCactus {
		if player.dueForTick(lifeTickCactus, suffocationTick) {
			srv.applyDamage(player, cactusDamage, DamageCauseCactus, nil, true)
		}
	}
	// 窒息：脚和头两格都在实体方块里才判定，只埋一格不算。
	if isSuffocating(feetID) && isSuffocating(headID) {
		if player.dueForTick(lifeTickSuffocation, suffocationTick) {
			srv.applyDamage(player, suffocationDmg, DamageCauseSuffocation, nil, true)
		}
	}
	// 火焰：岩浆那一帧刚点燃，这里只负责持续掉血。
	if player.tickFire() > 0 && player.dueForTick(lifeTickFire, fireDamageTick) {
		srv.applyDamage(player, fireDamage, DamageCauseFire, nil, true)
	}
}

// tickDrowning 处理憋气：头泡在水里就扣氧气，氧气见底开始掉血；浮出水面立刻回满。
// 每 tick 都要动氧气值，所以不能塞进按间隔跑的环境伤害里。
func (srv *Server) tickDrowning(player *Player) {
	var gameMode int32 = player.gameMode()
	if gameMode == int32(protocol.GameModeCreative) || gameMode == int32(protocol.GameModeSpectator) {
		return
	}
	var posX, posY, posZ = player.Position()
	var blockWorld *world.World = srv.playerWorld(player)
	var headX int32 = int32(math.Floor(float64(posX)))
	var headY int32 = int32(math.Floor(float64(posY)))
	var headZ int32 = int32(math.Floor(float64(posZ)))
	var headID byte = blockIDAt(blockWorld, headX, headY, headZ)
	if !isWaterBlock(headID) {
		player.refillAir()
		return
	}
	if player.drainAir(1) > 0 {
		return
	}
	if !player.dueForTick(lifeTickDrown, drowningTick) {
		return
	}
	srv.applyDamage(player, drowningDamage, DamageCauseDrowning, nil, true)
}

// fallDamageFor 把客户端上报的摔落高度换算成伤害，没到安全高度就是 0。
func fallDamageFor(distance float32) float32 {
	var extra float64 = float64(distance) - fallSafeBlocks
	if extra <= 0 {
		return 0
	}
	return float32(math.Floor(extra))
}

// blockIDAt 只取方块 ID，判定用不上元数据。
func blockIDAt(targetWorld *world.World, posX int32, posY int32, posZ int32) byte {
	return targetWorld.BlockID(posX, posY, posZ)
}

func isLavaBlock(id byte) bool {
	return id == world.BlockStillLava || id == world.BlockFlowingLava
}

func isWaterBlock(id byte) bool {
	return id == world.BlockStillWater || id == world.BlockFlowingWater
}

// isSuffocating 判断这个方块能不能让人窒息：空气和液体都不会。
func isSuffocating(id byte) bool {
	if id == world.BlockAir || isWaterBlock(id) || isLavaBlock(id) {
		return false
	}
	return true
}
