package protocol

import "testing"

func TestSlotRoundTrip(t *testing.T) {
	orig := Item{ID: 1, Meta: 0, Count: 64}
	w := NewWriter()
	w.WriteSlot(orig)
	r := NewStream(w.Bytes())
	got, err := r.ReadSlot()
	if err != nil {
		t.Fatalf("ReadSlot: %v", err)
	}
	if got.ID != orig.ID || got.Meta != orig.Meta || got.Count != orig.Count {
		t.Fatalf("got %+v want %+v", got, orig)
	}

	w2 := NewWriter()
	w2.WriteSlot(Air())
	r2 := NewStream(w2.Bytes())
	air, err := r2.ReadSlot()
	if err != nil {
		t.Fatalf("ReadSlot air: %v", err)
	}
	if !air.IsAir() {
		t.Fatalf("expected air, got %+v", air)
	}
}
