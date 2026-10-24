package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 通用放置规则：点击面决定落点，高草和水这类可顶替的方块直接被替换，
// 空气、非方块物品、世界上下边界外都放不了。
func TestPlaceGeneric(test *testing.T) {
	var east blockrule.Pos = at(1, 64, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "点地面顶面放在上方",
			View:    groundView(),
			Request: request(1, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStone, 0)},
		},
		{
			Name:    "羊毛保留颜色 meta",
			View:    groundView(),
			Request: request(35, 14, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, 35, 14)},
		},
		{
			Name:    "物品 meta 只取低 4 位",
			View:    groundView(),
			Request: request(35, 0x1f, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, 35, 15)},
		},
		{
			Name:    "点侧面放在侧面那格",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(1, 0, abovePos, blockrule.FaceEast, 0.5, 0),
			Expect:  []blockrule.Change{change(east, blockrule.BlockStone, 0)},
		},
		{
			Name:    "点到空气放不了",
			View:    newMapView(),
			Request: request(1, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "空手放不了",
			View:    groundView(),
			Request: request(0, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "铁铲不是方块",
			View:    groundView(),
			Request: request(256, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "小麦种子种在石头上不行",
			View:    groundView(),
			Request: request(295, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "落点已有方块放不了",
			View:    groundView().set(abovePos, blockrule.BlockDirt, 0),
			Request: request(1, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "落点是水可以顶替",
			View:    groundView().set(abovePos, blockrule.BlockStillWater, 0),
			Request: request(1, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStone, 0)},
		},
		{
			Name:    "点高草直接替换高草那格",
			View:    groundView().set(abovePos, blockrule.BlockTallGrass, 1),
			Request: request(1, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStone, 0)},
		},
		{
			Name:    "点雪片顶面直接替换没叠满的雪片",
			View:    groundView().set(abovePos, blockrule.BlockSnowLayer, 3),
			Request: request(1, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockStone, 0)},
		},
		{
			Name:    "叠满 8 层的雪片不能顶替，放到它上面",
			View:    groundView().set(abovePos, blockrule.BlockSnowLayer, 7),
			Request: request(1, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(at(0, 65, 0), blockrule.BlockStone, 0)},
		},
		{
			Name:    "世界顶部之外放不了",
			View:    newMapView().set(at(0, 255, 0), blockrule.BlockStone, 0),
			Request: request(1, 0, at(0, 255, 0), blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "世界底部之外放不了",
			View:    newMapView().set(at(0, 0, 0), blockrule.BlockBedrock, 0),
			Request: request(1, 0, at(0, 0, 0), blockrule.FaceDown, 0, 0),
			Expect:  nil,
		},
		{
			Name:    "面编号非法放不了",
			View:    groundView(),
			Request: request(1, 0, groundPos, 6, 1, 0),
			Expect:  nil,
		},
	}
	runPlaceCases(test, cases)
}

// Place 只读 view，不能顺手把世界改了，否则服务端先算后校验的流程会出错。
func TestPlaceDoesNotMutateView(test *testing.T) {
	var view *mapView = groundView()
	var _, ok = blockrule.Place(view, request(1, 0, groundPos, blockrule.FaceUp, 1, 0))
	if !ok {
		test.Fatal("应该能放下")
	}
	var id, _ = view.Block(abovePos)
	if id != blockrule.BlockAir {
		test.Fatalf("Place 不该改 view，落点变成了 %d", id)
	}
}
