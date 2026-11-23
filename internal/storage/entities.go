package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-mc-server/internal/protocol"
)

const EntitiesFile = "entities.json"

type DroppedItemData struct {
	ID          int32   `json:"id"`
	Meta        int32   `json:"meta"`
	Count       byte    `json:"count"`
	NBT         []byte  `json:"nbt,omitempty"`
	X           float32 `json:"x"`
	Y           float32 `json:"y"`
	Z           float32 `json:"z"`
	SpeedX      float32 `json:"speedX"`
	SpeedY      float32 `json:"speedY"`
	SpeedZ      float32 `json:"speedZ"`
	PickupDelay int     `json:"pickupDelay"`
	Age         int     `json:"age"`
}

func EntitiesPath(dataDir, worldName string) string {
	return filepath.Join(WorldDir(dataDir, worldName), EntitiesFile)
}

func LoadDroppedItems(dataDir, worldName string) ([]DroppedItemData, error) {
	raw, err := os.ReadFile(EntitiesPath(dataDir, worldName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []DroppedItemData
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func SaveDroppedItems(dataDir, worldName string, list []DroppedItemData) error {
	if err := os.MkdirAll(WorldDir(dataDir, worldName), 0o755); err != nil {
		return err
	}
	if list == nil {
		list = []DroppedItemData{}
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := EntitiesPath(dataDir, worldName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ItemFromDrop(d DroppedItemData) protocol.Item {
	return protocol.Item{ID: d.ID, Meta: d.Meta, Count: d.Count, NBT: d.NBT}
}
