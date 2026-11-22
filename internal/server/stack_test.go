package server

import (
	"testing"

	"go-mc-server/internal/blockrule"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
)

// 验证容量预测与实际入包一致：先补同类物品，再开新格，剩下的仍留给调用方。
func TestStackLimitsAtPickup(test *testing.T) {
	for _, example := range []struct {
		id    int32
		limit byte
	}{
		{332, 16}, {344, 16}, {359, 1}, {298, 1}, {444, 1}, {1, 64},
	} {
		var player = &Player{Inventory: make([]protocol.Item, 2)}
		player.Inventory[0] = protocol.Item{ID: example.id, Count: example.limit}
		var incoming = protocol.Item{ID: example.id, Count: example.limit + 1}
		if player.CanAddItem(incoming) {
			test.Fatalf("期望 %d 放不下整组，实际可放", example.id)
		}
		var rest, placed = player.AddItem(incoming)
		if !placed || rest.Count != 1 || player.Inventory[1].Count != example.limit {
			test.Fatalf("期望 %d 每格 %d 且余 1，实际 %+v、余 %+v", example.id, example.limit, player.Inventory, rest)
		}
	}
}

// 从箱子、背包和手持槽三个入口写入超限物品时，原槽位都应保留。
func TestSlotStackLimit(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	var valid = protocol.Item{ID: 344, Count: 16}
	var excess = protocol.Item{ID: 344, Count: 17}
	containerTestWrite(fixture, player, protocol.WindowContainer, 0, valid)
	containerTestWrite(fixture, player, protocol.WindowContainer, 0, excess)
	player.ApplyInventorySlot(0, valid)
	fixture.srv.handleContainerSetSlot(player.Session, &packets.ContainerSetSlotPacket{Item: excess})
	fixture.srv.handleMobEquipment(player.Session, &packets.MobEquipmentPacket{InventorySlot: 9, Item: excess})
	if containerTestSlot(fixture, 0).Count != 16 || player.Inventory[0].Count != 16 || player.HeldItem.Count != 16 {
		test.Fatalf("期望三个槽位均为 16，实际 %v、%v、%v", containerTestSlot(fixture, 0), player.Inventory[0], player.HeldItem)
	}
}
