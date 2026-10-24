package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

// 半砖：点顶面放下半砖，点底面放上半砖，点侧面看点击位置高于还是低于中线。
func TestPlaceSlabHalf(test *testing.T) {
	var ceilingPos blockrule.Pos = posAt(0, 65, 0)
	var cases []placeCase = []placeCase{
		{
			Name:    "点地面顶面放下半砖",
			View:    groundView(),
			Request: request(44, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSlab, 0)},
		},
		{
			Name:    "点天花板底面放上半砖",
			View:    newMapView().set(ceilingPos, blockrule.BlockStone, 0),
			Request: request(44, 3, ceilingPos, blockrule.FaceDown, 0, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSlab, 3|8)},
		},
		{
			Name:    "点侧面上半部分放上半砖",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(44, 0, abovePos, blockrule.FaceEast, 0.8, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockSlab, 8)},
		},
		{
			Name:    "点侧面下半部分放下半砖",
			View:    newMapView().set(abovePos, blockrule.BlockStone, 0),
			Request: request(44, 0, abovePos, blockrule.FaceEast, 0.2, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockSlab, 0)},
		},
		{
			Name:    "物品 meta 里的上半位被丢弃",
			View:    groundView(),
			Request: request(44, 8|2, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockSlab, 2)},
		},
	}
	runPlaceCases(test, cases)
}

// 同种半砖叠在一起合成双层：下半砖要从上半部分往里叠，上半砖要从下半部分往里叠。
func TestPlaceSlabMerge(test *testing.T) {
	var bottomSlab *mapView = groundView().set(abovePos, blockrule.BlockSlab, 0)
	var topSlab *mapView = newMapView().set(abovePos, blockrule.BlockSlab, 8)
	var cases []placeCase = []placeCase{
		{
			Name:    "点下半砖顶面合成双层，落在半砖那格",
			View:    bottomSlab,
			Request: request(44, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 0)},
		},
		{
			Name:    "直接点下半砖顶面也合成",
			View:    groundView().set(abovePos, blockrule.BlockSlab, 0),
			Request: request(44, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 0)},
		},
		{
			Name:    "点上半砖底面合成双层",
			View:    topSlab,
			Request: request(44, 0, abovePos, blockrule.FaceDown, 0, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 0)},
		},
		{
			Name:    "点下半砖侧面上半部分合成",
			View:    newMapView().set(abovePos, blockrule.BlockSlab, 0),
			Request: request(44, 0, abovePos, blockrule.FaceEast, 0.8, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 0)},
		},
		{
			Name:    "点下半砖侧面下半部分不合成，放到旁边",
			View:    newMapView().set(abovePos, blockrule.BlockSlab, 0),
			Request: request(44, 0, abovePos, blockrule.FaceEast, 0.2, 0),
			Expect:  []blockrule.Change{change(posAt(1, 64, 0), blockrule.BlockSlab, 0)},
		},
		{
			Name:    "点上半砖侧面下半部分合成",
			View:    topSlab,
			Request: request(44, 0, abovePos, blockrule.FaceEast, 0.2, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 0)},
		},
		{
			Name:    "种类不同的半砖占着落点，放不了",
			View:    groundView().set(abovePos, blockrule.BlockSlab, 3),
			Request: request(44, 0, groundPos, blockrule.FaceUp, 1, 0),
			Expect:  nil,
		},
		{
			Name:    "合成后保留种类",
			View:    groundView().set(abovePos, blockrule.BlockSlab, 5),
			Request: request(44, 5, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleSlab, 5)},
		},
		{
			Name:    "木半砖合成双层木半砖",
			View:    groundView().set(abovePos, blockrule.BlockWoodSlab, 1),
			Request: request(158, 1, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleWoodSlab, 1)},
		},
		{
			Name:    "红砂岩半砖合成双层红砂岩半砖",
			View:    groundView().set(abovePos, blockrule.BlockRedSandstoneSlab, 0),
			Request: request(182, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(abovePos, blockrule.BlockDoubleRedSandstoneSlab, 0)},
		},
		{
			Name:    "普通半砖和木半砖不互相合成，放到木半砖上方",
			View:    groundView().set(abovePos, blockrule.BlockWoodSlab, 0),
			Request: request(44, 0, abovePos, blockrule.FaceUp, 1, 0),
			Expect:  []blockrule.Change{change(posAt(0, 65, 0), blockrule.BlockSlab, 0)},
		},
	}
	runPlaceCases(test, cases)
}
