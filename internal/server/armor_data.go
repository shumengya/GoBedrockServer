package server

import "math"

import "go-mc-server/internal/protocol"

// 盔甲槽下标，和 ContainerSetContent / MobArmorEquipment 里的顺序一致。
const armorSlotHelmet int = 0
const armorSlotChestplate int = 1
const armorSlotLeggings int = 2
const armorSlotBoots int = 3

// armorPointReduction 每点护甲减掉的伤害比例，参考端 EntityDamageEvent：1 - 0.04 * 护甲点数。
const armorPointReduction float64 = 0.04

// armorInfo 一件盔甲能穿在哪个槽、提供几点护甲、能挨多少次打。
type armorInfo struct {
	Slot          int
	Defense       int32
	MaxDurability int32
}

// armorTable 数值取自参考端 item/*.php 的 getArmorValue 和 getMaxDurability，
// 只有锁链头盔和金头盔例外：参考端写的是 1 点，确认原版两个版本都是 2 点，这里按 2 点。
// 鞘翅占胸甲槽，没有护甲点数。
var armorTable map[int32]armorInfo = map[int32]armorInfo{
	298: {Slot: armorSlotHelmet, Defense: 1, MaxDurability: 56},      // 皮革帽子
	299: {Slot: armorSlotChestplate, Defense: 3, MaxDurability: 81},  // 皮革外套
	300: {Slot: armorSlotLeggings, Defense: 2, MaxDurability: 76},    // 皮革裤子
	301: {Slot: armorSlotBoots, Defense: 1, MaxDurability: 66},       // 皮革靴子
	302: {Slot: armorSlotHelmet, Defense: 2, MaxDurability: 166},     // 锁链头盔
	303: {Slot: armorSlotChestplate, Defense: 5, MaxDurability: 241}, // 锁链胸甲
	304: {Slot: armorSlotLeggings, Defense: 4, MaxDurability: 226},   // 锁链护腿
	305: {Slot: armorSlotBoots, Defense: 1, MaxDurability: 196},      // 锁链靴子
	306: {Slot: armorSlotHelmet, Defense: 2, MaxDurability: 166},     // 铁头盔
	307: {Slot: armorSlotChestplate, Defense: 6, MaxDurability: 241}, // 铁胸甲
	308: {Slot: armorSlotLeggings, Defense: 5, MaxDurability: 226},   // 铁护腿
	309: {Slot: armorSlotBoots, Defense: 2, MaxDurability: 196},      // 铁靴子
	310: {Slot: armorSlotHelmet, Defense: 3, MaxDurability: 364},     // 钻石头盔
	311: {Slot: armorSlotChestplate, Defense: 8, MaxDurability: 529}, // 钻石胸甲
	312: {Slot: armorSlotLeggings, Defense: 6, MaxDurability: 496},   // 钻石护腿
	313: {Slot: armorSlotBoots, Defense: 3, MaxDurability: 430},      // 钻石靴子
	314: {Slot: armorSlotHelmet, Defense: 2, MaxDurability: 78},      // 金头盔
	315: {Slot: armorSlotChestplate, Defense: 5, MaxDurability: 113}, // 金胸甲
	316: {Slot: armorSlotLeggings, Defense: 3, MaxDurability: 106},   // 金护腿
	317: {Slot: armorSlotBoots, Defense: 1, MaxDurability: 92},       // 金靴子
	444: {Slot: armorSlotChestplate, Defense: 0, MaxDurability: 431}, // 鞘翅
}

// armorInfoFor 查一件物品是不是盔甲。空物品、不在表里的都返回 false。
func armorInfoFor(item protocol.Item) (armorInfo, bool) {
	if item.IsAir() {
		return armorInfo{}, false
	}
	var info, found = armorTable[item.ID]
	return info, found
}

// armorAcceptable 这件东西能不能放进盔甲槽 slot：空的永远可以；
// 其余必须是这个槽该穿的盔甲，数量恰好 1，损耗值在 [0, 最大耐久) 里。
// 客户端想把一叠石头、靴子放进头盔槽，或者带一个负的损耗值，都不收。
func armorAcceptable(slot int, item protocol.Item) bool {
	if item.IsAir() {
		return true
	}
	var info, found = armorInfoFor(item)
	if !found || info.Slot != slot || item.Count != 1 {
		return false
	}
	return item.Meta >= 0 && item.Meta < info.MaxDurability
}

// armorPointsOf 身上这套盔甲的护甲点数之和。放错槽的不算。
func armorPointsOf(armor [protocol.ArmorSize]protocol.Item) int32 {
	var points int32 = 0
	var slot int = 0
	for slot = 0; slot < len(armor); slot++ {
		var info, found = armorInfoFor(armor[slot])
		if found && info.Slot == slot {
			points += info.Defense
		}
	}
	return points
}

// reduceByArmor 按护甲点数减伤并四舍五入到整数（参考端对玩家的最终伤害取整）。没有护甲原样返回。
func reduceByArmor(amount float32, points int32) float32 {
	if points <= 0 {
		return amount
	}
	var rate float64 = max(0, 1-armorPointReduction*float64(points))
	return float32(math.Round(float64(amount) * rate))
}

// armorReducesDamage 哪些伤害来源吃护甲减伤：玩家攻击、仙人掌（参考端的接触伤害）、岩浆。
// 着火持续掉的血（参考端的 FIRE_TICK）、摔落、虚空、窒息、溺水、饥饿都不减。
func armorReducesDamage(cause DamageCause) bool {
	return cause == DamageCausePlayer || cause == DamageCauseCactus || cause == DamageCauseLava
}

// armorWearsOnHit 哪些伤害会磨损盔甲。参考端只在实体攻击、弹射物、闪电这类命中之后调 useArmors，
// 仙人掌和岩浆虽然吃减伤，却不磨损。目前只有玩家攻击。
func armorWearsOnHit(cause DamageCause) bool {
	return cause == DamageCausePlayer
}
