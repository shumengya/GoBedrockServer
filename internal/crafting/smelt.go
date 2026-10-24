package crafting

import "go-mc-server/internal/protocol"

type smeltKey struct {
	ID   int32
	Meta int32
}

// smeltIndex 熔炼配方按输入物品建的索引：不限 meta 的按 ID，限 meta 的按 ID 加 meta。
type smeltIndex struct {
	wildcard map[int32]protocol.Item
	exact    map[smeltKey]protocol.Item
}

// indexSmelt 把一条熔炼配方登记进索引，不是熔炼配方就忽略。同一个输入有两条配方时后面的盖掉前面的。
func (book *Book) indexSmelt(recipe Recipe) {
	if recipe.Kind != KindFurnace && recipe.Kind != KindFurnaceData {
		return
	}
	if len(recipe.Input) != 1 || len(recipe.Output) != 1 {
		return
	}
	if book.smelt.wildcard == nil {
		book.smelt.wildcard = map[int32]protocol.Item{}
		book.smelt.exact = map[smeltKey]protocol.Item{}
	}
	var input protocol.Item = recipe.Input[0]
	if recipe.Kind == KindFurnaceData {
		book.smelt.exact[smeltKey{ID: input.ID, Meta: input.Meta}] = recipe.Output[0]
		return
	}
	book.smelt.wildcard[input.ID] = recipe.Output[0]
}

// Smelt 熔炉里放这个输入，能烤出什么。限 meta 的配方优先于不限 meta 的。
func (book *Book) Smelt(input protocol.Item) (protocol.Item, bool) {
	if input.IsAir() {
		return protocol.Air(), false
	}
	var exact, hasExact = book.smelt.exact[smeltKey{ID: input.ID, Meta: input.Meta}]
	if hasExact {
		return exact, true
	}
	var wildcard, hasWildcard = book.smelt.wildcard[input.ID]
	return wildcard, hasWildcard
}
