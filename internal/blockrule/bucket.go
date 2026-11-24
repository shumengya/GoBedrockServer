package blockrule

const ItemBucket int32 = 325

// UseBucket 返回要改的格子和操作后得到的桶。只有液位为零的液体能装进桶。
func UseBucket(view View, target Pos, face int32, contents int32) (Change, int32, bool) {
	var side, valid = target.Side(face)
	if !valid || target.PosY < 0 || target.PosY > 255 {
		return Change{}, 0, false
	}
	var source, level = view.Block(target)
	if contents == 0 {
		if level == 0 && isLiquid(source) {
			return Change{Pos: target, ID: BlockAir}, int32(source &^ 1), true
		}
		return Change{}, 0, false
	}
	if source == BlockAir || side.PosY < 0 || side.PosY > 255 || contents < 8 || contents > 11 {
		return Change{}, 0, false
	}
	var dest, meta = view.Block(side)
	if !IsReplaceable(dest, meta) {
		return Change{}, 0, false
	}
	return Change{Pos: side, ID: byte(contents &^ 1)}, 0, true
}
