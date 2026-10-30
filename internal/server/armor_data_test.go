package server

import "testing"

import "go-mc-server/internal/protocol"

type reduceCase struct {
	Name   string
	Amount float32
	Points int32
	Expect float32
}

// 减伤 = 伤害 × (1 - 0.04 × 护甲点数)，再四舍五入。预期值照公式手算：
// 7×0.4=2.8→3，8×0.2=1.6→2，1×0.2=0.2→0，5×0.72=3.6→4，10×0.96=9.6→10。
// 点数多到系数为负时按 0 算，不能把伤害变成负数（4×(1-1.2) 不夹住就会四舍五入成 -1）。
func TestReduceByArmor(test *testing.T) {
	var cases []reduceCase = []reduceCase{
		{Name: "没有护甲原样", Amount: 7, Points: 0, Expect: 7},
		{Name: "铁套 15 点挨铁剑", Amount: 7, Points: 15, Expect: 3},
		{Name: "钻石套 20 点挨钻石剑", Amount: 8, Points: 20, Expect: 2},
		{Name: "钻石套挨拳头被完全挡住", Amount: 1, Points: 20, Expect: 0},
		{Name: "皮革套 7 点挨木剑", Amount: 5, Points: 7, Expect: 4},
		{Name: "1 点护甲 1 点伤害四舍五入回 1", Amount: 1, Points: 1, Expect: 1},
		{Name: "1 点护甲 10 点伤害", Amount: 10, Points: 1, Expect: 10},
		{Name: "10 点护甲 3 点伤害", Amount: 3, Points: 10, Expect: 2},
		{Name: "刚好 25 点系数归零", Amount: 4, Points: 25, Expect: 0},
		{Name: "超过 25 点不能变成负伤害", Amount: 4, Points: 30, Expect: 0},
		{Name: "负点数当没有护甲", Amount: 6, Points: -3, Expect: 6},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item reduceCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var got float32 = reduceByArmor(item.Amount, item.Points)
			if got != item.Expect {
				sub.Fatalf("%v 点伤害、%d 点护甲：期望 %v，实际 %v", item.Amount, item.Points, item.Expect, got)
			}
		})
	}
}

type armorSetCase struct {
	Name   string
	First  int32
	Points int32
}

// 盔甲表：21 件，物品号 298~317 每四件一套，顺序都是头盔、胸甲、护腿、靴子，所以槽位 = (号 - 298) % 4。
// 整套的护甲点数照参考端：皮革 7，锁链 11，铁 15，钻石 20，金 10。
// 参考端的锁链头盔和金头盔只有 1 点，我印象里原版是 2，没有资料可对照，先按参考端。
func TestArmorTable(test *testing.T) {
	if len(armorTable) != 21 {
		test.Fatalf("盔甲表应有 21 件，实际 %d", len(armorTable))
	}
	var id int32 = 0
	for id = 298; id <= 317; id++ {
		var info, found = armorInfoFor(protocol.Item{ID: id, Count: 1})
		if !found {
			test.Fatalf("物品 %d 应在盔甲表里", id)
		}
		if info.Slot != int((id-298)%4) {
			test.Fatalf("物品 %d 应在槽 %d，实际 %d", id, (id-298)%4, info.Slot)
		}
		if info.MaxDurability <= 0 || info.Defense <= 0 {
			test.Fatalf("物品 %d 的耐久和护甲点数都应大于 0，实际 %+v", id, info)
		}
	}
	var elytra, elytraFound = armorInfoFor(protocol.Item{ID: 444, Count: 1})
	if !elytraFound || elytra.Slot != armorSlotChestplate || elytra.Defense != 0 || elytra.MaxDurability != 431 {
		test.Fatalf("鞘翅占胸甲槽、没有护甲点数、耐久 431，实际 找到=%v %+v", elytraFound, elytra)
	}
	var sets []armorSetCase = []armorSetCase{
		{Name: "皮革", First: 298, Points: 7},
		{Name: "锁链", First: 302, Points: 11},
		{Name: "铁", First: 306, Points: 15},
		{Name: "钻石", First: 310, Points: 20},
		{Name: "金", First: 314, Points: 10},
	}
	var index int = 0
	for index = 0; index < len(sets); index++ {
		var set armorSetCase = sets[index]
		var armor [protocol.ArmorSize]protocol.Item
		var slot int = 0
		for slot = 0; slot < protocol.ArmorSize; slot++ {
			armor[slot] = protocol.Item{ID: set.First + int32(slot), Count: 1}
		}
		if armorPointsOf(armor) != set.Points {
			test.Fatalf("%s套期望 %d 点，实际 %d", set.Name, set.Points, armorPointsOf(armor))
		}
	}
}

// 不是盔甲、空物品都查不到。
func TestArmorInfoForNonArmor(test *testing.T) {
	var _, swordFound = armorInfoFor(protocol.Item{ID: 267, Count: 1})
	var _, airFound = armorInfoFor(protocol.Air())
	var _, emptyFound = armorInfoFor(protocol.Item{ID: 306, Count: 0})
	if swordFound || airFound || emptyFound {
		test.Fatalf("剑、空气、数量为 0 的都不是盔甲，实际 剑=%v 空气=%v 数量0=%v", swordFound, airFound, emptyFound)
	}
}

type acceptCase struct {
	Name   string
	Slot   int
	Item   protocol.Item
	Expect bool
}

// 能放进盔甲槽的：空的；这个槽该穿的盔甲，数量恰好 1，损耗值在 [0, 最大耐久) 里。
func TestArmorAcceptable(test *testing.T) {
	var cases []acceptCase = []acceptCase{
		{Name: "空的放哪都行", Slot: 2, Item: protocol.Air(), Expect: true},
		{Name: "铁头盔放头盔槽", Slot: 0, Item: protocol.Item{ID: 306, Count: 1}, Expect: true},
		{Name: "铁头盔放胸甲槽", Slot: 1, Item: protocol.Item{ID: 306, Count: 1}, Expect: false},
		{Name: "钻石靴子放靴子槽", Slot: 3, Item: protocol.Item{ID: 313, Count: 1}, Expect: true},
		{Name: "钻石靴子放头盔槽", Slot: 0, Item: protocol.Item{ID: 313, Count: 1}, Expect: false},
		{Name: "鞘翅放胸甲槽", Slot: 1, Item: protocol.Item{ID: 444, Count: 1}, Expect: true},
		{Name: "鞘翅放头盔槽", Slot: 0, Item: protocol.Item{ID: 444, Count: 1}, Expect: false},
		{Name: "两件叠在一起", Slot: 0, Item: protocol.Item{ID: 306, Count: 2}, Expect: false},
		{Name: "石头", Slot: 0, Item: protocol.Item{ID: 1, Count: 1}, Expect: false},
		{Name: "铁剑", Slot: 0, Item: protocol.Item{ID: 267, Count: 1}, Expect: false},
		{Name: "损耗值刚好最后一点", Slot: 0, Item: protocol.Item{ID: 306, Meta: 165, Count: 1}, Expect: true},
		{Name: "损耗值等于最大耐久", Slot: 0, Item: protocol.Item{ID: 306, Meta: 166, Count: 1}, Expect: false},
		{Name: "损耗值是负数", Slot: 0, Item: protocol.Item{ID: 306, Meta: -1, Count: 1}, Expect: false},
		{Name: "数量 0 的当空物品", Slot: 0, Item: protocol.Item{ID: 306, Count: 0}, Expect: true},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item acceptCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var got bool = armorAcceptable(item.Slot, item.Item)
			if got != item.Expect {
				sub.Fatalf("期望 %v，实际 %v", item.Expect, got)
			}
		})
	}
}

// 护甲点数只算放在对的槽里的：钻石靴子塞进头盔槽不提供护甲；鞘翅 0 点；空的 0 点。
func TestArmorPointsOfSkipsMisplaced(test *testing.T) {
	var empty [protocol.ArmorSize]protocol.Item
	if armorPointsOf(empty) != 0 {
		test.Fatalf("空的应是 0 点，实际 %d", armorPointsOf(empty))
	}
	var partial [protocol.ArmorSize]protocol.Item
	partial[armorSlotHelmet] = protocol.Item{ID: 306, Count: 1}
	partial[armorSlotBoots] = protocol.Item{ID: 309, Count: 1}
	if armorPointsOf(partial) != 4 {
		test.Fatalf("铁头盔加铁靴期望 4 点，实际 %d", armorPointsOf(partial))
	}
	var misplaced [protocol.ArmorSize]protocol.Item
	misplaced[armorSlotHelmet] = protocol.Item{ID: 313, Count: 1}
	misplaced[armorSlotChestplate] = protocol.Item{ID: 444, Count: 1}
	if armorPointsOf(misplaced) != 0 {
		test.Fatalf("放错槽的和鞘翅都不提供护甲，实际 %d", armorPointsOf(misplaced))
	}
}

type armorCauseCase struct {
	Name    string
	Cause   DamageCause
	Reduces bool
	Wears   bool
}

// 吃护甲减伤的来源：玩家攻击、仙人掌、岩浆；磨损盔甲的只有玩家攻击。
// 着火持续掉血、摔落、虚空、窒息、溺水、饥饿既不减伤也不磨损。
func TestArmorCauses(test *testing.T) {
	var cases []armorCauseCase = []armorCauseCase{
		{Name: "玩家攻击", Cause: DamageCausePlayer, Reduces: true, Wears: true},
		{Name: "仙人掌", Cause: DamageCauseCactus, Reduces: true, Wears: false},
		{Name: "岩浆", Cause: DamageCauseLava, Reduces: true, Wears: false},
		{Name: "着火", Cause: DamageCauseFire, Reduces: false, Wears: false},
		{Name: "摔落", Cause: DamageCauseFall, Reduces: false, Wears: false},
		{Name: "虚空", Cause: DamageCauseVoid, Reduces: false, Wears: false},
		{Name: "窒息", Cause: DamageCauseSuffocation, Reduces: false, Wears: false},
		{Name: "溺水", Cause: DamageCauseDrowning, Reduces: false, Wears: false},
		{Name: "饥饿", Cause: DamageCauseStarvation, Reduces: false, Wears: false},
		{Name: "未知", Cause: DamageCauseGeneric, Reduces: false, Wears: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item armorCauseCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			if armorReducesDamage(item.Cause) != item.Reduces {
				sub.Fatalf("减伤期望 %v，实际 %v", item.Reduces, armorReducesDamage(item.Cause))
			}
			if armorWearsOnHit(item.Cause) != item.Wears {
				sub.Fatalf("磨损期望 %v，实际 %v", item.Wears, armorWearsOnHit(item.Cause))
			}
		})
	}
}
