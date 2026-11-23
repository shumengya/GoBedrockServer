package blockrule

// 花盆、头颅、物品展示框、漏斗的摆放：朝向和能不能放都由被点击的面和支撑方块决定，与玩家偏航角无关。

// itemFrameMetaByFace 点击面 → 展示框 meta：面 2 北放成 3，面 3 南放成 2，面 4 西放成 1，面 5 东放成 0。
// 上下两面放不了，表里是 0 但走不到。
var itemFrameMetaByFace [6]byte = [6]byte{0, 0, 3, 2, 1, 0}

// itemFrameSupportByMeta 展示框背后的支撑在哪个面：meta 0 朝东靠西，1 朝西靠东，2 朝南靠北，3 朝北靠南。
var itemFrameSupportByMeta [4]int32 = [4]int32{FaceWest, FaceEast, FaceNorth, FaceSouth}

// placeFlowerPot 花盆要放在不透明的方块上，花盆本身也只有一个 meta 0。
func placeFlowerPot(view View, spot Pos) ([]Change, bool) {
	var belowID, _ = view.Block(spot.Add(0, -1, 0))
	if IsTransparent(belowID) {
		return nil, false
	}
	return []Change{{Pos: spot, ID: BlockFlowerPot, Meta: 0}}, true
}

// placeSkull 头颅贴着被点击的那个面放，meta 就是那个面；点底面放不了。
// 支撑方块必须是实心的，否则放下去立刻会被邻居更新打掉。
func placeSkull(view View, req PlaceRequest, spot Pos) ([]Change, bool) {
	if req.Face <= FaceDown {
		return nil, false
	}
	var support, _ = spot.Side(req.Face ^ 1)
	if IsTransparent(IDAt(view, support)) {
		return nil, false
	}
	return []Change{{Pos: spot, ID: BlockSkull, Meta: byte(req.Face)}}, true
}

// placeItemFrame 展示框只能贴在四个侧面上，背后还得是实心方块。
func placeItemFrame(view View, req PlaceRequest, spot Pos) ([]Change, bool) {
	if req.Face < FaceNorth {
		return nil, false
	}
	var meta byte = itemFrameMetaByFace[req.Face]
	var support, _ = spot.Side(itemFrameSupportByMeta[meta&3])
	if !IsSolid(IDAt(view, support)) {
		return nil, false
	}
	return []Change{{Pos: spot, ID: BlockItemFrame, Meta: meta}}, true
}

// hopperMeta 漏斗的出口：点侧面放时出口朝着被点的方块，点上下面都朝下。
func hopperMeta(face int32) byte {
	if face < FaceNorth {
		return 0
	}
	return byte(face ^ 1)
}

// placeHopper 漏斗看的是出口朝向，不挑支撑。
func placeHopper(spot Pos, face int32) ([]Change, bool) {
	return []Change{{Pos: spot, ID: BlockHopper, Meta: hopperMeta(face)}}, true
}
