package blockrule

// bedHeadDirection 床脚 meta 低 2 位表示床头在哪个方向：0 南、1 西、2 北、3 东。
func bedHeadDirection(rotation byte) Direction {
	switch rotation & 3 {
	case 0:
		return DirectionSouth
	case 1:
		return DirectionWest
	case 2:
		return DirectionNorth
	}
	return DirectionEast
}

// partnerPos 按这一格自己的 meta 算另一半应该在哪。bit 3 在门和双层植物里表示上半，
// 在床里表示床头。不是两格方块返回 false。
func partnerPos(pos Pos, id byte, meta byte) (Pos, bool) {
	if isDoor(id) || id == BlockDoublePlant {
		if meta&8 != 0 {
			return pos.Add(0, -1, 0), true
		}
		return pos.Add(0, 1, 0), true
	}
	if id == BlockBed {
		var stepX, stepZ = bedHeadDirection(meta).Step()
		if meta&8 != 0 {
			return pos.Add(-stepX, 0, -stepZ), true
		}
		return pos.Add(stepX, 0, stepZ), true
	}
	return pos, false
}

// PairedHalf 返回两格方块（门、床、双层植物）另一半的位置。
// 另一半必须真的在那里：同一种方块、上下（头脚）相反；床还要朝向一致，双层植物还要是同一个品种。
func PairedHalf(view View, pos Pos) (Pos, bool) {
	var id, meta = view.Block(pos)
	var other, hasOther = partnerPos(pos, id, meta)
	if !hasOther {
		return pos, false
	}
	var otherID, otherMeta = view.Block(other)
	if otherID != id || otherMeta&8 == meta&8 {
		return pos, false
	}
	if id == BlockBed && otherMeta&3 != meta&3 {
		return pos, false
	}
	if id == BlockDoublePlant && otherMeta&7 != meta&7 {
		return pos, false
	}
	return other, true
}

// pairBroken 两格方块缺了另一半就要掉下来；门和双层植物的下半脚下撑不住也要掉。
// 床不看脚下，参考端也没有这条规则。另一半所在的区块没加载时保守地不动。
func pairBroken(view View, pos Pos, id byte, meta byte) bool {
	var other, hasOther = partnerPos(pos, id, meta)
	if !hasOther || !view.Loaded(other) {
		return false
	}
	var _, paired = PairedHalf(view, pos)
	if !paired {
		return true
	}
	if id == BlockBed || meta&8 != 0 {
		return false
	}
	var belowID, belowLoaded = neighborBlock(view, pos, FaceDown)
	if !belowLoaded {
		return false
	}
	if id == BlockDoublePlant {
		return IsTransparent(belowID)
	}
	return !IsSolid(belowID)
}
