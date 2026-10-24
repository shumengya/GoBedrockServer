package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 花、树苗、蘑菇、作物这些方块放下去之前要先看脚下，条件照参考端各方块的 place。
func TestPlaceNeedsSupport(test *testing.T) {
	var east blockrule.Pos = posAt(1, 63, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "红花放在草上",
			View:    newMapView().set(groundPos, blockrule.BlockGrass, 0),
			Request: request(38, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockPoppy, 0)},
		},
		{
			Name:    "红花放在石头上不行",
			View:    groundView(),
			Request: request(38, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "蒲公英放在泥土上",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(37, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDandelion, 0)},
		},
		{
			Name:    "树苗放在灰化土上，保留树种",
			View:    newMapView().set(groundPos, blockrule.BlockPodzol, 0),
			Request: request(6, 2, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSapling, 2)},
		},
		{
			Name:    "树苗放在沙子上不行",
			View:    newMapView().set(groundPos, blockrule.BlockSand, 0),
			Request: request(6, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "高草放在泥土上",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(31, 1, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTallGrass, 1)},
		},
		{
			Name:    "高草放在沙子上不行",
			View:    newMapView().set(groundPos, blockrule.BlockSand, 0),
			Request: request(31, 1, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "枯萎灌木放在沙子上",
			View:    newMapView().set(groundPos, blockrule.BlockSand, 0),
			Request: request(32, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDeadBush, 0)},
		},
		{
			Name:    "枯萎灌木放在硬化黏土上",
			View:    newMapView().set(groundPos, blockrule.BlockHardenedClay, 0),
			Request: request(32, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDeadBush, 0)},
		},
		{
			Name:    "枯萎灌木放在泥土上不行",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(32, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "蘑菇放在石头上",
			View:    groundView(),
			Request: request(39, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockBrownMushroom, 0)},
		},
		{
			Name:    "蘑菇放在玻璃上不行",
			View:    newMapView().set(groundPos, blockrule.BlockGlass, 0),
			Request: request(39, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
	}
	runPlaceCases(test, cases)
	var more []placeCase = []placeCase{
		{
			Name: "甘蔗放在沙子上，沙子旁边有水",
			View: newMapView().
				set(groundPos, blockrule.BlockSand, 0).
				set(east, blockrule.BlockStillWater, 0),
			Request: request(338, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSugarcane, 0)},
		},
		{
			Name:    "甘蔗放在没有水的沙子上不行",
			View:    newMapView().set(groundPos, blockrule.BlockSand, 0),
			Request: request(338, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "甘蔗接在甘蔗上",
			View:    newMapView().set(groundPos, blockrule.BlockSugarcane, 0),
			Request: request(338, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSugarcane, 0)},
		},
		{
			Name:    "仙人掌放在四周没有实心方块的沙子上",
			View:    newMapView().set(groundPos, blockrule.BlockSand, 0),
			Request: request(81, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockCactus, 0)},
		},
		{
			Name: "仙人掌旁边有石头不行",
			View: newMapView().
				set(groundPos, blockrule.BlockSand, 0).
				set(posAt(1, 64, 0), blockrule.BlockStone, 0),
			Request: request(81, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "仙人掌放在泥土上不行",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(81, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "小麦种子种在耕地上，物品 ID 查表成方块",
			View:    newMapView().set(groundPos, blockrule.BlockFarmland, 0),
			Request: request(295, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWheat, 0)},
		},
		{
			Name:    "小麦种子种在泥土上不行",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(295, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "胡萝卜种在耕地上",
			View:    newMapView().set(groundPos, blockrule.BlockFarmland, 0),
			Request: request(391, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockCarrot, 0)},
		},
		{
			Name:    "南瓜种子种在耕地上",
			View:    newMapView().set(groundPos, blockrule.BlockFarmland, 0),
			Request: request(361, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockPumpkinStem, 0)},
		},
	}
	runPlaceCases(test, more)
}
