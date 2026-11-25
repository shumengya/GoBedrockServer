package server

import "testing"

import "go-mc-server/internal/world"

type breakTimeCase struct {
	name  string
	block byte
	item  int32
	want  int32
}

func TestBreakTimeTicks(test *testing.T) {
	var wool int32 = int32(world.BlockWool)
	var cases []breakTimeCase = []breakTimeCase{
		{name: "空手泥土", block: world.BlockDirt, item: wool, want: 15},
		{name: "空手草方块", block: world.BlockGrass, item: wool, want: 18},
		{name: "空手沙子", block: world.BlockSand, item: wool, want: 15},
		{name: "空手玻璃", block: world.BlockGlass, item: wool, want: 9},
		{name: "空手羊毛", block: world.BlockWool, item: wool, want: 24},
		{name: "空手木板", block: world.BlockPlanks, item: wool, want: 60},
		{name: "空手原木", block: world.BlockLog, item: wool, want: 60},
		{name: "空手石头采集不了", block: world.BlockStone, item: wool, want: 150},
		{name: "空手圆石采集不了", block: world.BlockCobblestone, item: 0, want: 200},
		{name: "木镐圆石", block: world.BlockCobblestone, item: itemWoodenPickaxe, want: 30},
		{name: "木镐石头", block: world.BlockStone, item: itemWoodenPickaxe, want: 23},
		{name: "木镐铁矿采集不了", block: 15, item: itemWoodenPickaxe, want: 150},
		{name: "石镐铁矿", block: 15, item: itemStonePickaxe, want: 23},
		{name: "钻石镐黑曜石", block: 49, item: itemDiamondPickaxe, want: 132},
		{name: "铁斧原木", block: world.BlockLog, item: itemIronAxe, want: 10},
		{name: "镐子挖泥土不加速", block: world.BlockDirt, item: itemDiamondPickaxe, want: 15},
		{name: "剪刀剪羊毛", block: world.BlockWool, item: itemShears, want: 2},
		{name: "基岩不可破坏", block: world.BlockBedrock, item: wool, want: 0},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var current breakTimeCase = cases[index]
		var got int32 = breakTimeTicks(current.block, current.item, 0, 0)
		if got != current.want {
			test.Errorf("%s: block=%d item=%d got %d want %d", current.name, current.block, current.item, got, current.want)
		}
	}
}

func TestHardnessTableCoversRegisteredBlocks(test *testing.T) {
	// 抽查参考端硬度：完整表不能退化回默认值 10
	var expected map[byte]float64 = map[byte]float64{
		1:   1.5,
		14:  3,
		49:  35,
		54:  2.5,
		80:  0.2,
		130: 22.5,
		159: 1.25,
		236: 1.8,
	}
	var blockID byte = 0
	var hardness float64 = 0
	for blockID, hardness = range expected {
		var info blockInfo = miningInfoFor(blockID)
		if info.Hardness != hardness {
			test.Errorf("block %d hardness %.2f want %.2f", blockID, info.Hardness, hardness)
		}
	}
	if len(blockInfoTable) < 220 {
		test.Fatalf("硬度表只有 %d 项，应覆盖全部注册方块", len(blockInfoTable))
	}
}

type harvestCase struct {
	name  string
	block byte
	item  int32
	want  bool
}

func TestCanHarvestTierRequirements(test *testing.T) {
	var cases []harvestCase = []harvestCase{
		{name: "空手石头", block: 1, item: 0, want: false},
		{name: "木镐石头", block: 1, item: itemWoodenPickaxe, want: true},
		{name: "木镐铁矿", block: 15, item: itemWoodenPickaxe, want: false},
		{name: "金镐铁矿", block: 15, item: itemGoldPickaxe, want: false},
		{name: "石镐铁矿", block: 15, item: itemStonePickaxe, want: true},
		{name: "石镐金矿", block: 14, item: itemStonePickaxe, want: false},
		{name: "铁镐钻石矿", block: 56, item: itemIronPickaxe, want: true},
		{name: "金镐钻石矿", block: 56, item: itemGoldPickaxe, want: false},
		{name: "铁镐黑曜石", block: 49, item: itemIronPickaxe, want: false},
		{name: "钻石镐黑曜石", block: 49, item: itemDiamondPickaxe, want: true},
		{name: "木斧石头", block: 1, item: itemWoodenAxe, want: false},
		{name: "空手煤矿", block: 16, item: 0, want: false},
		{name: "空手泥土", block: 3, item: 0, want: true},
		{name: "基岩", block: 7, item: itemDiamondPickaxe, want: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var current harvestCase = cases[index]
		var got bool = canHarvest(miningInfoFor(current.block), toolInfoFor(itemOf(current.item)))
		if got != current.want {
			test.Errorf("%s: got %v want %v", current.name, got, current.want)
		}
	}
}
