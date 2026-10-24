package blockrule

// 物品 ID，核对自 internal/item 的注册表。
const itemWheatSeeds int32 = 295
const itemSugarcane int32 = 338
const itemPumpkinSeeds int32 = 361
const itemMelonSeeds int32 = 362
const itemCarrot int32 = 391
const itemPotato int32 = 392
const itemBeetrootSeeds int32 = 458
const itemWoodenDoor int32 = 324
const itemIronDoor int32 = 330
const itemSpruceDoor int32 = 427
const itemBirchDoor int32 = 428
const itemJungleDoor int32 = 429
const itemAcaciaDoor int32 = 430
const itemDarkOakDoor int32 = 431
const itemBed int32 = 355

// BlockForItem 返回手持物放下去会变成的方块 ID。
// ID 小于 256 的物品本身就是方块；种子、门、床、甘蔗这类物品的 ID 和方块不同，查表。
// 返回 false 表示这个物品不是放方块用的（工具、食物等）。
func BlockForItem(itemID int32) (byte, bool) {
	if itemID <= 0 {
		return BlockAir, false
	}
	if itemID < 256 {
		return byte(itemID), true
	}
	switch itemID {
	case itemWheatSeeds:
		return BlockWheat, true
	case itemSugarcane:
		return BlockSugarcane, true
	case itemPumpkinSeeds:
		return BlockPumpkinStem, true
	case itemMelonSeeds:
		return BlockMelonStem, true
	case itemCarrot:
		return BlockCarrot, true
	case itemPotato:
		return BlockPotato, true
	case itemBeetrootSeeds:
		return BlockBeetroot, true
	case itemWoodenDoor:
		return BlockWoodDoor, true
	case itemIronDoor:
		return BlockIronDoor, true
	case itemSpruceDoor:
		return BlockSpruceDoor, true
	case itemBirchDoor:
		return BlockBirchDoor, true
	case itemJungleDoor:
		return BlockJungleDoor, true
	case itemAcaciaDoor:
		return BlockAcaciaDoor, true
	case itemDarkOakDoor:
		return BlockDarkOakDoor, true
	case itemBed:
		return BlockBed, true
	}
	return BlockAir, false
}
