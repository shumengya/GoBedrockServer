package blockrule

// 锄头的物品 ID 是连续的一段：木、石、铁、钻石、金。
const itemHoeFirst int32 = 290
const itemHoeLast int32 = 294

// 染料物品 351，meta 15 是骨粉。
const itemDye int32 = 351
const itemDyeBoneMeal int32 = 15

// ItemUseRequest 玩家拿着不是方块的物品（锄头、骨粉）点击了世界里的某一格。
type ItemUseRequest struct {
	ItemID   int32
	ItemMeta int32
	Target   Pos
	Face     int32
}

// ItemUse 是这次使用的结果：要改的方块，要不要消耗一个物品，工具要磨损多少耐久。
type ItemUse struct {
	Update  Update
	Consume bool
	Wear    int32
}

func isHoe(itemID int32) bool {
	return itemID >= itemHoeFirst && itemID <= itemHoeLast
}

// 骨粉撒在草方块上长出来的植物：蒲公英、红花各一份，高草四份。
var growGrassPlants [6][2]byte = [6][2]byte{
	{BlockDandelion, 0},
	{BlockPoppy, 0},
	{BlockTallGrass, 1},
	{BlockTallGrass, 1},
	{BlockTallGrass, 1},
	{BlockTallGrass, 1},
}

// 骨粉撒草：试 8 次，每次在草方块四周 ±2 格的范围里挑一格。
const growGrassAttempts int = 8
const growGrassRadius int = 2

// UseItem 处理锄头、骨粉点击方块。第二个返回值为 false 表示这个物品对这个方块没有作用，
// 调用方接着走放置流程；为 true 表示点击被消耗了。
func UseItem(view View, req ItemUseRequest, random Random) (ItemUse, bool) {
	var id, meta = view.Block(req.Target)
	if isHoe(req.ItemID) {
		return useHoe(view, req, id)
	}
	if req.ItemID == itemDye && req.ItemMeta == itemDyeBoneMeal {
		return useBoneMeal(view, req.Target, id, meta, random)
	}
	return ItemUse{}, false
}

// useHoe 锄头点草方块或泥土(不能点底面，上面要是空气)变成耕地，磨损 1 点耐久。
// 参考端泥土不分普通和粗泥土，一律能耕；粗泥土在原版里只会变回普通泥土，这里没区分。
func useHoe(view View, req ItemUseRequest, id byte) (ItemUse, bool) {
	if (id != BlockGrass && id != BlockDirt) || req.Face == FaceDown {
		return ItemUse{}, false
	}
	if IDAt(view, req.Target.Add(0, 1, 0)) != BlockAir {
		return ItemUse{}, false
	}
	return ItemUse{
		Update: Update{Changes: []Change{{Pos: req.Target, ID: BlockFarmland, Meta: 0}}},
		Wear:   1,
	}, true
}

// useBoneMeal 骨粉对作物和瓜茎：一次加 2~5 个阶段，最多到 7；对草方块：撒一把花草。
// 树苗的骨粉催熟要等树的生成器，这里先不处理。已经成熟的作物不消耗骨粉（参考端会白白吃掉一个）。
func useBoneMeal(view View, target Pos, id byte, meta byte, random Random) (ItemUse, bool) {
	if isCrop(id) {
		if meta >= cropRipeStage {
			return ItemUse{}, false
		}
		var grown byte = min(cropRipeStage, meta+byte(2+random.IntN(4)))
		return ItemUse{
			Update:  Update{Changes: []Change{{Pos: target, ID: id, Meta: grown}}},
			Consume: true,
		}, true
	}
	if id == BlockGrass {
		return ItemUse{Update: Update{Changes: growGrass(view, target, random)}, Consume: true}, true
	}
	return ItemUse{}, false
}

// growGrass 对应参考端 TallGrassObject::growGrass。每次先抽 x、z 两个偏移，
// 目标格是草方块、上面是空气才再抽一次挑哪种植物；同一格上面只放一株。
func growGrass(view View, target Pos, random Random) []Change {
	var changes []Change = nil
	var placed map[Pos]bool = map[Pos]bool{}
	var attempt int = 0
	for attempt = 0; attempt < growGrassAttempts; attempt++ {
		var offsetX int32 = int32(random.IntN(2*growGrassRadius+1) - growGrassRadius)
		var offsetZ int32 = int32(random.IntN(2*growGrassRadius+1) - growGrassRadius)
		var ground Pos = Pos{PosX: target.PosX + offsetX, PosY: target.PosY, PosZ: target.PosZ + offsetZ}
		var above Pos = ground.Add(0, 1, 0)
		if above.PosY > 255 || !view.Loaded(ground) || !view.Loaded(above) || placed[above] {
			continue
		}
		if IDAt(view, ground) != BlockGrass || IDAt(view, above) != BlockAir {
			continue
		}
		var plant [2]byte = growGrassPlants[random.IntN(len(growGrassPlants))]
		changes = append(changes, Change{Pos: above, ID: plant[0], Meta: plant[1]})
		placed[above] = true
	}
	return changes
}
