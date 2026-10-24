package blockrule

// OnNeighborChange 在 pos 周围的方块变了之后，判断 pos 这一格要不要跟着变。
// 返回 nil 表示不用动。它是被动的：没人去点它，所以遇到没加载的区块一律保守，
// 当成"没问题"，免得因为读到空气就把边界上的火把、作物打掉。
func OnNeighborChange(view View, pos Pos) []Change {
	if !view.Loaded(pos) {
		return nil
	}
	var id, meta = view.Block(pos)
	var broken bool = false
	switch id {
	case BlockTorch:
		broken = torchBroken(view, pos, meta)
	case BlockDandelion, BlockPoppy, BlockSapling, BlockTallGrass, BlockDeadBush, BlockBrownMushroom, BlockRedMushroom:
		broken = plantBroken(view, pos, id)
	case BlockSugarcane:
		broken = sugarcaneBroken(view, pos)
	case BlockCactus:
		broken = cactusBroken(view, pos)
	case BlockLever:
		broken = attachedBroken(view, pos, leverSupportFace(meta))
	case BlockStoneButton, BlockWoodButton:
		broken = buttonBroken(view, pos, meta)
	}
	if isCrop(id) {
		broken = cropBroken(view, pos)
	}
	if isMultiCell(id) {
		broken = pairBroken(view, pos, id, meta)
	}
	if !broken {
		return nil
	}
	return []Change{{Pos: pos, ID: BlockAir, Meta: 0, Drop: true}}
}

// 邻居没加载就返回 false，调用方据此跳过判断。
func neighborBlock(view View, pos Pos, face int32) (byte, bool) {
	var side, sideOK = pos.Side(face)
	if !sideOK || !view.Loaded(side) {
		return BlockAir, false
	}
	return IDAt(view, side), true
}

// torchBroken 火把靠着的那一面变成透明方块（空气、水、玻璃…）就掉下来。
// meta 1~4 是贴墙的火把，靠着的是它反方向的墙；0 和 5 是立在脚下方块上的。
// 围栏、石墙、红石灯虽然透明，但参考端允许火把立在上面。
func torchBroken(view View, pos Pos, meta byte) bool {
	var face int32 = FaceDown
	switch meta {
	case 1:
		face = FaceWest
	case 2:
		face = FaceEast
	case 3:
		face = FaceNorth
	case 4:
		face = FaceSouth
	}
	var supportID, loaded = neighborBlock(view, pos, face)
	if !loaded || !IsTransparent(supportID) {
		return false
	}
	if face == FaceDown {
		switch supportID {
		case BlockFence, BlockCobbleWall, BlockRedstoneLamp, BlockLitRedstoneLamp:
			return false
		}
	}
	return true
}

// plantBroken 花、树苗、高草、枯萎灌木、蘑菇：脚下变成透明方块就掉下来。
// 参考端的透明表把耕地算作透明，但花和树苗可以种在耕地上，所以耕地要单独放过。
func plantBroken(view View, pos Pos, id byte) bool {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	if !loaded {
		return false
	}
	if belowID == BlockFarmland && id != BlockBrownMushroom && id != BlockRedMushroom {
		return false
	}
	return IsTransparent(belowID)
}

// sugarcaneBroken 甘蔗脚下是透明方块且不是甘蔗就掉下来。
func sugarcaneBroken(view View, pos Pos) bool {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	if !loaded {
		return false
	}
	return IsTransparent(belowID) && belowID != BlockSugarcane
}

// cactusBroken 仙人掌脚下不是沙子或仙人掌，或者旁边出现实心方块，就掉下来。
func cactusBroken(view View, pos Pos) bool {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	if loaded && belowID != BlockSand && belowID != BlockCactus {
		return true
	}
	var face int32 = 0
	for face = FaceNorth; face <= FaceEast; face++ {
		var sideID, sideLoaded = neighborBlock(view, pos, face)
		if sideLoaded && IsSolid(sideID) {
			return true
		}
	}
	return false
}

// cropBroken 作物和瓜茎脚下必须是耕地。
func cropBroken(view View, pos Pos) bool {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	if !loaded {
		return false
	}
	return belowID != BlockFarmland
}

// attachedBroken 靠在某个面上的方块（拉杆、按钮），靠着的那面变成透明方块就掉下来。
func attachedBroken(view View, pos Pos, face int32) bool {
	var supportID, loaded = neighborBlock(view, pos, face)
	return loaded && IsTransparent(supportID)
}

// leverSupportFace 拉杆 meta 低 3 位对应它靠着的那个面：
// 0 和 7 挂在天花板，1 靠西墙，2 靠东墙，3 靠北墙，4 靠南墙，5 和 6 立在地上。
func leverSupportFace(meta byte) int32 {
	switch meta & 7 {
	case 1:
		return FaceWest
	case 2:
		return FaceEast
	case 3:
		return FaceNorth
	case 4:
		return FaceSouth
	case 5, 6:
		return FaceDown
	}
	return FaceUp
}

// buttonBroken 按钮的 meta 低 3 位是被点击的面，它靠着的是那个面的对面。
// 面编号 0/1、2/3、4/5 两两相对，异或 1 就是对面。meta 超出 0~5 说明数据不对，不动它。
func buttonBroken(view View, pos Pos, meta byte) bool {
	var face int32 = int32(meta & 7)
	if face > FaceEast {
		return false
	}
	return attachedBroken(view, pos, face^1)
}
