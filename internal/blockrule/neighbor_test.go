package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

type neighborCase struct {
	Name   string
	View   *mapView
	Pos    blockrule.Pos
	Broken bool
}

// 邻居变了之后，附着在别的方块上的方块要么保持原样（返回 nil），
// 要么变成空气并标记 Drop，让服务端按"自己掉下来"结算掉落物。
func runNeighborCases(test *testing.T, cases []neighborCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item neighborCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var changes []blockrule.Change = blockrule.OnNeighborChange(item.View, item.Pos)
			var expect []blockrule.Change = nil
			if item.Broken {
				expect = []blockrule.Change{{Pos: item.Pos, ID: blockrule.BlockAir, Meta: 0, Drop: true}}
			}
			if !slices.Equal(changes, expect) {
				sub.Fatalf("期望 %+v，实际 %+v", expect, changes)
			}
		})
	}
}

func TestNeighborTorch(test *testing.T) {
	var west blockrule.Pos = posAt(-1, 64, 0)
	var north blockrule.Pos = posAt(0, 64, -1)
	var cases []neighborCase = []neighborCase{
		{
			Name: "立在石头上不掉",
			View: groundView().set(abovePos, blockrule.BlockTorch, 5),
			Pos:  abovePos,
		},
		{
			Name: "meta 0 立在石头上不掉",
			View: groundView().set(abovePos, blockrule.BlockTorch, 0),
			Pos:  abovePos,
		},
		{
			Name:   "脚下变成空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockTorch, 5),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "立在围栏上不掉",
			View: newMapView().set(groundPos, blockrule.BlockFence, 0).set(abovePos, blockrule.BlockTorch, 0),
			Pos:  abovePos,
		},
		{
			Name:   "立在玻璃上掉落",
			View:   newMapView().set(groundPos, blockrule.BlockGlass, 0).set(abovePos, blockrule.BlockTorch, 5),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "立在水上掉落",
			View:   newMapView().set(groundPos, blockrule.BlockStillWater, 0).set(abovePos, blockrule.BlockTorch, 5),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "贴西墙(meta 1)，西边有石头不掉，脚下是空气也不掉",
			View: newMapView().set(abovePos, blockrule.BlockTorch, 1).set(west, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name:   "贴西墙(meta 1)，西边被挖空掉落，即使脚下有石头",
			View:   groundView().set(abovePos, blockrule.BlockTorch, 1),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "贴北墙(meta 3)，北边有石头不掉",
			View: newMapView().set(abovePos, blockrule.BlockTorch, 3).set(north, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name:   "贴北墙(meta 3)，北边是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockTorch, 3),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "支撑方块在没加载的区块里，保守地不掉",
			View: newMapView().set(abovePos, blockrule.BlockTorch, 5).unload(groundPos),
			Pos:  abovePos,
		},
		{
			Name: "火把自己在没加载的区块里，不处理",
			View: newMapView().set(abovePos, blockrule.BlockTorch, 5).unload(abovePos),
			Pos:  abovePos,
		},
	}
	runNeighborCases(test, cases)
}

func TestNeighborPlants(test *testing.T) {
	var cases []neighborCase = []neighborCase{
		{
			Name: "红花在草上不掉",
			View: newMapView().set(groundPos, blockrule.BlockGrass, 0).set(abovePos, blockrule.BlockPoppy, 0),
			Pos:  abovePos,
		},
		{
			Name: "红花在耕地上不掉",
			View: newMapView().set(groundPos, blockrule.BlockFarmland, 0).set(abovePos, blockrule.BlockPoppy, 0),
			Pos:  abovePos,
		},
		{
			Name:   "红花脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockPoppy, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "蒲公英脚下是玻璃掉落",
			View:   newMapView().set(groundPos, blockrule.BlockGlass, 0).set(abovePos, blockrule.BlockDandelion, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "树苗在泥土上不掉",
			View: newMapView().set(groundPos, blockrule.BlockDirt, 0).set(abovePos, blockrule.BlockSapling, 2),
			Pos:  abovePos,
		},
		{
			Name:   "高草脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockTallGrass, 1),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "枯萎灌木在沙子上不掉",
			View: newMapView().set(groundPos, blockrule.BlockSand, 0).set(abovePos, blockrule.BlockDeadBush, 0),
			Pos:  abovePos,
		},
		{
			Name: "蘑菇在石头上不掉",
			View: groundView().set(abovePos, blockrule.BlockBrownMushroom, 0),
			Pos:  abovePos,
		},
		{
			Name:   "蘑菇在耕地上掉落，耕地对蘑菇算透明",
			View:   newMapView().set(groundPos, blockrule.BlockFarmland, 0).set(abovePos, blockrule.BlockBrownMushroom, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "花脚下的区块没加载，保守地不掉",
			View: newMapView().set(abovePos, blockrule.BlockPoppy, 0).unload(groundPos),
			Pos:  abovePos,
		},
		{
			Name: "石头不受邻居影响",
			View: groundView().set(abovePos, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name: "空气不受邻居影响",
			View: newMapView(),
			Pos:  abovePos,
		},
	}
	runNeighborCases(test, cases)
}

func TestNeighborSugarcaneAndCactus(test *testing.T) {
	var east blockrule.Pos = posAt(1, 64, 0)
	var cases []neighborCase = []neighborCase{
		{
			Name: "甘蔗在沙子上不掉",
			View: newMapView().set(groundPos, blockrule.BlockSand, 0).set(abovePos, blockrule.BlockSugarcane, 0),
			Pos:  abovePos,
		},
		{
			Name: "甘蔗接在甘蔗上不掉",
			View: newMapView().set(groundPos, blockrule.BlockSugarcane, 0).set(abovePos, blockrule.BlockSugarcane, 0),
			Pos:  abovePos,
		},
		{
			Name:   "甘蔗脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockSugarcane, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "仙人掌在沙子上不掉",
			View: newMapView().set(groundPos, blockrule.BlockSand, 0).set(abovePos, blockrule.BlockCactus, 0),
			Pos:  abovePos,
		},
		{
			Name: "仙人掌接在仙人掌上不掉",
			View: newMapView().set(groundPos, blockrule.BlockCactus, 0).set(abovePos, blockrule.BlockCactus, 0),
			Pos:  abovePos,
		},
		{
			Name:   "仙人掌在泥土上掉落",
			View:   newMapView().set(groundPos, blockrule.BlockDirt, 0).set(abovePos, blockrule.BlockCactus, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "仙人掌旁边是水不掉，水不是实心方块",
			View: newMapView().
				set(groundPos, blockrule.BlockSand, 0).
				set(abovePos, blockrule.BlockCactus, 0).
				set(east, blockrule.BlockStillWater, 0),
			Pos: abovePos,
		},
		{
			Name: "仙人掌旁边出现石头掉落",
			View: newMapView().
				set(groundPos, blockrule.BlockSand, 0).
				set(abovePos, blockrule.BlockCactus, 0).
				set(east, blockrule.BlockStone, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "仙人掌旁边没加载的区块不算实心",
			View: newMapView().
				set(groundPos, blockrule.BlockSand, 0).
				set(abovePos, blockrule.BlockCactus, 0).
				set(east, blockrule.BlockStone, 0).
				unload(east),
			Pos: abovePos,
		},
	}
	runNeighborCases(test, cases)
}

func TestNeighborCrops(test *testing.T) {
	var cases []neighborCase = []neighborCase{
		{
			Name: "小麦在耕地上不掉",
			View: newMapView().set(groundPos, blockrule.BlockFarmland, 0).set(abovePos, blockrule.BlockWheat, 3),
			Pos:  abovePos,
		},
		{
			Name:   "小麦在泥土上掉落",
			View:   newMapView().set(groundPos, blockrule.BlockDirt, 0).set(abovePos, blockrule.BlockWheat, 3),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "胡萝卜脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockCarrot, 7),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "南瓜茎在泥土上掉落",
			View:   newMapView().set(groundPos, blockrule.BlockDirt, 0).set(abovePos, blockrule.BlockPumpkinStem, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "作物脚下的区块没加载，保守地不掉",
			View: newMapView().set(abovePos, blockrule.BlockWheat, 3).unload(groundPos),
			Pos:  abovePos,
		},
	}
	runNeighborCases(test, cases)
}
