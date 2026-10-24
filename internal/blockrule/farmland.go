package blockrule

// farmlandRevert 耕地上面压了实心方块就变回泥土（参考端 Farmland::onUpdate）。变回泥土不掉落东西。
// 上面是庄稼、火把这类不实心的不受影响。
func farmlandRevert(view View, spot Pos) []Change {
	var aboveID, loaded = neighborBlock(view, spot, FaceUp)
	if !loaded || !IsSolid(aboveID) {
		return nil
	}
	return []Change{{Pos: spot, ID: BlockDirt, Meta: 0}}
}

// Trample 耕地被落地的实体踩坏：落差减 0.5 就是踩坏的概率，落差超过 1.5 格一定坏。
// 参考端没有这条规则，按原版的做法加上。踩坏之后上面的庄稼失去支撑，由级联引擎接着处理。
// 只有耕地且概率大于 0 时才取一次随机数。
func Trample(view View, spot Pos, fallDistance float32, random Random) (Update, bool) {
	var chance float32 = fallDistance - 0.5
	if chance <= 0 || !view.Loaded(spot) || IDAt(view, spot) != BlockFarmland {
		return Update{}, false
	}
	if float32(random.IntN(1000)) >= chance*1000 {
		return Update{}, false
	}
	return Update{Changes: []Change{{Pos: spot, ID: BlockDirt, Meta: 0}}}, true
}
