package server

import (
	"bytes"
	"testing"

	"go-mc-server/internal/blockrule"
	"go-mc-server/internal/crafting"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/world"
)

// 桌面端先取出材料再发空输入；错报另一个已知配方号时仍按产物和实际材料决定。
func TestDesktopCraftThroughSlots(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var plank = protocol.Item{ID: 5, Meta: 3, Count: 2}
	var sticks = protocol.Item{ID: 280, Count: 4}
	player.ApplyInventorySlot(15, plank)
	var wrongID, err = crafting.ParseUUID("bd882bfe-73b4-3b1e-851e-b5e4a55e568d")
	if err != nil {
		test.Fatal(err)
	}
	var craft = &packets.CraftingEventPacket{RecipeID: wrongID, Output: []protocol.Item{sticks}}
	fixture.srv.handleCraftingEvent(player.Session, craft)
	if player.Inventory[15].Count != 2 || len(player.floating) != 0 {
		test.Fatal("期望未入格的材料留在背包")
	}
	fixture.srv.handleContainerSetSlot(player.Session, &packets.ContainerSetSlotPacket{Slot: 15})
	fixture.srv.handleCraftingEvent(player.Session, craft)
	fixture.srv.handleCraftingEvent(player.Session, craft)
	fixture.srv.handleContainerSetSlot(player.Session, &packets.ContainerSetSlotPacket{Slot: 16, Item: sticks})
	if len(player.floating) != 0 || !player.Inventory[15].IsAir() || player.Inventory[16].ID != 280 || player.Inventory[16].Count != 4 {
		test.Fatalf("期望两木板只产出四根木棍，实际 %v / %v", player.floating, player.Inventory[15:17])
	}
	var eight = protocol.Item{ID: 5, Count: 8}
	player.ApplyInventorySlot(15, eight)
	fixture.srv.handleContainerSetSlot(player.Session, &packets.ContainerSetSlotPacket{Slot: 15})
	craft.Output = []protocol.Item{{ID: 54, Count: 1}}
	fixture.srv.handleCraftingEvent(player.Session, craft)
	if len(player.floating) != 1 || player.floating[0].ID != 5 || player.floating[0].Count != 8 {
		test.Fatalf("期望大配方缺工作台时保留材料，实际 %v", player.floating)
	}
	placeTestPut(test, fixture, 3, 64, 0, world.BlockCraftingTable, 0)
	fixture.srv.openWorkbench(player, 3, 64, 0)
	fixture.srv.handleCraftingEvent(player.Session, craft)
	if len(player.floating) != 1 || player.floating[0].ID != 54 {
		test.Fatalf("期望工作台可用时得到箱子，实际 %v", player.floating)
	}
}

// 从背包到箱子再取回，全程保留耐久和 NBT；断线重进、关窗或死亡只归还一次。
func TestFloatingContainerLifecycle(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	var stack = protocol.Item{ID: 257, Meta: 17, Count: 1, NBT: []byte{10, 0, 0, 0}}
	player.ApplyInventorySlot(20, stack)
	var write = func(window byte, slot int32, value protocol.Item) {
		fixture.srv.handleContainerSetSlot(player.Session, &packets.ContainerSetSlotPacket{WindowID: window, Slot: slot, Item: value})
	}
	write(0, 20, protocol.Air())
	var altered = stack
	altered.NBT = nil
	write(protocol.WindowContainer, 0, altered)
	if !containerTestSlot(fixture, 0).IsAir() {
		test.Fatal("期望标签不匹配时拒绝移入")
	}
	write(protocol.WindowContainer, 0, stack)
	if !sameStack(containerTestSlot(fixture, 0), stack) || len(player.floating) != 0 {
		test.Fatal("期望原物品已放入箱子")
	}
	write(protocol.WindowContainer, 0, protocol.Air())
	var restored = newTestPlayer(protocol.Air())
	restored.ApplySave(player.Snapshot("world"))
	fixture.srv.returnFloating(restored)
	if len(restored.floating) != 0 || countMatching(restored.Inventory, 257, 17) != 1 {
		test.Fatal("期望重连后归还一把原耐久的镐")
	}
	var died = restored.takeAllItems()
	var tagged bool
	for _, stack := range died {
		if stack.ID == 257 {
			tagged = bytes.Equal(stack.NBT, []byte{10, 0, 0, 0})
		}
	}
	if !tagged {
		test.Fatal("期望死亡掉落保留 NBT")
	}
	fixture.srv.handleContainerClose(player.Session, &packets.ContainerClosePacket{WindowID: protocol.WindowContainer})
	if len(player.floating) != 0 || countMatching(player.Inventory, 257, 17) != 1 || !player.payForDrop(stack) || player.payForDrop(stack) {
		test.Fatal("期望关窗后物品可且仅可丢弃一次")
	}
}
