package blockrule

// 参考端沿同类叶水平找三步，也接受路径末端下方一格的同组原木；不向上找。
// 搜到未加载的边界时保留待检查位，下次随机刻再判，而不把未知格子当空气。
func leafDecay(view View, spot Pos, leaf, meta byte) Update {
	if meta&12 != 8 {
		return Update{}
	}
	var log byte = BlockLog
	if leaf == BlockLeaves2 {
		log = BlockWood2
	}
	var frontier = []Pos{spot}
	var seen = map[Pos]bool{spot: true}
	var unknown = false
	for distance := 0; distance < 3 && len(frontier) > 0; distance++ {
		var next []Pos
		for _, current := range frontier {
			for face := int32(FaceDown); face <= FaceEast; face++ {
				if face == FaceUp {
					continue
				}
				var side, _ = current.Side(face)
				if side.PosY < 0 || seen[side] {
					continue
				}
				if !view.Loaded(side) {
					unknown = true
					continue
				}
				var neighbor = IDAt(view, side)
				if neighbor == log {
					return Update{Changes: []Change{{Pos: spot, ID: leaf, Meta: meta & 3, Quiet: true}}}
				}
				if face != FaceDown && neighbor == leaf {
					seen[side] = true
					next = append(next, side)
				}
			}
		}
		frontier = next
	}
	if unknown {
		return Update{}
	}
	return Update{Changes: []Change{{Pos: spot, ID: BlockAir, Drop: true}}}
}
