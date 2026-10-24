package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

type pairCase struct {
	Name   string
	View   *mapView
	Pos    blockrule.Pos
	Expect blockrule.Pos
	Paired bool
}

// PairedHalf 找门、床、双层植物的另一半。另一半必须真的在那里，种类、上下（头脚）位、
// 床的朝向、双层植物的品种都对得上才算配成一对。
func TestPairedHalf(test *testing.T) {
	var upper blockrule.Pos = posAt(0, 65, 0)
	var headPos blockrule.Pos = posAt(1, 64, 0)
	var cases []pairCase = []pairCase{
		{
			Name:   "门下半找到上半",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:    abovePos,
			Expect: upper,
			Paired: true,
		},
		{
			Name:   "门上半找到下半",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:    upper,
			Expect: abovePos,
			Paired: true,
		},
		{
			Name:   "门轴位不同不影响配对",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 2).set(upper, blockrule.BlockWoodDoor, 9),
			Pos:    abovePos,
			Expect: upper,
			Paired: true,
		},
		{
			Name:   "门上面是空气，没有另一半",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0),
			Pos:    abovePos,
			Paired: false,
		},
		{
			Name:   "上面是铁门，种类不同",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockIronDoor, 8),
			Pos:    abovePos,
			Paired: false,
		},
		{
			Name:   "上面也是下半位，不是一对",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 0),
			Pos:    abovePos,
			Paired: false,
		},
		{
			Name:   "床脚找到床头，床头在朝东的一格",
			View:   newMapView().set(abovePos, blockrule.BlockBed, 3).set(headPos, blockrule.BlockBed, 3|8),
			Pos:    abovePos,
			Expect: headPos,
			Paired: true,
		},
		{
			Name:   "床头找回床脚",
			View:   newMapView().set(abovePos, blockrule.BlockBed, 3).set(headPos, blockrule.BlockBed, 3|8),
			Pos:    headPos,
			Expect: abovePos,
			Paired: true,
		},
		{
			Name:   "床的占用位不影响配对",
			View:   newMapView().set(abovePos, blockrule.BlockBed, 3|4).set(headPos, blockrule.BlockBed, 3|8),
			Pos:    abovePos,
			Expect: headPos,
			Paired: true,
		},
		{
			Name:   "床头朝向对不上",
			View:   newMapView().set(abovePos, blockrule.BlockBed, 3).set(headPos, blockrule.BlockBed, 2|8),
			Pos:    abovePos,
			Paired: false,
		},
		{
			Name:   "双层植物下半找到上半",
			View:   newMapView().set(abovePos, blockrule.BlockDoublePlant, 4).set(upper, blockrule.BlockDoublePlant, 4|8),
			Pos:    abovePos,
			Expect: upper,
			Paired: true,
		},
		{
			Name:   "双层植物品种对不上",
			View:   newMapView().set(abovePos, blockrule.BlockDoublePlant, 4).set(upper, blockrule.BlockDoublePlant, 8),
			Pos:    abovePos,
			Paired: false,
		},
		{
			Name:   "普通方块没有另一半",
			View:   newMapView().set(abovePos, blockrule.BlockStone, 0).set(upper, blockrule.BlockStone, 8),
			Pos:    abovePos,
			Paired: false,
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item pairCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var other, paired = blockrule.PairedHalf(item.View, item.Pos)
			if paired != item.Paired {
				sub.Fatalf("配对期望 %v，实际 %v", item.Paired, paired)
			}
			if paired && other != item.Expect {
				sub.Fatalf("另一半期望 %+v，实际 %+v", item.Expect, other)
			}
		})
	}
}

// 两格方块缺了另一半就要掉下来；门和双层植物的下半脚下撑不住也要掉，床不看脚下。
func TestNeighborPairs(test *testing.T) {
	var upper blockrule.Pos = posAt(0, 65, 0)
	var headPos blockrule.Pos = posAt(1, 64, 0)
	var door *mapView = newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8)
	var cases []neighborCase = []neighborCase{
		{
			Name: "门立在石头上，两半齐全，不动",
			View: door.set(groundPos, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name: "门上半不看脚下，只看配对",
			View: newMapView().set(groundPos, blockrule.BlockStone, 0).set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:  upper,
		},
		{
			Name:   "门下半脚下是空气，掉下来",
			View:   newMapView().set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "门下半脚下是水，掉下来",
			View:   newMapView().set(groundPos, blockrule.BlockStillWater, 0).set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "门下半脚下是玻璃，玻璃是实心的，不掉",
			View: newMapView().set(groundPos, blockrule.BlockGlass, 0).set(abovePos, blockrule.BlockWoodDoor, 0).set(upper, blockrule.BlockWoodDoor, 8),
			Pos:  abovePos,
		},
		{
			Name:   "门下半没有上半，掉下来",
			View:   newMapView().set(groundPos, blockrule.BlockStone, 0).set(abovePos, blockrule.BlockWoodDoor, 0),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "门上半没有下半，掉下来",
			View:   newMapView().set(upper, blockrule.BlockWoodDoor, 8),
			Pos:    upper,
			Broken: true,
		},
		{
			Name: "门的另一半在没加载的区块里，保守地不动",
			View: newMapView().set(groundPos, blockrule.BlockStone, 0).set(abovePos, blockrule.BlockWoodDoor, 0).unload(upper),
			Pos:  abovePos,
		},
		{
			Name: "床两半齐全，脚下是空气也不动",
			View: newMapView().set(abovePos, blockrule.BlockBed, 3).set(headPos, blockrule.BlockBed, 3|8),
			Pos:  abovePos,
		},
		{
			Name:   "床脚没有床头，掉下来",
			View:   newMapView().set(abovePos, blockrule.BlockBed, 3),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "床头没有床脚，掉下来",
			View:   newMapView().set(headPos, blockrule.BlockBed, 3|8),
			Pos:    headPos,
			Broken: true,
		},
		{
			Name: "双层植物立在草上两半齐全，不动",
			View: newMapView().set(groundPos, blockrule.BlockGrass, 0).set(abovePos, blockrule.BlockDoublePlant, 0).set(upper, blockrule.BlockDoublePlant, 8),
			Pos:  abovePos,
		},
		{
			Name:   "双层植物下半脚下是空气，掉下来",
			View:   newMapView().set(abovePos, blockrule.BlockDoublePlant, 0).set(upper, blockrule.BlockDoublePlant, 8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "双层植物下半脚下是玻璃，玻璃透明，掉下来",
			View:   newMapView().set(groundPos, blockrule.BlockGlass, 0).set(abovePos, blockrule.BlockDoublePlant, 0).set(upper, blockrule.BlockDoublePlant, 8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "双层植物上半没有下半，掉下来",
			View:   newMapView().set(upper, blockrule.BlockDoublePlant, 8),
			Pos:    upper,
			Broken: true,
		},
	}
	runNeighborCases(test, cases)
}
