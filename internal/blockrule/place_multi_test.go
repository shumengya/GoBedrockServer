package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 门只能点方块顶面放。下半 meta 低 2 位是玩家朝向，上半 meta 是 8，
// 旁边有同种门或者"远侧是墙、近侧是空"时上半再加 1 表示门轴在另一边。
// 偏航 0 朝南，90 朝西，180 朝北，270 朝东。
func TestPlaceDoor(test *testing.T) {
	var upper blockrule.Pos = at(0, 65, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "朝东",
			View:    groundView(),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 0),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
		{
			Name:    "朝南",
			View:    groundView(),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 1),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
		{
			Name:    "朝西",
			View:    groundView(),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 2),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
		{
			Name:    "朝北",
			View:    groundView(),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 3),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
		{
			Name:    "铁门同样处理",
			View:    groundView(),
			Request: request(330, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockIronDoor, 0),
				change(upper, blockrule.BlockIronDoor, 8),
			},
		},
		{
			Name:    "云杉门物品 ID 查表成云杉门方块",
			View:    groundView(),
			Request: request(427, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockSpruceDoor, 0),
				change(upper, blockrule.BlockSpruceDoor, 8),
			},
		},
		{
			Name:    "朝东时北边是空、南边是石头，近侧是空远侧是墙，门轴换边",
			View:    groundView().set(at(0, 64, 1), blockrule.BlockStone, 0),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 0),
				change(upper, blockrule.BlockWoodDoor, 9),
			},
		},
		{
			Name:    "近侧已经有一扇同种门，门轴换边",
			View:    groundView().set(at(0, 64, -1), blockrule.BlockWoodDoor, 0),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 0),
				change(upper, blockrule.BlockWoodDoor, 9),
			},
		},
		{
			Name: "两侧都是墙，近侧不透明，门轴不换边",
			View: groundView().
				set(at(0, 64, -1), blockrule.BlockStone, 0).
				set(at(0, 64, 1), blockrule.BlockStone, 0),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 0),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
		{
			Name:    "点侧面不能放门",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0).set(groundPos, blockrule.BlockStone, 0),
			Request: request(324, 0, abovePos, blockrule.FaceEast, 0.5, 270),
			Expect:  nil,
		},
		{
			Name:    "上半的位置被占了放不了",
			View:    groundView().set(upper, blockrule.BlockStone, 0),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name:    "脚下是玻璃放不了",
			View:    newMapView().set(groundPos, blockrule.BlockGlass, 0),
			Request: request(324, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name:    "世界最高处放不了，上半会超出高度",
			View:    newMapView().set(at(0, 254, 0), blockrule.BlockStone, 0),
			Request: request(324, 0, at(0, 254, 0), blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name:    "点高草直接替换高草那一格放门",
			View:    groundView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(324, 0, abovePos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockWoodDoor, 0),
				change(upper, blockrule.BlockWoodDoor, 8),
			},
		},
	}
	runPlaceCases(test, cases)
}

// 床脚放在点击的那格，床头放在玩家正前方，两格脚下都要有不透明方块。
// 方块 meta 低 2 位是床头所在方向（0 南、1 西、2 北、3 东），床头多 8；颜色不在方块 meta 里。
func TestPlaceBed(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "朝东，床头在东边",
			View:    groundView().set(at(1, 63, 0), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockBed, 3),
				change(at(1, 64, 0), blockrule.BlockBed, 3|8),
			},
		},
		{
			Name:    "朝南，床头在南边",
			View:    groundView().set(at(0, 63, 1), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockBed, 0),
				change(at(0, 64, 1), blockrule.BlockBed, 8),
			},
		},
		{
			Name:    "朝西，床头在西边",
			View:    groundView().set(at(-1, 63, 0), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 90),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockBed, 1),
				change(at(-1, 64, 0), blockrule.BlockBed, 1|8),
			},
		},
		{
			Name:    "朝北，床头在北边",
			View:    groundView().set(at(0, 63, -1), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 180),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockBed, 2),
				change(at(0, 64, -1), blockrule.BlockBed, 2|8),
			},
		},
		{
			Name:    "床头脚下是空气放不了",
			View:    groundView(),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name: "床头那格被占了放不了",
			View: groundView().
				set(at(1, 63, 0), blockrule.BlockStone, 0).
				set(at(1, 64, 0), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name: "床脚脚下是玻璃放不了",
			View: newMapView().
				set(groundPos, blockrule.BlockGlass, 0).
				set(at(1, 63, 0), blockrule.BlockStone, 0),
			Request: request(355, 14, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  nil,
		},
		{
			Name: "床头那格是高草可以顶替",
			View: groundView().
				set(at(1, 63, 0), blockrule.BlockStone, 0).
				set(at(1, 64, 0), blockrule.BlockTallGrass, 1),
			Request: request(355, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockBed, 3),
				change(at(1, 64, 0), blockrule.BlockBed, 3|8),
			},
		},
	}
	runPlaceCases(test, cases)
}

// 双层植物：下半放在草或泥土上，上半在正上方，上半 meta 多一个 8。
func TestPlaceDoublePlant(test *testing.T) {
	var upper blockrule.Pos = at(0, 65, 0)
	var grass *mapView = newMapView().set(groundPos, blockrule.BlockGrass, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "向日葵",
			View:    grass,
			Request: request(175, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockDoublePlant, 0),
				change(upper, blockrule.BlockDoublePlant, 8),
			},
		},
		{
			Name:    "玫瑰丛保留品种",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			Request: request(175, 4, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockDoublePlant, 4),
				change(upper, blockrule.BlockDoublePlant, 4|8),
			},
		},
		{
			Name:    "物品 meta 里带的上半位被丢弃",
			View:    newMapView().set(groundPos, blockrule.BlockGrass, 0),
			Request: request(175, 8|1, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockDoublePlant, 1),
				change(upper, blockrule.BlockDoublePlant, 1|8),
			},
		},
		{
			Name:    "放在石头上不行",
			View:    groundView(),
			Request: request(175, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "上方被占了放不了",
			View:    newMapView().set(groundPos, blockrule.BlockGrass, 0).set(upper, blockrule.BlockStone, 0),
			Request: request(175, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "上方是高草可以顶替",
			View:    newMapView().set(groundPos, blockrule.BlockGrass, 0).set(upper, blockrule.BlockTallGrass, 1),
			Request: request(175, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect: []blockrule.Change{
				change(abovePos, blockrule.BlockDoublePlant, 0),
				change(upper, blockrule.BlockDoublePlant, 8),
			},
		},
	}
	runPlaceCases(test, cases)
}
