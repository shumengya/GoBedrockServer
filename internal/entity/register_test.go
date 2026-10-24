package entity

import (
	"testing"

	"go-mc-server/internal/protocol"
)

func TestFromString(t *testing.T) {
	typ, err := FromString("zombie")
	if err != nil || typ.NetworkID != 32 {
		t.Fatalf("zombie -> %+v err=%v", typ, err)
	}
	typ, err = FromString("minecraft:zombie")
	if err != nil || typ.NetworkID != 32 {
		t.Fatalf("minecraft:zombie -> %+v err=%v", typ, err)
	}
	typ, err = FromString("32")
	if err != nil || typ.NetworkID != 32 {
		t.Fatalf("32 -> %+v err=%v", typ, err)
	}
	if _, err := FromString("nope"); err == nil {
		t.Fatal("unknown name must fail")
	}
	typ, err = FromString("painting")
	if err != nil || typ.CanSummon {
		t.Fatalf("painting should register but not summon: %+v err=%v", typ, err)
	}
	typ, err = FromString("item")
	if err != nil || typ.NetworkID != NetworkIDItem || typ.CanSummon {
		t.Fatalf("dropped item should register but not summon: %+v err=%v", typ, err)
	}
}

func TestDroppedItemInstance(t *testing.T) {
	typ, ok := ByID(NetworkIDItem)
	if !ok {
		t.Fatal("item type missing")
	}
	e := &Instance{Type: typ, Item: protocol.Item{ID: 1, Count: 8}}
	if !e.IsDroppedItem() {
		t.Fatal("stone stack should be a dropped item")
	}
	e.Item = protocol.Air()
	if e.IsDroppedItem() {
		t.Fatal("air must not count as dropped item")
	}
}
