package server

import "time"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"

const maxPlayerHealth float32 = 20

// 受伤后的无敌时间，对齐原版 10 tick（0.5 秒），防止客户端连点刷伤害。
const damageCooldown time.Duration = 500 * time.Millisecond

// 攻击距离上限，对齐 PocketMine canInteract($target, 8)。
const maxAttackDistance float32 = 8

// 破坏距离上限：生存 7 格、创造 13 格，对齐 PocketMine REMOVE_BLOCK 的 canInteract。
const maxBreakDistanceSurvival float32 = 7
const maxBreakDistanceCreative float32 = 13

// 放置距离上限：生存 7 格、创造 13 格，与破坏一致。
const maxPlaceDistanceSurvival float32 = 7
const maxPlaceDistanceCreative float32 = 13

// 交互距离上限（开箱子、开工作台）。
const maxInteractDistance float32 = 8

// 挖掘时间容差：客户端动画和网络延迟会让实际耗时略短，只要不明显秒破就放行。
const breakTimeTolerance float64 = 0.7

// 击退力度：水平 0.4 格/tick，Y 抬 0.4 让目标微微弹起，对齐原版玩家互殴的手感。
const knockbackHorizontal float64 = 0.4
const knockbackVertical float32 = 0.4

// 岩浆点燃时长（tick）与火焰自然熄灭前的每 tick 衰减。
const fireTicksOnLava int32 = 300
const fireTicksStep int32 = 1

type combatState struct {
	Effects     map[int32]storage.EffectData
	Absorption  float32
	Health      float32
	LastDamaged time.Time
	Dead        bool

	// LastCause 记录最后一次伤害来源，只用于日志排查。
	LastCause DamageCause

	// TickTimers 是各种环境伤害的节拍器，下标为 lifeTick。
	// 值表示距离下次触发还剩多少 tick，到 0 就触发并重置为该伤害的间隔。
	TickTimers [lifeTickCount]int32

	// FireTicks > 0 表示身上着火。
	FireTicks int32
	// AirTicks 是剩余憋气 tick，浮出水面回满。
	AirTicks int32
}

// miningState 记录玩家正在挖的方块，RemoveBlock 时用来校验挖掘时间。
type miningState struct {
	Active  bool
	BlockX  int32
	BlockY  int32
	BlockZ  int32
	BlockID byte
	Started time.Time
	Ticks   int32
}

// 生命上限由当前效果决定，不另存一份，避免清除效果或读档时留下旧上限。
func (state *combatState) healthLimit() float32 {
	var boost = state.Effects[effectHealthBoost]
	if boost.Duration > 0 {
		return maxPlayerHealth + 4*float32(boost.Amplifier+1)
	}
	return maxPlayerHealth
}

func (player *Player) Health() float32 {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.combat.Health
}

func (player *Player) IsDead() bool {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.combat.Dead
}

// takeDamage 扣血。冷却中、已死亡、创造/旁观模式都不受伤。
// bypassCooldown 给环境伤害用：它们自己有触发间隔，再叠无敌时间会永远打不死人。
func (player *Player) takeDamage(amount float32, cause DamageCause, now time.Time, bypassCooldown bool) (applied bool, health float32, died bool) {
	player.mu.Lock()
	defer player.mu.Unlock()
	if amount <= 0 || player.combat.Dead {
		return false, player.combat.Health, false
	}
	if player.GameMode == int32(protocol.GameModeCreative) || player.GameMode == int32(protocol.GameModeSpectator) {
		return false, player.combat.Health, false
	}
	if !bypassCooldown && !player.combat.LastDamaged.IsZero() && now.Sub(player.combat.LastDamaged) < damageCooldown {
		return false, player.combat.Health, false
	}
	if player.combat.Effects[effectFireResistance].Duration > 0 && (cause == DamageCauseFire || cause == DamageCauseLava) {
		return false, player.combat.Health, false
	}
	if effect := player.combat.Effects[effectResistance]; effect.Duration > 0 && cause != DamageCauseVoid {
		amount *= max(0, 1-0.2*float32(effect.Amplifier+1))
	}
	if cause == DamageCausePoison {
		amount = min(amount, max(0, player.combat.Health-1))
	}
	if amount <= 0 {
		return false, player.combat.Health, false
	}
	var absorbed float32 = min(player.combat.Absorption, amount)
	player.combat.Absorption -= absorbed
	amount -= absorbed
	player.combat.LastDamaged = now
	player.combat.LastCause = cause
	player.combat.Health = max(0, player.combat.Health-amount)
	if player.combat.Health > 0 {
		return true, player.combat.Health, false
	}
	player.combat.Dead = true
	return true, 0, true
}

// heal 回血，不超过上限。返回是否真的回了血。
func (player *Player) heal(amount float32) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	var limit = player.combat.healthLimit()
	if player.combat.Dead || amount <= 0 || player.combat.Health >= limit {
		return false
	}
	player.combat.Health = min(limit, player.combat.Health+amount)
	return true
}

// dueForTick 是环境伤害的节拍器：每隔 period tick 返回一次 true。
// 每种伤害一个独立格位，互不干扰。
func (player *Player) dueForTick(kind lifeTick, period int32) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.combat.TickTimers[kind]--
	if player.combat.TickTimers[kind] > 0 {
		return false
	}
	player.combat.TickTimers[kind] = period
	return true
}

func (player *Player) setFireTicks(ticks int32) {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.combat.FireTicks = max(player.combat.FireTicks, ticks)
}

// tickFire 火焰每 tick 衰减一次，返回衰减后剩余 tick。
func (player *Player) tickFire() int32 {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.combat.FireTicks > 0 {
		player.combat.FireTicks = max(0, player.combat.FireTicks-fireTicksStep)
	}
	return player.combat.FireTicks
}

// drainAir 每 tick 扣一点氧气，返回剩余量（可能为负，表示已经憋不住了）。
func (player *Player) drainAir(amount int32) int32 {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.combat.AirTicks -= amount
	return player.combat.AirTicks
}

func (player *Player) refillAir() {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.combat.AirTicks = airMax
}

// Revive 重生时回满血、清冷却、灭火、回满氧气，饥饿也重置。
func (player *Player) Revive() {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.combat = combatState{Health: maxPlayerHealth, AirTicks: airMax}
	player.food = newFoodState()
}

// DamageHeldTool 给手持工具加 amount 点损耗，用完变空气。
// 创造模式、非工具都不扣；返回被修改的槽位和新物品。
func (player *Player) DamageHeldTool(amount int32) (slot int, item protocol.Item, changed bool) {
	player.mu.Lock()
	defer player.mu.Unlock()
	slot = player.heldInventorySlotLocked()
	if amount <= 0 || player.GameMode == int32(protocol.GameModeCreative) {
		return slot, player.HeldItem, false
	}
	if slot < 0 || slot >= len(player.Inventory) {
		return slot, player.HeldItem, false
	}
	var current protocol.Item = player.Inventory[slot]
	var tool toolInfo = toolInfoFor(current)
	if current.IsAir() || tool.MaxDurability <= 0 {
		return slot, current, false
	}
	current.Meta = max(current.Meta, 0) + amount
	if current.Meta >= tool.MaxDurability {
		current = protocol.Air()
	}
	player.Inventory[slot] = current
	player.HeldItem = current
	return slot, current, true
}

// heldSnapshot 读服务端记录的手持物，不用客户端包里带的物品。
func (player *Player) heldSnapshot() protocol.Item {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.HeldItem
}

// setSneaking 记录客户端上报的潜行状态。潜行时右键不触发门、拉杆这类方块的交互。
func (player *Player) setSneaking(sneaking bool) {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.sneaking = sneaking
}

func (player *Player) isSneaking() bool {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.sneaking
}

func (player *Player) gameMode() int32 {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.GameMode
}

func (player *Player) startMining(state miningState) {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.mining = state
}

func (player *Player) stopMining() {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.mining = miningState{}
}

func (player *Player) miningSnapshot() miningState {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.mining
}
