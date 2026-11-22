package command

import (
	"go-mc-server/internal/item"
	"go-mc-server/internal/protocol"
)

// ParseItem parses "1", "1:0", "apple", or "minecraft:stone" into an Item (count unset).
func ParseItem(spec string) (protocol.Item, error) {
	return item.FromString(spec)
}

// defaultGiveCount 未指定数量时给满一组；工具一组只有 1 个。
func defaultGiveCount(target protocol.Item) int {
	return int(item.StackLimit(target.ID))
}
