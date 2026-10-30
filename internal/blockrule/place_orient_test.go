package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 楼梯升高的一侧在玩家前方：朝东 0、朝西 1、朝南 2、朝北 3。
// 偏航角 0 看向南，90 看向西，180 看向北，270 看向东。
func TestPlaceStairFacing(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "朝南",
			View:    groundView(),
			Request: request(53, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodStairs, 2)},
		},
		{
			Name:    "朝西",
			View:    groundView(),
			Request: request(53, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodStairs, 1)},
		},
		{
			Name:    "朝北",
			View:    groundView(),
			Request: request(53, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodStairs, 3)},
		},
		{
			Name:    "朝东",
			View:    groundView(),
			Request: request(53, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodStairs, 0)},
		},
		{
			Name:    "点侧面上半部分倒置",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(53, 0, abovePos, blockrule.FaceEast, 0.8, 270),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockWoodStairs, 0|4)},
		},
		{
			Name:    "点侧面下半部分正放",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(53, 0, abovePos, blockrule.FaceEast, 0.2, 270),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockWoodStairs, 0)},
		},
		{
			Name:    "点方块底面倒置",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(53, 0, abovePos, blockrule.FaceDown, 0, 0),
			Expect:  []blockrule.Change{change(groundPos, blockrule.BlockWoodStairs, 2|4)},
		},
		{
			Name:    "点顶面即使点击位置偏高也不倒置",
			View:    groundView(),
			Request: request(53, 0, groundPos, blockrule.FaceUp, 0.9, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodStairs, 2)},
		},
		{
			Name:    "石砖楼梯同样处理",
			View:    groundView(),
			Request: request(109, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStoneBrickStairs, 1)},
		},
		{
			Name:    "红砂岩楼梯同样处理",
			View:    groundView(),
			Request: request(180, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockRedSandstoneStairs, 3)},
		},
	}
	runPlaceCases(test, cases)
}

// 原木：低 2 位是树种，点上下面竖直，点东西面横向 X（4），点南北面横向 Z（8）。
func TestPlaceLogAxis(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "点顶面竖直",
			View:    groundView(),
			Request: request(17, 1, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLog, 1)},
		},
		{
			Name:    "点南面横向 Z",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(17, 2, abovePos, blockrule.FaceSouth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, 1), blockrule.BlockLog, 2|8)},
		},
		{
			Name:    "点北面横向 Z",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(17, 0, abovePos, blockrule.FaceNorth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, -1), blockrule.BlockLog, 8)},
		},
		{
			Name:    "点东面横向 X",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(17, 3, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockLog, 3|4)},
		},
		{
			Name:    "物品自带的方向位被丢弃",
			View:    groundView(),
			Request: request(17, 1|8, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLog, 1)},
		},
		{
			Name:    "金合欢原木同样处理",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(162, 1, abovePos, blockrule.FaceWest, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(-1, 64, 0), blockrule.BlockWood2, 1|4)},
		},
	}
	runPlaceCases(test, cases)
}

// 箱子、末影箱和熔炉正面朝向玩家：玩家朝东正面朝西，meta 取 5。
// 陷阱箱那条只是钉住现状：参考端其实也按朝向设 meta，这里还没跟上（已知缺口，见 docs/方块行为.md），
// 补上朝向时这条用例要一起改。
func TestPlaceFacingBlocks(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "末影箱玩家朝东",
			View:    groundView(),
			Request: request(130, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockEnderChest, 4)},
		},
		{
			Name:    "末影箱玩家朝南",
			View:    groundView(),
			Request: request(130, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockEnderChest, 2)},
		},
		{
			Name:    "末影箱玩家朝西",
			View:    groundView(),
			Request: request(130, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockEnderChest, 5)},
		},
		{
			Name:    "末影箱玩家朝北",
			View:    groundView(),
			Request: request(130, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockEnderChest, 3)},
		},
		{
			Name:    "箱子玩家朝东",
			View:    groundView(),
			Request: request(54, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockChest, 4)},
		},
		{
			Name:    "箱子玩家朝南",
			View:    groundView(),
			Request: request(54, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockChest, 2)},
		},
		{
			Name:    "箱子玩家朝西",
			View:    groundView(),
			Request: request(54, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockChest, 5)},
		},
		{
			Name:    "箱子玩家朝北",
			View:    groundView(),
			Request: request(54, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockChest, 3)},
		},
		{
			Name:    "熔炉玩家朝南",
			View:    groundView(),
			Request: request(61, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFurnace, 2)},
		},
		{
			Name:    "陷阱箱现状：不处理朝向（已知缺口）",
			View:    groundView(),
			Request: request(146, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, 146, 0)},
		},
	}
	runPlaceCases(test, cases)
}

// 火把：点不透明方块的顶面或侧面就贴着放；点透明方块或底面，只有脚下是不透明方块才能立着放。
func TestPlaceTorch(test *testing.T) {
	var glassPos blockrule.Pos = posAt(0, 64, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "点顶面立在地上",
			View:    groundView(),
			Request: request(50, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTorch, 5)},
		},
		{
			Name:    "点东面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(50, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockTorch, 1)},
		},
		{
			Name:    "点西面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(50, 0, abovePos, blockrule.FaceWest, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(-1, 64, 0), blockrule.BlockTorch, 2)},
		},
		{
			Name:    "点南面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(50, 0, abovePos, blockrule.FaceSouth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, 1), blockrule.BlockTorch, 3)},
		},
		{
			Name:    "点北面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(50, 0, abovePos, blockrule.FaceNorth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, -1), blockrule.BlockTorch, 4)},
		},
		{
			Name: "点玻璃侧面，落点脚下是石头，立着放",
			View: newMapView().
				set(posAt(0, 64, 0), blockrule.BlockGlass, 0).
				set(posAt(1, 63, 0), blockrule.BlockStone, 0),
			Request: request(50, 0, glassPos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockTorch, 0)},
		},
		{
			Name:    "点玻璃侧面，落点脚下是空气，放不了",
			View:    newMapView().set(posAt(0, 64, 0), blockrule.BlockGlass, 0),
			Request: request(50, 0, glassPos, blockrule.FaceEast, 0.5, 0),
			Expect:  nil,
		},
		{
			Name: "点玻璃侧面，落点脚下是围栏，立着放",
			View: newMapView().
				set(posAt(0, 64, 0), blockrule.BlockGlass, 0).
				set(posAt(1, 63, 0), blockrule.BlockFence, 0),
			Request: request(50, 0, glassPos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockTorch, 0)},
		},
		{
			Name:    "点方块底面，脚下是石头，立着放",
			View:    groundView().set(posAt(0, 65, 0), blockrule.BlockStone, 0),
			Request: request(50, 0, posAt(0, 65, 0), blockrule.FaceDown, 0, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTorch, 0)},
		},
		{
			Name:    "点方块底面，脚下是空气，放不了",
			View:    newMapView().set(posAt(0, 70, 0), blockrule.BlockStone, 0),
			Request: request(50, 0, posAt(0, 70, 0), blockrule.FaceDown, 0, 0),
			Expect:  nil,
		},
	}
	runPlaceCases(test, cases)
}
