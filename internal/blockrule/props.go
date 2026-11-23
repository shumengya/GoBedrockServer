package blockrule

// Props 是方块的静态属性。数值来自参考端各 Block 类，由 scripts/gen_pm_blockprops
// 生成到 props_table.go。依赖 meta 的属性不在这里，见 IsReplaceable、BurnChance、BurnAbility。
type Props struct {
	// Solid 实体能不能站上去、能不能撞到。火把、花、水都不是。
	Solid bool
	// Transparent 不能给火把、拉杆这类附着方块当支撑。玻璃、树叶、台阶、箱子都算透明。
	Transparent bool
	// Replaceable 放别的方块时可以直接顶替，比如空气、水、高草。
	Replaceable bool
	// FlowInto 液体能流进去。
	FlowInto bool
	// BurnChance 被火点燃的几率，BurnAbility 是火在旁边时烧掉的几率，都是 0 表示不可燃。
	BurnChance  int
	BurnAbility int
	Opacity     byte
	DiffusesSky bool
	Emission    byte
}

// 表里没有的 ID 当成普通实心方块。
var defaultProps Props = Props{Solid: true, Opacity: 15}

// PropsOf 返回方块的静态属性。
func PropsOf(id byte) Props {
	var props, known = propsTable[id]
	if !known {
		return defaultProps
	}
	return props
}

func IsSolid(id byte) bool {
	return PropsOf(id).Solid
}

func IsTransparent(id byte) bool {
	return PropsOf(id).Transparent
}

func CanFlowInto(id byte) bool {
	return PropsOf(id).FlowInto
}

// IsReplaceable 判断这格能不能被新放的方块顶替。
// 雪片没叠满 8 层、双层植物里的高草和大型蕨的下半，参考端都按 meta 判定。
func IsReplaceable(id byte, meta byte) bool {
	switch id {
	case BlockSnowLayer:
		return meta < 7
	case BlockDoublePlant:
		return meta == 2 || meta == 3
	}
	return PropsOf(id).Replaceable
}

// 台阶 meta 的低 3 位是种类，2 是木头。
const slabKindWooden byte = 2

// BurnChance 是被火点燃的几率。台阶按种类区分；158 号木台阶参考端误继承了
// 台阶的判断，这里按原版固定为可燃。
func BurnChance(id byte, meta byte) int {
	if id == BlockSlab {
		if meta&7 == slabKindWooden {
			return 5
		}
		return 0
	}
	if id == BlockWoodSlab {
		return 5
	}
	return PropsOf(id).BurnChance
}

// BurnAbility 是火在旁边时这个方块被烧掉的几率。
func BurnAbility(id byte, meta byte) int {
	if id == BlockSlab {
		if meta&7 == slabKindWooden {
			return 5
		}
		return 0
	}
	if id == BlockWoodSlab {
		return 20
	}
	return PropsOf(id).BurnAbility
}
