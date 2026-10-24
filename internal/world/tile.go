package world

import (
	"fmt"
	"math"

	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/nbt"
)

const (
	TileChest       = nbt.IDChest
	TileBed         = nbt.IDBed
	ChestSize       = protocol.ChestSize
	BedDefaultColor = 14
)

// Tile is a block entity (chest inventory / bed color). Workbenches have no tile.
type Tile struct {
	ID      string
	X, Y, Z int32
	Color   byte
	Items   []protocol.Item
}

func tileKey(x, y, z int32) string {
	return fmt.Sprintf("%d,%d,%d", x, y, z)
}

func NewChestTile(x, y, z int32) Tile {
	return Tile{
		ID:    TileChest,
		X:     x,
		Y:     y,
		Z:     z,
		Items: make([]protocol.Item, ChestSize),
	}
}

func NewBedTile(x, y, z int32, color byte) Tile {
	return Tile{
		ID:    TileBed,
		X:     x,
		Y:     y,
		Z:     z,
		Color: color & 0x0f,
	}
}

func (t Tile) SpawnNBT() []byte {
	switch t.ID {
	case TileBed:
		color := t.Color
		if color == 0 {
			color = BedDefaultColor
		}
		return nbt.SpawnBed(t.X, t.Y, t.Z, color)
	default:
		return nbt.SpawnChest(t.X, t.Y, t.Z)
	}
}

func (t Tile) clone() Tile {
	out := t
	if t.Items != nil {
		out.Items = append([]protocol.Item(nil), t.Items...)
	}
	return out
}

func (w *World) ensureTiles() {
	if w.tiles == nil {
		w.tiles = make(map[string]Tile)
	}
}

func (w *World) SetTile(t Tile) {
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	w.ensureTiles()
	if t.ID == TileChest && len(t.Items) != ChestSize {
		items := make([]protocol.Item, ChestSize)
		copy(items, t.Items)
		t.Items = items
	}
	w.tiles[tileKey(t.X, t.Y, t.Z)] = t.clone()
	w.tilesDirty = true
}

func (w *World) TileAt(x, y, z int32) (Tile, bool) {
	w.tileMu.RLock()
	defer w.tileMu.RUnlock()
	t, ok := w.tiles[tileKey(x, y, z)]
	if !ok {
		return Tile{}, false
	}
	return t.clone(), true
}

func (w *World) RemoveTile(x, y, z int32) (Tile, bool) {
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	key := tileKey(x, y, z)
	t, ok := w.tiles[key]
	if !ok {
		return Tile{}, false
	}
	delete(w.tiles, key)
	w.tilesDirty = true
	return t.clone(), true
}

func (w *World) SetChestSlot(x, y, z int32, slot int, item protocol.Item) bool {
	if slot < 0 || slot >= ChestSize {
		return false
	}
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	key := tileKey(x, y, z)
	t, ok := w.tiles[key]
	if !ok || t.ID != TileChest {
		return false
	}
	if len(t.Items) != ChestSize {
		items := make([]protocol.Item, ChestSize)
		copy(items, t.Items)
		t.Items = items
	}
	t.Items[slot] = item
	w.tiles[key] = t
	w.tilesDirty = true
	return true
}

func (w *World) SnapshotTiles() []Tile {
	w.tileMu.RLock()
	defer w.tileMu.RUnlock()
	out := make([]Tile, 0, len(w.tiles))
	for _, t := range w.tiles {
		out = append(out, t.clone())
	}
	return out
}

func (w *World) LoadTiles(list []Tile) {
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	w.tiles = make(map[string]Tile, len(list))
	for _, t := range list {
		if t.ID != TileChest && t.ID != TileBed {
			continue
		}
		if t.ID == TileChest && len(t.Items) != ChestSize {
			items := make([]protocol.Item, ChestSize)
			copy(items, t.Items)
			t.Items = items
		}
		w.tiles[tileKey(t.X, t.Y, t.Z)] = t.clone()
	}
	w.tilesDirty = false
}

func (w *World) columnTileNBT(cx, cz int32) []byte {
	w.tileMu.RLock()
	defer w.tileMu.RUnlock()
	var buf []byte
	for _, t := range w.tiles {
		if blockToChunk(t.X) != cx || blockToChunk(t.Z) != cz {
			continue
		}
		buf = append(buf, t.SpawnNBT()...)
	}
	return buf
}

func blockToChunk(v int32) int32 {
	return int32(math.Floor(float64(v) / 16.0))
}
