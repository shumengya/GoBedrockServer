package item

import (
	_ "embed"
	"encoding/json"
	"sync"

	"go-mc-server/internal/protocol"
)

//go:embed creativeitems.json
var creativeItemsJSON []byte

type creativeEntry struct {
	ID     int32  `json:"id"`
	Damage int32  `json:"damage"`
	Count  byte   `json:"count"`
	NBTHex string `json:"nbt_hex"`
}

var (
	creativeOnce  sync.Once
	creativeItems []protocol.Item
)

// CreativeItems returns PocketMine's 1.1 creative inventory list.
func CreativeItems() []protocol.Item {
	creativeOnce.Do(func() {
		var entries []creativeEntry
		if err := json.Unmarshal(creativeItemsJSON, &entries); err != nil {
			return
		}
		out := make([]protocol.Item, 0, len(entries))
		for _, e := range entries {
			if e.ID <= 0 {
				continue
			}
			n := e.Count
			if n == 0 {
				n = 1
			}
			out = append(out, protocol.Item{ID: e.ID, Meta: e.Damage, Count: n})
		}
		creativeItems = out
	})
	return creativeItems
}
