package server

import (
	"testing"

	"go-mc-server/internal/protocol"
)

func TestIsBodyColumn(t *testing.T) {
	px, py, pz := float32(1.30), float32(18.87), float32(0.50)

	if !IsBodyColumn(px, py, pz, 1, 18, 0) {
		t.Fatal("feet cell must be denied")
	}
	if !IsBodyColumn(px, py, pz, 1, 19, 0) {
		t.Fatal("head cell must be denied")
	}
	if !IsBodyColumn(px, py, pz, 1, 17, 0) {
		t.Fatal("cell below feet must be denied")
	}
	if IsBodyColumn(px, py, pz, 1, 16, 0) {
		t.Fatal("two below feet must stay placeable")
	}
	if IsBodyColumn(px, py, pz, 1, 20, 0) {
		t.Fatal("two above feet must stay placeable")
	}
	if IsBodyColumn(px, py, pz, 0, 18, 0) {
		t.Fatal("adjacent column must stay placeable")
	}
}

func TestApplyInventorySlot(t *testing.T) {
	p := &Player{
		Inventory: make([]protocol.Item, 36),
		HeldSlot:  8,
	}
	p.Inventory[8] = protocol.Item{ID: 35, Count: 64}
	if !p.ApplyInventorySlot(8, protocol.Item{ID: 35, Count: 63}) {
		t.Fatal("slot 8 should apply")
	}
	if p.Inventory[8].Count != 63 || p.HeldItem.Count != 63 {
		t.Fatalf("count=%d held=%d", p.Inventory[8].Count, p.HeldItem.Count)
	}
	if p.ApplyInventorySlot(99, protocol.Item{ID: 1, Count: 1}) {
		t.Fatal("out of range slot must be rejected")
	}
}

func TestConsumeHeld(t *testing.T) {
	p := &Player{
		Inventory: make([]protocol.Item, protocol.PlayerInventorySize),
		HeldSlot:  0,
		GameMode:  int32(protocol.GameModeSurvival),
	}
	p.Inventory[0] = protocol.Item{ID: 1, Count: 2}
	p.HeldItem = p.Inventory[0]
	slot, item, changed := p.ConsumeHeld(1)
	if !changed || slot != 0 || item.Count != 1 {
		t.Fatalf("first consume slot=%d count=%d changed=%v", slot, item.Count, changed)
	}
	_, item, changed = p.ConsumeHeld(1)
	if !changed || !item.IsAir() {
		t.Fatalf("second consume should empty the stack: %+v changed=%v", item, changed)
	}
	p.GameMode = int32(protocol.GameModeCreative)
	p.Inventory[0] = protocol.Item{ID: 1, Count: 64}
	p.HeldItem = p.Inventory[0]
	_, item, changed = p.ConsumeHeld(1)
	if changed || item.Count != 64 {
		t.Fatal("creative must not consume")
	}
}

func TestApplyGameMode(t *testing.T) {
	srv := &Server{}
	p := &Player{GameMode: protocol.GameModeSurvival}
	if !srv.applyGameMode(p, protocol.GameModeCreative) || p.GameMode != protocol.GameModeCreative {
		t.Fatal("survival -> creative")
	}
	if srv.applyGameMode(p, protocol.GameModeCreative) {
		t.Fatal("same mode must return false")
	}
	if srv.applyGameMode(p, protocol.GameModeAdventure) {
		t.Fatal("adventure must be rejected this round")
	}
	if !srv.applyGameMode(p, protocol.GameModeSurvival) || p.GameMode != protocol.GameModeSurvival {
		t.Fatal("creative -> survival")
	}
}

func TestAddItemMergeAndFill(t *testing.T) {
	p := &Player{Inventory: make([]protocol.Item, protocol.PlayerInventorySize)}
	p.Inventory[0] = protocol.Item{ID: 1, Count: 60}
	leftover, ok := p.AddItem(protocol.Item{ID: 1, Count: 10})
	if !ok || !leftover.IsAir() || p.Inventory[0].Count != 64 || p.Inventory[1].Count != 6 {
		t.Fatalf("merge leftover=%+v slot0=%+v slot1=%+v ok=%v", leftover, p.Inventory[0], p.Inventory[1], ok)
	}
	full := &Player{Inventory: make([]protocol.Item, protocol.PlayerInventorySize)}
	for i := range full.Inventory {
		full.Inventory[i] = protocol.Item{ID: 2, Count: 64}
	}
	leftover, ok = full.AddItem(protocol.Item{ID: 1, Count: 1})
	if ok || leftover.Count != 1 {
		t.Fatalf("full bag leftover=%+v ok=%v", leftover, ok)
	}
}

func TestCanAddAndRemoveItem(t *testing.T) {
	p := &Player{Inventory: make([]protocol.Item, protocol.PlayerInventorySize), HeldSlot: 0}
	p.Inventory[0] = protocol.Item{ID: 1, Count: 10}
	p.HeldItem = p.Inventory[0]
	if !p.CanAddItem(protocol.Item{ID: 1, Count: 54}) {
		t.Fatal("should fit remaining stack plus empty slots")
	}
	full := &Player{Inventory: make([]protocol.Item, protocol.PlayerInventorySize)}
	for i := range full.Inventory {
		full.Inventory[i] = protocol.Item{ID: 2, Count: 64}
	}
	if full.CanAddItem(protocol.Item{ID: 1, Count: 1}) {
		t.Fatal("full bag of other items cannot add")
	}
	if !p.RemoveItem(protocol.Item{ID: 1, Count: 3}) {
		t.Fatal("remove 3 from held")
	}
	if p.Inventory[0].Count != 7 || p.HeldItem.Count != 7 {
		t.Fatalf("held=%+v", p.Inventory[0])
	}
	if p.RemoveItem(protocol.Item{ID: 1, Count: 99}) {
		t.Fatal("cannot remove more than owned")
	}
	p.GameMode = int32(protocol.GameModeCreative)
	if !p.RemoveItem(protocol.Item{ID: 1, Count: 64}) {
		t.Fatal("creative remove should succeed")
	}
	if p.Inventory[0].Count != 7 {
		t.Fatal("creative must not change slots")
	}
}

func TestDropMotionAndOrigin(t *testing.T) {
	// yaw 0 = south (+Z), matching PocketMine getDirectionVector * 0.4
	sx, sy, sz := dropMotion(0, 0)
	if sx != 0 || sz <= 0 || sy != 0 {
		t.Fatalf("south throw motion=(%v,%v,%v) want (0,0,+0.4)", sx, sy, sz)
	}
	ox, oy, oz := dropOrigin(10, 64, 10)
	if ox != 10 || oy != 65.3 || oz != 10 {
		t.Fatalf("origin=(%v,%v,%v) want body Y+1.3, same XZ", ox, oy, oz)
	}
}
