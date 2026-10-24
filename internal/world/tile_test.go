package world

import (
	"bytes"
	"testing"

	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/nbt"
)

func TestNetworkSerializeAppendsTileNBT(t *testing.T) {
	w := New("test")
	plain := w.NetworkSerialize(0, 0)
	chest := NewChestTile(1, 5, 2)
	w.SetTile(chest)
	with := w.NetworkSerialize(0, 0)
	if !bytes.HasPrefix(with, plain) {
		t.Fatal("tile nbt must be appended after column payload")
	}
	spawn := nbt.SpawnChest(1, 5, 2)
	if !bytes.HasSuffix(with, spawn) {
		t.Fatalf("missing chest spawn nbt suffix\n got ...%x\nwant %x", with[len(with)-len(spawn):], spawn)
	}
	other := w.NetworkSerialize(1, 0)
	if bytes.Contains(other, spawn) {
		t.Fatal("tile nbt must only attach to its column")
	}
}

func TestChestSlotRoundtrip(t *testing.T) {
	w := New("test")
	w.SetTile(NewChestTile(3, 4, 5))
	item := protocol.Item{ID: 1, Count: 8}
	if !w.SetChestSlot(3, 4, 5, 2, item) {
		t.Fatal("set slot")
	}
	got, ok := w.TileAt(3, 4, 5)
	if !ok || got.Items[2].ID != 1 || got.Items[2].Count != 8 {
		t.Fatalf("tile %+v", got)
	}
	if _, ok := w.RemoveTile(3, 4, 5); !ok {
		t.Fatal("remove")
	}
	if _, ok := w.TileAt(3, 4, 5); ok {
		t.Fatal("tile should be gone")
	}
}
