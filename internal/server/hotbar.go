package server

import "slices"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 映射存网络槽号（实际槽 + 9）；-1 是空的快捷键，nil 沿用初始的九格。
func (player *Player) heldInventorySlotLocked() int {
	if player.hotbar == nil {
		return int(player.HeldSlot)
	}
	var linked = player.hotbar[player.HeldSlot]
	if linked < 0 {
		return -1
	}
	return int(linked) - protocol.PlayerHotbarSize
}

func (player *Player) hotbarSnapshotLocked() []int32 {
	if player.hotbar == nil {
		return protocol.DefaultHotbarMapping()
	}
	return slices.Clone(player.hotbar)
}

func (player *Player) equipmentLocked() *packets.MobEquipmentPacket {
	var slot = player.heldInventorySlotLocked()
	var wire byte = 255
	if slot >= 0 {
		wire = byte(slot + protocol.PlayerHotbarSize)
	}
	return &packets.MobEquipmentPacket{EntityRuntimeID: player.EntityRuntimeID, Item: player.HeldItem,
		InventorySlot: wire, HotbarSlot: player.HeldSlot}
}

func (player *Player) selectHotbar(selected byte, wire byte) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.combat.Dead || selected >= protocol.PlayerHotbarSize {
		return false
	}
	var slot = protocol.InventorySlotFromEquipment(wire)
	if (slot < 0 && wire != 255) || slot >= len(player.Inventory) {
		return false
	}
	var linked int32 = -1
	if slot >= 0 {
		linked = int32(slot + protocol.PlayerHotbarSize)
	}
	if player.hotbar == nil {
		player.hotbar = protocol.DefaultHotbarMapping()
	}
	if linked >= 0 {
		if previous := slices.Index(player.hotbar, linked); previous >= 0 {
			player.hotbar[previous] = player.hotbar[selected]
		}
	}
	player.hotbar[selected] = linked
	player.HeldSlot = selected
	player.HeldItem = protocol.Air()
	if slot >= 0 {
		player.HeldItem = player.Inventory[slot]
	}
	return true
}
