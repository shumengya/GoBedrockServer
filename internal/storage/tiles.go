package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-mc-server/internal/world"
)

const TilesFile = "tiles.json"

type TileRecord struct {
	ID               string     `json:"id"`
	X                int32      `json:"x"`
	Y                int32      `json:"y"`
	Z                int32      `json:"z"`
	Color            byte       `json:"color,omitempty"`
	SkullType        byte       `json:"skullType,omitempty"`
	Rotation         byte       `json:"rotation,omitempty"`
	TransferCooldown byte       `json:"transferCooldown,omitempty"`
	Items            []SlotData `json:"items,omitempty"`
	BurnTime         int32      `json:"burnTime,omitempty"`
	CookTime         int32      `json:"cookTime,omitempty"`
	MaxTime          int32      `json:"maxTime,omitempty"`
	Paired           bool       `json:"paired,omitempty"`
	PairX            int32      `json:"pairX,omitempty"`
	PairZ            int32      `json:"pairZ,omitempty"`
	Lines            []string   `json:"lines,omitempty"`
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
	var out []TileRecord = make([]TileRecord, 0, len(list))
	var index int = 0
	for index = 0; index < len(list); index++ {
		var tile world.Tile = list[index]
		var record TileRecord = TileRecord{
			ID:               tile.ID,
			X:                tile.X,
			Y:                tile.Y,
			Z:                tile.Z,
			Color:            tile.Color,
			SkullType:        tile.SkullType,
			Rotation:         tile.Rotation,
			TransferCooldown: tile.TransferCooldown,
			BurnTime:         tile.BurnTime,
			CookTime:         tile.CookTime,
			MaxTime:          tile.MaxTime,
			Paired:           tile.Paired,
			PairX:            tile.PairX,
			PairZ:            tile.PairZ,
		}
		if world.SlotCount(tile.ID) > 0 {
			record.Items = SlotsFromItems(tile.Items)
		}
		if tile.ID == world.TileSign {
			record.Lines = linesToSlice(tile.Lines)
		}
		out = append(out, record)
	}
	return out
}

// linesToSlice 告示牌四行字全空就不写，省得每块牌子都存四个空串。
func linesToSlice(lines [4]string) []string {
	if lines == [4]string{} {
		return nil
	}
	return lines[:]
}

func WorldTilesFromRecords(list []TileRecord) []world.Tile {
	var out []world.Tile = make([]world.Tile, 0, len(list))
	var index int = 0
	for index = 0; index < len(list); index++ {
		var record TileRecord = list[index]
		var tile world.Tile = world.Tile{
			ID:               record.ID,
			X:                record.X,
			Y:                record.Y,
			Z:                record.Z,
			Color:            record.Color,
			SkullType:        record.SkullType,
			Rotation:         record.Rotation,
			TransferCooldown: record.TransferCooldown,
			BurnTime:         record.BurnTime,
			CookTime:         record.CookTime,
			MaxTime:          record.MaxTime,
			Paired:           record.Paired,
			PairX:            record.PairX,
			PairZ:            record.PairZ,
		}
		var size int = world.SlotCount(record.ID)
		if size > 0 {
			tile.Items = ItemsFromSlots(record.Items, size)
		}
		copy(tile.Lines[:], record.Lines)
		out = append(out, tile)
	}
	return out
}
