package server

import "go-mc-server/internal/protocol"

const toolNone int = 0
const toolPickaxe int = 1
const toolAxe int = 2
const toolShovel int = 3
const toolSword int = 4
const toolShears int = 5
const toolHoe int = 6

// 等级数值对齐 PocketMine Tool::TIER_*：金镐采集等级只比木镐高，挖不了铁矿。
const tierNone int = 0
const tierWood int = 1
const tierGold int = 2
const tierStone int = 3
const tierIron int = 4
const tierDiamond int = 5

type toolInfo struct {
	Kind          int
	Tier          int
	Speed         float64
	MaxDurability int32
	AttackDamage  float32
}

type blockInfo struct {
	Hardness      float64
	PreferredTool int
	RequiredTool  int
	RequiredTier  int
	Unbreakable   bool
}

const itemIronShovel int32 = 256
const itemIronPickaxe int32 = 257
const itemIronAxe int32 = 258
const itemIronSword int32 = 267
const itemWoodenSword int32 = 268
const itemWoodenShovel int32 = 269
const itemWoodenPickaxe int32 = 270
const itemWoodenAxe int32 = 271
const itemStoneSword int32 = 272
const itemStoneShovel int32 = 273
const itemStonePickaxe int32 = 274
const itemStoneAxe int32 = 275
const itemDiamondSword int32 = 276
const itemDiamondShovel int32 = 277
const itemDiamondPickaxe int32 = 278
const itemDiamondAxe int32 = 279
const itemGoldSword int32 = 283
const itemGoldShovel int32 = 284
const itemGoldPickaxe int32 = 285
const itemGoldAxe int32 = 286
const itemShears int32 = 359
const itemWoodenHoe int32 = 290
const itemStoneHoe int32 = 291
const itemIronHoe int32 = 292
const itemDiamondHoe int32 = 293
const itemGoldHoe int32 = 294

// 硬度与工具规则来自 PocketMine 参考端各 Block 类的 getHardness/getToolType。
// RequiredTool/RequiredTier 取参考端 getDrops 里 isPickaxe() >= TIER 的门槛；
// 门槛不足时方块仍可挖掉，只是没有掉落。
var blockInfoTable map[byte]blockInfo = map[byte]blockInfo{
	0:   {Hardness: 0, Unbreakable: true},
	1:   {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	2:   {Hardness: 0.6, PreferredTool: toolShovel},
	3:   {Hardness: 0.5, PreferredTool: toolShovel},
	4:   {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	5:   {Hardness: 2, PreferredTool: toolAxe},
	6:   {Hardness: 0},
	7:   {Hardness: -1, Unbreakable: true},
	12:  {Hardness: 0.5, PreferredTool: toolShovel},
	13:  {Hardness: 0.6, PreferredTool: toolShovel},
	14:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	15:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierStone},
	16:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	17:  {Hardness: 2, PreferredTool: toolAxe},
	18:  {Hardness: 0.2, PreferredTool: toolShears},
	20:  {Hardness: 0.3},
	21:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierStone},
	22:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierStone},
	23:  {Hardness: 3.5, PreferredTool: toolPickaxe},
	24:  {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	25:  {Hardness: 0.8, PreferredTool: toolAxe},
	26:  {Hardness: 0.2},
	27:  {Hardness: 0.7},
	28:  {Hardness: 0.7},
	29:  {Hardness: 1, PreferredTool: toolAxe},
	30:  {Hardness: 4, PreferredTool: toolShears},
	31:  {Hardness: 0, PreferredTool: toolShears},
	32:  {Hardness: 0},
	33:  {Hardness: 1, PreferredTool: toolAxe},
	34:  {Hardness: 10},
	35:  {Hardness: 0.8, PreferredTool: toolShears},
	37:  {Hardness: 0},
	38:  {Hardness: 0},
	39:  {Hardness: 0},
	40:  {Hardness: 0},
	41:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	42:  {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierStone},
	43:  {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	44:  {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	45:  {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	46:  {Hardness: 0},
	47:  {Hardness: 1.5, PreferredTool: toolAxe},
	48:  {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	49:  {Hardness: 35, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierDiamond},
	50:  {Hardness: 0},
	51:  {Hardness: 0},
	52:  {Hardness: 5, PreferredTool: toolPickaxe},
	53:  {Hardness: 2, PreferredTool: toolAxe},
	54:  {Hardness: 2.5, PreferredTool: toolAxe},
	56:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	57:  {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	58:  {Hardness: 2.5, PreferredTool: toolAxe},
	59:  {Hardness: 0},
	60:  {Hardness: 0.6, PreferredTool: toolShovel},
	61:  {Hardness: 3.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	62:  {Hardness: 3.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	63:  {Hardness: 1, PreferredTool: toolAxe},
	64:  {Hardness: 3, PreferredTool: toolAxe},
	65:  {Hardness: 0.4, PreferredTool: toolAxe},
	66:  {Hardness: 0.7},
	67:  {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	68:  {Hardness: 1, PreferredTool: toolAxe},
	69:  {Hardness: 0.5},
	70:  {Hardness: 0.5},
	71:  {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	72:  {Hardness: 0.5},
	73:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	74:  {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	77:  {Hardness: 0.5},
	78:  {Hardness: 0.1, PreferredTool: toolShovel},
	79:  {Hardness: 0.5, PreferredTool: toolPickaxe},
	80:  {Hardness: 0.2, PreferredTool: toolShovel},
	81:  {Hardness: 0.4},
	82:  {Hardness: 0.6, PreferredTool: toolShovel},
	83:  {Hardness: 0},
	85:  {Hardness: 2, PreferredTool: toolAxe},
	86:  {Hardness: 1, PreferredTool: toolAxe},
	87:  {Hardness: 0.4, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	88:  {Hardness: 0.5, PreferredTool: toolShovel},
	89:  {Hardness: 0.3, PreferredTool: toolPickaxe},
	90:  {Hardness: -1, PreferredTool: toolPickaxe},
	91:  {Hardness: 1, PreferredTool: toolAxe},
	92:  {Hardness: 0.5},
	94:  {Hardness: 10},
	95:  {Hardness: -1},
	96:  {Hardness: 3, PreferredTool: toolAxe},
	98:  {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	99:  {Hardness: 0.2},
	100: {Hardness: 0.2},
	101: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	102: {Hardness: 0.3},
	103: {Hardness: 1, PreferredTool: toolAxe},
	104: {Hardness: 0},
	105: {Hardness: 0},
	106: {Hardness: 0.2, PreferredTool: toolShears},
	107: {Hardness: 2, PreferredTool: toolAxe},
	108: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	109: {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	110: {Hardness: 0.6, PreferredTool: toolShovel},
	111: {Hardness: 0.6},
	112: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	113: {Hardness: 2, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	114: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	115: {Hardness: 0},
	116: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	117: {Hardness: 0.5, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	118: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	119: {Hardness: -1, Unbreakable: true},
	120: {Hardness: -1},
	121: {Hardness: 3, PreferredTool: toolPickaxe},
	122: {Hardness: -1, Unbreakable: true},
	125: {Hardness: 3.5, PreferredTool: toolPickaxe},
	126: {Hardness: 0.7},
	127: {Hardness: 0.2},
	128: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	129: {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	130: {Hardness: 22.5, PreferredTool: toolPickaxe},
	131: {Hardness: 0},
	132: {Hardness: 0, PreferredTool: toolShears},
	133: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierIron},
	134: {Hardness: 2, PreferredTool: toolAxe},
	135: {Hardness: 2, PreferredTool: toolAxe},
	136: {Hardness: 2, PreferredTool: toolAxe},
	137: {Hardness: -1},
	138: {Hardness: 3},
	139: {Hardness: 2, PreferredTool: toolPickaxe},
	140: {Hardness: 0},
	141: {Hardness: 0},
	142: {Hardness: 0},
	143: {Hardness: 0.5},
	144: {Hardness: 1},
	145: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	146: {Hardness: 2.5, PreferredTool: toolAxe},
	147: {Hardness: 0.5},
	148: {Hardness: 0.5},
	151: {Hardness: 0.2},
	152: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	153: {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	154: {Hardness: 3, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	155: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	156: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	157: {Hardness: 2, PreferredTool: toolAxe},
	158: {Hardness: 2, PreferredTool: toolAxe},
	159: {Hardness: 1.25, PreferredTool: toolPickaxe},
	161: {Hardness: 0.2, PreferredTool: toolShears},
	162: {Hardness: 2, PreferredTool: toolAxe},
	163: {Hardness: 2, PreferredTool: toolAxe},
	164: {Hardness: 2, PreferredTool: toolAxe},
	165: {Hardness: 0},
	167: {Hardness: 5},
	168: {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	169: {Hardness: 0.3},
	170: {Hardness: 0.5},
	171: {Hardness: 0.1},
	172: {Hardness: 1.25, PreferredTool: toolPickaxe},
	173: {Hardness: 5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	174: {Hardness: 0.5, PreferredTool: toolPickaxe},
	175: {Hardness: 0},
	178: {Hardness: 0.2},
	183: {Hardness: 2, PreferredTool: toolAxe},
	184: {Hardness: 2, PreferredTool: toolAxe},
	185: {Hardness: 2, PreferredTool: toolAxe},
	186: {Hardness: 2, PreferredTool: toolAxe},
	187: {Hardness: 2, PreferredTool: toolAxe},
	193: {Hardness: 3, PreferredTool: toolAxe},
	194: {Hardness: 3, PreferredTool: toolAxe},
	195: {Hardness: 3, PreferredTool: toolAxe},
	196: {Hardness: 3, PreferredTool: toolAxe},
	197: {Hardness: 3, PreferredTool: toolAxe},
	198: {Hardness: 0.6, PreferredTool: toolShovel},
	200: {Hardness: 0.4, PreferredTool: toolAxe},
	201: {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	203: {Hardness: 1.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	206: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	208: {Hardness: 0},
	219: {Hardness: 1.4, PreferredTool: toolPickaxe},
	220: {Hardness: 1.4, PreferredTool: toolPickaxe},
	221: {Hardness: 1.4, PreferredTool: toolPickaxe},
	222: {Hardness: 1.4, PreferredTool: toolPickaxe},
	223: {Hardness: 1.4, PreferredTool: toolPickaxe},
	224: {Hardness: 1.4, PreferredTool: toolPickaxe},
	225: {Hardness: 1.4, PreferredTool: toolPickaxe},
	226: {Hardness: 1.4, PreferredTool: toolPickaxe},
	227: {Hardness: 1.4, PreferredTool: toolPickaxe},
	228: {Hardness: 1.4, PreferredTool: toolPickaxe},
	229: {Hardness: 1.4, PreferredTool: toolPickaxe},
	231: {Hardness: 1.4, PreferredTool: toolPickaxe},
	232: {Hardness: 1.4, PreferredTool: toolPickaxe},
	233: {Hardness: 1.4, PreferredTool: toolPickaxe},
	234: {Hardness: 1.4, PreferredTool: toolPickaxe},
	235: {Hardness: 1.4, PreferredTool: toolPickaxe},
	236: {Hardness: 1.8, PreferredTool: toolPickaxe},
	237: {Hardness: 0.5, PreferredTool: toolShovel},
	240: {Hardness: 0.4, PreferredTool: toolAxe},
	243: {Hardness: 0.5, PreferredTool: toolShovel},
	244: {Hardness: 0},
	245: {Hardness: 10, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	246: {Hardness: 10},
	247: {Hardness: 10},
	// 以下方块参考端没有对应类，按原版数值补齐
	8:   {Hardness: 100, Unbreakable: true},
	9:   {Hardness: 100, Unbreakable: true},
	10:  {Hardness: 100, Unbreakable: true},
	11:  {Hardness: 100, Unbreakable: true},
	19:  {Hardness: 0.6},
	55:  {Hardness: 0},
	75:  {Hardness: 0},
	76:  {Hardness: 0},
	93:  {Hardness: 0},
	97:  {Hardness: 0.75},
	123: {Hardness: 0.3},
	124: {Hardness: 0.3},
	149: {Hardness: 0},
	150: {Hardness: 0},
	160: {Hardness: 0.3},
	179: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	180: {Hardness: 0.8, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	181: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	182: {Hardness: 2, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	188: {Hardness: -1, Unbreakable: true},
	189: {Hardness: -1, Unbreakable: true},
	199: {Hardness: 0.25},
	207: {Hardness: 0.5, PreferredTool: toolPickaxe},
	218: {Hardness: 2, PreferredTool: toolPickaxe},
	241: {Hardness: 0.3},
	248: {Hardness: -1, Unbreakable: true},
	249: {Hardness: -1, Unbreakable: true},
	250: {Hardness: -1, Unbreakable: true},
	251: {Hardness: 3.5, PreferredTool: toolPickaxe, RequiredTool: toolPickaxe, RequiredTier: tierWood},
	255: {Hardness: -1, Unbreakable: true},
}

// 工具耐久取 PocketMine Tool::getMaxDurability（钻石 1562，铁 251，石 132，木 60，金 33）；
// 攻击伤害取各 Tool 子类 getAttackDamage。剑的挖掘加速固定 1.5，与材质无关。
var toolInfoTable map[int32]toolInfo = map[int32]toolInfo{
	itemWoodenPickaxe:  {Kind: toolPickaxe, Tier: tierWood, Speed: 2, MaxDurability: 60, AttackDamage: 3},
	itemWoodenAxe:      {Kind: toolAxe, Tier: tierWood, Speed: 2, MaxDurability: 60, AttackDamage: 4},
	itemWoodenShovel:   {Kind: toolShovel, Tier: tierWood, Speed: 2, MaxDurability: 60, AttackDamage: 2},
	itemWoodenSword:    {Kind: toolSword, Tier: tierWood, Speed: 1.5, MaxDurability: 60, AttackDamage: 5},
	itemStonePickaxe:   {Kind: toolPickaxe, Tier: tierStone, Speed: 4, MaxDurability: 132, AttackDamage: 4},
	itemStoneAxe:       {Kind: toolAxe, Tier: tierStone, Speed: 4, MaxDurability: 132, AttackDamage: 5},
	itemStoneShovel:    {Kind: toolShovel, Tier: tierStone, Speed: 4, MaxDurability: 132, AttackDamage: 3},
	itemStoneSword:     {Kind: toolSword, Tier: tierStone, Speed: 1.5, MaxDurability: 132, AttackDamage: 6},
	itemIronPickaxe:    {Kind: toolPickaxe, Tier: tierIron, Speed: 6, MaxDurability: 251, AttackDamage: 5},
	itemIronAxe:        {Kind: toolAxe, Tier: tierIron, Speed: 6, MaxDurability: 251, AttackDamage: 6},
	itemIronShovel:     {Kind: toolShovel, Tier: tierIron, Speed: 6, MaxDurability: 251, AttackDamage: 4},
	itemIronSword:      {Kind: toolSword, Tier: tierIron, Speed: 1.5, MaxDurability: 251, AttackDamage: 7},
	itemDiamondPickaxe: {Kind: toolPickaxe, Tier: tierDiamond, Speed: 8, MaxDurability: 1562, AttackDamage: 6},
	itemDiamondAxe:     {Kind: toolAxe, Tier: tierDiamond, Speed: 8, MaxDurability: 1562, AttackDamage: 7},
	itemDiamondShovel:  {Kind: toolShovel, Tier: tierDiamond, Speed: 8, MaxDurability: 1562, AttackDamage: 5},
	itemDiamondSword:   {Kind: toolSword, Tier: tierDiamond, Speed: 1.5, MaxDurability: 1562, AttackDamage: 8},
	itemGoldPickaxe:    {Kind: toolPickaxe, Tier: tierGold, Speed: 12, MaxDurability: 33, AttackDamage: 3},
	itemGoldAxe:        {Kind: toolAxe, Tier: tierGold, Speed: 12, MaxDurability: 33, AttackDamage: 4},
	itemGoldShovel:     {Kind: toolShovel, Tier: tierGold, Speed: 12, MaxDurability: 33, AttackDamage: 2},
	itemGoldSword:      {Kind: toolSword, Tier: tierGold, Speed: 1.5, MaxDurability: 33, AttackDamage: 5},
	itemShears:         {Kind: toolShears, Tier: tierNone, Speed: 1, MaxDurability: 239, AttackDamage: 1},
	itemWoodenHoe:       {Kind: toolHoe, Tier: tierWood, Speed: 1, MaxDurability: 60, AttackDamage: 1},
	itemStoneHoe:        {Kind: toolHoe, Tier: tierStone, Speed: 1, MaxDurability: 132, AttackDamage: 1},
	itemIronHoe:         {Kind: toolHoe, Tier: tierIron, Speed: 1, MaxDurability: 251, AttackDamage: 1},
	itemDiamondHoe:      {Kind: toolHoe, Tier: tierDiamond, Speed: 1, MaxDurability: 1562, AttackDamage: 1},
	itemGoldHoe:         {Kind: toolHoe, Tier: tierGold, Speed: 1, MaxDurability: 33, AttackDamage: 1},
}

// toolInfoFor 非工具（含空手）一律当作速度 1、伤害 1 的拳头。
func toolInfoFor(item protocol.Item) toolInfo {
	var info toolInfo = toolInfo{}
	var exists bool = false
	info, exists = toolInfoTable[item.ID]
	if exists && !item.IsAir() {
		return info
	}
	return toolInfo{Kind: toolNone, Tier: tierNone, Speed: 1, AttackDamage: 1}
}

// miningInfoFor 返回方块挖掘规则。未登记方块按参考端 Block::getHardness 默认 10 处理，
// 且不掉落任何物品，避免未知方块凭空刷物品。
func miningInfoFor(id byte) blockInfo {
	var info blockInfo = blockInfo{}
	var exists bool = false
	info, exists = blockInfoTable[id]
	if exists {
		return info
	}
	return blockInfo{Hardness: 10}
}

// blockUnbreakable 判定方块是否完全不可破坏（基岩、传送门等）。
func blockUnbreakable(info blockInfo) bool {
	return info.Unbreakable || info.Hardness < 0
}

// canHarvest 表示能否拿到掉落，而不是能否挖掉：等级或工具不对时方块照样能挖碎，只是空掉。
func canHarvest(info blockInfo, tool toolInfo) bool {
	if info.Unbreakable {
		return false
	}
	if info.RequiredTool == toolNone {
		return true
	}
	return tool.Kind == info.RequiredTool && tool.Tier >= info.RequiredTier
}
