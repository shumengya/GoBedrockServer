package blockrule

// isFallable 会下落的方块：沙子和沙砾。参考端还有铁砧、混凝土粉末、龙蛋，这里先不做。
func isFallable(id byte) bool {
	return id == BlockSand || id == BlockGravel
}

func isLiquid(id byte) bool {
	return id == BlockFlowingWater || id == BlockStillWater || id == BlockFlowingLava || id == BlockStillLava
}

// fallsThrough 下落的方块能穿过的东西：空气、液体、火。
// 火把、花这类不实心的方块不在里面：沙子压在它们上面不会掉。
func fallsThrough(id byte) bool {
	return id == BlockAir || isLiquid(id) || id == BlockFire
}

// fallChange 沙子、沙砾脚下是空气、液体或火就要掉下来。脚下的区块没加载时保守地不动。
func fallChange(view View, pos Pos) []Change {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	if !loaded || !fallsThrough(belowID) {
		return nil
	}
	return []Change{{Pos: pos, ID: BlockAir, Meta: 0, Fall: true}}
}
