package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

type fallCase struct {
	Name  string
	View  *mapView
	Falls bool
}

// 沙子、沙砾脚下是空气、液体或火就变成下落中的方块；压在别的东西上（哪怕是火把、高草）不掉。
// 脚下的区块没加载时保守地不动，别的方块不受重力影响。
func TestNeighborGravity(test *testing.T) {
	var expect []blockrule.Change = []blockrule.Change{
		{Pos: abovePos, ID: blockrule.BlockAir, Meta: 0, Fall: true},
	}
	var cases []fallCase = []fallCase{
		{Name: "沙子下面是空气", View: newMapView().set(abovePos, blockrule.BlockSand, 0), Falls: true},
		{Name: "沙砾下面是空气", View: newMapView().set(abovePos, blockrule.BlockGravel, 0), Falls: true},
		{
			Name:  "沙子下面是静水",
			View:  newMapView().set(groundPos, blockrule.BlockStillWater, 0).set(abovePos, blockrule.BlockSand, 0),
			Falls: true,
		},
		{
			Name:  "沙子下面是流动岩浆",
			View:  newMapView().set(groundPos, blockrule.BlockFlowingLava, 0).set(abovePos, blockrule.BlockSand, 0),
			Falls: true,
		},
		{
			Name:  "沙子下面是火",
			View:  newMapView().set(groundPos, blockrule.BlockFire, 0).set(abovePos, blockrule.BlockSand, 0),
			Falls: true,
		},
		{Name: "沙子压在石头上不掉", View: groundView().set(abovePos, blockrule.BlockSand, 0)},
		{
			Name: "沙子压在沙子上不掉",
			View: newMapView().set(groundPos, blockrule.BlockSand, 0).set(abovePos, blockrule.BlockSand, 0),
		},
		{
			Name: "沙子压在火把上不掉",
			View: newMapView().set(groundPos, blockrule.BlockTorch, 5).set(abovePos, blockrule.BlockSand, 0),
		},
		{
			Name: "沙子压在高草上不掉",
			View: newMapView().set(groundPos, blockrule.BlockTallGrass, 1).set(abovePos, blockrule.BlockSand, 0),
		},
		{
			Name: "脚下的区块没加载，保守地不掉",
			View: newMapView().set(abovePos, blockrule.BlockSand, 0).unload(groundPos),
		},
		{
			Name: "沙子自己在没加载的区块里，不处理",
			View: newMapView().set(abovePos, blockrule.BlockSand, 0).unload(abovePos),
		},
		{Name: "石头下面是空气也不掉", View: newMapView().set(abovePos, blockrule.BlockStone, 0)},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item fallCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var changes []blockrule.Change = blockrule.OnNeighborChange(item.View, abovePos)
			var want []blockrule.Change = nil
			if item.Falls {
				want = expect
			}
			if !slices.Equal(changes, want) {
				sub.Fatalf("期望 %+v，实际 %+v", want, changes)
			}
		})
	}
}
