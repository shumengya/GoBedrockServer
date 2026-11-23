package server

import "testing"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 客户端选择的是背包格；写在包里的物品不改变背包，实际消耗与回包都跟随重映射。
func TestEquipmentUsesMappedInventory(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var original = protocol.Item{ID: 5, Meta: 2, Count: 4}
	player.ApplyInventorySlot(24, original)
	fixture.srv.handleMobEquipment(player.Session, &packets.MobEquipmentPacket{
		HotbarSlot: 3, InventorySlot: 33, Item: protocol.Item{ID: 264, Count: 64},
	})
	if player.HeldItem.ID != original.ID || player.HeldItem.Meta != original.Meta || player.HeldItem.Count != original.Count {
		test.Fatalf("期望选中背包原有的木板，实际 %+v", player.HeldItem)
	}
	var slot, remaining, consumed = player.ConsumeHeld(1)
	var wire = player.equipmentLocked()
	if !consumed || slot != 24 || remaining.ID != 5 || remaining.Meta != 2 || remaining.Count != 3 ||
		wire.InventorySlot != 33 || wire.HotbarSlot != 3 || wire.Item.Count != 3 {
		test.Fatalf("期望消耗背包 24 的木板并同步快捷键 3，实际 %d、%+v、%+v", slot, remaining, wire)
	}
	var restored = newTestPlayer(protocol.Air())
	restored.ApplySave(player.Snapshot("world"))
	if restored.heldInventorySlotLocked() != 24 || restored.HeldItem.Count != 3 {
		test.Fatalf("期望重连保留选中格和物品，实际 %d、%+v", restored.heldInventorySlotLocked(), restored.HeldItem)
	}
	restored.selectHotbar(3, 10)
	if restored.hotbar[1] != 33 || restored.hotbar[3] != 10 {
		test.Fatalf("期望已绑定的格子交换，实际 %v", restored.hotbar)
	}
	restored.selectHotbar(3, 255)
	if _, _, changed := restored.ConsumeHeld(1); changed || !restored.HeldItem.IsAir() || restored.equipmentLocked().InventorySlot != 255 {
		test.Fatal("期望空快捷键不能消耗背包物品")
	}
}
