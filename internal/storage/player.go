package storage

import (
	"encoding/json"
	"os"

	"go-mc-server/internal/protocol"
)

type SlotData struct {
	ID    int32 `json:"id"`
	Meta  int32 `json:"meta"`
	Count byte  `json:"count"`
}

type PlayerData struct {
	UUID      string     `json:"uuid"`
	Name      string     `json:"name"`
	XUID      string     `json:"xuid"`
	World     string     `json:"world"`
	Dimension int32      `json:"dimension"`
	X         float32    `json:"x"`
	Y         float32    `json:"y"`
	Z         float32    `json:"z"`
	Yaw       float32    `json:"yaw"`
	Pitch     float32    `json:"pitch"`
	GameMode  int32      `json:"gamemode"`
	Inventory []SlotData `json:"inventory"`
	HeldSlot  byte       `json:"heldSlot"`
	// EnderChest 是玩家的末影箱，所有末影箱方块共用这一份。没用过的玩家没有这个字段。
	EnderChest []SlotData `json:"enderChest,omitempty"`
	// Food 是饥饿状态。老存档没有这个字段，读出来是 nil，调用方按满饥饿处理，不能当成 0 饿死人。
	Food *FoodData `json:"food,omitempty"`
}

// FoodData 是持久化的饥饿状态，防止玩家靠退服重进来回满饥饿。
type FoodData struct {
	Level      float32 `json:"level"`
	Saturation float32 `json:"saturation"`
	Exhaustion float32 `json:"exhaustion"`
	Timer      int32   `json:"timer"`
}

func LoadPlayer(dataDir, uuid string) (PlayerData, bool, error) {
	path := PlayerPath(dataDir, uuid)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PlayerData{}, false, nil
		}
		return PlayerData{}, false, err
	}
	var d PlayerData
	if err := json.Unmarshal(raw, &d); err != nil {
		return PlayerData{}, false, err
	}
	return d, true, nil
}

func SavePlayer(dataDir string, d PlayerData) error {
	if err := os.MkdirAll(PlayersDir(dataDir), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := PlayerPath(dataDir, d.UUID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func SlotsFromItems(items []protocol.Item) []SlotData {
	out := make([]SlotData, len(items))
	for i, it := range items {
		out[i] = SlotData{ID: it.ID, Meta: it.Meta, Count: it.Count}
	}
	return out
}

func ItemsFromSlots(slots []SlotData, n int) []protocol.Item {
	out := make([]protocol.Item, n)
	for i := 0; i < n && i < len(slots); i++ {
		s := slots[i]
		if s.ID <= 0 || s.Count == 0 {
			continue
		}
		out[i] = protocol.Item{ID: s.ID, Meta: s.Meta, Count: s.Count}
	}
	return out
}
