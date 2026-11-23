package blockrule

// FlowerPotAccepts 花盆能不能种下这件东西。参考端 tile/FlowerPot::canAddItem 的白名单：
// 高草（排除 meta 1）、树苗、枯萎灌木、蒲公英、虞美人、
// 两种蘑菇、仙人掌。花盆自己有没有东西不在这里判断，由调用方看方块实体。
func FlowerPotAccepts(itemID int32, itemMeta int32) bool {
	switch itemID {
	case int32(BlockTallGrass):
		return itemMeta != 1
	case int32(BlockSapling),
		int32(BlockDeadBush),
		int32(BlockDandelion),
		int32(BlockPoppy),
		int32(BlockBrownMushroom),
		int32(BlockRedMushroom),
		int32(BlockCactus):
		return true
	}
	return false
}
