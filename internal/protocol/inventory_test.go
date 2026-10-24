package protocol

import "testing"

func TestNetworkPlayerInventoryAndHotbar(t *testing.T) {
	inv := make([]Item, PlayerInventorySize)
	inv[0] = Item{ID: 1, Count: 64}
	inv[8] = Item{ID: 35, Count: 63}
	slots := NetworkPlayerInventory(inv)
	if len(slots) != PlayerInventorySize+PlayerHotbarSize {
		t.Fatalf("len=%d", len(slots))
	}
	if slots[0].ID != 1 || slots[8].Count != 63 {
		t.Fatalf("copied slots %+v %+v", slots[0], slots[8])
	}
	if !slots[36].IsAir() || !slots[44].IsAir() {
		t.Fatal("dummy tail must be air")
	}
	h := DefaultHotbarMapping()
	if len(h) != 9 || h[0] != 9 || h[8] != 17 {
		t.Fatalf("hotbar=%v", h)
	}
}

func TestInventorySlotMapping(t *testing.T) {
	if InventorySlotFromWindow(8) != 8 {
		t.Fatal("window slot 8")
	}
	if InventorySlotFromWindow(36) != -1 {
		t.Fatal("dummy window slot must be ignored")
	}
	if InventorySlotFromEquipment(17) != 8 {
		t.Fatal("equipment 17 is hotbar 8")
	}
	if InventorySlotFromEquipment(8) != 8 {
		t.Fatal("equipment 8 without offset")
	}
	if InventorySlotFromEquipment(255) != -1 {
		t.Fatal("empty-hand 255")
	}
}
