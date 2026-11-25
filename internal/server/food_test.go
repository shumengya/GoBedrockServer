package server

import "testing"
import "time"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"

func nearly(left float32, right float32) bool {
	var diff float32 = left - right
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.0001
}

type tickTotals struct {
	Heal    float32
	Starved int
	Changed int
}

func runFoodTicks(state *foodState, ticks int, health float32, difficulty int32) tickTotals {
	var totals tickTotals = tickTotals{}
	var index int = 0
	for index = 0; index < ticks; index++ {
		var effect foodTickEffect = state.tick(health, maxPlayerHealth, difficulty)
		totals.Heal += effect.Heal
		if effect.Starve {
			totals.Starved++
		}
		if effect.Changed {
			totals.Changed++
		}
	}
	return totals
}

// 疲劳先耗饱和度，饱和度见底才扣饥饿值；不足 4 点不结算。
func TestExhaustDrainsSaturationBeforeFood(test *testing.T) {
	var state foodState = newFoodState()
	var changed bool = state.exhaust(4)
	if !changed || !nearly(state.Saturation, 4) || state.Food != 20 {
		test.Fatalf("饱和度应先掉: changed=%v 饱和度=%v 饥饿=%v", changed, state.Saturation, state.Food)
	}
	state.Saturation = 0
	changed = state.exhaust(4)
	if !changed || state.Food != 19 {
		test.Fatalf("饱和度见底后应扣饥饿值: changed=%v 饥饿=%v", changed, state.Food)
	}
	changed = state.exhaust(3.9)
	if changed || state.Food != 19 {
		test.Fatal("疲劳不足 4 点不该结算")
	}
}

// 饥饿值跨过 17 / 6 / 0 这几档才清计时，同档内变化不动计时。
func TestSetFoodResetsTimerOnlyAtThresholds(test *testing.T) {
	var cases []struct {
		from  float32
		to    float32
		reset bool
	} = []struct {
		from  float32
		to    float32
		reset bool
	}{
		{20, 17, true},
		{17, 16, false},
		{8, 7, false},
		{7, 6, true},
		{1, 0, true},
		{20, 19, false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var state foodState = newFoodState()
		state.Food = cases[index].from
		state.Timer = 50
		state.setFood(cases[index].to)
		var reset bool = state.Timer == 0
		if reset != cases[index].reset {
			test.Errorf("%v → %v: 计时被清=%v，期望 %v", cases[index].from, cases[index].to, reset, cases[index].reset)
		}
	}
}

// 吃饱（>=18）后每 80 tick 回 1 点血，同时耗 3 点疲劳；满血不回。
func TestFoodTickRegenerates(test *testing.T) {
	var state foodState = newFoodState()
	var early tickTotals = runFoodTicks(&state, 79, 10, protocol.DifficultyEasy)
	if early.Heal != 0 {
		test.Fatal("计时没走满 80 tick 不该回血")
	}
	var onTime tickTotals = runFoodTicks(&state, 1, 10, protocol.DifficultyEasy)
	if onTime.Heal != 1 {
		test.Fatalf("第 80 tick 应回 1 点血，实际 %v", onTime.Heal)
	}
	if !nearly(state.Exhaustion, 3) {
		test.Fatalf("回血应耗 3 点疲劳，实际 %v", state.Exhaustion)
	}
	var full foodState = newFoodState()
	if runFoodTicks(&full, 80, maxPlayerHealth, protocol.DifficultyEasy).Heal != 0 {
		test.Fatal("满血不该回血")
	}
	var hungry foodState = newFoodState()
	hungry.Food = 10
	if runFoodTicks(&hungry, 80, 10, protocol.DifficultyEasy).Heal != 0 {
		test.Fatal("饥饿值低于 18 不该自然回血")
	}
}

// 饿血的血量下限随难度变：简单留 5 颗心，普通留半颗，困难会饿死，和平不饿血。
func TestFoodTickStarvationByDifficulty(test *testing.T) {
	var cases []struct {
		difficulty int32
		health     float32
		starves    bool
	} = []struct {
		difficulty int32
		health     float32
		starves    bool
	}{
		{protocol.DifficultyEasy, 11, true},
		{protocol.DifficultyEasy, 10, false},
		{protocol.DifficultyNormal, 2, true},
		{protocol.DifficultyNormal, 1, false},
		{protocol.DifficultyHard, 1, true},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var state foodState = newFoodState()
		state.Food = 0
		var totals tickTotals = runFoodTicks(&state, 80, cases[index].health, cases[index].difficulty)
		if (totals.Starved == 1) != cases[index].starves {
			test.Errorf("难度 %d 血量 %v: 饿血=%d，期望 %v", cases[index].difficulty, cases[index].health, totals.Starved, cases[index].starves)
		}
	}
}

// 和平模式：饥饿值每 10 tick 补 1，血每 20 tick 回 1。
func TestFoodTickPeaceful(test *testing.T) {
	var state foodState = newFoodState()
	state.Food = 10
	var totals tickTotals = runFoodTicks(&state, 20, 10, protocol.DifficultyPeaceful)
	if state.Food != 12 || totals.Heal != 1 || totals.Changed != 2 {
		test.Fatalf("和平 20 tick: 饥饿=%v 回血=%v 同步=%d，期望 12/1/2", state.Food, totals.Heal, totals.Changed)
	}
}

// 饱和度不能超过饥饿值（原版规则），吃满后饥饿值封顶 20。
func TestEatCapsSaturationAtFood(test *testing.T) {
	var state foodState = newFoodState()
	state.Food = 10
	state.Saturation = 0
	state.eat(4, 9.6)
	if state.Food != 14 || !nearly(state.Saturation, 9.6) {
		test.Fatalf("吃完: 饥饿=%v 饱和度=%v", state.Food, state.Saturation)
	}
	state.eat(8, 12.8)
	if state.Food != 20 || !nearly(state.Saturation, 20) {
		test.Fatalf("吃撑: 饥饿=%v 饱和度=%v，都应封顶 20", state.Food, state.Saturation)
	}
}

// 走路 / 疾跑的疲劳；单次大位移（传送、卡顿）只按上限算。
func TestMoveExhaustion(test *testing.T) {
	if !nearly(moveExhaustion(0, 0, 1, 0, false), 0.01) {
		test.Fatal("走 1 格应耗 0.01")
	}
	if !nearly(moveExhaustion(0, 0, 0, 1, true), 0.1) {
		test.Fatal("疾跑 1 格应耗 0.1")
	}
	if !nearly(moveExhaustion(0, 0, 500, 0, true), maxExhaustMoveStep*exhaustSprintPerBlock) {
		test.Fatal("传送级位移应被夹到单次上限")
	}
}

func TestStarvationApplies(test *testing.T) {
	if starvationApplies(20, protocol.DifficultyPeaceful) {
		test.Fatal("和平模式不饿血")
	}
	if !starvationApplies(20, protocol.DifficultyHard) || !starvationApplies(0.5, protocol.DifficultyHard) {
		test.Fatal("困难模式任何血量都饿血")
	}
}

// 读档夹到合法范围，坏档不能让玩家一进服就溢出；没有存档数据则保持满饥饿。
func TestFoodRestoreClamps(test *testing.T) {
	var state foodState = newFoodState()
	state.restore(nil)
	if state.Food != maxFoodLevel {
		test.Fatal("老存档没有饥饿数据，必须保持满饥饿而不是 0")
	}
	state.restore(&storage.FoodData{Level: 99, Saturation: 99, Exhaustion: 99, Timer: 999})
	if state.Food != 20 || state.Saturation > state.Food || state.Exhaustion > exhaustionPerPoint || state.Timer >= foodTickPeriod {
		test.Fatalf("越界数据没被夹住: %+v", state)
	}
	state.restore(&storage.FoodData{Level: -5, Saturation: -1, Exhaustion: -1, Timer: -9})
	if state.Food != 0 || state.Saturation != 0 || state.Exhaustion != 0 || state.Timer != 0 {
		test.Fatalf("负数据没被夹到 0: %+v", state)
	}
}

func TestFoodFor(test *testing.T) {
	var apple, isApple = foodFor(protocol.Item{ID: 260, Count: 1})
	if !isApple || apple.Restore != 4 {
		test.Fatal("苹果应回 4 点")
	}
	var _, isStone = foodFor(protocol.Item{ID: 1, Count: 1})
	if isStone {
		test.Fatal("石头不是食物")
	}
	var _, isAir = foodFor(protocol.Air())
	if isAir {
		test.Fatal("空手不是食物")
	}
	var salmon, _ = foodFor(protocol.Item{ID: itemCookedFish, Meta: fishMetaSalmon, Count: 1})
	if salmon.Restore != 6 || !nearly(salmon.Saturation, 9.6) {
		test.Fatalf("熟鲑鱼应回 6/9.6: %+v", salmon)
	}
	var cod, _ = foodFor(protocol.Item{ID: itemCookedFish, Meta: 0, Count: 1})
	if cod.Restore != 5 {
		test.Fatalf("熟鳕鱼应回 5: %+v", cod)
	}
	var stew, _ = foodFor(protocol.Item{ID: 282, Count: 1})
	if stew.Residue.ID != itemBowl {
		test.Fatal("蘑菇煲吃完应留碗")
	}
	var golden, _ = foodFor(protocol.Item{ID: 322, Count: 1})
	if !golden.AlwaysEdible {
		test.Fatal("金苹果饱了也能吃")
	}
}

// 饥饿死亡没有专用消息（参考端也没有），走通用的「xxx died」。
func TestStarvationUsesGenericDeathMessage(test *testing.T) {
	if deathTranslationKey(DamageCauseStarvation) != "death.attack.generic" {
		test.Fatal("饥饿死亡应走通用消息，客户端没有对应的内置翻译键")
	}
	if damageLabel(DamageCauseStarvation) != "饥饿" {
		test.Fatal("日志里应显示饥饿")
	}
}

func foodPlayer(held protocol.Item, food float32) *Player {
	var player *Player = newTestPlayer(held)
	player.food = newFoodState()
	player.food.Food = food
	return player
}

// 吃一口：扣 1 个、回饥饿值和饱和度；一秒内再吃被挡掉，过了间隔可以继续吃。
func TestEatHeldConsumesOneAndRateLimits(test *testing.T) {
	var start time.Time = time.Unix(9000, 0)
	var player *Player = foodPlayer(protocol.Item{ID: 260, Count: 2}, 10)
	var outcome, reason = player.eatHeld(start)
	if reason != "" || !outcome.Accepted {
		test.Fatalf("第一口应通过: %s", reason)
	}
	if player.Inventory[0].Count != 1 || player.HeldItem.Count != 1 || outcome.Held.Count != 1 {
		test.Fatalf("应扣 1 个: 背包=%d 手持=%d", player.Inventory[0].Count, player.HeldItem.Count)
	}
	if player.food.Food != 14 || !nearly(player.food.Saturation, 7.4) {
		test.Fatalf("应回到 14 / 7.4: %v %v", player.food.Food, player.food.Saturation)
	}
	outcome, reason = player.eatHeld(start.Add(300 * time.Millisecond))
	if outcome.Accepted || reason != "吃得太快" {
		test.Fatalf("间隔内应被挡: accepted=%v reason=%q", outcome.Accepted, reason)
	}
	if player.Inventory[0].Count != 1 || player.food.Food != 14 {
		test.Fatal("被挡的那口不该扣物品也不该回饥饿")
	}
	outcome, reason = player.eatHeld(start.Add(2 * time.Second))
	if !outcome.Accepted || !player.Inventory[0].IsAir() {
		test.Fatalf("过了间隔应能吃，最后一个吃完手里变空: %s", reason)
	}
}

func TestEatHeldRejections(test *testing.T) {
	var start time.Time = time.Unix(9100, 0)
	var full *Player = foodPlayer(protocol.Item{ID: 260, Count: 2}, 20)
	var outcome, reason = full.eatHeld(start)
	if outcome.Accepted || reason != "不饿" || full.Inventory[0].Count != 2 {
		test.Fatalf("吃饱了不能吃普通食物: %q", reason)
	}
	var golden *Player = foodPlayer(protocol.Item{ID: 322, Count: 1}, 20)
	outcome, reason = golden.eatHeld(start)
	if !outcome.Accepted {
		test.Fatalf("金苹果饱了也能吃: %q", reason)
	}
	var stone *Player = foodPlayer(protocol.Item{ID: 1, Count: 5}, 10)
	outcome, reason = stone.eatHeld(start)
	if outcome.Accepted || reason != "手里不是食物" {
		test.Fatalf("石头不能吃: %q", reason)
	}
	var spectator *Player = foodPlayer(protocol.Item{ID: 260, Count: 1}, 10)
	spectator.GameMode = int32(protocol.GameModeSpectator)
	outcome, reason = spectator.eatHeld(start)
	if outcome.Accepted || reason != "旁观模式" {
		test.Fatalf("旁观不能吃: %q", reason)
	}
	var dead *Player = foodPlayer(protocol.Item{ID: 260, Count: 1}, 10)
	dead.combat.Dead = true
	outcome, reason = dead.eatHeld(start)
	if outcome.Accepted || reason != "已死亡" {
		test.Fatalf("死人不能吃: %q", reason)
	}
}

// 创造模式照常回饥饿但不扣物品。
func TestEatHeldCreativeKeepsItem(test *testing.T) {
	var player *Player = foodPlayer(protocol.Item{ID: 260, Count: 2}, 10)
	player.GameMode = int32(protocol.GameModeCreative)
	var outcome, _ = player.eatHeld(time.Unix(9200, 0))
	if !outcome.Accepted || player.Inventory[0].Count != 2 {
		test.Fatal("创造模式吃东西不该扣物品")
	}
}

// 炖菜吃完留碗：最后一个直接把手换成碗；还有剩的就把碗放进背包；放不下的溢出到地上。
func TestEatHeldResidue(test *testing.T) {
	var single *Player = foodPlayer(protocol.Item{ID: 282, Count: 1}, 10)
	var outcome, _ = single.eatHeld(time.Unix(9300, 0))
	if !outcome.Accepted || single.Inventory[0].ID != itemBowl || single.HeldItem.ID != itemBowl {
		test.Fatalf("最后一碗吃完手里应换成碗: %+v", single.Inventory[0])
	}
	var stack *Player = foodPlayer(protocol.Item{ID: 282, Count: 2}, 10)
	outcome, _ = stack.eatHeld(time.Unix(9301, 0))
	if stack.Inventory[0].ID != 282 || stack.Inventory[0].Count != 1 || stack.Inventory[1].ID != itemBowl {
		test.Fatalf("还有剩的，碗应进背包: 槽0=%+v 槽1=%+v", stack.Inventory[0], stack.Inventory[1])
	}
	var crowded *Player = foodPlayer(protocol.Item{ID: 282, Count: 2}, 10)
	var index int = 1
	for index = 1; index < len(crowded.Inventory); index++ {
		crowded.Inventory[index] = protocol.Item{ID: 1, Count: 64}
	}
	outcome, _ = crowded.eatHeld(time.Unix(9302, 0))
	if !outcome.Accepted || outcome.Overflow.ID != itemBowl {
		test.Fatalf("背包满了碗应溢出: %+v", outcome.Overflow)
	}
}

// 疲劳只对存活的生存玩家生效；疾跑在饥饿值太低时不被承认。
func TestExhaustAndSprintGuards(test *testing.T) {
	var survival *Player = foodPlayer(protocol.Air(), 20)
	if !survival.exhaust(4) {
		test.Fatal("生存玩家攒满 4 点疲劳应扣饱和度并要求同步")
	}
	var creative *Player = foodPlayer(protocol.Air(), 20)
	creative.GameMode = int32(protocol.GameModeCreative)
	if creative.exhaust(40) || creative.food.Food != 20 {
		test.Fatal("创造模式不耗疲劳")
	}
	var dead *Player = foodPlayer(protocol.Air(), 20)
	dead.combat.Dead = true
	if dead.exhaust(40) {
		test.Fatal("死人不耗疲劳")
	}
	if dead.advanceFood(protocol.DifficultyHard) != (foodTickEffect{}) {
		test.Fatal("死人不结算饥饿")
	}
	var runner *Player = foodPlayer(protocol.Air(), 20)
	runner.setSprinting(true)
	if !runner.isSprinting() {
		test.Fatal("吃饱了可以疾跑")
	}
	var weak *Player = foodPlayer(protocol.Air(), 6)
	weak.setSprinting(true)
	if weak.isSprinting() {
		test.Fatal("饥饿值 <= 6 不能疾跑")
	}
}

// 重生后饥饿重置；存档往返不丢饥饿状态；老存档（没有 food 字段）保持满饥饿。
func TestFoodSurvivesSaveAndRevive(test *testing.T) {
	var player *Player = foodPlayer(protocol.Air(), 7)
	player.food.Saturation = 1.5
	player.food.Exhaustion = 2.5
	player.food.Timer = 33
	var data = player.Snapshot("world")
	var restored *Player = newTestPlayer(protocol.Air())
	restored.food = newFoodState()
	restored.ApplySave(data)
	if restored.food.Food != 7 || !nearly(restored.food.Saturation, 1.5) || !nearly(restored.food.Exhaustion, 2.5) || restored.food.Timer != 33 {
		test.Fatalf("存档往返后丢了饥饿状态: %+v", restored.food)
	}
	data.Food = nil
	var legacy *Player = newTestPlayer(protocol.Air())
	legacy.food = newFoodState()
	legacy.ApplySave(data)
	if legacy.food.Food != maxFoodLevel {
		test.Fatal("老存档不能把玩家读成饿死状态")
	}
	player.combat.Dead = true
	player.Revive()
	if player.food != newFoodState() || player.combat.Dead {
		test.Fatalf("重生后饥饿应重置: %+v", player.food)
	}
}
