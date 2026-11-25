package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go-mc-server/internal/protocol"
)

const EntitiesFile = "entities.json"

type EntityData struct {
	// 旧记录只有物品 ID，没有实体类型；读取时按掉落物处理。
	NetworkID   int32   `json:"networkId,omitempty"`
	CustomName  string  `json:"customName,omitempty"`
	Yaw         float32 `json:"yaw,omitempty"`
	Pitch       float32 `json:"pitch,omitempty"`
	ID          int32   `json:"id,omitempty"`
	Meta        int32   `json:"meta,omitempty"`
	Count       byte    `json:"count,omitempty"`
	NBT         []byte  `json:"nbt,omitempty"`
	X           float32 `json:"x"`
	Y           float32 `json:"y"`
	Z           float32 `json:"z"`
	SpeedX      float32 `json:"speedX"`
	SpeedY      float32 `json:"speedY"`
	SpeedZ      float32 `json:"speedZ"`
	PickupDelay int     `json:"pickupDelay"`
	Age         int     `json:"age"`
	OnGround    bool    `json:"onGround,omitempty"`
}

func EntitiesPath(dataDir, worldName string) string {
	return filepath.Join(WorldDir(dataDir, worldName), EntitiesFile)
}

func LoadEntities(dataDir, worldName string) ([]EntityData, error) {
	raw, err := os.ReadFile(EntitiesPath(dataDir, worldName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []EntityData
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func SaveEntities(dataDir, worldName string, list []EntityData) error {
	if err := os.MkdirAll(WorldDir(dataDir, worldName), 0o755); err != nil {
		return err
	}
	return saveEntityRecords(EntitiesPath(dataDir, worldName), list)
}

func saveEntityRecords(path string, list []EntityData) error {
	if list == nil {
		list = []EntityData{}
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ItemFromDrop(d EntityData) protocol.Item {
	return protocol.Item{ID: d.ID, Meta: d.Meta, Count: d.Count, NBT: d.NBT}
}
