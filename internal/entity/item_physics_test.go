package entity

import "testing"

func TestCanPickupItemMatchesPocketMineExpand(t *testing.T) {
	px, py, pz := float32(0), float32(5), float32(0)
	if !CanPickupItem(px, py, pz, 0, 5, 0) {
		t.Fatal("item at feet should be pickupable")
	}
	if CanPickupItem(px, py, pz, 2, 5, 0) {
		t.Fatal("item 2 blocks away should be outside pickup radius")
	}
	if !CanPickupItem(px, py, pz, 1.2, 5, 0) {
		t.Fatal("item 1.2 blocks away should still be inside radius")
	}
	// 1.1 客户端有时上报眼睛高度；地上的物品仍应能捡。
	if !CanPickupItem(0, 5+1.62, 0, 0, 5, 0) {
		t.Fatal("item on ground should be pickupable when player Y is eye height")
	}
	if CanPickupItem(px, py, pz, 0, 9, 0) {
		t.Fatal("item 4 blocks above feet should not pickup")
	}
}

func TestStepDroppedItemLandsOnGround(t *testing.T) {
	e := &Instance{Y: 6.3, SpeedZ: 0.4}
	solid := func(x, y, z float32) bool {
		return y < 5
	}
	for i := 0; i < 80; i++ {
		StepDroppedItem(e, solid)
	}
	if !e.OnGround {
		t.Fatal("item should land")
	}
	if e.Y < 4.95 || e.Y > 5.15 {
		t.Fatalf("land y=%v want ~5", e.Y)
	}
	if e.Z <= 0 {
		t.Fatalf("should have thrown +Z, z=%v", e.Z)
	}
	if e.Z > 8 {
		t.Fatalf("throw went too far z=%v", e.Z)
	}
	landedY := e.Y
	for i := 0; i < 20; i++ {
		StepDroppedItem(e, solid)
	}
	if !e.OnGround || e.SpeedY != 0 || e.SpeedZ != 0 {
		t.Fatalf("should stay still on ground y=%v speed=(%v,%v,%v)", e.Y, e.SpeedX, e.SpeedY, e.SpeedZ)
	}
	if abs32(e.Y-landedY) > 0.001 {
		t.Fatalf("must not bounce: y %v -> %v", landedY, e.Y)
	}
}
