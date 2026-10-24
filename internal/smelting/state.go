package smelting

import "go-mc-server/internal/protocol"

// 烤熟一个要多少 tick。
const CookTicks int32 = 200

// 产物格最多叠多少。熔炼出来的东西（矿锭、熟肉、玻璃…）都是 64 一叠。
const MaxStack byte = 64

// 熔炉三格的编号，和窗口里的槽位号一致。
const SlotInput int = 0
const SlotFuel int = 1
const SlotOutput int = 2

// 火焰图标满格对应的数值。
const fireScale int32 = 200

// Recipes 按输入物品查熔炼产物。crafting.Book 实现了它。
type Recipes interface {
	Smelt(input protocol.Item) (protocol.Item, bool)
}

// State 是熔炉此刻的全部状态。
type State struct {
	Input  protocol.Item
	Fuel   protocol.Item
	Output protocol.Item
	// BurnTime 当前这块燃料还能烧多少 tick，MaxTime 是它一共能烧多少 tick，火焰图标按比例画。
	BurnTime int32
	CookTime int32
	MaxTime  int32
}

// SameItem 两格物品算不算一样：都是空的就一样，不然 ID、meta、数量都要相同。
func SameItem(left protocol.Item, right protocol.Item) bool {
	if left.IsAir() && right.IsAir() {
		return true
	}
	return left.ID == right.ID && left.Meta == right.Meta && left.Count == right.Count
}

// Equal 两个状态是不是完全一样。
func (state State) Equal(other State) bool {
	return SameItem(state.Input, other.Input) &&
		SameItem(state.Fuel, other.Fuel) &&
		SameItem(state.Output, other.Output) &&
		state.BurnTime == other.BurnTime &&
		state.CookTime == other.CookTime &&
		state.MaxTime == other.MaxTime
}

// Lit 正在烧。
func (state State) Lit() bool {
	return state.BurnTime > 0
}

// Idle 这一个 tick 肯定什么都不会发生：没在烧、没在烤，而且缺输入或缺燃料所以也点不着。
func (state State) Idle() bool {
	return state.BurnTime == 0 && state.CookTime == 0 && (state.Input.IsAir() || state.Fuel.IsAir())
}

// FireTicks 发给客户端画火焰的数值：剩余燃烧占比乘 200，向上取整。
func (state State) FireTicks() int32 {
	if state.MaxTime <= 0 || state.BurnTime <= 0 {
		return 0
	}
	return (state.BurnTime*fireScale + state.MaxTime - 1) / state.MaxTime
}

// outputAccepts 产物格还能不能再收一个 result：空的，或者是同一种且没叠满。
func outputAccepts(output protocol.Item, result protocol.Item) bool {
	if output.IsAir() {
		return true
	}
	return output.ID == result.ID && output.Meta == result.Meta && output.Count < MaxStack
}

// OutputChangeAllowed 客户端想把产物格改成 proposed，允不允许。
// 产物格只能取，不能放：清空或者变少、而且还是同一种东西才行，不能凭空多出来或者换成别的。
func OutputChangeAllowed(current protocol.Item, proposed protocol.Item) bool {
	if proposed.IsAir() {
		return true
	}
	if current.IsAir() {
		return false
	}
	return proposed.ID == current.ID && proposed.Meta == current.Meta && proposed.Count <= current.Count
}

// finishSmelt 烤熟一个：产物格加一个，输入扣一个，烤制进度减掉 200。
// 产物数量固定按一个算，和参考端一致；配方数据里的熔炼产物也确实都是一个。
func finishSmelt(state State, result protocol.Item) State {
	var produced protocol.Item = protocol.Item{ID: result.ID, Meta: result.Meta, Count: 1}
	if !state.Output.IsAir() {
		produced.Count = state.Output.Count + 1
	}
	state.Output = produced
	state.Input = consumeOne(state.Input)
	state.CookTime -= CookTicks
	return state
}

// Step 推进一个 tick，逻辑照参考端 tile\Furnace::onUpdate：
//  1. 没在烧、而且有东西可烤，就点一块燃料；
//  2. 在烧就烧掉一个 tick，能烤就加一个 tick 的烤制进度，攒满 200 出一个；
//     烧着但烤不了，烤制进度清零；
//  3. 没在烧，烧制和烤制进度都清零（燃料烧完中途断了，进度就丢了）。
func Step(state State, recipes Recipes) State {
	var result, smeltable = recipes.Smelt(state.Input)
	var canSmelt bool = smeltable && !state.Input.IsAir() && outputAccepts(state.Output, result)
	if state.BurnTime <= 0 && canSmelt {
		var burn int32 = FuelTicks(state.Fuel)
		if burn > 0 {
			state.MaxTime = burn
			state.BurnTime = burn
			state.Fuel = spendFuel(state.Fuel)
		}
	}
	if state.BurnTime <= 0 {
		state.BurnTime = 0
		state.CookTime = 0
		return state
	}
	state.BurnTime--
	if !canSmelt {
		state.CookTime = 0
		return state
	}
	state.CookTime++
	if state.CookTime >= CookTicks {
		return finishSmelt(state, result)
	}
	return state
}
