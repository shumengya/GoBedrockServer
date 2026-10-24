package server

import "go-mc-server/internal/protocol"

// foodInfo 是一种食物的营养数据，数值取自参考端各 Food 子类的 getFoodRestore / getSaturationRestore。
//
// 暂未实现的附带效果（药水效果系统做完后补）：毒马铃薯 / 生鸡肉 / 腐肉 / 蜘蛛眼 / 河豚的中毒与饥饿，
// 金苹果的再生与吸收，紫颈果的随机传送。现在这些食物只有营养，没有副作用。
type foodInfo struct {
	Restore    float32
	Saturation float32
	// Residue 吃完留下的容器（炖菜留碗）；ID 为 0 表示没有。
	Residue protocol.Item
	// AlwaysEdible 为真时饥饿值满了也能吃（金苹果、紫颈果）。
	AlwaysEdible bool
}

const itemBowl int32 = 281
const itemRawFish int32 = 349
const itemCookedFish int32 = 350

// 1.1 的鱼类 meta：0 鱼、1 鲑鱼、2 小丑鱼、3 河豚。
const fishMetaSalmon int32 = 1
const fishMetaClown int32 = 2
const fishMetaPuffer int32 = 3

// foodByID 是不看 meta 的食物表。
var foodByID map[int32]foodInfo = map[int32]foodInfo{
	260: {Restore: 4, Saturation: 2.4},                      // 苹果
	297: {Restore: 5, Saturation: 6},                        // 面包
	319: {Restore: 3, Saturation: 0.6},                      // 生猪排
	320: {Restore: 8, Saturation: 12.8},                     // 熟猪排
	322: {Restore: 4, Saturation: 9.6, AlwaysEdible: true},  // 金苹果
	357: {Restore: 2, Saturation: 0.4},                      // 曲奇
	360: {Restore: 2, Saturation: 1.2},                      // 西瓜片
	363: {Restore: 3, Saturation: 1.8},                      // 生牛肉
	364: {Restore: 8, Saturation: 12.8},                     // 牛排
	365: {Restore: 2, Saturation: 1.2},                      // 生鸡肉
	366: {Restore: 6, Saturation: 7.2},                      // 熟鸡肉
	367: {Restore: 4, Saturation: 0.8},                      // 腐肉
	375: {Restore: 2, Saturation: 3.2},                      // 蜘蛛眼
	391: {Restore: 3, Saturation: 4.8},                      // 胡萝卜
	392: {Restore: 1, Saturation: 0.6},                      // 马铃薯
	393: {Restore: 5, Saturation: 7.2},                      // 烤马铃薯
	394: {Restore: 2, Saturation: 1.2},                      // 毒马铃薯
	396: {Restore: 6, Saturation: 14.4},                     // 金胡萝卜
	400: {Restore: 8, Saturation: 4.8},                      // 南瓜派
	411: {Restore: 3, Saturation: 1.8},                      // 生兔肉
	412: {Restore: 5, Saturation: 6},                        // 熟兔肉
	423: {Restore: 2, Saturation: 1.2},                      // 生羊肉
	424: {Restore: 6, Saturation: 9.6},                      // 熟羊肉
	432: {Restore: 4, Saturation: 2.4, AlwaysEdible: true},  // 紫颈果
	457: {Restore: 1, Saturation: 1.2},                      // 甜菜根
	466: {Restore: 4, Saturation: 9.6, AlwaysEdible: true},  // 附魔金苹果
	282: {Restore: 6, Saturation: 7.2, Residue: bowlItem()}, // 蘑菇煲
	413: {Restore: 10, Saturation: 12, Residue: bowlItem()}, // 兔肉煲
	459: {Restore: 6, Saturation: 7.2, Residue: bowlItem()}, // 甜菜汤
	// 1.1 后期把鲑鱼 / 小丑鱼 / 河豚拆成了独立物品 id，这里一并认。
	460: {Restore: 2, Saturation: 0.4}, // 生鲑鱼
	461: {Restore: 1, Saturation: 0.2}, // 小丑鱼
	462: {Restore: 1, Saturation: 0.2}, // 河豚
	463: {Restore: 6, Saturation: 9.6}, // 熟鲑鱼
}

func bowlItem() protocol.Item {
	return protocol.Item{ID: itemBowl, Count: 1}
}

// foodFor 查物品的营养数据；第二个返回值为 false 表示这不是食物。
// 鱼是同一个 id 靠 meta 区分品种，所以单独处理。
func foodFor(item protocol.Item) (foodInfo, bool) {
	if item.IsAir() {
		return foodInfo{}, false
	}
	if item.ID == itemRawFish {
		return rawFishInfo(item.Meta), true
	}
	if item.ID == itemCookedFish {
		return cookedFishInfo(item.Meta), true
	}
	var info, found = foodByID[item.ID]
	return info, found
}

func rawFishInfo(meta int32) foodInfo {
	if meta == fishMetaSalmon {
		return foodInfo{Restore: 2, Saturation: 0.4}
	}
	if meta == fishMetaClown || meta == fishMetaPuffer {
		return foodInfo{Restore: 1, Saturation: 0.2}
	}
	return foodInfo{Restore: 2, Saturation: 0.4}
}

func cookedFishInfo(meta int32) foodInfo {
	if meta == fishMetaSalmon {
		return foodInfo{Restore: 6, Saturation: 9.6}
	}
	return foodInfo{Restore: 5, Saturation: 6}
}
