package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

type itemCase struct {
	Name     string
	ItemID   int32
	Expect   byte
	Placable bool
}

// ID 小于 256 的物品本身就是方块；种子、门、床、甘蔗的物品 ID 要查表；工具食物不是方块。
func TestBlockForItem(test *testing.T) {
	var cases []itemCase = []itemCase{
		{Name: "空气不能放", ItemID: 0, Expect: blockrule.BlockAir, Placable: false},
		{Name: "负数不能放", ItemID: -1, Expect: blockrule.BlockAir, Placable: false},
		{Name: "石头", ItemID: 1, Expect: blockrule.BlockStone, Placable: true},
		{Name: "木板", ItemID: 5, Expect: blockrule.BlockPlanks, Placable: true},
		{Name: "最大的方块 ID", ItemID: 255, Expect: 255, Placable: true},
		{Name: "铁铲不是方块", ItemID: 256, Expect: blockrule.BlockAir, Placable: false},
		{Name: "铁剑不是方块", ItemID: 267, Expect: blockrule.BlockAir, Placable: false},
		{Name: "染料不是方块", ItemID: 351, Expect: blockrule.BlockAir, Placable: false},
		{Name: "小麦种子", ItemID: 295, Expect: blockrule.BlockWheat, Placable: true},
		{Name: "甘蔗", ItemID: 338, Expect: blockrule.BlockSugarcane, Placable: true},
		{Name: "南瓜种子", ItemID: 361, Expect: blockrule.BlockPumpkinStem, Placable: true},
		{Name: "西瓜种子", ItemID: 362, Expect: blockrule.BlockMelonStem, Placable: true},
		{Name: "胡萝卜", ItemID: 391, Expect: blockrule.BlockCarrot, Placable: true},
		{Name: "马铃薯", ItemID: 392, Expect: blockrule.BlockPotato, Placable: true},
		{Name: "甜菜种子", ItemID: 458, Expect: blockrule.BlockBeetroot, Placable: true},
		{Name: "橡木门", ItemID: 324, Expect: blockrule.BlockWoodDoor, Placable: true},
		{Name: "铁门", ItemID: 330, Expect: blockrule.BlockIronDoor, Placable: true},
		{Name: "云杉门", ItemID: 427, Expect: blockrule.BlockSpruceDoor, Placable: true},
		{Name: "白桦门", ItemID: 428, Expect: blockrule.BlockBirchDoor, Placable: true},
		{Name: "丛林门", ItemID: 429, Expect: blockrule.BlockJungleDoor, Placable: true},
		{Name: "金合欢门", ItemID: 430, Expect: blockrule.BlockAcaciaDoor, Placable: true},
		{Name: "深色橡木门", ItemID: 431, Expect: blockrule.BlockDarkOakDoor, Placable: true},
		{Name: "床", ItemID: 355, Expect: blockrule.BlockBed, Placable: true},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item itemCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var actual, ok = blockrule.BlockForItem(item.ItemID)
			if ok != item.Placable {
				sub.Fatalf("物品 %d 可放置期望 %v，实际 %v", item.ItemID, item.Placable, ok)
			}
			if actual != item.Expect {
				sub.Fatalf("物品 %d 期望方块 %d，实际 %d", item.ItemID, item.Expect, actual)
			}
		})
	}
}
