package blockrule

// isSlab 是否为单层半砖。
func isSlab(id byte) bool {
	var _, found = doubleSlabOf(id)
	return found
}

// doubleSlabOf 返回单层半砖合并后的双层方块 ID。
func doubleSlabOf(id byte) (byte, bool) {
	switch id {
	case BlockSlab:
		return BlockDoubleSlab, true
	case BlockWoodSlab:
		return BlockDoubleWoodSlab, true
	case BlockRedSandstoneSlab:
		return BlockDoubleRedSandstoneSlab, true
	}
	return BlockAir, false
}

// placeSlab 对应参考端 Slab::place。meta 低 3 位是种类，bit 3 是上半砖。
// 点方块底面默认放上半砖，点顶面默认放下半砖，点侧面看点击位置高于还是低于中线；
// 目标格或落点已经有同种类的半砖，并且叠的方向对，就合并成双层。
//
// 参考端红砂岩半砖合并时拿的是普通半砖的 ID 去比，红砂岩半砖永远合并不上，
// 这里按每种半砖自己的 ID 比，行为和原版一致。
func placeSlab(
	view View,
	req PlaceRequest,
	hand byte,
	handMeta byte,
	spot Pos,
	targetID byte,
	targetMeta byte,
) ([]Change, bool) {
	var double, _ = doubleSlabOf(hand)
	var variant byte = handMeta & 7
	var spotID, spotMeta = view.Block(spot)
	var sameSpot bool = spotID == hand && spotMeta&7 == variant
	var sameTarget bool = targetID == hand && targetMeta&7 == variant
	var merged []Change = []Change{{Pos: spot, ID: double, Meta: variant}}
	var meta byte = variant
	switch req.Face {
	case FaceDown:
		if sameTarget && targetMeta&8 != 0 {
			return []Change{{Pos: req.Target, ID: double, Meta: variant}}, true
		}
		if sameSpot {
			return merged, true
		}
		meta |= 8
	case FaceUp:
		if sameTarget && targetMeta&8 == 0 {
			return []Change{{Pos: req.Target, ID: double, Meta: variant}}, true
		}
		if sameSpot {
			return merged, true
		}
	default:
		if sameSpot {
			return merged, true
		}
		if req.ClickY > 0.5 {
			meta |= 8
		}
	}
	return []Change{{Pos: spot, ID: hand, Meta: meta}}, true
}
