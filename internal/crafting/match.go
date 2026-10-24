package crafting

import "go-mc-server/internal/protocol"

// gridSide 是合成格最大边长（工作台 3×3，背包合成区是它的左上 2×2）。
const gridSide int = 3

// ingredientMatches 判断玩家放的物品是否满足配方材料：id 要相同，meta 仅在材料指定时才比。
func ingredientMatches(want protocol.Item, have protocol.Item) bool {
	if want.ID != have.ID {
		return false
	}
	if want.Meta == MetaAny {
		return true
	}
	return want.Meta == have.Meta
}

// OutputMatches 校验客户端声称的产物和配方一致。只比 id 和 meta，不比数量（对齐参考端 equals）。
// 真正发给玩家的永远是配方里的产物，这里只是识别「客户端和服务端对配方的理解不一致」。
func (recipe Recipe) OutputMatches(claimed []protocol.Item) bool {
	if len(claimed) == 0 || len(recipe.Output) == 0 {
		return false
	}
	return claimed[0].ID == recipe.Output[0].ID && claimed[0].Meta == recipe.Output[0].Meta
}

// Match 判断玩家摆在合成格里的东西是否正好是这条配方，匹配则返回要从背包扣掉的物品。
//
// 扣料以玩家摆放的实际物品为准（材料不限 meta 时才能知道扣的是哪种木板），
// 并且每个有东西的格子只扣 1 个，不信客户端上报的格内数量。
func (recipe Recipe) Match(input []protocol.Item) ([]protocol.Item, bool) {
	if len(input) > gridSide*gridSide {
		return nil, false
	}
	switch recipe.Kind {
	case KindShaped:
		return matchShaped(recipe, input)
	case KindShapeless:
		return matchShapeless(recipe, input)
	default:
		return nil, false
	}
}

// gridSideFor 从输入长度推出格子边长：背包 2×2 或工作台 3×3，其它长度不认。
func gridSideFor(length int) int {
	if length == 9 {
		return 3
	}
	if length == 4 {
		return 2
	}
	return 0
}

// trimmedGrid 是裁掉四周空行空列后的格子，摆在合成格哪个角都能比。
type trimmedGrid struct {
	cells  []protocol.Item
	width  int
	height int
}

func trimGrid(cells []protocol.Item, width int, height int) trimmedGrid {
	var minX int = width
	var maxX int = -1
	var minY int = height
	var maxY int = -1
	var posY int = 0
	var posX int = 0
	for posY = 0; posY < height; posY++ {
		for posX = 0; posX < width; posX++ {
			if cells[posY*width+posX].IsAir() {
				continue
			}
			minX = min(minX, posX)
			maxX = max(maxX, posX)
			minY = min(minY, posY)
			maxY = max(maxY, posY)
		}
	}
	if maxX < 0 {
		return trimmedGrid{}
	}
	var out trimmedGrid = trimmedGrid{width: maxX - minX + 1, height: maxY - minY + 1}
	for posY = minY; posY <= maxY; posY++ {
		for posX = minX; posX <= maxX; posX++ {
			out.cells = append(out.cells, cells[posY*width+posX])
		}
	}
	return out
}

// flipRows 左右镜像。斧头、锄头这类不对称配方，原版允许镜像摆放。
func flipRows(grid trimmedGrid) trimmedGrid {
	var out trimmedGrid = trimmedGrid{width: grid.width, height: grid.height}
	out.cells = make([]protocol.Item, len(grid.cells))
	var posY int = 0
	var posX int = 0
	for posY = 0; posY < grid.height; posY++ {
		for posX = 0; posX < grid.width; posX++ {
			out.cells[posY*grid.width+posX] = grid.cells[posY*grid.width+(grid.width-1-posX)]
		}
	}
	return out
}

func compareGrids(want trimmedGrid, have trimmedGrid) ([]protocol.Item, bool) {
	if want.width == 0 || want.width != have.width || want.height != have.height {
		return nil, false
	}
	var consumed []protocol.Item = make([]protocol.Item, 0, len(have.cells))
	var index int = 0
	for index = 0; index < len(want.cells); index++ {
		var wantCell protocol.Item = want.cells[index]
		var haveCell protocol.Item = have.cells[index]
		if wantCell.IsAir() != haveCell.IsAir() {
			return nil, false
		}
		if wantCell.IsAir() {
			continue
		}
		if !ingredientMatches(wantCell, haveCell) {
			return nil, false
		}
		consumed = append(consumed, protocol.Item{ID: haveCell.ID, Meta: haveCell.Meta, Count: 1})
	}
	return consumed, true
}

func matchShaped(recipe Recipe, input []protocol.Item) ([]protocol.Item, bool) {
	var side int = gridSideFor(len(input))
	if side == 0 {
		return nil, false
	}
	var want trimmedGrid = trimGrid(recipe.Input, int(recipe.Width), int(recipe.Height))
	var have trimmedGrid = trimGrid(input, side, side)
	var consumed, matched = compareGrids(want, have)
	if matched {
		return consumed, true
	}
	return compareGrids(want, flipRows(have))
}

// matchShapeless 无序配方：摆放的每一格都必须对上一条材料，且材料不能有剩。
// 先配指定 meta 的、再配不限 meta 的，免得「任意木板」抢走本该给某种木板的料。
func matchShapeless(recipe Recipe, input []protocol.Item) ([]protocol.Item, bool) {
	var units []protocol.Item = nil
	var index int = 0
	for index = 0; index < len(input); index++ {
		if input[index].IsAir() {
			continue
		}
		units = append(units, protocol.Item{ID: input[index].ID, Meta: input[index].Meta, Count: 1})
	}
	if len(units) != len(recipe.Input) {
		return nil, false
	}
	var used []bool = make([]bool, len(units))
	var pass int = 0
	for pass = 0; pass < 2; pass++ {
		var wantAny bool = pass == 1
		var needIndex int = 0
		for needIndex = 0; needIndex < len(recipe.Input); needIndex++ {
			var need protocol.Item = recipe.Input[needIndex]
			if (need.Meta == MetaAny) != wantAny {
				continue
			}
			var found bool = false
			var unitIndex int = 0
			for unitIndex = 0; unitIndex < len(units) && !found; unitIndex++ {
				if used[unitIndex] || !ingredientMatches(need, units[unitIndex]) {
					continue
				}
				used[unitIndex] = true
				found = true
			}
			if !found {
				return nil, false
			}
		}
	}
	return units, true
}
