package blockrule_test

import "testing"
import "go-mc-server/internal/blockrule"

// 不同树种的树高、原木与叶的 ID/meta 按参考端分别核对；包括两个低概率变种。
func TestSaplingSpecies(test *testing.T) {
	var cases = []struct {
		name                      string
		species, wood, leaf, meta byte
		draws                     []int
		top                       int32
	}{
		{"橡树", 0, 17, 18, 0, []int{1, 2}, 6},
		{"大橡树", 0, 17, 18, 0, []int{0, 11, 1<<24 - 1, 0}, 15},
		{"云杉", 1, 17, 18, 1, []int{3, 0, 1, 2, 0}, 9},
		{"白桦", 2, 17, 18, 2, []int{1, 2}, 7},
		{"高白桦", 2, 17, 18, 2, []int{0, 1}, 11},
		{"丛林木", 3, 17, 18, 3, nil, 8},
		{"金合欢", 4, 162, 161, 0, nil, 8},
		{"深色橡木", 5, 162, 161, 1, nil, 8},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item = cases[index]
		test.Run(item.name, func(sub *testing.T) {
			var view = cropView(blockrule.BlockSapling, item.species)
			var random = &scriptedRandom{values: item.draws}
			var result, handled = blockrule.UseItem(view, blockrule.ItemUseRequest{ItemID: 351, ItemMeta: 15, Target: abovePos}, random)
			if random.bad || !handled || !result.Consume {
				sub.Fatalf("期望骨粉消耗并使用合法随机数，实际 %+v / %v / %v", result, handled, random.bad)
			}
			var seen = map[blockrule.Pos]bool{}
			var highest = int32(-1)
			var branch = false
			var idx int = 0
			for idx = 0; idx < len(result.Update.Changes); idx++ {
				var edit = result.Update.Changes[idx]
				if seen[edit.Pos] {
					sub.Fatalf("期望每格只更新一次，实际重复 %+v", edit)
				}
				seen[edit.Pos] = true
				view.set(edit.Pos, edit.ID, edit.Meta)
				if edit.ID == item.leaf {
					highest = max(highest, edit.Pos.PosY-abovePos.PosY)
				}
				if edit.ID == item.wood && edit.Pos.PosX != abovePos.PosX {
					branch = true
				}
			}
			if view.cells[groundPos].ID != blockrule.BlockDirt || view.cells[abovePos] != (cellState{item.wood, item.meta}) || view.cells[abovePos.Add(0, item.top, 0)] != (cellState{item.leaf, item.meta}) || highest != item.top {
				sub.Fatalf("期望土、根部 %d:%d、顶层 %d 处的叶 %d:%d，实际 %+v / %+v / %+v / 高 %d", item.wood, item.meta, item.top, item.leaf, item.meta, view.cells[groundPos], view.cells[abovePos], view.cells[abovePos.Add(0, item.top, 0)], highest)
			}
			if item.name == "大橡树" && !branch {
				sub.Fatal("期望大橡树有偏离主干的实木枝，实际无")
			}
		})
	}
}

// 自然生长命中两次才成树；障碍、未加载的冠缘或世界顶端都不能留下半棵树。
func TestSaplingGrowthConditions(test *testing.T) {
	if !blockrule.IsRandomTicked(blockrule.BlockSapling) {
		test.Fatal("期望树苗参与随机刻，实际未注册")
	}
	runRandomCases(test, []randomCase{
		{Name: "未命中", View: cropView(6, 3), Pos: abovePos, Script: []int{6}},
		{Name: "成熟位保留树种", View: cropView(6, 3), Pos: abovePos, Script: []int{0}, Changes: []blockrule.Change{{Pos: abovePos, ID: 6, Meta: 11, Quiet: true}}},
		{Name: "头顶石头", View: cropView(6, 11).set(abovePos.Add(0, 3, 0), 1, 0), Pos: abovePos},
		{Name: "尚未加载的冠缘", View: cropView(6, 11).unload(abovePos.Add(2, 5, 0)), Pos: abovePos},
		{Name: "世界顶端", View: newMapView().set(posAt(0, 251, 0), 6, 11).set(posAt(0, 250, 0), 2, 0), Pos: posAt(0, 251, 0)},
	})
	var view = cropView(6, 11).set(abovePos.Add(1, 7, 0), 18, 4)
	var result = blockrule.OnRandomTick(view, abovePos, &scriptedRandom{})
	var grew = false
	var idx int = 0
	for idx = 0; idx < len(result.Changes); idx++ {
		var edit = result.Changes[idx]
		if edit.Pos == abovePos.Add(1, 7, 0) {
			test.Fatalf("期望保留玩家放置的叶，实际 %+v", edit)
		}
		if edit.Pos == abovePos && edit.ID == 17 && edit.Meta == 3 {
			grew = true
		}
	}
	if !grew {
		test.Fatal("期望成熟丛林树苗命中后长成原木，实际没有")
	}
}
