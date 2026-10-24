package item

import "go-mc-server/internal/block"

// fromBlock wraps a registered block (id 0–255) as an inventory item type.
func fromBlock(id int32) (*Type, bool) {
	if id < 0 || id >= 256 {
		return nil, false
	}
	bt, ok := block.ByID(id)
	if !ok {
		return nil, false
	}
	return &Type{
		ID:       bt.ID,
		Name:     bt.Name,
		Aliases:  append([]string(nil), bt.Aliases...),
		MaxStack: bt.MaxStack,
	}, true
}
