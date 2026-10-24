package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

// cellState 是测试世界里一格方块的状态。
type cellState struct {
	ID   byte
	Meta byte
}

// mapView 是内存里的假世界：没设置过的格子是空气，默认所有区块都算已加载，
// 用 unload 标记某一格所在的区块没加载。
type mapView struct {
	cells    map[blockrule.Pos]cellState
	unloaded map[blockrule.Pos]bool
}

func newMapView() *mapView {
	return &mapView{cells: map[blockrule.Pos]cellState{}, unloaded: map[blockrule.Pos]bool{}}
}

func (view *mapView) set(pos blockrule.Pos, id byte, meta byte) *mapView {
	view.cells[pos] = cellState{ID: id, Meta: meta}
	return view
}

func (view *mapView) unload(pos blockrule.Pos) *mapView {
	view.unloaded[pos] = true
	return view
}

func (view *mapView) Block(pos blockrule.Pos) (byte, byte) {
	var state cellState = view.cells[pos]
	return state.ID, state.Meta
}

func (view *mapView) Loaded(pos blockrule.Pos) bool {
	return !view.unloaded[pos]
}

func posAt(posX int32, posY int32, posZ int32) blockrule.Pos {
	return blockrule.Pos{PosX: posX, PosY: posY, PosZ: posZ}
}

// 测试里反复用到的两个位置：地面方块和它正上方的空位。
var groundPos blockrule.Pos = posAt(0, 63, 0)
var abovePos blockrule.Pos = posAt(0, 64, 0)

// groundView 返回只有一块石头地面的世界。
func groundView() *mapView {
	return newMapView().set(groundPos, blockrule.BlockStone, 0)
}

// request 构造"手拿 itemID，点击 target 的 face 面"的放置请求。
// 点击点只有高度 clickY 影响结果（楼梯倒置、半砖上下），水平方向固定取中心。
func request(
	itemID int32,
	itemMeta int32,
	target blockrule.Pos,
	face int32,
	clickY float32,
	yaw float32,
) blockrule.PlaceRequest {
	return blockrule.PlaceRequest{
		ItemID:   itemID,
		ItemMeta: itemMeta,
		Target:   target,
		Face:     face,
		ClickX:   0.5,
		ClickY:   clickY,
		ClickZ:   0.5,
		Yaw:      yaw,
	}
}

func change(pos blockrule.Pos, id byte, meta byte) blockrule.Change {
	return blockrule.Change{Pos: pos, ID: id, Meta: meta}
}

// placeCase 是一条放置用例。Expect 为空表示这次应该放不了。
type placeCase struct {
	Name    string
	View    *mapView
	Request blockrule.PlaceRequest
	Expect  []blockrule.Change
}

func runPlaceCases(test *testing.T, cases []placeCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item placeCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var changes, placed = blockrule.Place(item.View, item.Request)
			if placed != (len(item.Expect) > 0) {
				sub.Fatalf("可放置期望 %v，实际 %v（改动 %+v）", len(item.Expect) > 0, placed, changes)
			}
			if !slices.Equal(changes, item.Expect) {
				sub.Fatalf("期望改动 %+v，实际 %+v", item.Expect, changes)
			}
		})
	}
}
