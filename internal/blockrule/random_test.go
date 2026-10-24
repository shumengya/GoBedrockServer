package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

// scriptedRandom 按事先排好的序列吐随机数，用完之后一直返回 0。
// 要求的范围装不下脚本里的值时记下 bad，说明测试脚本写错了。
type scriptedRandom struct {
	values []int
	next   int
	bad    bool
}

func (random *scriptedRandom) IntN(count int) int {
	var value int = 0
	if random.next < len(random.values) {
		value = random.values[random.next]
	}
	random.next++
	if value < 0 || value >= count {
		random.bad = true
		return 0
	}
	return value
}

type randomCase struct {
	Name    string
	View    *mapView
	Pos     blockrule.Pos
	Script  []int
	Changes []blockrule.Change
}

func runRandomCases(test *testing.T, cases []randomCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item randomCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var random *scriptedRandom = &scriptedRandom{values: item.Script}
			var update blockrule.Update = blockrule.OnRandomTick(item.View, item.Pos, random)
			if random.bad {
				sub.Fatal("脚本里的随机数超出了代码要求的范围，测试写错了")
			}
			if !slices.Equal(update.Changes, item.Changes) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Changes, update.Changes)
			}
			if len(update.Schedules) != 0 {
				sub.Fatalf("不该排定时任务，实际 %+v", update.Schedules)
			}
		})
	}
}

func cropView(id byte, meta byte) *mapView {
	return newMapView().set(groundPos, blockrule.BlockFarmland, 0).set(abovePos, id, meta)
}

// 作物每次被抽中只有三分之一的机会长一个阶段，长到 7 就不再长。
func TestRandomTickCrops(test *testing.T) {
	var cases []randomCase = []randomCase{
		{
			Name:    "小麦抽中 1 长一个阶段",
			View:    cropView(blockrule.BlockWheat, 3),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockWheat, 4)},
		},
		{Name: "抽中 0 不长", View: cropView(blockrule.BlockWheat, 3), Pos: abovePos, Script: []int{0}},
		{Name: "抽中 2 不长", View: cropView(blockrule.BlockWheat, 3), Pos: abovePos, Script: []int{2}},
		{Name: "成熟的不再长", View: cropView(blockrule.BlockWheat, 7), Pos: abovePos, Script: []int{1}},
		{
			Name:    "胡萝卜",
			View:    cropView(blockrule.BlockCarrot, 0),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockCarrot, 1)},
		},
		{
			Name:    "马铃薯",
			View:    cropView(blockrule.BlockPotato, 5),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockPotato, 6)},
		},
		{
			Name:    "甜菜长到成熟",
			View:    cropView(blockrule.BlockBeetroot, 6),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockBeetroot, 7)},
		},
		{
			Name:   "区块没加载不处理",
			View:   cropView(blockrule.BlockWheat, 3).unload(abovePos),
			Pos:    abovePos,
			Script: []int{1},
		},
	}
	runRandomCases(test, cases)
}

func stemView(stemID byte, meta byte) *mapView {
	return newMapView().set(groundPos, blockrule.BlockFarmland, 0).set(abovePos, stemID, meta)
}

// 瓜茎先长到 7；长满后四周没有果实，就随机挑一个水平方向(北、南、西、东)，
// 那格是空气且脚下是耕地、草或泥土才结果。随机数顺序：先一次三选一的"长不长"，再一次四选一的方向。
func TestRandomTickStems(test *testing.T) {
	var north blockrule.Pos = posAt(0, 64, -1)
	var east blockrule.Pos = posAt(1, 64, 0)
	var cases []randomCase = []randomCase{
		{
			Name:    "南瓜茎长一个阶段",
			View:    stemView(blockrule.BlockPumpkinStem, 2),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockPumpkinStem, 3)},
		},
		{
			Name:    "西瓜茎长一个阶段",
			View:    stemView(blockrule.BlockMelonStem, 0),
			Pos:     abovePos,
			Script:  []int{1},
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockMelonStem, 1)},
		},
		{Name: "没抽中不长", View: stemView(blockrule.BlockPumpkinStem, 2), Pos: abovePos, Script: []int{0}},
		{
			Name:    "长满了往北结南瓜",
			View:    stemView(blockrule.BlockPumpkinStem, 7).set(posAt(0, 63, -1), blockrule.BlockGrass, 0),
			Pos:     abovePos,
			Script:  []int{1, 0},
			Changes: []blockrule.Change{change(north, blockrule.BlockPumpkin, 0)},
		},
		{
			Name:    "长满了往东结南瓜，脚下是泥土",
			View:    stemView(blockrule.BlockPumpkinStem, 7).set(posAt(1, 63, 0), blockrule.BlockDirt, 0),
			Pos:     abovePos,
			Script:  []int{1, 3},
			Changes: []blockrule.Change{change(east, blockrule.BlockPumpkin, 0)},
		},
		{
			Name:    "西瓜茎结西瓜",
			View:    stemView(blockrule.BlockMelonStem, 7).set(posAt(0, 63, -1), blockrule.BlockGrass, 0),
			Pos:     abovePos,
			Script:  []int{1, 0},
			Changes: []blockrule.Change{change(north, blockrule.BlockMelonBlock, 0)},
		},
		{
			Name:   "目标格脚下是石头，不结果",
			View:   stemView(blockrule.BlockPumpkinStem, 7).set(posAt(0, 63, -1), blockrule.BlockStone, 0),
			Pos:    abovePos,
			Script: []int{1, 0},
		},
		{
			Name:   "目标格不是空气，不结果",
			View:   stemView(blockrule.BlockPumpkinStem, 7).set(posAt(0, 63, -1), blockrule.BlockGrass, 0).set(north, blockrule.BlockStone, 0),
			Pos:    abovePos,
			Script: []int{1, 0},
		},
		{
			Name:   "四周已经有南瓜，不再结",
			View:   stemView(blockrule.BlockPumpkinStem, 7).set(posAt(0, 63, -1), blockrule.BlockGrass, 0).set(posAt(-1, 64, 0), blockrule.BlockPumpkin, 0),
			Pos:    abovePos,
			Script: []int{1, 0},
		},
		{
			Name:   "目标格在没加载的区块里，不结果",
			View:   stemView(blockrule.BlockPumpkinStem, 7).set(posAt(0, 63, -1), blockrule.BlockGrass, 0).unload(north),
			Pos:    abovePos,
			Script: []int{1, 0},
		},
	}
	runRandomCases(test, cases)
}

func columnView(ground byte, id byte, meta byte) *mapView {
	return newMapView().set(groundPos, ground, 0).set(abovePos, id, meta)
}

// 甘蔗：只有最底下那格会长，meta 当计数；数满 15 往上接一格，已经是甘蔗的跳过，最高三格。
func TestRandomTickSugarcane(test *testing.T) {
	var second blockrule.Pos = posAt(0, 65, 0)
	var third blockrule.Pos = posAt(0, 66, 0)
	var reset blockrule.Change = change(abovePos, blockrule.BlockSugarcane, 0)
	var cases []randomCase = []randomCase{
		{
			Name:    "计数加一",
			View:    columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 5),
			Pos:     abovePos,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockSugarcane, 6)},
		},
		{
			Name:    "数满往上接第二格并清零",
			View:    columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 15),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset, change(second, blockrule.BlockSugarcane, 0)},
		},
		{
			Name:    "已有两格，接第三格",
			View:    columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 15).set(second, blockrule.BlockSugarcane, 0),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset, change(third, blockrule.BlockSugarcane, 0)},
		},
		{
			Name: "已有三格，不再往上",
			View: columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 15).
				set(second, blockrule.BlockSugarcane, 0).
				set(third, blockrule.BlockSugarcane, 0),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset},
		},
		{
			Name:    "上面是石头，只清零不接",
			View:    columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 15).set(second, blockrule.BlockStone, 0),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset},
		},
		{
			Name:    "上面在没加载的区块，只清零不接",
			View:    columnView(blockrule.BlockSand, blockrule.BlockSugarcane, 15).unload(second),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset},
		},
		{
			Name: "不是最底下那格，不长",
			View: columnView(blockrule.BlockSugarcane, blockrule.BlockSugarcane, 15),
			Pos:  abovePos,
		},
	}
	runRandomCases(test, cases)
}

// 仙人掌：数满 15 后一口气往上接最多两格；某一格不是空气，或者长出来四周有实心方块，就停。
func TestRandomTickCactus(test *testing.T) {
	var second blockrule.Pos = posAt(0, 65, 0)
	var third blockrule.Pos = posAt(0, 66, 0)
	var reset blockrule.Change = change(abovePos, blockrule.BlockCactus, 0)
	var cases []randomCase = []randomCase{
		{
			Name:    "计数加一",
			View:    columnView(blockrule.BlockSand, blockrule.BlockCactus, 5),
			Pos:     abovePos,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockCactus, 6)},
		},
		{
			Name: "数满一口气接两格",
			View: columnView(blockrule.BlockSand, blockrule.BlockCactus, 15),
			Pos:  abovePos,
			Changes: []blockrule.Change{
				reset,
				change(second, blockrule.BlockCactus, 0),
				change(third, blockrule.BlockCactus, 0),
			},
		},
		{
			Name:    "上面已经是仙人掌，不接",
			View:    columnView(blockrule.BlockSand, blockrule.BlockCactus, 15).set(second, blockrule.BlockCactus, 0),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset},
		},
		{
			Name:    "要长出来的那格旁边有石头，不接",
			View:    columnView(blockrule.BlockSand, blockrule.BlockCactus, 15).set(posAt(1, 65, 0), blockrule.BlockStone, 0),
			Pos:     abovePos,
			Changes: []blockrule.Change{reset},
		},
		{
			Name: "不是最底下那格，不长",
			View: columnView(blockrule.BlockCactus, blockrule.BlockCactus, 15),
			Pos:  abovePos,
		},
	}
	runRandomCases(test, cases)
}

// 草方块每次试四次，每次依次要 x(三选一)、y(五选一)、z(三选一) 三个随机数，
// 目标是草方块位置加 (x-1, y-3, z-1)。目标是普通泥土且上面透明、不是液体，才长成草。
func TestRandomTickGrassSpread(test *testing.T) {
	var east blockrule.Pos = posAt(1, 64, 0)
	var west blockrule.Pos = posAt(-1, 64, 0)
	var low blockrule.Pos = posAt(0, 61, 0)
	var idle []int = []int{1, 4, 1}
	var grass *mapView = newMapView().set(abovePos, blockrule.BlockGrass, 0)
	var cases []randomCase = []randomCase{
		{
			Name:    "同一层东边的泥土长成草",
			View:    newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 0),
			Pos:     abovePos,
			Script:  slices.Concat([]int{2, 3, 1}, idle, idle, idle),
			Changes: []blockrule.Change{change(east, blockrule.BlockGrass, 0)},
		},
		{
			Name:    "往下三格的泥土也能长",
			View:    newMapView().set(abovePos, blockrule.BlockGrass, 0).set(low, blockrule.BlockDirt, 0),
			Pos:     abovePos,
			Script:  slices.Concat([]int{1, 0, 1}, idle, idle, idle),
			Changes: []blockrule.Change{change(low, blockrule.BlockGrass, 0)},
		},
		{
			Name:   "一次最多试四回，两回命中就长两块",
			View:   newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 0).set(west, blockrule.BlockDirt, 0),
			Pos:    abovePos,
			Script: slices.Concat([]int{2, 3, 1}, []int{0, 3, 1}, idle, idle),
			Changes: []blockrule.Change{
				change(east, blockrule.BlockGrass, 0),
				change(west, blockrule.BlockGrass, 0),
			},
		},
		{
			Name:   "泥土上面盖着石头不长",
			View:   newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 0).set(posAt(1, 65, 0), blockrule.BlockStone, 0),
			Pos:    abovePos,
			Script: slices.Concat([]int{2, 3, 1}, idle, idle, idle),
		},
		{
			Name:   "泥土上面是水不长",
			View:   newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 0).set(posAt(1, 65, 0), blockrule.BlockStillWater, 0),
			Pos:    abovePos,
			Script: slices.Concat([]int{2, 3, 1}, idle, idle, idle),
		},
		{
			Name:   "粗泥土(meta 1)不长",
			View:   newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 1),
			Pos:    abovePos,
			Script: slices.Concat([]int{2, 3, 1}, idle, idle, idle),
		},
		{
			Name:   "目标在没加载的区块里不长",
			View:   newMapView().set(abovePos, blockrule.BlockGrass, 0).set(east, blockrule.BlockDirt, 0).unload(east),
			Pos:    abovePos,
			Script: slices.Concat([]int{2, 3, 1}, idle, idle, idle),
		},
		{
			Name:   "试到的不是泥土不长",
			View:   grass,
			Pos:    abovePos,
			Script: slices.Concat(idle, idle, idle, idle),
		},
	}
	runRandomCases(test, cases)
}

// 菌丝每次只试一格，x(三选一)、y(五选一)、z(三选一)，目标是菌丝位置加 (x-1, y-2, z-1)。
func TestRandomTickMyceliumSpread(test *testing.T) {
	var east blockrule.Pos = posAt(1, 64, 0)
	var low blockrule.Pos = posAt(0, 62, 0)
	var cases []randomCase = []randomCase{
		{
			Name:    "同一层东边的泥土长成菌丝",
			View:    newMapView().set(abovePos, blockrule.BlockMycelium, 0).set(east, blockrule.BlockDirt, 0),
			Pos:     abovePos,
			Script:  []int{2, 2, 1},
			Changes: []blockrule.Change{change(east, blockrule.BlockMycelium, 0)},
		},
		{
			Name:    "往下两格的泥土也能长",
			View:    newMapView().set(abovePos, blockrule.BlockMycelium, 0).set(low, blockrule.BlockDirt, 0),
			Pos:     abovePos,
			Script:  []int{1, 0, 1},
			Changes: []blockrule.Change{change(low, blockrule.BlockMycelium, 0)},
		},
		{
			Name:   "试到的不是泥土不长",
			View:   newMapView().set(abovePos, blockrule.BlockMycelium, 0).set(east, blockrule.BlockStone, 0),
			Pos:    abovePos,
			Script: []int{2, 2, 1},
		},
		{
			Name:   "泥土上面盖着石头不长",
			View:   newMapView().set(abovePos, blockrule.BlockMycelium, 0).set(east, blockrule.BlockDirt, 0).set(posAt(1, 65, 0), blockrule.BlockStone, 0),
			Pos:    abovePos,
			Script: []int{2, 2, 1},
		},
		{
			Name:   "泥土上面是水不长",
			View:   newMapView().set(abovePos, blockrule.BlockMycelium, 0).set(east, blockrule.BlockDirt, 0).set(posAt(1, 65, 0), blockrule.BlockStillWater, 0),
			Pos:    abovePos,
			Script: []int{2, 2, 1},
		},
	}
	runRandomCases(test, cases)
}

type randomTickedCase struct {
	Name   string
	ID     byte
	Expect bool
}

// 参与随机刻的方块清单，服务端靠它先筛掉绝大多数格子。
func TestIsRandomTicked(test *testing.T) {
	var cases []randomTickedCase = []randomTickedCase{
		{Name: "小麦", ID: blockrule.BlockWheat, Expect: true},
		{Name: "胡萝卜", ID: blockrule.BlockCarrot, Expect: true},
		{Name: "马铃薯", ID: blockrule.BlockPotato, Expect: true},
		{Name: "甜菜", ID: blockrule.BlockBeetroot, Expect: true},
		{Name: "南瓜茎", ID: blockrule.BlockPumpkinStem, Expect: true},
		{Name: "西瓜茎", ID: blockrule.BlockMelonStem, Expect: true},
		{Name: "甘蔗", ID: blockrule.BlockSugarcane, Expect: true},
		{Name: "仙人掌", ID: blockrule.BlockCactus, Expect: true},
		{Name: "草方块", ID: blockrule.BlockGrass, Expect: true},
		{Name: "菌丝", ID: blockrule.BlockMycelium, Expect: true},
		{Name: "石头不参与", ID: blockrule.BlockStone, Expect: false},
		{Name: "空气不参与", ID: blockrule.BlockAir, Expect: false},
		{Name: "南瓜果实自己不长", ID: blockrule.BlockPumpkin, Expect: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item randomTickedCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			if blockrule.IsRandomTicked(item.ID) != item.Expect {
				sub.Fatalf("期望 %v，实际 %v", item.Expect, blockrule.IsRandomTicked(item.ID))
			}
		})
	}
	var update blockrule.Update = blockrule.OnRandomTick(newMapView().set(abovePos, blockrule.BlockStone, 0), abovePos, &scriptedRandom{})
	if len(update.Changes) != 0 || len(update.Schedules) != 0 {
		test.Fatalf("石头被抽中不该有任何动作，实际 %+v", update)
	}
}
