package blockrule

// Schedule 请求过 Delay 个 tick 之后再评估 Pos 这一格。
type Schedule struct {
	Pos   Pos
	Delay int
}

// Update 是一次规则评估的结果：要改的格子，以及之后要再评估的格子。
type Update struct {
	Changes   []Change
	Schedules []Schedule
}

// 按钮按下后弹回来的等待时间，单位是游戏 tick：石按钮 1 秒，木按钮 1.5 秒（原版数值）。
const stoneButtonDelay int = 20
const woodButtonDelay int = 30

// Interact 处理玩家右键点击 pos 这一格。第二个返回值为 false 表示这个方块不能交互，
// 调用方接着走放置流程；为 true 表示点击已经被它消耗了，哪怕什么都没改（比如铁门）。
// yaw 只有栅栏门要用：开门时门框朝向跟着玩家。
func Interact(view View, pos Pos, yaw float32) (Update, bool) {
	var id, meta = view.Block(pos)
	if isDoor(id) {
		return interactDoor(view, pos, id, meta)
	}
	if id == BlockTrapdoor {
		return Update{Changes: []Change{{Pos: pos, ID: id, Meta: meta ^ 8}}}, true
	}
	// 铁活板门、铁门只能用红石开，手点没反应，但点击也不能再往旁边放方块。
	if id == BlockIronTrapdoor {
		return Update{}, true
	}
	if isFenceGate(id) {
		var direction int = int(DirectionFromYaw(yaw))
		var toggled byte = (meta^4)&^2 | byte((direction-1)&2)
		return Update{Changes: []Change{{Pos: pos, ID: id, Meta: toggled}}}, true
	}
	if id == BlockLever {
		return Update{Changes: []Change{{Pos: pos, ID: id, Meta: meta ^ 8}}}, true
	}
	if isButton(id) {
		return interactButton(pos, id, meta), true
	}
	return Update{}, false
}

// interactDoor 门的开关位 4 只记在下半。点上半就去改它下面那一格。
func interactDoor(view View, pos Pos, id byte, meta byte) (Update, bool) {
	if id == BlockIronDoor {
		return Update{}, true
	}
	var lowerPos Pos = pos
	if meta&8 != 0 {
		lowerPos = pos.Add(0, -1, 0)
	}
	var lowerID, lowerMeta = view.Block(lowerPos)
	if lowerID != id || lowerMeta&8 != 0 {
		return Update{}, false
	}
	return Update{Changes: []Change{{Pos: lowerPos, ID: id, Meta: lowerMeta ^ 4}}}, true
}

// interactButton 没按下就按下并排队弹回；已经按下的不再改，但仍然排一次弹回。
// 排队时已有更早的到期时间会保留，所以连点不会把弹回时间越推越晚，
// 而重启丢了队列、停在按下状态的按钮，点一下就能重新排上队弹回。
func interactButton(pos Pos, id byte, meta byte) Update {
	var delay int = stoneButtonDelay
	if id == BlockWoodButton {
		delay = woodButtonDelay
	}
	var schedules []Schedule = []Schedule{{Pos: pos, Delay: delay}}
	if meta&8 != 0 {
		return Update{Schedules: schedules}
	}
	return Update{Changes: []Change{{Pos: pos, ID: id, Meta: meta | 8}}, Schedules: schedules}
}

// OnScheduled 在 Interact 等排过队的时间到了之后评估 pos：按钮弹回、液体流动。
func OnScheduled(view View, pos Pos) Update {
	if !view.Loaded(pos) {
		return Update{}
	}
	var id, meta = view.Block(pos)
	if isButton(id) && meta&8 != 0 {
		return Update{Changes: []Change{{Pos: pos, ID: id, Meta: meta &^ 8}}}
	}
	if isLiquid(id) {
		return liquidScheduled(view, pos, id, meta)
	}
	return Update{}
}
