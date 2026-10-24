package blockrule

func isTrapdoor(id byte) bool {
	return id == BlockTrapdoor || id == BlockIronTrapdoor
}

func isFenceGate(id byte) bool {
	switch id {
	case BlockFenceGate,
		BlockFenceGateSpruce,
		BlockFenceGateBirch,
		BlockFenceGateJungle,
		BlockFenceGateDarkOak,
		BlockFenceGateAcacia:
		return true
	}
	return false
}

func isButton(id byte) bool {
	return id == BlockStoneButton || id == BlockWoodButton
}

// isInteractive 能右键切换状态、放置时 meta 带朝向或贴附面的方块。
func isInteractive(id byte) bool {
	return isTrapdoor(id) || isFenceGate(id) || isButton(id) || id == BlockLever
}

// interactivePlacementMeta 给活板门、栅栏门、拉杆、按钮算放置时的 meta，条件照参考端各类的 place。
// 第二个返回值为 false 表示放不了：拉杆和按钮必须贴在不透明的方块上。
func interactivePlacementMeta(req PlaceRequest, hand byte, targetID byte) (byte, bool) {
	var direction Direction = DirectionFromYaw(req.Yaw)
	if isTrapdoor(hand) {
		return trapdoorMeta(direction, req), true
	}
	if isFenceGate(hand) {
		return byte((int(direction) + 3) & 3), true
	}
	if IsTransparent(targetID) {
		return 0, false
	}
	if hand == BlockLever {
		return leverMeta(direction, req.Face), true
	}
	// 按钮的 meta 就是被点击的面。
	return byte(req.Face), true
}

// trapdoorMeta 低 2 位是朝向，bit 2 是吊在上半格；点方块底面，或点侧面的上半部分，放上半格。
func trapdoorMeta(direction Direction, req PlaceRequest) byte {
	var meta byte = 0
	switch direction {
	case DirectionEast:
		meta = 1
	case DirectionSouth:
		meta = 3
	case DirectionNorth:
		meta = 2
	}
	if (req.ClickY > 0.5 && req.Face != FaceUp) || req.Face == FaceDown {
		meta |= 4
	}
	return meta
}

// leverMeta 点侧面时贴在那面墙上（1 东、2 西、3 南、4 北）；
// 点顶面或底面时立在地上（5、6）或挂在天花板上（0、7），两种朝向由玩家朝东西还是南北决定。
func leverMeta(direction Direction, face int32) byte {
	var alongX bool = direction == DirectionEast || direction == DirectionWest
	switch face {
	case FaceDown:
		if alongX {
			return 0
		}
		return 7
	case FaceUp:
		if alongX {
			return 6
		}
		return 5
	case FaceSouth:
		return 3
	case FaceNorth:
		return 4
	case FaceWest:
		return 2
	}
	return 1
}
