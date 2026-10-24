package blockrule

// isStair 是否为楼梯。
func isStair(id byte) bool {
	switch id {
	case BlockWoodStairs,
		BlockCobblestoneStairs,
		BlockBrickStairs,
		BlockStoneBrickStairs,
		BlockNetherBrickStairs,
		BlockSandstoneStairs,
		BlockSpruceStairs,
		BlockBirchStairs,
		BlockJungleStairs,
		BlockQuartzStairs,
		BlockAcaciaStairs,
		BlockDarkOakStairs,
		BlockRedSandstoneStairs,
		BlockPurpurStairs:
		return true
	}
	return false
}

// hasFacing 放下时正面朝向玩家的方块：箱子和两种熔炉。
// 陷阱箱、末影箱在参考端不继承箱子，没有朝向逻辑，所以不在这里。
func hasFacing(id byte) bool {
	return id == BlockChest || id == BlockFurnace || id == BlockBurningFurnace
}

// stairMeta 低 2 位是"往哪边升高"，bit 2 是倒置。
// 升高的一侧在玩家前方，所以玩家朝东，楼梯往东升（0），朝西往西升（1），朝南 2，朝北 3。
// 点击面的上半部分（顶面除外）或点击方块底面，楼梯倒置放。
func stairMeta(direction Direction, face int32, clickY float32) byte {
	var meta byte = 3
	switch direction {
	case DirectionEast:
		meta = 0
	case DirectionWest:
		meta = 1
	case DirectionSouth:
		meta = 2
	}
	if (clickY > 0.5 && face != FaceUp) || face == FaceDown {
		meta |= 4
	}
	return meta
}

// logMeta 低 2 位是木头种类，bit 2、3 是方向：贴着东西面放横向 X（4），贴着南北面放横向 Z（8），
// 上下面保持竖直（0）。
func logMeta(handMeta byte, face int32) byte {
	var axis byte = 0
	switch face {
	case FaceNorth, FaceSouth:
		axis = 8
	case FaceWest, FaceEast:
		axis = 4
	}
	return handMeta&3 | axis
}

// facingMeta 箱子、熔炉正面朝向玩家：玩家朝东，正面朝西，meta 取 5，其余同理。
func facingMeta(direction Direction) byte {
	switch direction {
	case DirectionEast:
		return 4
	case DirectionSouth:
		return 2
	case DirectionWest:
		return 5
	}
	return 3
}

// torchPlacementMeta 火把的 meta 表示它朝哪边倾斜：点哪个面，就贴在被点击方块的那个面上，
// 1 朝东、2 朝西、3 朝南、4 朝北，5 是立在方块顶上。点击的是不透明方块的侧面或顶面时按这个表放；
// 否则只有脚下是不透明方块（或围栏、石墙、红石灯这几种特例）才能立着放，立着放的 meta 是 0。
func torchPlacementMeta(view View, req PlaceRequest, at Pos, targetID byte) (byte, bool) {
	if !IsTransparent(targetID) && req.Face != FaceDown {
		switch req.Face {
		case FaceUp:
			return 5, true
		case FaceNorth:
			return 4, true
		case FaceSouth:
			return 3, true
		case FaceWest:
			return 2, true
		case FaceEast:
			return 1, true
		}
	}
	var belowID, _ = view.Block(at.Add(0, -1, 0))
	if !IsTransparent(belowID) {
		return 0, true
	}
	switch belowID {
	case BlockFence, BlockCobbleWall, BlockRedstoneLamp, BlockLitRedstoneLamp:
		return 0, true
	}
	return 0, false
}
