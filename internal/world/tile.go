package world

import (
	"fmt"
	"math"

	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/nbt"
)

const (
	TileChest   = nbt.IDChest
	TileBed     = nbt.IDBed
	TileFurnace = nbt.IDFurnace
	TileSign    = nbt.IDSign
	FurnaceSize = protocol.FurnaceSize
	ChestSize   = protocol.ChestSize
)

// Tile is a block entity (chest inventory / bed color). Workbenches have no tile.
type Tile struct {
	ID      string
	X, Y, Z int32
	Color   byte
	Items   []protocol.Item

	// 熔炉：剩余燃烧 tick、已经烤了多少 tick、这块燃料一共能烧多少 tick。
	BurnTime int32
	CookTime int32
	MaxTime  int32

	// 箱子：和相邻箱子配成大箱子之后，另一半的 X、Z。
	Paired bool
	PairX  int32
	PairZ  int32

	// 告示牌的四行字，以及还有权写字的玩家 UUID：放下牌子的人可以写一次，写完清空，不存盘，重启后也清空。
	Lines  [4]string
	Editor string
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
		// 颜色 0 是白色，是合法颜色，不能当成没设置而换成红色。
		return nbt.SpawnBed(t.X, t.Y, t.Z, t.Color)
	case TileFurnace:
		return nbt.SpawnFurnace(t.X, t.Y, t.Z, int16(t.BurnTime), int16(t.CookTime))
	case TileSign:
		return nbt.SpawnSign(t.X, t.Y, t.Z, t.Lines)
	default:
		if t.ID == TileChest && t.Paired {
			return nbt.SpawnPairedChest(t.X, t.Y, t.Z, t.PairX, t.PairZ)
		}
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
	var size int = SlotCount(t.ID)
	if size > 0 && len(t.Items) != size {
		t.Items = padItems(t.Items, size)
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
		if !KnownTile(t.ID) {
			continue
		}
		var size int = SlotCount(t.ID)
		if size > 0 && len(t.Items) != size {
			t.Items = padItems(t.Items, size)
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
