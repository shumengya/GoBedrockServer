package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

type itemUseCase struct {
	Name    string
	View    *mapView
	ItemID  int32
	Meta    int32
	Face    int32
	Script  []int
	Handled bool
	Consume bool
	Wear    int32
	Changes []blockrule.Change
}

// runItemUseCases 点击的方块固定是 groundPos。
func runItemUseCases(test *testing.T, cases []itemUseCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item itemUseCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var random *scriptedRandom = &scriptedRandom{values: item.Script}
			var result, handled = blockrule.UseItem(
				item.View,
				blockrule.ItemUseRequest{ItemID: item.ItemID, ItemMeta: item.Meta, Target: groundPos, Face: item.Face},
				random,
			)
			if random.bad {
				sub.Fatal("脚本里的随机数超出了代码要求的范围，测试写错了")
			}
			if handled != item.Handled || result.Consume != item.Consume || result.Wear != item.Wear {
				sub.Fatalf(
					"期望 handled=%v 消耗=%v 磨损=%d，实际 handled=%v 消耗=%v 磨损=%d",
					item.Handled, item.Consume, item.Wear, handled, result.Consume, result.Wear,
				)
			}
			if !slices.Equal(result.Update.Changes, item.Changes) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Changes, result.Update.Changes)
			}
		})
	}
}

func grassBelowAir() *mapView {
	return newMapView().set(groundPos, blockrule.BlockGrass, 0)
}

// 锄头点草方块或泥土(不能点底面，上面要是空气)变成耕地，磨损 1 点；木、石、铁、钻石、金五种都算锄头。
func TestUseHoe(test *testing.T) {
	var tilled []blockrule.Change = []blockrule.Change{change(groundPos, blockrule.BlockFarmland, 0)}
	var cases []itemUseCase = []itemUseCase{
		{Name: "木锄头点草顶面", View: grassBelowAir(), ItemID: 290, Face: blockrule.FaceUp, Handled: true, Wear: 1, Changes: tilled},
		{
			Name:    "铁锄头点泥土",
			View:    newMapView().set(groundPos, blockrule.BlockDirt, 0),
			ItemID:  292,
			Face:    blockrule.FaceUp,
			Handled: true,
			Wear:    1,
			Changes: tilled,
		},
		{Name: "金锄头点侧面也行", View: grassBelowAir(), ItemID: 294, Face: blockrule.FaceEast, Handled: true, Wear: 1, Changes: tilled},
		{Name: "石锄头", View: grassBelowAir(), ItemID: 291, Face: blockrule.FaceUp, Handled: true, Wear: 1, Changes: tilled},
		{Name: "钻石锄头", View: grassBelowAir(), ItemID: 293, Face: blockrule.FaceUp, Handled: true, Wear: 1, Changes: tilled},
		{Name: "点底面不行", View: grassBelowAir(), ItemID: 293, Face: blockrule.FaceDown},
		{Name: "点石头不行", View: newMapView().set(groundPos, blockrule.BlockStone, 0), ItemID: 291, Face: blockrule.FaceUp},
		{
			Name:   "上面有火把不行",
			View:   grassBelowAir().set(abovePos, blockrule.BlockTorch, 5),
			ItemID: 290,
			Face:   blockrule.FaceUp,
		},
		{Name: "289 不是锄头", View: grassBelowAir(), ItemID: 289, Face: blockrule.FaceUp},
		{Name: "295 不是锄头", View: grassBelowAir(), ItemID: 295, Face: blockrule.FaceUp},
	}
	runItemUseCases(test, cases)
}

func cropAtGround(id byte, meta byte) *mapView {
	return newMapView().set(groundPos, id, meta)
}

// 骨粉(染料 meta 15)对作物和瓜茎：一次加 2~5 个阶段，最多到 7，随机数是 0~3 对应加 2~5；
// 已经成熟的不消耗骨粉。树苗的催熟要等树的生成器，先不处理。
func TestUseBoneMealOnCrops(test *testing.T) {
	var cases []itemUseCase = []itemUseCase{
		{
			Name:    "小麦 3 加 2 变 5",
			View:    cropAtGround(blockrule.BlockWheat, 3),
			ItemID:  351,
			Meta:    15,
			Script:  []int{0},
			Handled: true,
			Consume: true,
			Changes: []blockrule.Change{change(groundPos, blockrule.BlockWheat, 5)},
		},
		{
			Name:    "小麦 3 加 5 超出，封顶 7",
			View:    cropAtGround(blockrule.BlockWheat, 3),
			ItemID:  351,
			Meta:    15,
			Script:  []int{3},
			Handled: true,
			Consume: true,
			Changes: []blockrule.Change{change(groundPos, blockrule.BlockWheat, 7)},
		},
		{
			Name:    "胡萝卜 0 加 3 变 3",
			View:    cropAtGround(blockrule.BlockCarrot, 0),
			ItemID:  351,
			Meta:    15,
			Script:  []int{1},
			Handled: true,
			Consume: true,
			Changes: []blockrule.Change{change(groundPos, blockrule.BlockCarrot, 3)},
		},
		{
			Name:    "南瓜茎 1 加 4 变 5",
			View:    cropAtGround(blockrule.BlockPumpkinStem, 1),
			ItemID:  351,
			Meta:    15,
			Script:  []int{2},
			Handled: true,
			Consume: true,
			Changes: []blockrule.Change{change(groundPos, blockrule.BlockPumpkinStem, 5)},
		},
		{Name: "成熟的小麦不消耗骨粉", View: cropAtGround(blockrule.BlockWheat, 7), ItemID: 351, Meta: 15, Script: []int{0}},
		{Name: "树苗暂不处理", View: cropAtGround(blockrule.BlockSapling, 0), ItemID: 351, Meta: 15},
		{Name: "石头没反应", View: cropAtGround(blockrule.BlockStone, 0), ItemID: 351, Meta: 15},
		{Name: "红色染料(meta 1)不是骨粉", View: cropAtGround(blockrule.BlockWheat, 3), ItemID: 351, Meta: 1},
	}
	runItemUseCases(test, cases)
}

// 骨粉撒在草方块上：试 8 次，每次依次要 x、z 两个偏移(五选一，对应 -2~+2)，
// 目标是草方块且上面是空气才再要一个"长什么"(六选一：蒲公英、红花、四份高草)；同一格上面只放一株。
func TestUseBoneMealOnGrass(test *testing.T) {
	var meadow *mapView = grassBelowAir().
		set(posAt(2, 63, 0), blockrule.BlockGrass, 0).
		set(posAt(-2, 63, 0), blockrule.BlockGrass, 0)
	var cases []itemUseCase = []itemUseCase{
		{
			Name:    "撒出蒲公英和高草，重复命中同一格只放一株，打到没草的格子跳过",
			View:    meadow,
			ItemID:  351,
			Meta:    15,
			Script:  []int{4, 2, 0, 0, 2, 5, 4, 2, 2, 4},
			Handled: true,
			Consume: true,
			Changes: []blockrule.Change{
				change(posAt(2, 64, 0), blockrule.BlockDandelion, 0),
				change(posAt(-2, 64, 0), blockrule.BlockTallGrass, 1),
			},
		},
		{
			Name:    "目标格上面在没加载的区块里，跳过但骨粉照样消耗",
			View:    newMapView().set(groundPos, blockrule.BlockGrass, 0).set(posAt(2, 63, 0), blockrule.BlockGrass, 0).unload(posAt(2, 64, 0)),
			ItemID:  351,
			Meta:    15,
			Script:  []int{4, 2, 0},
			Handled: true,
			Consume: true,
		},
		{
			Name:    "上面不是空气的草不长",
			View:    grassBelowAir().set(posAt(2, 63, 0), blockrule.BlockGrass, 0).set(posAt(2, 64, 0), blockrule.BlockStone, 0),
			ItemID:  351,
			Meta:    15,
			Script:  []int{4, 2},
			Handled: true,
			Consume: true,
			Changes: nil,
		},
	}
	runItemUseCases(test, cases)
}

type trampleCase struct {
	Name    string
	View    *mapView
	Fall    float32
	Roll    int
	Trample bool
}

// 耕地被落地的实体踩坏：落差减 0.5 就是概率，用 0~999 的随机数和"概率乘 1000"比，
// 随机数比它小才踩坏；落差不到 0.5 或者脚下不是耕地，不踩也不取随机数。
func TestTrample(test *testing.T) {
	var farmland *mapView = newMapView().set(groundPos, blockrule.BlockFarmland, 0)
	var cases []trampleCase = []trampleCase{
		{Name: "落差 1.5 概率 100%", View: farmland, Fall: 1.5, Roll: 999, Trample: true},
		{Name: "落差 5 概率超过 100%", View: farmland, Fall: 5, Roll: 999, Trample: true},
		{Name: "落差 1 概率 50%，随机数 499 踩坏", View: farmland, Fall: 1, Roll: 499, Trample: true},
		{Name: "落差 1 概率 50%，随机数 500 不踩", View: farmland, Fall: 1, Roll: 500},
		{Name: "落差 0.5 概率为 0", View: farmland, Fall: 0.5, Roll: 0},
		{Name: "落差 0.2 不踩", View: farmland, Fall: 0.2, Roll: 0},
		{Name: "脚下是泥土不踩", View: newMapView().set(groundPos, blockrule.BlockDirt, 0), Fall: 5, Roll: 0},
		{Name: "区块没加载不踩", View: newMapView().set(groundPos, blockrule.BlockFarmland, 0).unload(groundPos), Fall: 5, Roll: 0},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item trampleCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var random *scriptedRandom = &scriptedRandom{values: []int{item.Roll}}
			var update, trampled = blockrule.Trample(item.View, groundPos, item.Fall, random)
			if trampled != item.Trample {
				sub.Fatalf("踩坏期望 %v，实际 %v", item.Trample, trampled)
			}
			var expect []blockrule.Change = nil
			if item.Trample {
				expect = []blockrule.Change{change(groundPos, blockrule.BlockDirt, 0)}
			}
			if !slices.Equal(update.Changes, expect) {
				sub.Fatalf("改动期望 %+v，实际 %+v", expect, update.Changes)
			}
		})
	}
}

// 耕地上面压了实心方块就变回泥土，不掉东西；庄稼、火把、水这类不实心的不影响。
func TestNeighborFarmland(test *testing.T) {
	var above blockrule.Pos = posAt(0, 65, 0)
	var revert []blockrule.Change = []blockrule.Change{change(abovePos, blockrule.BlockDirt, 0)}
	type farmlandCase struct {
		Name   string
		View   *mapView
		Expect []blockrule.Change
	}
	var cases []farmlandCase = []farmlandCase{
		{Name: "上面压石头", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockStone, 0), Expect: revert},
		{Name: "上面是仙人掌", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockCactus, 0), Expect: revert},
		{Name: "上面是玻璃", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockGlass, 0), Expect: revert},
		{Name: "上面是空气", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0)},
		{Name: "上面是小麦", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockWheat, 3)},
		{Name: "上面是火把", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockTorch, 5)},
		{Name: "上面是水", View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockStillWater, 0)},
		{
			Name: "上面的区块没加载，保守地不动",
			View: newMapView().set(abovePos, blockrule.BlockFarmland, 0).set(above, blockrule.BlockStone, 0).unload(above),
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item farmlandCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var changes []blockrule.Change = blockrule.OnNeighborChange(item.View, abovePos)
			if !slices.Equal(changes, item.Expect) {
				sub.Fatalf("期望 %+v，实际 %+v", item.Expect, changes)
			}
		})
	}
}
