package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 活板门：低 2 位是朝向，bit 2 是吊在上半格；点方块底面或点侧面上半部分时放上半格。
func TestPlaceTrapdoor(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "朝东",
			View:    groundView(),
			Request: request(96, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 1)},
		},
		{
			Name:    "朝南",
			View:    groundView(),
			Request: request(96, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 3)},
		},
		{
			Name:    "朝西",
			View:    groundView(),
			Request: request(96, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 0)},
		},
		{
			Name:    "朝北",
			View:    groundView(),
			Request: request(96, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 2)},
		},
		{
			Name:    "点侧面上半部分吊在上半格",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(96, 0, abovePos, blockrule.FaceEast, 0.8, 270),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockTrapdoor, 1|4)},
		},
		{
			Name:    "点侧面下半部分放下半格",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(96, 0, abovePos, blockrule.FaceEast, 0.2, 270),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockTrapdoor, 1)},
		},
		{
			Name:    "点方块底面放上半格",
			View:    newMapView().set(posAt(0, 65, 0), blockrule.BlockStone, 0),
			Request: request(96, 0, posAt(0, 65, 0), blockrule.FaceDown, 0, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 1|4)},
		},
		{
			Name:    "点顶面即使点击位置偏高也放下半格",
			View:    groundView(),
			Request: request(96, 0, groundPos, blockrule.FaceUp, 0.9, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 1)},
		},
		{
			Name:    "铁活板门同样处理",
			View:    groundView(),
			Request: request(167, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockIronTrapdoor, 3)},
		},
	}
	runPlaceCases(test, cases)
}

// 栅栏门的 meta 低 2 位是玩家朝向减一：朝东 3、朝南 0、朝西 1、朝北 2，各种木头都一样。
func TestPlaceFenceGate(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "朝东",
			View:    groundView(),
			Request: request(107, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 3)},
		},
		{
			Name:    "朝南",
			View:    groundView(),
			Request: request(107, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 0)},
		},
		{
			Name:    "朝西",
			View:    groundView(),
			Request: request(107, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 1)},
		},
		{
			Name:    "朝北",
			View:    groundView(),
			Request: request(107, 0, groundPos, blockrule.FaceUp, 1, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 2)},
		},
		{
			Name:    "金合欢栅栏门",
			View:    groundView(),
			Request: request(187, 0, groundPos, blockrule.FaceUp, 1, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockFenceGateAcacia, 1)},
		},
	}
	runPlaceCases(test, cases)
}

// 拉杆：点侧面贴墙（东 1、西 2、南 3、北 4），点顶面或底面按玩家朝向东西还是南北分两种。
// 必须贴在不透明的方块上。
func TestPlaceLever(test *testing.T) {
	var stone *mapView = newMapView().set(abovePos, blockrule.BlockStone, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "点东面",
			View:    stone,
			Request: request(69, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockLever, 1)},
		},
		{
			Name:    "点西面",
			View:    stone,
			Request: request(69, 0, abovePos, blockrule.FaceWest, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(-1, 64, 0), blockrule.BlockLever, 2)},
		},
		{
			Name:    "点南面",
			View:    stone,
			Request: request(69, 0, abovePos, blockrule.FaceSouth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, 1), blockrule.BlockLever, 3)},
		},
		{
			Name:    "点北面",
			View:    stone,
			Request: request(69, 0, abovePos, blockrule.FaceNorth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, -1), blockrule.BlockLever, 4)},
		},
		{
			Name:    "点顶面，朝东西的玩家",
			View:    groundView(),
			Request: request(69, 0, groundPos, blockrule.FaceUp, 1, 270),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLever, 6)},
		},
		{
			Name:    "点顶面，朝南北的玩家",
			View:    groundView(),
			Request: request(69, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLever, 5)},
		},
		{
			Name:    "点底面挂天花板，朝东西的玩家",
			View:    newMapView().set(posAt(0, 65, 0), blockrule.BlockStone, 0),
			Request: request(69, 0, posAt(0, 65, 0), blockrule.FaceDown, 0, 90),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLever, 0)},
		},
		{
			Name:    "点底面挂天花板，朝南北的玩家",
			View:    newMapView().set(posAt(0, 65, 0), blockrule.BlockStone, 0),
			Request: request(69, 0, posAt(0, 65, 0), blockrule.FaceDown, 0, 180),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockLever, 7)},
		},
		{
			Name:    "贴在玻璃上不行",
			View:    newMapView().set(abovePos, blockrule.BlockGlass, 0),
			Request: request(69, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  nil,
		},
		{
			Name:    "点高草(可顶替且透明)不行",
			View:    groundView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(69, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
	}
	runPlaceCases(test, cases)
}

// 按钮的 meta 就是被点击的面，同样必须贴在不透明的方块上。
func TestPlaceButton(test *testing.T) {
	var cases []placeCase = []placeCase{
		{
			Name:    "石按钮点顶面",
			View:    groundView(),
			Request: request(77, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStoneButton, 1)},
		},
		{
			Name:    "石按钮点东面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(77, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockStoneButton, 5)},
		},
		{
			Name:    "木按钮点底面",
			View:    newMapView().set(posAt(0, 65, 0), blockrule.BlockStone, 0),
			Request: request(143, 0, posAt(0, 65, 0), blockrule.FaceDown, 0, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockWoodButton, 0)},
		},
		{
			Name:    "木按钮点北面",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(143, 0, abovePos, blockrule.FaceNorth, 0.5, 0),
			Expect:  []blockrule.Change{change(posAt(0, 64, -1), blockrule.BlockWoodButton, 2)},
		},
		{
			Name:    "贴在玻璃上不行",
			View:    newMapView().set(abovePos, blockrule.BlockGlass, 0),
			Request: request(77, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  nil,
		},
	}
	runPlaceCases(test, cases)
}
