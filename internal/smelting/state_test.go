package smelting_test

import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/smelting"

// 点燃第一块燃料的那个 tick：燃料扣一个，燃烧时间是燃料的时长减掉这个 tick，烤制进度加一。
func TestStepStartsBurning(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 4), Fuel: stack(263, 0, 2)}
	var after smelting.State = smelting.Step(state, newFakeRecipes())
	if after.BurnTime != 1599 || after.MaxTime != 1600 || after.CookTime != 1 {
		test.Fatalf("期望 烧1599/最大1600/烤1，实际 %d/%d/%d", after.BurnTime, after.MaxTime, after.CookTime)
	}
	if after.Fuel.Count != 1 || after.Input.Count != 4 || !after.Output.IsAir() {
		test.Fatalf("燃料应剩 1、输入不变、产物为空，实际 燃料=%d 输入=%d 产物=%+v", after.Fuel.Count, after.Input.Count, after.Output)
	}
}

// 烤满 200 个 tick 才出一个：199 个 tick 时还没有，200 个 tick 时出一个，输入扣一个，烤制进度归零。
func TestStepFinishesAtTwoHundredTicks(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 4), Fuel: stack(263, 0, 2)}
	var early smelting.State = runSteps(state, newFakeRecipes(), 199)
	if !early.Output.IsAir() || early.CookTime != 199 {
		test.Fatalf("199 个 tick 时不该出产物，实际 产物=%+v 烤制=%d", early.Output, early.CookTime)
	}
	var done smelting.State = runSteps(state, newFakeRecipes(), 200)
	if done.Output.ID != 265 || done.Output.Count != 1 || done.Input.Count != 3 || done.CookTime != 0 {
		test.Fatalf("200 个 tick 应出 1 个铁锭，实际 产物=%+v 输入=%d 烤制=%d", done.Output, done.Input.Count, done.CookTime)
	}
	if done.BurnTime != 1400 || done.Fuel.Count != 1 {
		test.Fatalf("应还剩燃烧 1400、燃料 1，实际 燃烧=%d 燃料=%d", done.BurnTime, done.Fuel.Count)
	}
}

// 一块煤烧 1600 个 tick，正好烤完 4 个矿(800 tick)，剩下的 800 个 tick 空烧完；
// 输入没了不会再点燃第二块煤，燃料不浪费。
func TestStepSmeltsWholeStackAndWastesNoFuel(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 4), Fuel: stack(263, 0, 2)}
	var smelted smelting.State = runSteps(state, newFakeRecipes(), 800)
	if smelted.Output.Count != 4 || !smelted.Input.IsAir() || smelted.BurnTime != 800 {
		test.Fatalf("800 个 tick 应出 4 个，实际 产物=%d 输入=%+v 燃烧=%d", smelted.Output.Count, smelted.Input, smelted.BurnTime)
	}
	var burnedOut smelting.State = runSteps(state, newFakeRecipes(), 1700)
	if burnedOut.BurnTime != 0 || burnedOut.Lit() || burnedOut.Fuel.Count != 1 || burnedOut.Output.Count != 4 {
		test.Fatalf(
			"烧完后应熄火且还剩 1 块煤，实际 燃烧=%d 亮=%v 燃料=%d 产物=%d",
			burnedOut.BurnTime, burnedOut.Lit(), burnedOut.Fuel.Count, burnedOut.Output.Count,
		)
	}
}

type stillCase struct {
	Name  string
	State smelting.State
}

// 这些情况下推进一个 tick 之后状态完全不变：没燃料、输入烤不了、产物格满了、产物格里是别的东西、
// 没有输入、燃料格里放的不是燃料。
func TestStepLeavesStateUnchanged(test *testing.T) {
	var cases []stillCase = []stillCase{
		{Name: "没有燃料", State: smelting.State{Input: stack(15, 0, 1)}},
		{Name: "输入烤不了(泥土)", State: smelting.State{Input: stack(3, 0, 5), Fuel: stack(263, 0, 1)}},
		{Name: "产物格满了", State: smelting.State{Input: stack(15, 0, 1), Fuel: stack(263, 0, 1), Output: stack(265, 0, 64)}},
		{Name: "产物格里是别的东西", State: smelting.State{Input: stack(15, 0, 1), Fuel: stack(263, 0, 1), Output: stack(1, 0, 1)}},
		{Name: "没有输入", State: smelting.State{Fuel: stack(263, 0, 1)}},
		{Name: "燃料格里放的是泥土", State: smelting.State{Input: stack(15, 0, 1), Fuel: stack(3, 0, 5)}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item stillCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var after smelting.State = smelting.Step(item.State, newFakeRecipes())
			if !after.Equal(item.State) {
				sub.Fatalf("状态不该变，期望 %+v，实际 %+v", item.State, after)
			}
		})
	}
}

// 产物格里已经有同一种东西且没满，新烤出来的叠上去。
func TestStepStacksOntoOutput(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 1), Fuel: stack(263, 0, 1), Output: stack(265, 0, 10)}
	var done smelting.State = runSteps(state, newFakeRecipes(), 200)
	if done.Output.Count != 11 || !done.Input.IsAir() || !done.Fuel.IsAir() {
		test.Fatalf("产物应叠到 11，输入燃料用完，实际 产物=%d 输入=%+v 燃料=%+v", done.Output.Count, done.Input, done.Fuel)
	}
}

// 烧到一半燃料烧完，只要还有燃料就接着点，烤制进度不丢：两根木棍各烧 100 个 tick，
// 一个矿 200 个 tick 刚好烤完。
func TestStepRefuelsWithoutLosingProgress(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 1), Fuel: stack(280, 0, 2)}
	var half smelting.State = runSteps(state, newFakeRecipes(), 100)
	if half.BurnTime != 0 || half.CookTime != 100 || half.Fuel.Count != 1 {
		test.Fatalf("第 100 个 tick 应 燃烧0/烤100/剩1根，实际 %d/%d/%d", half.BurnTime, half.CookTime, half.Fuel.Count)
	}
	var next smelting.State = smelting.Step(half, newFakeRecipes())
	if next.BurnTime != 99 || next.CookTime != 101 || !next.Fuel.IsAir() {
		test.Fatalf("第 101 个 tick 应接着点第二根，实际 %d/%d/%+v", next.BurnTime, next.CookTime, next.Fuel)
	}
	var done smelting.State = runSteps(state, newFakeRecipes(), 200)
	if done.Output.Count != 1 || done.CookTime != 0 || done.BurnTime != 0 || done.Lit() {
		test.Fatalf("200 个 tick 应出 1 个并熄火，实际 产物=%d 烤=%d 燃烧=%d", done.Output.Count, done.CookTime, done.BurnTime)
	}
}

// 烤到一半把输入拿走：烤制进度清零，但燃料照常烧。
func TestStepRemovingInputResetsCooking(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 2), Fuel: stack(263, 0, 1)}
	var cooking smelting.State = runSteps(state, newFakeRecipes(), 50)
	if cooking.CookTime != 50 || cooking.BurnTime != 1550 {
		test.Fatalf("前提不对，实际 烤=%d 燃烧=%d", cooking.CookTime, cooking.BurnTime)
	}
	cooking.Input = protocol.Air()
	var after smelting.State = smelting.Step(cooking, newFakeRecipes())
	if after.CookTime != 0 || after.BurnTime != 1549 {
		test.Fatalf("拿走输入后应 烤0/燃烧1549，实际 %d/%d", after.CookTime, after.BurnTime)
	}
}

// 岩浆桶当燃料烧 20000 个 tick，用完留下一个空桶，不是整个消失。
func TestStepLavaBucketLeavesEmptyBucket(test *testing.T) {
	var state smelting.State = smelting.State{Input: stack(15, 0, 1), Fuel: stack(325, 10, 1)}
	var after smelting.State = smelting.Step(state, newFakeRecipes())
	if after.MaxTime != 20000 || after.BurnTime != 19999 {
		test.Fatalf("期望 最大20000/燃烧19999，实际 %d/%d", after.MaxTime, after.BurnTime)
	}
	if after.Fuel.ID != 325 || after.Fuel.Meta != 0 || after.Fuel.Count != 1 {
		test.Fatalf("应留下一个空桶，实际 %+v", after.Fuel)
	}
}

type fireCase struct {
	Name string
	Burn int32
	Max  int32
	Fire int32
}

// 火焰图标 = 剩余燃烧占比 × 200 向上取整，不能因为取整变成 0 让火焰提前消失。
func TestFireTicks(test *testing.T) {
	var cases []fireCase = []fireCase{
		{Name: "刚点燃", Burn: 1600, Max: 1600, Fire: 200},
		{Name: "烧了一半", Burn: 800, Max: 1600, Fire: 100},
		{Name: "只剩 1 个 tick 也要有一格", Burn: 1, Max: 1600, Fire: 1},
		{Name: "差一个 tick 烧满", Burn: 1599, Max: 1600, Fire: 200},
		{Name: "三分之一取整向上", Burn: 100, Max: 300, Fire: 67},
		{Name: "没在烧", Burn: 0, Max: 1600, Fire: 0},
		{Name: "最大值是 0", Burn: 5, Max: 0, Fire: 0},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item fireCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var state smelting.State = smelting.State{BurnTime: item.Burn, MaxTime: item.Max}
			if state.FireTicks() != item.Fire {
				sub.Fatalf("期望 %d，实际 %d", item.Fire, state.FireTicks())
			}
		})
	}
}

type idleCase struct {
	Name  string
	State smelting.State
	Idle  bool
}

// Idle：这一个 tick 肯定什么都不会发生，服务端据此跳过不用算的熔炉。
func TestIdleAndLit(test *testing.T) {
	var cases []idleCase = []idleCase{
		{Name: "全空", State: smelting.State{}, Idle: true},
		{Name: "只有输入", State: smelting.State{Input: stack(15, 0, 1)}, Idle: true},
		{Name: "只有燃料", State: smelting.State{Fuel: stack(263, 0, 1)}, Idle: true},
		{Name: "输入和燃料都有", State: smelting.State{Input: stack(15, 0, 1), Fuel: stack(263, 0, 1)}, Idle: false},
		{Name: "还在烧", State: smelting.State{BurnTime: 5}, Idle: false},
		{Name: "烤制进度没清", State: smelting.State{CookTime: 3}, Idle: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item idleCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			if item.State.Idle() != item.Idle {
				sub.Fatalf("Idle 期望 %v，实际 %v", item.Idle, item.State.Idle())
			}
		})
	}
	if !(smelting.State{BurnTime: 1}).Lit() || (smelting.State{}).Lit() {
		test.Fatal("燃烧时间大于 0 才算亮")
	}
}

// 空气不管写成什么样都算一样；数量、燃烧时间不同就不一样。
func TestStateEqual(test *testing.T) {
	var base smelting.State = smelting.State{Input: stack(15, 0, 4), BurnTime: 10}
	var airVariant smelting.State = smelting.State{Input: stack(15, 0, 4), Fuel: protocol.Item{ID: 5, Count: 0}, BurnTime: 10}
	if !base.Equal(airVariant) {
		test.Fatal("数量为 0 的物品就是空气，应当相等")
	}
	var moreItems smelting.State = base
	moreItems.Input = stack(15, 0, 5)
	var moreBurn smelting.State = base
	moreBurn.BurnTime = 11
	if base.Equal(moreItems) || base.Equal(moreBurn) {
		test.Fatal("数量或燃烧时间不同不该相等")
	}
}

type outputChangeCase struct {
	Name     string
	Current  protocol.Item
	Proposed protocol.Item
	Allowed  bool
}

// 产物格只能取不能放：清空、变少、不变都行；变多、换成别的、往空格里放都不行。
func TestOutputChangeAllowed(test *testing.T) {
	var current protocol.Item = stack(265, 0, 5)
	var cases []outputChangeCase = []outputChangeCase{
		{Name: "整叠取走", Current: current, Proposed: protocol.Air(), Allowed: true},
		{Name: "取走一部分", Current: current, Proposed: stack(265, 0, 3), Allowed: true},
		{Name: "不变", Current: current, Proposed: stack(265, 0, 5), Allowed: true},
		{Name: "凭空变多", Current: current, Proposed: stack(265, 0, 6), Allowed: false},
		{Name: "换成别的东西", Current: current, Proposed: stack(266, 0, 1), Allowed: false},
		{Name: "同 ID 不同 meta", Current: current, Proposed: stack(265, 1, 1), Allowed: false},
		{Name: "往空格里放东西", Current: protocol.Air(), Proposed: stack(265, 0, 1), Allowed: false},
		{Name: "空格保持空", Current: protocol.Air(), Proposed: protocol.Air(), Allowed: true},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item outputChangeCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			if smelting.OutputChangeAllowed(item.Current, item.Proposed) != item.Allowed {
				sub.Fatalf("期望 %v，实际 %v", item.Allowed, smelting.OutputChangeAllowed(item.Current, item.Proposed))
			}
		})
	}
}
