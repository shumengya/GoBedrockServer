package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

// floorView 是一块在 y=63 铺满石头、范围 radius 的世界，水和岩浆放在 y=64 上。
func floorView(radius int32) *mapView {
	var view *mapView = newMapView()
	var posX int32 = 0
	var posZ int32 = 0
	for posX = -radius; posX <= radius; posX++ {
		for posZ = -radius; posZ <= radius; posZ++ {
			view.set(posAt(posX, 63, posZ), blockrule.BlockStone, 0)
		}
	}
	return view
}

// 水平四个方向的邻格，顺序和参考端一致：西、东、北、南。
var liquidWest blockrule.Pos = posAt(-1, 64, 0)
var liquidEast blockrule.Pos = posAt(1, 64, 0)
var liquidNorth blockrule.Pos = posAt(0, 64, -1)
var liquidSouth blockrule.Pos = posAt(0, 64, 1)

// washed 是被液体冲掉的方块：变成液体，原来的方块按空手掉落。
func washed(pos blockrule.Pos, id byte, meta byte) blockrule.Change {
	return blockrule.Change{Pos: pos, ID: id, Meta: meta, Drop: true}
}

type liquidCase struct {
	Name      string
	View      *mapView
	Changes   []blockrule.Change
	Schedules []blockrule.Schedule
}

func runLiquidCases(test *testing.T, cases []liquidCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item liquidCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var update blockrule.Update = blockrule.OnScheduled(item.View, abovePos, nil, 0, false)
			if !slices.Equal(update.Changes, item.Changes) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Changes, update.Changes)
			}
			if !slices.Equal(update.Schedules, item.Schedules) {
				sub.Fatalf("定时任务期望 %+v，实际 %+v", item.Schedules, update.Schedules)
			}
		})
	}
}

// 定时评估一格水：重算自己的水位，再往下、往旁边流。预期值按参考端 Liquid::onUpdate 手算。
func TestLiquidScheduledWater(test *testing.T) {
	var gap blockrule.Pos = posAt(3, 63, 0)
	var cases []liquidCase = []liquidCase{
		{
			Name: "源头在平地上向四周铺开，水位 1",
			View: floorView(6).set(abovePos, blockrule.BlockStillWater, 0),
			Changes: []blockrule.Change{
				change(liquidWest, blockrule.BlockStillWater, 1),
				change(liquidEast, blockrule.BlockStillWater, 1),
				change(liquidNorth, blockrule.BlockStillWater, 1),
				change(liquidSouth, blockrule.BlockStillWater, 1),
			},
		},
		{
			Name: "源头脚下有空位，先往下流(掉落位 8)再向四周铺开",
			View: floorView(6).set(groundPos, blockrule.BlockAir, 0).set(abovePos, blockrule.BlockFlowingWater, 0),
			Changes: []blockrule.Change{
				change(groundPos, blockrule.BlockFlowingWater, 8),
				change(liquidWest, blockrule.BlockFlowingWater, 1),
				change(liquidEast, blockrule.BlockFlowingWater, 1),
				change(liquidNorth, blockrule.BlockFlowingWater, 1),
				change(liquidSouth, blockrule.BlockFlowingWater, 1),
			},
		},
		{
			Name: "水柱落到地面，从水位 1 开始铺，不再有重算",
			View: floorView(6).
				set(posAt(0, 65, 0), blockrule.BlockStillWater, 0).
				set(abovePos, blockrule.BlockFlowingWater, 8),
			Changes: []blockrule.Change{
				change(liquidWest, blockrule.BlockFlowingWater, 1),
				change(liquidEast, blockrule.BlockFlowingWater, 1),
				change(liquidNorth, blockrule.BlockFlowingWater, 1),
				change(liquidSouth, blockrule.BlockFlowingWater, 1),
			},
		},
		{
			Name:    "四周没有水的流动水干掉变空气",
			View:    floorView(6).set(abovePos, blockrule.BlockFlowingWater, 3),
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockAir, 0)},
		},
		{
			Name: "旁边有水位 1 的水，自己从 3 重算成 2，再往没有水的方向铺",
			View: floorView(6).
				set(liquidWest, blockrule.BlockFlowingWater, 1).
				set(abovePos, blockrule.BlockFlowingWater, 3),
			Changes: []blockrule.Change{
				change(abovePos, blockrule.BlockFlowingWater, 2),
				change(liquidEast, blockrule.BlockFlowingWater, 3),
				change(liquidNorth, blockrule.BlockFlowingWater, 3),
				change(liquidSouth, blockrule.BlockFlowingWater, 3),
			},
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 5}},
		},
		{
			Name: "两个源头夹着的水变成新的源头(脚下是实心方块)",
			View: floorView(6).
				set(liquidWest, blockrule.BlockStillWater, 0).
				set(liquidEast, blockrule.BlockStillWater, 0).
				set(abovePos, blockrule.BlockFlowingWater, 2),
			Changes: []blockrule.Change{
				change(abovePos, blockrule.BlockFlowingWater, 0),
				change(liquidNorth, blockrule.BlockFlowingWater, 1),
				change(liquidSouth, blockrule.BlockFlowingWater, 1),
			},
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 5}},
		},
		{
			Name: "两个源头夹着，但脚下是空气，不变源头",
			View: floorView(6).
				set(groundPos, blockrule.BlockAir, 0).
				set(liquidWest, blockrule.BlockStillWater, 0).
				set(liquidEast, blockrule.BlockStillWater, 0).
				set(abovePos, blockrule.BlockFlowingWater, 2),
			Changes: []blockrule.Change{
				change(abovePos, blockrule.BlockFlowingWater, 1),
				change(groundPos, blockrule.BlockFlowingWater, 9),
			},
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 5}},
		},
		{
			Name:    "只往有落差的方向流：东边两格外的地面缺了一块",
			View:    floorView(6).set(gap, blockrule.BlockAir, 0).set(abovePos, blockrule.BlockStillWater, 0),
			Changes: []blockrule.Change{change(liquidEast, blockrule.BlockStillWater, 1)},
		},
		{
			Name: "东边的区块没加载，当成墙，只往另外三个方向流",
			View: floorView(6).set(abovePos, blockrule.BlockStillWater, 0).unload(liquidEast),
			Changes: []blockrule.Change{
				change(liquidWest, blockrule.BlockStillWater, 1),
				change(liquidNorth, blockrule.BlockStillWater, 1),
				change(liquidSouth, blockrule.BlockStillWater, 1),
			},
		},
		{
			Name: "流进火把：火把被冲掉，标记按空手掉落",
			View: floorView(6).
				set(liquidEast, blockrule.BlockTorch, 5).
				set(abovePos, blockrule.BlockStillWater, 0),
			Changes: []blockrule.Change{
				change(liquidWest, blockrule.BlockStillWater, 1),
				washed(liquidEast, blockrule.BlockStillWater, 1),
				change(liquidNorth, blockrule.BlockStillWater, 1),
				change(liquidSouth, blockrule.BlockStillWater, 1),
			},
		},
		{
			Name: "水位 7 的水不再往外铺",
			View: floorView(6).
				set(liquidWest, blockrule.BlockFlowingWater, 6).
				set(abovePos, blockrule.BlockFlowingWater, 7),
		},
	}
	runLiquidCases(test, cases)
}

// 岩浆每流一格水位加 2，所以只能铺到水位 6；遇到水：水那格变石头，岩浆自己按水位硬化。
func TestLiquidScheduledLava(test *testing.T) {
	var cases []liquidCase = []liquidCase{
		{
			Name: "岩浆源头在平地上铺开，水位 2",
			View: floorView(6).set(abovePos, blockrule.BlockStillLava, 0),
			Changes: []blockrule.Change{
				change(liquidWest, blockrule.BlockStillLava, 2),
				change(liquidEast, blockrule.BlockStillLava, 2),
				change(liquidNorth, blockrule.BlockStillLava, 2),
				change(liquidSouth, blockrule.BlockStillLava, 2),
			},
		},
		{
			Name: "水位 6 的岩浆再流就是 8，不再往外铺",
			View: floorView(6).
				set(liquidWest, blockrule.BlockFlowingLava, 4).
				set(abovePos, blockrule.BlockFlowingLava, 6),
		},
		{
			Name: "岩浆流到流动的水上：水那格变石头，岩浆自己变圆石",
			View: floorView(6).
				set(liquidWest, blockrule.BlockStillLava, 0).
				set(liquidEast, blockrule.BlockFlowingWater, 3).
				set(abovePos, blockrule.BlockFlowingLava, 2),
			Changes: []blockrule.Change{
				change(abovePos, blockrule.BlockCobblestone, 0),
				change(liquidEast, blockrule.BlockStone, 0),
				change(liquidNorth, blockrule.BlockFlowingLava, 4),
				change(liquidSouth, blockrule.BlockFlowingLava, 4),
			},
		},
	}
	runLiquidCases(test, cases)
}

type liquidNeighborCase struct {
	Name      string
	View      *mapView
	Changes   []blockrule.Change
	Schedules []blockrule.Schedule
}

// 液体旁边有方块变了：水排一次定时评估；岩浆先看要不要硬化——源头碰水变黑曜石，
// 水位 1~4 碰水变圆石，更浅的不变；只在正下方有水不算。其余方块照原来的"要不要掉下来"处理。
func TestOnNeighborUpdate(test *testing.T) {
	var cases []liquidNeighborCase = []liquidNeighborCase{
		{
			Name:      "水源排定时评估，间隔 5 tick",
			View:      newMapView().set(abovePos, blockrule.BlockStillWater, 0),
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 5}},
		},
		{
			Name:      "流动的水同样排",
			View:      newMapView().set(abovePos, blockrule.BlockFlowingWater, 3),
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 5}},
		},
		{
			Name:      "岩浆间隔 30 tick",
			View:      newMapView().set(abovePos, blockrule.BlockStillLava, 0),
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 30}},
		},
		{
			Name:    "岩浆源头旁边有水，立刻变黑曜石",
			View:    newMapView().set(abovePos, blockrule.BlockStillLava, 0).set(liquidEast, blockrule.BlockStillWater, 0),
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockObsidian, 0)},
		},
		{
			Name:    "水位 3 的岩浆上面有水，变圆石",
			View:    newMapView().set(abovePos, blockrule.BlockFlowingLava, 3).set(posAt(0, 65, 0), blockrule.BlockFlowingWater, 2),
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockCobblestone, 0)},
		},
		{
			Name:      "水位 5 的岩浆碰水不硬化，改排定时评估",
			View:      newMapView().set(abovePos, blockrule.BlockFlowingLava, 5).set(liquidEast, blockrule.BlockStillWater, 0),
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 30}},
		},
		{
			Name:      "正下方的水不让岩浆硬化",
			View:      newMapView().set(abovePos, blockrule.BlockStillLava, 0).set(groundPos, blockrule.BlockStillWater, 0),
			Schedules: []blockrule.Schedule{{Pos: abovePos, Delay: 30}},
		},
		{
			Name: "石头不管",
			View: newMapView().set(abovePos, blockrule.BlockStone, 0),
		},
		{
			Name: "空气不管",
			View: newMapView(),
		},
		{
			Name: "没加载的液体不管",
			View: newMapView().set(abovePos, blockrule.BlockStillWater, 0).unload(abovePos),
		},
		{
			Name:    "没了支撑的火把仍然照旧掉下来",
			View:    newMapView().set(abovePos, blockrule.BlockTorch, 5),
			Changes: []blockrule.Change{{Pos: abovePos, ID: blockrule.BlockAir, Meta: 0, Drop: true}},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item liquidNeighborCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var update blockrule.Update = blockrule.OnNeighborUpdate(item.View, abovePos)
			if !slices.Equal(update.Changes, item.Changes) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Changes, update.Changes)
			}
			if !slices.Equal(update.Schedules, item.Schedules) {
				sub.Fatalf("定时任务期望 %+v，实际 %+v", item.Schedules, update.Schedules)
			}
		})
	}
}

// 世界最底层(y=0)的源头：脚下已经是世界之外，不往下流，只向四周铺开。
func TestLiquidScheduledAtWorldBottom(test *testing.T) {
	var origin blockrule.Pos = posAt(0, 0, 0)
	var view *mapView = newMapView().set(origin, blockrule.BlockStillWater, 0)
	var update blockrule.Update = blockrule.OnScheduled(view, origin, nil, 0, false)
	var expect []blockrule.Change = []blockrule.Change{
		change(posAt(-1, 0, 0), blockrule.BlockStillWater, 1),
		change(posAt(1, 0, 0), blockrule.BlockStillWater, 1),
		change(posAt(0, 0, -1), blockrule.BlockStillWater, 1),
		change(posAt(0, 0, 1), blockrule.BlockStillWater, 1),
	}
	if !slices.Equal(update.Changes, expect) {
		test.Fatalf("改动期望 %+v，实际 %+v", expect, update.Changes)
	}
}
