package blockrule_test

import "testing"
import "slices"
import "go-mc-server/internal/blockrule"

// 无燃料时石面上的火超过年龄 3 才灭；关闭蔓延不老化，撤掉支撑仍会灭。
func TestFireLifetime(test *testing.T) {
	var cases = []struct {
		name             string
		base, age        byte
		enabled, unknown bool
		want             byte
		delay            int
	}{
		{"幼火继续", 1, 3, true, false, 51, 30},
		{"石上老火熄灭", 1, 4, true, false, 0, 0},
		{"地狱岩永燃", 87, 15, true, false, 51, 30},
		{"关闭蔓延不老化", 1, 15, false, false, 51, 0},
		{"关闭蔓延仍需支撑", 0, 0, false, false, 0, 0},
		{"区块边界燃料未知", 0, 15, true, true, 51, 30},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item = cases[index]
		test.Run(item.name, func(sub *testing.T) {
			var view = newMapView().set(abovePos, 51, item.age).set(groundPos, item.base, 0)
			if item.unknown {
				view.unload(abovePos.Add(1, 0, 0))
			}
			var update = blockrule.OnScheduled(view, abovePos, &scriptedRandom{}, 1, item.enabled)
			var actual = byte(51)
			var actualAge = item.age
			var idx int = 0
			for idx = 0; idx < len(update.Changes); idx++ {
				var edit = update.Changes[idx]
				if edit.Pos == abovePos {
					actual, actualAge = edit.ID, edit.Meta
				}
			}
			var due int = 0
			if len(update.Schedules) > 0 {
				due = update.Schedules[0].Delay
			}
			if actual != item.want || due != item.delay || !item.enabled && item.want == 51 && actualAge != item.age || item.name == "幼火继续" && actualAge != 4 {
				sub.Fatalf("期望方块 %d、延迟 %d，实际 %d:%d / %d", item.want, item.delay, actual, actualAge, due)
			}
		})
	}
}

// 同一块木头可以烧尽或变成下一簇火；烧掉的是燃料，原火不因此变成空气。
func TestFireConsumesFuel(test *testing.T) {
	var target = abovePos.Add(1, 0, 0)
	var view = newMapView().set(groundPos, 1, 0).set(abovePos, 51, 2).set(target, 5, 0)
	var roll int = 0
	for roll = 4; roll <= 5; roll++ {
		var update = blockrule.OnScheduled(view, abovePos, &scriptedRandom{values: []int{1, 0, 0, roll, 4}}, 1, true)
		var want = change(target, 0, 0)
		if roll == 4 {
			want = change(target, 51, 3)
		}
		if !slices.Equal(update.Changes, []blockrule.Change{want}) {
			test.Fatalf("期望只烧东面木板 %+v，实际 %+v", want, update.Changes)
		}
	}
}

type highFlameRoll struct{}

func (highFlameRoll) IntN(count int) int {
	if count == 401 {
		return 66
	}
	return count - 1
}

// 距原火上方四格的空气以 0..400 抽签；东侧原木助燃 5，困难阈值为 66，普通为 59。
func TestFireSpreadsUpward(test *testing.T) {
	var target = abovePos.Add(1, 4, 0)
	var view = newMapView().set(groundPos, 87, 0).set(abovePos, 51, 0).set(target.Add(1, 0, 0), 17, 0)
	var difficulty int32 = 0
	for difficulty = 2; difficulty <= 3; difficulty++ {
		var update = blockrule.OnScheduled(view, abovePos, highFlameRoll{}, difficulty, true)
		var found = slices.Contains(update.Changes, change(target, 51, 1))
		if found != (difficulty == 3) {
			test.Fatalf("期望难度 3 才向上四格蔓延，实际 %d: %+v", difficulty, update.Changes)
		}
	}
	view.unload(target)
	var update = blockrule.OnScheduled(view, abovePos, highFlameRoll{}, 3, true)
	if len(update.Changes) != 0 {
		test.Fatalf("期望不写未加载区块，实际 %+v", update.Changes)
	}
}
