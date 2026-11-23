package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

type interactCase struct {
	Name      string
	View      *mapView
	Pos       blockrule.Pos
	Yaw       float32
	Handled   bool
	Changes   []blockrule.Change
	Schedules []blockrule.Schedule
}

func runInteractCases(test *testing.T, cases []interactCase) {
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item interactCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var update, handled = blockrule.Interact(item.View, item.Pos, item.Yaw)
			if handled != item.Handled {
				sub.Fatalf("handled 期望 %v，实际 %v", item.Handled, handled)
			}
			if !slices.Equal(update.Changes, item.Changes) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Changes, update.Changes)
			}
			if !slices.Equal(update.Schedules, item.Schedules) {
				sub.Fatalf("定时任务期望 %+v，实际 %+v", item.Schedules, update.Schedules)
			}
		})
	}
}

// 门的开关位(4)只记在下半：点哪一半都去改下半。铁门手点没反应，但点击仍然被消耗。
func TestInteractDoor(test *testing.T) {
	var upper blockrule.Pos = posAt(0, 65, 0)
	var woodDoor *mapView = newMapView().
		set(abovePos, blockrule.BlockWoodDoor, 0).
		set(upper, blockrule.BlockWoodDoor, 8)
	var cases []interactCase = []interactCase{
		{
			Name:    "点下半打开",
			View:    woodDoor,
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockWoodDoor, 4)},
		},
		{
			Name:    "点上半也是改下半",
			View:    woodDoor,
			Pos:     upper,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockWoodDoor, 4)},
		},
		{
			Name: "已经打开的门再点就关上",
			View: newMapView().
				set(abovePos, blockrule.BlockWoodDoor, 4).
				set(upper, blockrule.BlockWoodDoor, 8),
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockWoodDoor, 0)},
		},
		{
			Name: "开关不影响朝向位",
			View: newMapView().
				set(abovePos, blockrule.BlockWoodDoor, 3).
				set(upper, blockrule.BlockWoodDoor, 9),
			Pos:     upper,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockWoodDoor, 3|4)},
		},
		{
			Name:    "上半下面没有门，不处理",
			View:    newMapView().set(upper, blockrule.BlockWoodDoor, 8),
			Pos:     upper,
			Handled: false,
		},
		{
			Name: "铁门手点不开，但点击被消耗",
			View: newMapView().
				set(abovePos, blockrule.BlockIronDoor, 0).
				set(upper, blockrule.BlockIronDoor, 8),
			Pos:     abovePos,
			Handled: true,
		},
	}
	runInteractCases(test, cases)
}

// 活板门、拉杆各自翻转一个位；栅栏门开关时朝向位跟着玩家偏航角。
func TestInteractTrapdoorLeverGate(test *testing.T) {
	var cases []interactCase = []interactCase{
		{
			Name:    "活板门打开",
			View:    newMapView().set(abovePos, blockrule.BlockTrapdoor, 1),
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 1|8)},
		},
		{
			Name:    "活板门关上",
			View:    newMapView().set(abovePos, blockrule.BlockTrapdoor, 1|8),
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockTrapdoor, 1)},
		},
		{
			Name:    "铁活板门手点不开，点击被消耗",
			View:    newMapView().set(abovePos, blockrule.BlockIronTrapdoor, 0),
			Pos:     abovePos,
			Handled: true,
		},
		{
			Name:    "拉杆拨开",
			View:    newMapView().set(abovePos, blockrule.BlockLever, 5),
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockLever, 5|8)},
		},
		{
			Name:    "拉杆拨回",
			View:    newMapView().set(abovePos, blockrule.BlockLever, 5|8),
			Pos:     abovePos,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockLever, 5)},
		},
		{
			Name:    "栅栏门朝东的玩家打开，带上 2 位",
			View:    newMapView().set(abovePos, blockrule.BlockFenceGate, 0),
			Pos:     abovePos,
			Yaw:     270,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 4|2)},
		},
		{
			Name:    "栅栏门朝南的玩家打开，不带 2 位",
			View:    newMapView().set(abovePos, blockrule.BlockFenceGate, 0),
			Pos:     abovePos,
			Yaw:     0,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 4)},
		},
		{
			Name:    "栅栏门朝北的玩家打开，带上 2 位",
			View:    newMapView().set(abovePos, blockrule.BlockFenceGate, 1),
			Pos:     abovePos,
			Yaw:     180,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 1|4|2)},
		},
		{
			Name:    "栅栏门关上时清掉 2 位",
			View:    newMapView().set(abovePos, blockrule.BlockFenceGate, 4|2),
			Pos:     abovePos,
			Yaw:     0,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockFenceGate, 0)},
		},
		{
			Name:    "云杉栅栏门同样处理",
			View:    newMapView().set(abovePos, blockrule.BlockFenceGateSpruce, 0),
			Pos:     abovePos,
			Yaw:     0,
			Handled: true,
			Changes: []blockrule.Change{change(abovePos, blockrule.BlockFenceGateSpruce, 4)},
		},
	}
	runInteractCases(test, cases)
}

// 按钮按下并排队弹回：石按钮 20 tick，木按钮 30 tick。已经按下的不再改，但仍排一次队，
// 这样重启丢了队列、停在按下状态的按钮，点一下就能重新弹回来。
func TestInteractButton(test *testing.T) {
	var stoneDelay []blockrule.Schedule = []blockrule.Schedule{{Pos: abovePos, Delay: 20}}
	var woodDelay []blockrule.Schedule = []blockrule.Schedule{{Pos: abovePos, Delay: 30}}
	var cases []interactCase = []interactCase{
		{
			Name:      "石按钮按下",
			View:      newMapView().set(abovePos, blockrule.BlockStoneButton, 1),
			Pos:       abovePos,
			Handled:   true,
			Changes:   []blockrule.Change{change(abovePos, blockrule.BlockStoneButton, 1|8)},
			Schedules: stoneDelay,
		},
		{
			Name:      "木按钮按下，等待更久",
			View:      newMapView().set(abovePos, blockrule.BlockWoodButton, 5),
			Pos:       abovePos,
			Handled:   true,
			Changes:   []blockrule.Change{change(abovePos, blockrule.BlockWoodButton, 5|8)},
			Schedules: woodDelay,
		},
		{
			Name:      "已经按下的石按钮不改方块，仍然排队",
			View:      newMapView().set(abovePos, blockrule.BlockStoneButton, 1|8),
			Pos:       abovePos,
			Handled:   true,
			Schedules: stoneDelay,
		},
	}
	runInteractCases(test, cases)
}

// 不能交互的方块返回 handled=false，调用方接着走放置流程。
func TestInteractIgnoresOtherBlocks(test *testing.T) {
	var cases []interactCase = []interactCase{
		{Name: "石头", View: newMapView().set(abovePos, blockrule.BlockStone, 0), Pos: abovePos},
		{Name: "空气", View: newMapView(), Pos: abovePos},
		{Name: "箱子归服务端另外处理", View: newMapView().set(abovePos, blockrule.BlockChest, 2), Pos: abovePos},
		{Name: "火把", View: newMapView().set(abovePos, blockrule.BlockTorch, 5), Pos: abovePos},
	}
	runInteractCases(test, cases)
}

type scheduledCase struct {
	Name   string
	View   *mapView
	Expect []blockrule.Change
}

// 定时到了之后：按下的按钮弹回；其他方块、已经弹回的按钮、不在内存里的格子都不动。
func TestOnScheduled(test *testing.T) {
	var cases []scheduledCase = []scheduledCase{
		{
			Name:   "石按钮弹回",
			View:   newMapView().set(abovePos, blockrule.BlockStoneButton, 1|8),
			Expect: []blockrule.Change{change(abovePos, blockrule.BlockStoneButton, 1)},
		},
		{
			Name:   "木按钮弹回",
			View:   newMapView().set(abovePos, blockrule.BlockWoodButton, 4|8),
			Expect: []blockrule.Change{change(abovePos, blockrule.BlockWoodButton, 4)},
		},
		{
			Name: "已经弹回的按钮不动",
			View: newMapView().set(abovePos, blockrule.BlockStoneButton, 1),
		},
		{
			Name: "按钮被挖掉了不动",
			View: newMapView(),
		},
		{
			Name: "拉杆不会自己弹回",
			View: newMapView().set(abovePos, blockrule.BlockLever, 5|8),
		},
		{
			Name: "区块没加载不动",
			View: newMapView().set(abovePos, blockrule.BlockStoneButton, 1|8).unload(abovePos),
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item scheduledCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var update blockrule.Update = blockrule.OnScheduled(item.View, abovePos, nil, 0, false)
			if !slices.Equal(update.Changes, item.Expect) {
				sub.Fatalf("改动期望 %+v，实际 %+v", item.Expect, update.Changes)
			}
			if len(update.Schedules) != 0 {
				sub.Fatalf("不该再排队，实际 %+v", update.Schedules)
			}
		})
	}
}

// 拉杆、按钮靠在某个面上：meta 低 3 位决定靠哪面，靠着的那面变成透明方块就掉下来。
func TestNeighborLeverAndButton(test *testing.T) {
	var west blockrule.Pos = posAt(-1, 64, 0)
	var ceiling blockrule.Pos = posAt(0, 65, 0)
	var cases []neighborCase = []neighborCase{
		{
			Name: "拉杆立在石头上不掉",
			View: groundView().set(abovePos, blockrule.BlockLever, 5),
			Pos:  abovePos,
		},
		{
			Name:   "拉杆脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockLever, 5),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "拉杆通电后仍按低 3 位找支撑",
			View:   newMapView().set(abovePos, blockrule.BlockLever, 5|8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "拉杆贴西墙，西边有石头不掉",
			View: newMapView().set(abovePos, blockrule.BlockLever, 1).set(west, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name:   "拉杆贴西墙，西边是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockLever, 1),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "拉杆挂天花板，上面有石头不掉",
			View: newMapView().set(abovePos, blockrule.BlockLever, 0).set(ceiling, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name:   "拉杆挂天花板，上面是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockLever, 7),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "按钮点顶面，脚下是石头不掉",
			View: groundView().set(abovePos, blockrule.BlockStoneButton, 1),
			Pos:  abovePos,
		},
		{
			Name:   "按钮点顶面，脚下是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockStoneButton, 1),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name:   "按钮按下后同样看支撑",
			View:   newMapView().set(abovePos, blockrule.BlockStoneButton, 1|8),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "木按钮点东面，靠着的是西边",
			View: newMapView().set(abovePos, blockrule.BlockWoodButton, 5).set(west, blockrule.BlockStone, 0),
			Pos:  abovePos,
		},
		{
			Name:   "木按钮点东面，西边是空气掉落",
			View:   newMapView().set(abovePos, blockrule.BlockWoodButton, 5),
			Pos:    abovePos,
			Broken: true,
		},
		{
			Name: "按钮 meta 无效，不动",
			View: newMapView().set(abovePos, blockrule.BlockStoneButton, 6),
			Pos:  abovePos,
		},
		{
			Name: "支撑在没加载的区块里，保守地不掉",
			View: newMapView().set(abovePos, blockrule.BlockLever, 5).unload(groundPos),
			Pos:  abovePos,
		},
	}
	runNeighborCases(test, cases)
}
