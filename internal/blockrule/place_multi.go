package blockrule

// placeMulti 放门、床、双层植物这类占两格的方块，条件和 meta 照参考端 Door、Bed、DoublePlant 的 place。
// at 是放下去的那一格（床和门的下半、双层植物的下半）。
func placeMulti(view View, req PlaceRequest, hand byte, handMeta byte, at Pos) ([]Change, bool) {
	if isDoor(hand) {
		return placeDoor(view, req, hand, at)
	}
	if hand == BlockBed {
		return placeBed(view, req, at)
	}
	return placeDoublePlant(view, handMeta, at)
}

// doorSideFaces 参考端判断门轴用的两个邻居：按玩家朝向取 near 和 far 两格。
func doorSideFaces(direction Direction) (int32, int32) {
	switch direction {
	case DirectionEast:
		return FaceNorth, FaceSouth
	case DirectionSouth:
		return FaceEast, FaceWest
	case DirectionWest:
		return FaceSouth, FaceNorth
	}
	return FaceWest, FaceEast
}

// placeDoor 只能点方块顶面放。下半 meta 低 2 位是朝向，上半 meta 是 8，
// 旁边已经有一扇同种门，或者远侧是实心墙而近侧是空的，上半再加 1 表示门轴在另一边。
func placeDoor(view View, req PlaceRequest, hand byte, at Pos) ([]Change, bool) {
	if req.Face != FaceUp {
		return nil, false
	}
	var upPos Pos = at.Add(0, 1, 0)
	if upPos.PosY > 255 {
		return nil, false
	}
	var upID, upMeta = view.Block(upPos)
	var belowID, _ = view.Block(at.Add(0, -1, 0))
	if !IsReplaceable(upID, upMeta) || IsTransparent(belowID) {
		return nil, false
	}
	var direction Direction = DirectionFromYaw(req.Yaw)
	var nearFace, farFace = doorSideFaces(direction)
	var nearPos, _ = at.Side(nearFace)
	var farPos, _ = at.Side(farFace)
	var nearID, _ = view.Block(nearPos)
	var farID, _ = view.Block(farPos)
	var upperMeta byte = 8
	if nearID == hand || (!IsTransparent(farID) && IsTransparent(nearID)) {
		upperMeta |= 1
	}
	return []Change{
		{Pos: at, ID: hand, Meta: byte(direction) & 3},
		{Pos: upPos, ID: hand, Meta: upperMeta},
	}, true
}

// placeBed 床脚放在点击的那格，床头放在玩家正前方的一格。两格脚下都要有不透明方块。
// 床的颜色在物品的 meta 里，不在方块 meta 里，由服务端写进方块实体。
func placeBed(view View, req PlaceRequest, at Pos) ([]Change, bool) {
	var belowID, _ = view.Block(at.Add(0, -1, 0))
	if IsTransparent(belowID) {
		return nil, false
	}
	var direction Direction = DirectionFromYaw(req.Yaw)
	var stepX, stepZ = direction.Step()
	var headPos Pos = at.Add(stepX, 0, stepZ)
	var headID, headMeta = view.Block(headPos)
	var headBelowID, _ = view.Block(headPos.Add(0, -1, 0))
	if !IsReplaceable(headID, headMeta) || IsTransparent(headBelowID) {
		return nil, false
	}
	var rotation byte = byte(direction+3) & 3
	return []Change{
		{Pos: at, ID: BlockBed, Meta: rotation},
		{Pos: headPos, ID: BlockBed, Meta: rotation | 8},
	}, true
}

// placeDoublePlant 下半放在草或泥土上，上半放在它正上方，上半 meta 多一个 8。
func placeDoublePlant(view View, handMeta byte, at Pos) ([]Change, bool) {
	var belowID, _ = view.Block(at.Add(0, -1, 0))
	if belowID != BlockGrass && belowID != BlockDirt {
		return nil, false
	}
	var upPos Pos = at.Add(0, 1, 0)
	if upPos.PosY > 255 {
		return nil, false
	}
	var upID, upMeta = view.Block(upPos)
	if !IsReplaceable(upID, upMeta) {
		return nil, false
	}
	var variant byte = handMeta & 7
	return []Change{
		{Pos: at, ID: BlockDoublePlant, Meta: variant},
		{Pos: upPos, ID: BlockDoublePlant, Meta: variant | 8},
	}, true
}
