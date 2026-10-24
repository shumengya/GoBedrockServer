package server

import "testing"

import "go-mc-server/internal/protocol"

func planks(count byte) protocol.Item {
	return protocol.Item{ID: 5, Count: count}
}

// 合成：扣掉材料、产物进背包，两步在同一把锁里完成。
func TestCraftConsumesAndGrants(test *testing.T) {
	var player *Player = newTestPlayer(planks(4))
	var leftovers, crafted = player.craft(
		[]protocol.Item{{ID: 5, Count: 1}, {ID: 5, Count: 1}},
		[]protocol.Item{{ID: 280, Count: 4}},
	)
	if !crafted || len(leftovers) != 0 {
		test.Fatalf("应合成成功且无溢出: crafted=%v leftovers=%v", crafted, leftovers)
	}
	if player.Inventory[0].Count != 2 || player.HeldItem.Count != 2 {
		test.Fatalf("应扣 2 块木板: 背包=%d 手持=%d", player.Inventory[0].Count, player.HeldItem.Count)
	}
	if countMatching(player.Inventory, 280, 0) != 4 {
		test.Fatal("应得到 4 根木棍")
	}
}

// 材料不够整笔作废：不能扣一半料、也不能凭空给产物。
func TestCraftRejectsInsufficientMaterials(test *testing.T) {
	var player *Player = newTestPlayer(planks(1))
	var leftovers, crafted = player.craft(
		[]protocol.Item{{ID: 5, Count: 1}, {ID: 5, Count: 1}},
		[]protocol.Item{{ID: 280, Count: 4}},
	)
	if crafted || len(leftovers) != 0 {
		test.Fatal("只有 1 块木板，合成需要 2 块，必须拒绝")
	}
	if player.Inventory[0].Count != 1 || countMatching(player.Inventory, 280, 0) != 0 {
		test.Fatal("拒绝时背包不该有任何变化")
	}
	var mixed *Player = newTestPlayer(planks(8))
	_, crafted = mixed.craft(
		[]protocol.Item{{ID: 5, Count: 1}, {ID: 17, Count: 1}},
		[]protocol.Item{{ID: 280, Count: 1}},
	)
	if crafted || mixed.Inventory[0].Count != 8 {
		test.Fatal("缺的那一种料会让整笔作废，已有的料也不能被先扣掉")
	}
}

// meta 不同就是不同的料：云杉木板不能顶橡木板。
func TestCraftMatchesExactMeta(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: 5, Meta: 1, Count: 4})
	var _, crafted = player.craft([]protocol.Item{{ID: 5, Meta: 0, Count: 1}}, []protocol.Item{{ID: 280, Count: 1}})
	if crafted {
		test.Fatal("背包里只有云杉木板，不能拿来合成要橡木板的配方")
	}
}

// 背包满了：材料正好吃掉一整格就能腾出位置放产物；腾不出来的部分溢出，由调用方丢到地上。
func TestCraftOverflow(test *testing.T) {
	var full *Player = newTestPlayer(planks(1))
	var index int = 1
	for index = 1; index < len(full.Inventory); index++ {
		full.Inventory[index] = protocol.Item{ID: 1, Count: 64}
	}
	var leftovers, crafted = full.craft([]protocol.Item{{ID: 5, Count: 1}}, []protocol.Item{{ID: 280, Count: 4}})
	if !crafted || len(leftovers) != 0 || full.Inventory[0].ID != 280 {
		test.Fatalf("木板那一格被吃空，产物应放进去: crafted=%v 溢出=%v 槽0=%+v", crafted, leftovers, full.Inventory[0])
	}
	var tight *Player = newTestPlayer(planks(8))
	for index = 1; index < len(tight.Inventory); index++ {
		tight.Inventory[index] = protocol.Item{ID: 1, Count: 64}
	}
	leftovers, crafted = tight.craft([]protocol.Item{{ID: 5, Count: 1}}, []protocol.Item{{ID: 280, Count: 4}})
	if !crafted || len(leftovers) != 1 || leftovers[0].ID != 280 || leftovers[0].Count != 4 {
		test.Fatalf("没有空格，产物应整个溢出: crafted=%v 溢出=%v", crafted, leftovers)
	}
	if tight.Inventory[0].Count != 7 {
		test.Fatal("溢出时材料仍然要扣")
	}
}

// 蛋糕这类多产物配方：每个产物都要给。
func TestCraftMultipleOutputs(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: 92, Count: 1})
	var _, crafted = player.craft(
		[]protocol.Item{{ID: 92, Count: 1}},
		[]protocol.Item{{ID: 354, Count: 1}, {ID: 325, Count: 3}},
	)
	if !crafted || countMatching(player.Inventory, 354, 0) != 1 || countMatching(player.Inventory, 325, 0) != 3 {
		test.Fatal("两个产物都应入包")
	}
}

func TestCraftRejectsDeadAndEmpty(test *testing.T) {
	var dead *Player = newTestPlayer(planks(4))
	dead.combat.Dead = true
	var _, crafted = dead.craft([]protocol.Item{{ID: 5, Count: 1}}, []protocol.Item{{ID: 280, Count: 1}})
	if crafted {
		test.Fatal("死人不能合成")
	}
	var alive *Player = newTestPlayer(planks(4))
	_, crafted = alive.craft(nil, []protocol.Item{{ID: 280, Count: 1}})
	if crafted {
		test.Fatal("不扣任何料的「合成」等于凭空造物，必须拒绝")
	}
}

func TestTallyMaterialsMergesSameKind(test *testing.T) {
	var needs []materialNeed = tallyMaterials([]protocol.Item{
		{ID: 5, Meta: 0, Count: 1},
		{ID: 5, Meta: 0, Count: 1},
		{ID: 5, Meta: 1, Count: 1},
	})
	if len(needs) != 2 || needs[0].Count != 2 || needs[1].Count != 1 {
		test.Fatalf("同 id 同 meta 要合并: %+v", needs)
	}
}
