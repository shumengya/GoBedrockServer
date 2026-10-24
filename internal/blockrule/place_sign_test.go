package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 告示牌物品 323 放下去是立牌 63；点顶面立在地上，meta 是朝向；点侧面变成贴墙牌 68，
// meta 就是被点击的那个面；点底面放不了。靠着的那一格是空气也放不了。
func TestPlaceSign(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "点顶面放立牌，偏航 0 朝南 meta 8",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 8)},
		},
		{
			Name:    "偏航 90 朝西 meta 12",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 12)},
		},
		{
			Name:    "偏航 180 朝北 meta 0",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 0)},
		},
		{
			Name:    "偏航 270 朝东 meta 4",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 4)},
		},
		{
			Name:    "负偏航也折回 0~15",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, -168.75),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 1)},
		},
		{
			Name:    "转过很多圈的负偏航也能折回 0~15",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, -40000),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 6)},
		},
		{
			Name:    "偏航 179.9 折回 meta 0，不能超出 0~15",
			View:    groundView(),
			Request: request(323, 0, groundPos, blockrule.FaceUp, 1, 179.9),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 0)},
		},
		{
			Name:    "物品带的方向 meta 被丢掉，只看玩家偏航",
			View:    groundView(),
			Request: request(323, 5, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 0)},
		},
		{
			Name:    "点北面放贴墙牌，meta 就是北 2，牌子落在被点方块北边",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(323, 0, abovePos, blockrule.FaceNorth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, -1), blockrule.BlockWallSign, 2)},
		},
		{
			Name:    "点东面，牌子落在东边，meta 5",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(323, 0, abovePos, blockrule.FaceEast, 0.5, 90),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockWallSign, 5)},
		},
		{
			Name:    "点底面放不了",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(323, 0, abovePos, blockrule.FaceDown, 0, 0),
			Expect:  nil,
		},
		{
			Name:    "点高草顶面，立在草地上",
			View:    groundView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(323, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSignPost, 8)},
		},
		{
			Name:    "点高草顶面但脚下是空气，放不了",
			View:    newMapView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(323, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "点高草东面，贴墙牌背后是空气，放不了",
			View:    groundView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(323, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  nil,
		},
		{
			Name: "点高草东面，贴墙牌背后有石头，放得下",
			View: groundView().
				set(abovePos, blockrule.BlockTallGrass, 1).
				set(posAt(-1, 64, 0), blockrule.BlockStone, 0),
			Request: request(323, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWallSign, 5)},
		},
	}
	runPlaceCases(test, cases)
}

// 立牌脚下变成空气就掉下来；水、火把、花都撑得住，只有空气算没有支撑。
// 贴墙牌背后是空气就掉下来，meta 不在 2~5 说明数据坏了，不动它。
func TestNeighborSign(test *testing.T) {
	var south blockrule.Pos = posAt(0, 64, 1)
	var east blockrule.Pos = posAt(1, 64, 0)
	var cases []neighborCase = []neighborCase{
		{
			Name: "立牌立在石头上不掉",
			View: groundView().set(abovePos, blockrule.BlockSignPost, 8),
			Pos:  abovePos,
		},
		{
			Name:   "立牌脚下变成空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockSignPost, 8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "立牌脚下是水不掉",
			View: newMapView().
				set(abovePos, blockrule.BlockSignPost, 8).
				set(groundPos, blockrule.BlockStillWater, 0),
			Pos: abovePos,
		},
		{
			Name: "立牌脚下是花不掉",
			View: newMapView().
				set(abovePos, blockrule.BlockSignPost, 8).
				set(groundPos, blockrule.BlockDandelion, 0),
			Pos: abovePos,
		},
		{
			Name: "立牌脚下区块没加载，保守当成有支撑",
			View: newMapView().set(abovePos, blockrule.BlockSignPost, 8).unload(groundPos),
			Pos:  abovePos,
		},
		{
			Name: "贴墙牌 meta 4 是点西面，背后在东边，东边是石头不掉",
			View: newMapView().
				set(abovePos, blockrule.BlockWallSign, 4).
				set(east, blockrule.BlockStone, 0),
			Pos: abovePos,
		},
		{
			Name:   "贴墙牌背后变成空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockWallSign, 4),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "贴墙牌 meta 2 背后在南边，南边是石头不掉",
			View: newMapView().
				set(abovePos, blockrule.BlockWallSign, 2).
				set(south, blockrule.BlockStone, 0),
			Pos: abovePos,
		},
		{
			Name:   "贴墙牌 meta 2 南边是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockWallSign, 2),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "贴墙牌 meta 1 不是合法方向，不动它",
			View:   newMapView().set(abovePos, blockrule.BlockWallSign, 1),
			Pos:    abovePos,
			Broken: false,
		},
		{
			Name:   "贴墙牌 meta 6 不是合法方向，不动它",
			View:   newMapView().set(abovePos, blockrule.BlockWallSign, 6),
			Pos:    abovePos,
			Broken: false,
		},
		{
			Name:   "贴墙牌背后的区块没加载，保守当成有支撑",
			View:   newMapView().set(abovePos, blockrule.BlockWallSign, 4).unload(east),
			Pos:    abovePos,
			Broken: false,
		},
	}
	runNeighborCases(test, cases)
}
