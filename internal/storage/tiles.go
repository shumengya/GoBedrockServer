package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-mc-server/internal/protocol"
	"go-mc-server/internal/world"
)

const TilesFile = "tiles.json"

type TileRecord struct {
	ID    string     `json:"id"`
	X     int32      `json:"x"`
	Y     int32      `json:"y"`
	Z     int32      `json:"z"`
	Color byte       `json:"color,omitempty"`
	Items []SlotData `json:"items,omitempty"`
}

func TilesPath(dataDir, worldName string) string {
	return filepath.Join(WorldDir(dataDir, worldName), TilesFile)
}

func LoadTiles(dataDir, worldName string) ([]TileRecord, error) {
	raw, err := os.ReadFile(TilesPath(dataDir, worldName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []TileRecord
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func SaveTiles(dataDir, worldName string, list []TileRecord) error {
	if err := os.MkdirAll(WorldDir(dataDir, worldName), 0o755); err != nil {
		return err
	}
	if list == nil {
		list = []TileRecord{}
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := TilesPath(dataDir, worldName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func TilesFromWorld(list []world.Tile) []TileRecord {
	out := make([]TileRecord, 0, len(list))
	for _, t := range list {
		rec := TileRecord{ID: t.ID, X: t.X, Y: t.Y, Z: t.Z, Color: t.Color}
		if t.ID == world.TileChest {
			rec.Items = SlotsFromItems(t.Items)
		}
		out = append(out, rec)
	}
	return out
}

func WorldTilesFromRecords(list []TileRecord) []world.Tile {
	out := make([]world.Tile, 0, len(list))
	for _, rec := range list {
		t := world.Tile{ID: rec.ID, X: rec.X, Y: rec.Y, Z: rec.Z, Color: rec.Color}
		if rec.ID == world.TileChest {
			t.Items = ItemsFromSlots(rec.Items, protocol.ChestSize)
		}
		out = append(out, t)
	}
	return out
}
