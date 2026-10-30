package nbt

import (
	"bytes"
	"testing"
)

func TestSpawnChestNetworkNBT(t *testing.T) {
	got := SpawnChest(1, 2, 3)
	want := []byte{
		TagCompound, 0x00, // unnamed compound
		TagString, 0x02, 'i', 'd', 0x05, 'C', 'h', 'e', 's', 't',
		TagInt, 0x01, 'x', 0x02, // zigzag 1
		TagInt, 0x01, 'y', 0x04, // zigzag 2
		TagInt, 0x01, 'z', 0x06, // zigzag 3
		TagEnd,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("chest nbt\n got %x\nwant %x", got, want)
	}
}

// 末影箱的方块实体只有 id 和坐标，没有物品、没有配对字段：东西在玩家身上，不在方块里。
func TestSpawnEnderChestNetworkNBT(t *testing.T) {
	got := SpawnEnderChest(1, 2, 3)
	want := []byte{
		TagCompound, 0x00, // unnamed compound
		TagString, 0x02, 'i', 'd', 0x0a, 'E', 'n', 'd', 'e', 'r', 'C', 'h', 'e', 's', 't',
		TagInt, 0x01, 'x', 0x02, // zigzag 1
		TagInt, 0x01, 'y', 0x04, // zigzag 2
		TagInt, 0x01, 'z', 0x06, // zigzag 3
		TagEnd,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ender chest nbt\n got %x\nwant %x", got, want)
	}
}

func TestSpawnBedIncludesColor(t *testing.T) {
	got := SpawnBed(0, 64, 0, 14)
	if got[0] != TagCompound || got[len(got)-1] != TagEnd {
		t.Fatalf("wrapper: %x", got)
	}
	if !bytes.Contains(got, []byte{TagString, 0x02, 'i', 'd', 0x03, 'B', 'e', 'd'}) {
		t.Fatalf("missing id Bed: %x", got)
	}
	if !bytes.Contains(got, []byte{TagByte, 0x05, 'c', 'o', 'l', 'o', 'r', 14}) {
		t.Fatalf("missing color: %x", got)
	}
}
