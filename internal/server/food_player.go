package server

import "math"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// vitals 打包客户端 HUD 关心的体征。属性包是整包覆盖的，血量必须一起带上。
func (player *Player) vitals() packets.PlayerVitals {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return packets.PlayerVitals{
		Absorption: player.combat.Absorption,
		Health:     player.combat.Health,
		Hunger:     player.food.Food,
		Saturation: player.food.Saturation,
		Exhaustion: player.food.Exhaustion,
	}
}

// survivalOnly 报告饥饿规则是否对该玩家生效：只有存活的生存模式玩家会饿。
// 调用方必须已持有 player.mu。
func (player *Player) survivalOnly() bool {
	return !player.combat.Dead && player.GameMode == int32(protocol.GameModeSurvival)
}

// exhaust 累加疲劳度，返回是否需要把饥饿值同步给客户端。
func (player *Player) exhaust(amount float32) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	if amount <= 0 || !player.survivalOnly() {
		return false
	}
	return player.food.exhaust(amount)
}

// advanceFood 推进一个 tick 的饥饿结算。
func (player *Player) advanceFood(difficulty int32) foodTickEffect {
	player.mu.Lock()
	defer player.mu.Unlock()
	if !player.survivalOnly() {
		return foodTickEffect{}
	}
	return player.food.tick(player.combat.Health, difficulty)
}

// setSprinting 记录疾跑状态，只用来决定走路疲劳按哪个速率算。
// 饥饿值太低时客户端自己不让疾跑，这里再兜一层，防止被伪造的疾跑包少算疲劳。
func (player *Player) setSprinting(sprinting bool) {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.food.Sprinting = sprinting && player.food.Food > sprintBlockedFood
}

func (player *Player) isSprinting() bool {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.food.Sprinting
}

// sendVitals 整包下发血量 + 饥饿，用在进服和重生。
func (srv *Server) sendVitals(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.UpdateAttributesPacket{
		EntityRuntimeID: player.EntityRuntimeID,
		Attributes:      packets.PlayerAttributes(player.vitals()),
	})
}

// sendHunger 只下发饥饿相关三项。整包会把移动速度等别的属性也覆盖回默认值，增量同步不能用。
func (srv *Server) sendHunger(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.UpdateAttributesPacket{
		EntityRuntimeID: player.EntityRuntimeID,
		Attributes:      packets.HungerAttributes(player.vitals()),
	})
}

// exhaustPlayer 给玩家加疲劳，饥饿值或饱和度变了才同步。
// 疲劳度本身客户端看不见，只在跟着变的时候顺带带过去。
func (srv *Server) exhaustPlayer(player *Player, amount float32) {
	if player.exhaust(amount) {
		srv.sendHunger(player)
	}
}

// difficulty 取服务器难度；配置没挂上时（测试）按简单处理。
func (srv *Server) difficulty() int32 {
	if srv.cfg == nil {
		return protocol.DifficultyEasy
	}
	return int32(srv.cfg.Difficulty)
}

// tickFood 每 tick 给每个在线生存玩家结算一次：回血、饿血、同步。
func (srv *Server) tickFood(player *Player) {
	var effect foodTickEffect = player.advanceFood(srv.difficulty())
	if effect.Heal > 0 && player.heal(effect.Heal) {
		srv.sendHealth(player, player.Health())
	}
	if effect.Starve {
		srv.applyDamage(player, starvationDamage, DamageCauseStarvation, nil, true)
	}
	if effect.Changed {
		srv.sendHunger(player)
	}
}

// moveExhaustion 换算一次位移的疲劳度。单次位移夹到 maxExhaustMoveStep，
// 传送和卡顿后的大跳变不是真的走了那么远。
func moveExhaustion(prevX float32, prevZ float32, nextX float32, nextZ float32, sprinting bool) float32 {
	var diffX float64 = float64(nextX - prevX)
	var diffZ float64 = float64(nextZ - prevZ)
	var distance float32 = min(float32(math.Sqrt(diffX*diffX+diffZ*diffZ)), maxExhaustMoveStep)
	if sprinting {
		return distance * exhaustSprintPerBlock
	}
	return distance * exhaustWalkPerBlock
}

func (srv *Server) exhaustFromMove(player *Player, prevX float32, prevZ float32, nextX float32, nextZ float32) {
	srv.exhaustPlayer(player, moveExhaustion(prevX, prevZ, nextX, nextZ, player.isSprinting()))
}

// onJump 跳跃的疲劳：疾跑跳比普通跳高 4 倍。
func (srv *Server) onJump(player *Player) {
	if player.isSprinting() {
		srv.exhaustPlayer(player, exhaustSprintJump)
		return
	}
	srv.exhaustPlayer(player, exhaustJump)
}
