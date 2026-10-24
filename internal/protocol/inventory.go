package protocol

// PocketMine 1.1 PlayerInventory: 36 real slots, then 9 dummy air so PE
// shows the full inventory. Hotbar mapping values are inventoryIndex + 9.
const (
	PlayerInventorySize = 36
	PlayerHotbarSize    = 9

	// Window IDs we assign (0 is the player inventory).
	WindowWorkbench = 1
	WindowChest     = 2

	// InventoryNetworkIds (PocketMine protocol/types/InventoryNetworkIds.php).
	InventoryTypeContainer = 0
	InventoryTypeWorkbench = 1

	ChestSize = 27
)

// NetworkPlayerInventory pads 36 server slots to the 45-slot PE payload.
func NetworkPlayerInventory(inv []Item) []Item {
	out := make([]Item, PlayerInventorySize+PlayerHotbarSize)
	copy(out, inv)
	return out
}

// DefaultHotbarMapping links hotbar i to inventory slot i (encoded as i+9).
func DefaultHotbarMapping() []int32 {
	h := make([]int32, PlayerHotbarSize)
	for i := int32(0); i < PlayerHotbarSize; i++ {
		h[i] = i + PlayerHotbarSize
	}
	return h
}

// InventorySlotFromWindow maps a window-0 ContainerSetSlot index to 0-35.
// Dummy slots 36-44 are ignored.
func InventorySlotFromWindow(slot int32) int {
	s := int(slot)
	if s < 0 || s >= PlayerInventorySize {
		return -1
	}
	return s
}

// InventorySlotFromEquipment maps MobEquipment inventorySlot to 0-35.
// The client usually sends 9-44 (raw + hotbar size).
func InventorySlotFromEquipment(networkSlot byte) int {
	s := int(networkSlot)
	if s >= PlayerHotbarSize && s < PlayerInventorySize+PlayerHotbarSize {
		return s - PlayerHotbarSize
	}
	if s < PlayerHotbarSize {
		return s
	}
	return -1
}
