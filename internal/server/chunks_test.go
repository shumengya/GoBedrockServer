package server

import "testing"

func TestOrderChunksNearestFirst(t *testing.T) {
	list := [][2]int32{{4, 0}, {0, 0}, {1, 1}, {2, 0}}
	orderChunksNearestFirst(list, 0, 0)
	if list[0] != [2]int32{0, 0} {
		t.Fatalf("first=%v want origin", list[0])
	}
	if chunkChebyshev(list[1][0], list[1][1], 0, 0) > chunkChebyshev(list[2][0], list[2][1], 0, 0) {
		t.Fatalf("not sorted by distance: %v", list)
	}
}

func TestEnqueueChunksSkipsSentAndQueued(t *testing.T) {
	p := &Player{
		X:           8,
		Z:           8,
		SentChunks:  map[string]struct{}{"0,0": {}},
		chunkQueued: map[string]struct{}{},
	}
	p.enqueueChunks([][2]int32{{0, 0}, {1, 0}, {0, 1}, {1, 0}})
	if len(p.ChunkQueue) != 2 {
		t.Fatalf("queue=%v want 2 new columns", p.ChunkQueue)
	}
	p.enqueueChunks([][2]int32{{1, 0}, {0, 1}, {2, 0}})
	if len(p.ChunkQueue) != 3 {
		t.Fatalf("second enqueue should only add (2,0), got %v", p.ChunkQueue)
	}
	if p.ChunkQueue[0] != [2]int32{1, 0} && p.ChunkQueue[0] != [2]int32{0, 1} {
		// nearest unsent around chunk (0,0) from pos (8,8) is chebyshev 1
		if chunkChebyshev(p.ChunkQueue[0][0], p.ChunkQueue[0][1], 0, 0) > 1 {
			t.Fatalf("nearest should be first: %v", p.ChunkQueue)
		}
	}
}
