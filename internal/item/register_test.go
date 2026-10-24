package item

import "testing"

func TestFromString(t *testing.T) {
	it, err := FromString("apple")
	if err != nil || it.ID != 260 {
		t.Fatalf("apple -> %+v err=%v", it, err)
	}
	it, err = FromString("minecraft:stone")
	if err != nil || it.ID != 1 {
		t.Fatalf("minecraft:stone -> %+v err=%v", it, err)
	}
	it, err = FromString("260")
	if err != nil || it.ID != 260 {
		t.Fatalf("260 -> %+v err=%v", it, err)
	}
	it, err = FromString("cobble")
	if err != nil || it.ID != 4 {
		t.Fatalf("cobble -> %+v err=%v", it, err)
	}
	if _, err := FromString("nope"); err == nil {
		t.Fatal("unknown name must fail")
	}
	it, err = FromString("999")
	if err != nil || it.ID != 999 {
		t.Fatalf("unregistered numeric id should still work: %+v err=%v", it, err)
	}
}

func TestCreativeItems(t *testing.T) {
	items := CreativeItems()
	if len(items) < 100 {
		t.Fatalf("creative items=%d, want PocketMine list", len(items))
	}
	if items[0].ID != 138 || items[0].Count != 1 {
		t.Fatalf("first creative item %+v want beacon id=138", items[0])
	}
}
