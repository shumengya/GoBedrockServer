package blockrule

// Random 是随机刻用的随机源，IntN(count) 返回 [0, count) 里的整数。
type Random interface {
	IntN(count int) int
}

type RandomView interface {
	View
	Light(Pos) (byte, byte)
}

// 作物每次被随机刻选中，有三分之一的机会长一个阶段，长到 7 就成熟。
const cropGrowthOutOf int = 3
const cropRipeStage byte = 7

// 甘蔗、仙人掌的 meta 当作生长计数，数到 15 才往上长一格。
const columnRipeCount byte = 15

// IsRandomTicked 这种方块要不要参与随机刻。服务端据此先筛一遍，大部分格子不用往下评估。
func IsRandomTicked(id byte) bool {
	switch id {
	case BlockIce, BlockSnowLayer, BlockFire, BlockSapling, BlockLeaves, BlockLeaves2, BlockWheat,
		BlockCarrot,
		BlockPotato,
		BlockBeetroot,
		BlockPumpkinStem,
		BlockMelonStem,
		BlockSugarcane,
		BlockCactus,
		BlockGrass,
		BlockMycelium:
		return true
	}
	return false
}

// OnRandomTick 评估被随机刻选中的 spot。同一次调用里随机数的取用顺序是固定的，测试靠这个编排。
func OnRandomTick(view RandomView, spot Pos, random Random) Update {
	if !view.Loaded(spot) {
		return Update{}
	}
	var id, meta = view.Block(spot)
	switch id {
	case BlockIce, BlockSnowLayer:
		var face int32 = FaceDown
		for face = FaceDown; face <= FaceEast; face++ {
			var side, _ = spot.Side(face)
			var _, light = view.Light(side)
			if light >= 12 {
				var next byte = BlockAir
				if id == BlockIce {
					next = BlockFlowingWater
				}
				return Update{Changes: []Change{{Pos: spot, ID: next}}}
			}
		}
	case BlockFire:
		return fireWake(view, spot, fireDelay+random.IntN(11))
	case BlockSapling:
		var light, _ = view.Light(spot)
		if light < 8 || random.IntN(7) != 0 {
			return Update{}
		}
		if meta&8 == 0 {
			return Update{Changes: []Change{{Pos: spot, ID: id, Meta: meta | 8, Quiet: true}}}
		}
		return growSapling(view, spot, meta&7, random)
	case BlockLeaves, BlockLeaves2:
		return leafDecay(view, spot, id, meta)
	case BlockWheat, BlockCarrot, BlockPotato, BlockBeetroot:
		return cropGrowth(spot, id, meta, random)
	case BlockPumpkinStem:
		return stemGrowth(view, spot, id, meta, BlockPumpkin, random)
	case BlockMelonStem:
		return stemGrowth(view, spot, id, meta, BlockMelonBlock, random)
	case BlockSugarcane:
		return sugarcaneGrowth(view, spot, meta)
	case BlockCactus:
		return cactusGrowth(view, spot, meta)
	case BlockGrass:
		return grassSpread(view, spot, random)
	case BlockMycelium:
		return myceliumSpread(view, spot, random)
	}
	return Update{}
}

func growOneStage(spot Pos, id byte, meta byte) Update {
	return Update{Changes: []Change{{Pos: spot, ID: id, Meta: meta + 1}}}
}

// cropGrowth 小麦、胡萝卜、马铃薯、甜菜：三分之一的机会长一个阶段，到 7 为止。
func cropGrowth(spot Pos, id byte, meta byte, random Random) Update {
	if random.IntN(cropGrowthOutOf) != 1 || meta >= cropRipeStage {
		return Update{}
	}
	return growOneStage(spot, id, meta)
}

// stemGrowth 南瓜茎、西瓜茎：先像作物一样长到 7；长满之后，四周没有果实时，
// 随机挑一个水平方向，那格是空气并且脚下是耕地、草或泥土，就结一个果实。
func stemGrowth(view View, spot Pos, id byte, meta byte, fruit byte, random Random) Update {
	if random.IntN(cropGrowthOutOf) != 1 {
		return Update{}
	}
	if meta < cropRipeStage {
		return growOneStage(spot, id, meta)
	}
	var face int32 = 0
	for face = FaceNorth; face <= FaceEast; face++ {
		var side, _ = spot.Side(face)
		if IDAt(view, side) == fruit {
			return Update{}
		}
	}
	var target, _ = spot.Side(FaceNorth + int32(random.IntN(4)))
	if !view.Loaded(target) || IDAt(view, target) != BlockAir {
		return Update{}
	}
	var groundID byte = IDAt(view, target.Add(0, -1, 0))
	if groundID != BlockFarmland && groundID != BlockGrass && groundID != BlockDirt {
		return Update{}
	}
	return Update{Changes: []Change{{Pos: target, ID: fruit, Meta: 0}}}
}

// sugarcaneGrowth 只有一根甘蔗的最底下那格会长：计数加一，数满 15 就往上接一格并清零。
// 往上最多看两格：已经是甘蔗的跳过，遇到空气就接上去，遇到别的东西就停，所以高度最多是 3。
// 参考端遇到别的东西会接着看更上面一格，可能在石头上方凭空接出一截，这里不学这个。
func sugarcaneGrowth(view View, spot Pos, meta byte) Update {
	if IDAt(view, spot.Add(0, -1, 0)) == BlockSugarcane {
		return Update{}
	}
	if meta < columnRipeCount {
		return growOneStage(spot, BlockSugarcane, meta)
	}
	var changes []Change = []Change{{Pos: spot, ID: BlockSugarcane, Meta: 0}}
	var height int32 = 0
	for height = 1; height <= 2; height++ {
		var target Pos = spot.Add(0, height, 0)
		var targetID byte = IDAt(view, target)
		if targetID == BlockSugarcane {
			continue
		}
		if target.PosY <= 255 && view.Loaded(target) && targetID == BlockAir {
			changes = append(changes, Change{Pos: target, ID: BlockSugarcane, Meta: 0})
		}
		break
	}
	return Update{Changes: changes}
}

// cactusGrowth 和甘蔗一样只有最底下那格会长，数满 15 之后往上接，最多接两格；
// 某一格不是空气，或者四周有实心方块（长出来也活不了），就停在那里。
func cactusGrowth(view View, spot Pos, meta byte) Update {
	if IDAt(view, spot.Add(0, -1, 0)) == BlockCactus {
		return Update{}
	}
	if meta < columnRipeCount {
		return growOneStage(spot, BlockCactus, meta)
	}
	var changes []Change = []Change{{Pos: spot, ID: BlockCactus, Meta: 0}}
	var height int32 = 0
	for height = 1; height <= 2; height++ {
		var target Pos = spot.Add(0, height, 0)
		if target.PosY > 255 || !view.Loaded(target) || IDAt(view, target) != BlockAir || solidAround(view, target) {
			break
		}
		changes = append(changes, Change{Pos: target, ID: BlockCactus, Meta: 0})
	}
	return Update{Changes: changes}
}

// bareDirtUnderOpenSky 可以长草或菌丝的泥土：普通泥土(meta 0，粗泥土不算)，
// 上面是透明的非液体方块。参考端的透明表把液体也算进去，水底下长草不对，这里排除。
func bareDirtUnderOpenSky(view View, target Pos) bool {
	if !view.Loaded(target) || target.PosY < 0 || target.PosY > 254 {
		return false
	}
	var id, meta = view.Block(target)
	if id != BlockDirt || meta != 0 {
		return false
	}
	var aboveID byte = IDAt(view, target.Add(0, 1, 0))
	return IsTransparent(aboveID) && !isLiquid(aboveID)
}

// grassSpread 草方块试四次：在 x±1、y-3~y+1、z±1 的范围里随机挑一格，是裸露的泥土就长成草。
func grassSpread(view RandomView, spot Pos, random Random) Update {
	var above = spot.Add(0, 1, 0)
	var light, _ = view.Light(above)
	if light < 4 && PropsOf(IDAt(view, above)).Opacity >= 2 {
		return Update{Changes: []Change{{Pos: spot, ID: BlockDirt}}}
	}
	if light < 9 {
		return Update{}
	}
	var changes []Change = nil
	var attempt int = 0
	for attempt = 0; attempt < 4; attempt++ {
		var target Pos = Pos{
			PosX: spot.PosX - 1 + int32(random.IntN(3)),
			PosY: spot.PosY - 3 + int32(random.IntN(5)),
			PosZ: spot.PosZ - 1 + int32(random.IntN(3)),
		}
		if bareDirtUnderOpenSky(view, target) {
			var top = target.Add(0, 1, 0)
			var targetLight, _ = view.Light(top)
			if targetLight < 4 || PropsOf(IDAt(view, top)).Opacity >= 2 {
				continue
			}
			changes = append(changes, Change{Pos: target, ID: BlockGrass, Meta: 0})
		}
	}
	return Update{Changes: changes}
}

// myceliumSpread 菌丝每次试一格：x±1、y±2、z±1，是裸露的泥土就长成菌丝。
func myceliumSpread(view View, spot Pos, random Random) Update {
	var target Pos = Pos{
		PosX: spot.PosX - 1 + int32(random.IntN(3)),
		PosY: spot.PosY - 2 + int32(random.IntN(5)),
		PosZ: spot.PosZ - 1 + int32(random.IntN(3)),
	}
	if !bareDirtUnderOpenSky(view, target) {
		return Update{}
	}
	return Update{Changes: []Change{{Pos: target, ID: BlockMycelium, Meta: 0}}}
}
