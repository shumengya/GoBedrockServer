package blockrule

func isWater(id byte) bool {
	return id == BlockFlowingWater || id == BlockStillWater
}

// isDoor 各种木门和铁门。
func isDoor(id byte) bool {
	switch id {
	case BlockWoodDoor,
		BlockIronDoor,
		BlockSpruceDoor,
		BlockBirchDoor,
		BlockJungleDoor,
		BlockAcaciaDoor,
		BlockDarkOakDoor:
		return true
	}
	return false
}

// isMultiCell 占两格的方块：门、床、双层植物。
func isMultiCell(id byte) bool {
	return isDoor(id) || id == BlockBed || id == BlockDoublePlant
}

func isCrop(id byte) bool {
	switch id {
	case BlockWheat, BlockCarrot, BlockPotato, BlockBeetroot, BlockPumpkinStem, BlockMelonStem:
		return true
	}
	return false
}

// supportedAt 判断 id 这种方块能不能放在 spot 这一格，条件照参考端各方块的 place。
// 没有特殊要求的方块一律返回 true。
func supportedAt(view View, id byte, spot Pos) bool {
	var belowPos Pos = spot.Add(0, -1, 0)
	var belowID, _ = view.Block(belowPos)
	switch id {
	case BlockDandelion, BlockPoppy:
		return belowID == BlockGrass || belowID == BlockDirt || belowID == BlockFarmland
	case BlockSapling:
		return belowID == BlockGrass || belowID == BlockDirt || belowID == BlockFarmland || belowID == BlockPodzol
	case BlockTallGrass:
		return belowID == BlockGrass || belowID == BlockDirt
	case BlockDeadBush:
		return belowID == BlockSand || belowID == BlockPodzol || belowID == BlockHardenedClay || belowID == BlockStainedClay
	case BlockBrownMushroom, BlockRedMushroom:
		return !IsTransparent(belowID)
	case BlockSugarcane:
		return sugarcaneSupported(view, belowPos, belowID)
	case BlockCactus:
		return cactusSupported(view, spot, belowID)
	}
	if isCrop(id) {
		return belowID == BlockFarmland
	}
	return true
}

// sugarcaneSupported 甘蔗接在甘蔗上，或者立在草、泥土、沙子上且这块地的四周有水。
func sugarcaneSupported(view View, belowPos Pos, belowID byte) bool {
	if belowID == BlockSugarcane {
		return true
	}
	if belowID != BlockGrass && belowID != BlockDirt && belowID != BlockSand {
		return false
	}
	var face int32 = 0
	for face = FaceNorth; face <= FaceEast; face++ {
		var side, _ = belowPos.Side(face)
		if isWater(IDAt(view, side)) {
			return true
		}
	}
	return false
}

// cactusSupported 仙人掌立在沙子或仙人掌上，四周四格都不能是实心方块。
func cactusSupported(view View, spot Pos, belowID byte) bool {
	if belowID != BlockSand && belowID != BlockCactus {
		return false
	}
	return !solidAround(view, spot)
}

// solidAround 四个水平邻居里有没有实心方块。
func solidAround(view View, spot Pos) bool {
	var face int32 = 0
	for face = FaceNorth; face <= FaceEast; face++ {
		var side, _ = spot.Side(face)
		if IsSolid(IDAt(view, side)) {
			return true
		}
	}
	return false
}
