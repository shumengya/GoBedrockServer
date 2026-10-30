package server

import "testing"

import "go-mc-server/internal/protocol"

// fixedRandom 按顺序返回预设值，用完后一直返回最后一个，让概率掉落可预测。
type fixedRandom struct {
	values []int
	cursor int
}

func (source *fixedRandom) IntN(count int) int {
	if len(source.values) == 0 {
		return 0
	}
	var value int = source.values[min(source.cursor, len(source.values)-1)]
	source.cursor++
	return min(value, count-1)
}

// alwaysHit 让所有 oneIn 判定命中（返回 0 即 mt_rand 取到下限 1）。
func alwaysHit() *fixedRandom {
	return &fixedRandom{values: []int{0}}
}

// neverHit 让所有 oneIn 判定落空，randRange 取上限。
func neverHit() *fixedRandom {
	return &fixedRandom{values: []int{1 << 20}}
}

func itemOf(itemID int32) protocol.Item {
	if itemID == 0 {
		return protocol.Air()
	}
	return protocol.Item{ID: itemID, Count: 1}
}

func sameDrops(left []protocol.Item, right []protocol.Item) bool {
	if len(left) != len(right) {
		return false
	}
	var index int = 0
	for index = 0; index < len(left); index++ {
		if left[index].ID != right[index].ID || left[index].Meta != right[index].Meta || left[index].Count != right[index].Count {
			return false
		}
	}
	return true
}

type dropCase struct {
	name  string
	block byte
	meta  byte
	held  int32
	want  []protocol.Item
}

func TestBlockDropsTable(test *testing.T) {
	var cases []dropCase = []dropCase{
		{name: "木镐挖石头掉圆石", block: 1, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(4, 0, 1)}},
		{name: "空手挖石头不掉", block: 1, held: 0, want: nil},
		{name: "木镐挖花岗岩掉自身", block: 1, meta: 1, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(1, 1, 1)}},
		{name: "空手挖草掉泥土", block: 2, held: 0, want: []protocol.Item{dropOf(3, 0, 1)}},
		{name: "空手挖泥土", block: 3, held: 0, want: []protocol.Item{dropOf(3, 0, 1)}},
		{name: "木镐煤矿掉煤", block: 16, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(263, 0, 1)}},
		{name: "空手煤矿不掉", block: 16, held: 0, want: nil},
		{name: "石镐铁矿掉铁矿石", block: 15, held: itemStonePickaxe, want: []protocol.Item{dropOf(15, 0, 1)}},
		{name: "木镐铁矿不掉", block: 15, held: itemWoodenPickaxe, want: nil},
		{name: "铁镐金矿掉金矿石", block: 14, held: itemIronPickaxe, want: []protocol.Item{dropOf(14, 0, 1)}},
		{name: "铁镐钻石矿掉钻石", block: 56, held: itemIronPickaxe, want: []protocol.Item{dropOf(264, 0, 1)}},
		{name: "石镐钻石矿不掉", block: 56, held: itemStonePickaxe, want: nil},
		{name: "铁镐绿宝石矿", block: 129, held: itemIronPickaxe, want: []protocol.Item{dropOf(388, 0, 1)}},
		{name: "木镐石英矿", block: 153, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(406, 0, 1)}},
		{name: "玻璃不掉", block: 20, held: itemDiamondPickaxe, want: nil},
		{name: "玻璃板不掉", block: 102, held: 0, want: nil},
		{name: "冰不掉", block: 79, held: itemDiamondPickaxe, want: nil},
		{name: "基岩不掉", block: 7, held: itemDiamondPickaxe, want: nil},
		{name: "原木去掉朝向位", block: 17, meta: 2 | 4, held: 0, want: []protocol.Item{dropOf(17, 2, 1)}},
		{name: "书架掉三本书", block: 47, held: 0, want: []protocol.Item{dropOf(340, 0, 3)}},
		{name: "双石台阶拆两个", block: 43, meta: 3, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(44, 3, 2)}},
		{name: "门上半扇不掉", block: 64, meta: 8, held: 0, want: nil},
		{name: "门下半扇掉门", block: 64, meta: 1, held: 0, want: []protocol.Item{dropOf(324, 0, 1)}},
		{name: "床尾不掉", block: 26, meta: 0, held: 0, want: nil},
		{name: "床头掉红床", block: 26, meta: 8, held: 0, want: []protocol.Item{dropOf(355, 14, 1)}},
		{name: "工作台掉自身", block: 58, held: 0, want: []protocol.Item{dropOf(58, 0, 1)}},
		{name: "箱子朝向清零", block: 54, meta: 3, held: 0, want: []protocol.Item{dropOf(54, 0, 1)}},
		{name: "燃烧熔炉掉熔炉", block: 62, held: itemWoodenPickaxe, want: []protocol.Item{dropOf(61, 0, 1)}},
		{name: "空手雪块不掉", block: 80, held: 0, want: nil},
		{name: "铲子雪块掉雪球", block: 80, held: itemWoodenShovel, want: []protocol.Item{dropOf(332, 0, 4)}},
		{name: "剑砍蜘蛛网掉线", block: 30, held: itemIronSword, want: []protocol.Item{dropOf(287, 0, 1)}},
		{name: "钻石镐黑曜石", block: 49, held: itemDiamondPickaxe, want: []protocol.Item{dropOf(49, 0, 1)}},
		{name: "末影箱掉 8 个黑曜石", block: 130, meta: 4, held: itemDiamondPickaxe, want: []protocol.Item{dropOf(49, 0, 8)}},
		{name: "空手拆末影箱也掉 8 个黑曜石", block: 130, held: 0, want: []protocol.Item{dropOf(49, 0, 8)}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var current dropCase = cases[index]
		var got []protocol.Item = blockDrops(current.block, current.meta, itemOf(current.held), neverHit())
		if !sameDrops(got, current.want) {
			test.Errorf("%s: got %+v want %+v", current.name, got, current.want)
		}
	}
}

func TestLeavesDropChance(test *testing.T) {
	var got []protocol.Item = blockDrops(18, 0, protocol.Air(), alwaysHit())
	var want []protocol.Item = []protocol.Item{dropOf(6, 0, 1), dropOf(260, 0, 1)}
	if !sameDrops(got, want) {
		test.Fatalf("橡树叶全命中应掉树苗+苹果, got %+v", got)
	}
	got = blockDrops(18, 0, protocol.Air(), neverHit())
	if len(got) != 0 {
		test.Fatalf("橡树叶全落空应不掉, got %+v", got)
	}
	got = blockDrops(18, 1, protocol.Air(), alwaysHit())
	want = []protocol.Item{dropOf(6, 1, 1)}
	if !sameDrops(got, want) {
		test.Fatalf("云杉叶只掉云杉树苗, 不掉苹果, got %+v", got)
	}
	got = blockDrops(161, 1, protocol.Air(), alwaysHit())
	want = []protocol.Item{dropOf(6, 5, 1)}
	if !sameDrops(got, want) {
		test.Fatalf("深色橡树叶树苗 meta 应为 5, got %+v", got)
	}
}

func TestOreCountRanges(test *testing.T) {
	var got []protocol.Item = blockDrops(21, 0, itemOf(itemStonePickaxe), alwaysHit())
	if len(got) != 1 || got[0].ID != 351 || got[0].Meta != 4 || got[0].Count != 4 {
		test.Fatalf("青金石矿下限应为 4 个, got %+v", got)
	}
	got = blockDrops(21, 0, itemOf(itemStonePickaxe), neverHit())
	if len(got) != 1 || got[0].Count != 8 {
		test.Fatalf("青金石矿上限应为 8 个, got %+v", got)
	}
	got = blockDrops(73, 0, itemOf(itemIronPickaxe), neverHit())
	if len(got) != 1 || got[0].ID != 331 || got[0].Count != 5 {
		test.Fatalf("红石矿上限应为 5 个, got %+v", got)
	}
	got = blockDrops(73, 0, itemOf(itemStonePickaxe), neverHit())
	if len(got) != 0 {
		test.Fatalf("石镐挖红石矿不应掉落, got %+v", got)
	}
}

func TestGravelDrops(test *testing.T) {
	// 第一次判定燧石命中，第二次判定 1/10 命中表示沙砾不掉
	var got []protocol.Item = blockDrops(13, 0, protocol.Air(), alwaysHit())
	if !sameDrops(got, []protocol.Item{dropOf(318, 0, 1)}) {
		test.Fatalf("沙砾全命中应只掉燧石, got %+v", got)
	}
	got = blockDrops(13, 0, protocol.Air(), neverHit())
	if !sameDrops(got, []protocol.Item{dropOf(13, 0, 1)}) {
		test.Fatalf("沙砾全落空应只掉沙砾, got %+v", got)
	}
}
