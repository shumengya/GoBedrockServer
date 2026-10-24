package smelting_test

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/smelting"

// fakeRecipes 只认铁矿(15)烤铁锭(265)和圆石(4)烤石头(1)，够测状态机用。
type fakeRecipes struct {
	results map[int32]protocol.Item
}

func newFakeRecipes() fakeRecipes {
	return fakeRecipes{results: map[int32]protocol.Item{
		15: {ID: 265, Count: 1},
		4:  {ID: 1, Count: 1},
	}}
}

func (recipes fakeRecipes) Smelt(input protocol.Item) (protocol.Item, bool) {
	if input.IsAir() {
		return protocol.Air(), false
	}
	var result, found = recipes.results[input.ID]
	return result, found
}

func stack(id int32, meta int32, count byte) protocol.Item {
	return protocol.Item{ID: id, Meta: meta, Count: count}
}

// runSteps 连续推进 steps 个 tick。
func runSteps(state smelting.State, recipes smelting.Recipes, steps int) smelting.State {
	var step int = 0
	for step = 0; step < steps; step++ {
		state = smelting.Step(state, recipes)
	}
	return state
}
