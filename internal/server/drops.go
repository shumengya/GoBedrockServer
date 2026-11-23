package server

import "math/rand/v2"

import "go-mc-server/internal/protocol"

// dropRandom 抽象随机源，测试里换成固定序列，避免概率掉落让测试抖动。
type dropRandom interface {
	// IntN 返回 [0, count) 内的整数。
	IntN(count int) int
}

type defaultDropRandom struct{}

func (defaultDropRandom) IntN(count int) int {
	return rand.IntN(count)
}

// randRange 对齐 PHP mt_rand(low, high)，两端都包含。
func randRange(source dropRandom, low int, high int) int {
	if high <= low {
		return low
	}
	return low + source.IntN(high-low+1)
}

// oneIn 对齐 PHP 里的 mt_rand(1, count) === 1。
func oneIn(source dropRandom, count int) bool {
	return randRange(source, 1, count) == 1
}

func dropOf(id int32, meta int32, count int) protocol.Item {
	return protocol.Item{ID: id, Meta: meta, Count: byte(count)}
}

// blockDrops 计算生存模式挖掉方块后的掉落，规则对齐 PocketMine 1.1 各 Block::getDrops。
// 不处理附魔（精准采集、时运），本服还没有附魔系统。
func blockDrops(id byte, meta byte, held protocol.Item, source dropRandom) []protocol.Item {
	var info blockInfo = miningInfoFor(id)
	var tool toolInfo = toolInfoFor(held)
	if blockUnbreakable(info) || !canHarvest(info, tool) {
		return nil
	}
	var data int32 = int32(meta)
	switch id {
	case 1: // 石头：普通石头变圆石，花岗岩等变种掉自身
		if data == 0 {
			return []protocol.Item{dropOf(4, 0, 1)}
		}
		return []protocol.Item{dropOf(1, data, 1)}
	case 2, 60, 110, 198, 243: // 草方块、耕地、菌丝、草径、灰化土都掉泥土
		return []protocol.Item{dropOf(3, 0, 1)}
	case 13: // 沙砾：10% 燧石，90% 沙砾，两者独立判定
		var drops []protocol.Item = nil
		if oneIn(source, 10) {
			drops = append(drops, dropOf(318, 0, 1))
		}
		if !oneIn(source, 10) {
			drops = append(drops, dropOf(13, 0, 1))
		}
		return drops
	case 16: // 煤矿石
		return []protocol.Item{dropOf(263, 0, 1)}
	case 18: // 树叶：剪刀掉树叶本身；否则 1/20 树苗，橡树叶另有 1/200 苹果
		if tool.Kind == toolShears {
			return []protocol.Item{dropOf(18, data&3, 1)}
		}
		var drops []protocol.Item = nil
		if oneIn(source, 20) {
			drops = append(drops, dropOf(6, data&3, 1))
		}
		if data&3 == 0 && oneIn(source, 200) {
			drops = append(drops, dropOf(260, 0, 1))
		}
		return drops
	case 161: // 金合欢/深色橡树叶：树苗 meta 要加 4，没有苹果
		if tool.Kind == toolShears {
			return []protocol.Item{dropOf(161, data&1, 1)}
		}
		if oneIn(source, 20) {
			return []protocol.Item{dropOf(6, data&1|4, 1)}
		}
		return nil
	case 20, 102, 160, 241, 79, 174, 51, 90, 92, 34, 52: // 玻璃类、冰、火、传送门、蛋糕、活塞头、刷怪笼
		return nil
	case 21: // 青金石矿：4~8 青金石
		return []protocol.Item{dropOf(351, 4, randRange(source, 4, 8))}
	case 56: // 钻石矿
		return []protocol.Item{dropOf(264, 0, 1)}
	case 73, 74: // 红石矿（含发光态）：4~5 红石
		return []protocol.Item{dropOf(331, 0, randRange(source, 4, 5))}
	case 129: // 绿宝石矿
		return []protocol.Item{dropOf(388, 0, 1)}
	case 153: // 下界石英矿
		return []protocol.Item{dropOf(406, 0, 1)}
	case 89: // 萤石：2~4 萤石粉
		return []protocol.Item{dropOf(348, 0, randRange(source, 2, 4))}
	case 82: // 粘土块：4 个粘土球
		return []protocol.Item{dropOf(337, 0, 4)}
	case 47: // 书架：3 本书
		return []protocol.Item{dropOf(340, 0, 3)}
	case 103: // 西瓜：3~7 西瓜片
		return []protocol.Item{dropOf(360, 0, randRange(source, 3, 7))}
	case 78: // 雪片：只有铲子能拿到雪球
		if tool.Kind != toolShovel {
			return nil
		}
		return []protocol.Item{dropOf(332, 0, 1)}
	case 80: // 雪块：铲子掉 4 个雪球
		if tool.Kind != toolShovel {
			return nil
		}
		return []protocol.Item{dropOf(332, 0, 4)}
	case 30: // 蜘蛛网：剪刀掉网，剑掉线
		if tool.Kind == toolShears {
			return []protocol.Item{dropOf(30, 0, 1)}
		}
		if tool.Kind == toolSword {
			return []protocol.Item{dropOf(287, 0, 1)}
		}
		return nil
	case 31: // 草丛：1/16 小麦种子
		if randRange(source, 0, 15) == 0 {
			return []protocol.Item{dropOf(295, 0, 1)}
		}
		return nil
	case 32: // 枯萎灌木：剪刀掉自身，否则 0~2 木棍
		if tool.Kind == toolShears {
			return []protocol.Item{dropOf(32, 0, 1)}
		}
		var sticks int = randRange(source, 0, 2)
		if sticks == 0 {
			return nil
		}
		return []protocol.Item{dropOf(280, 0, sticks)}
	case 106: // 藤蔓：只有剪刀能拿到
		if tool.Kind != toolShears {
			return nil
		}
		return []protocol.Item{dropOf(106, 0, 1)}
	case 99, 100: // 蘑菇方块：0~2 对应蘑菇
		var count int = randRange(source, 0, 2)
		if count == 0 {
			return nil
		}
		var mushroom int32 = 39
		if id == 100 {
			mushroom = 40
		}
		return []protocol.Item{dropOf(mushroom, 0, count)}
	case 130: // 末影箱：8 个黑曜石
		return []protocol.Item{dropOf(49, 0, 8)}
	case 169: // 海晶灯：3 个海晶砂粒
		return []protocol.Item{dropOf(422, 0, 3)}
	case 43: // 双石台阶：拆成 2 个台阶
		return []protocol.Item{dropOf(44, data&7, 2)}
	case 157: // 双木台阶：拆成 2 个木台阶
		return []protocol.Item{dropOf(158, data&7, 2)}
	case 44, 158: // 单台阶：去掉上下半位
		return []protocol.Item{dropOf(int32(id), data&7, 1)}
	case 17, 162: // 原木：去掉朝向位
		return []protocol.Item{dropOf(int32(id), data&3, 1)}
	case 6: // 树苗：去掉生长位
		return []protocol.Item{dropOf(6, data&7, 1)}
	case 24, 98, 155, 168, 179: // 砂岩、石砖、石英块、海晶石、红砂岩保留变种
		return []protocol.Item{dropOf(int32(id), data&3, 1)}
	case 62: // 燃烧的熔炉掉普通熔炉
		return []protocol.Item{dropOf(61, 0, 1)}
	case 26: // 床只由床头掉一张红床，床尾不掉，避免一张床掉两次
		if data&8 == 0 {
			return nil
		}
		return []protocol.Item{dropOf(355, 14, 1)}
	case 55: // 红石线
		return []protocol.Item{dropOf(331, 0, 1)}
	case 63, 68: // 告示牌
		return []protocol.Item{dropOf(323, 0, 1)}
	case 64: // 橡木门：只由下半扇掉落
		return doorDrop(data, 324)
	case 71: // 铁门
		return doorDrop(data, 330)
	case 193:
		return doorDrop(data, 427)
	case 194:
		return doorDrop(data, 428)
	case 195:
		return doorDrop(data, 429)
	case 196:
		return doorDrop(data, 430)
	case 197:
		return doorDrop(data, 431)
	case 83: // 甘蔗
		return []protocol.Item{dropOf(338, 0, 1)}
	case 93, 94: // 红石中继器
		return []protocol.Item{dropOf(356, 0, 1)}
	case 117: // 酿造台
		return []protocol.Item{dropOf(379, 0, 1)}
	case 118: // 炼药锅
		return []protocol.Item{dropOf(380, 0, 1)}
	case 154: // 漏斗
		return []protocol.Item{dropOf(410, 0, 1)}
	case 140: // 花盆
		return []protocol.Item{dropOf(390, 0, 1)}
	case 199: // 展示框物品 ID 与方块 ID 不同
		return []protocol.Item{dropOf(389, 0, 1)}
	case 144: // 头颅种类要从方块实体中读取
		return nil
	case 59: // 小麦：成熟掉小麦 + 0~3 种子，未成熟只掉 1 种子
		if data >= 7 {
			var drops []protocol.Item = []protocol.Item{dropOf(296, 0, 1)}
			var seeds int = randRange(source, 0, 3)
			if seeds > 0 {
				drops = append(drops, dropOf(295, 0, seeds))
			}
			return drops
		}
		return []protocol.Item{dropOf(295, 0, 1)}
	case 141: // 胡萝卜
		if data >= 7 {
			return []protocol.Item{dropOf(391, 0, randRange(source, 1, 4))}
		}
		return []protocol.Item{dropOf(391, 0, 1)}
	case 142: // 马铃薯
		if data >= 7 {
			return []protocol.Item{dropOf(392, 0, randRange(source, 1, 4))}
		}
		return []protocol.Item{dropOf(392, 0, 1)}
	case 244: // 甜菜
		if data >= 7 {
			var drops []protocol.Item = []protocol.Item{dropOf(457, 0, 1)}
			var seeds int = randRange(source, 0, 3)
			if seeds > 0 {
				drops = append(drops, dropOf(458, 0, seeds))
			}
			return drops
		}
		return []protocol.Item{dropOf(458, 0, 1)}
	case 104: // 南瓜梗：0~2 南瓜种子
		return seedsDrop(source, 361)
	case 105: // 西瓜梗：0~2 西瓜种子
		return seedsDrop(source, 362)
	case 115: // 地狱疣：成熟 2~4，否则 1
		if data >= 3 {
			return []protocol.Item{dropOf(372, 0, randRange(source, 2, 4))}
		}
		return []protocol.Item{dropOf(372, 0, 1)}
	case 127: // 可可果：成熟 3 颗，否则 1 颗（染料 meta 3）
		if data >= 8 {
			return []protocol.Item{dropOf(351, 3, 3)}
		}
		return []protocol.Item{dropOf(351, 3, 1)}
	case 200, 240: // 紫颂花/紫颂植株：成熟才掉紫颂果
		if data >= 7 {
			return []protocol.Item{dropOf(432, 0, 1)}
		}
		return nil
	case 175: // 大型花：只由上半部分掉落；高草丛/大型蕨 1/25 种子
		if data&8 == 0 {
			return nil
		}
		var kind int32 = data & 7
		if kind == 2 || kind == 3 {
			if tool.Kind == toolShears {
				return []protocol.Item{dropOf(175, kind, 1)}
			}
			if randRange(source, 0, 24) == 0 {
				return []protocol.Item{dropOf(295, 0, 1)}
			}
			return nil
		}
		return []protocol.Item{dropOf(175, kind, 1)}
	case 145: // 铁砧保留损坏程度
		return []protocol.Item{dropOf(145, data&12, 1)}
	}
	if isPlainBlockDrop(id) {
		return []protocol.Item{dropOf(int32(id), 0, 1)}
	}
	return []protocol.Item{dropOf(int32(id), data, 1)}
}

// doorDrop 门由上下两格组成，只让下半扇掉落，避免一扇门掉两个。
func doorDrop(data int32, itemID int32) []protocol.Item {
	if data&8 != 0 {
		return nil
	}
	return []protocol.Item{dropOf(itemID, 0, 1)}
}

func seedsDrop(source dropRandom, itemID int32) []protocol.Item {
	var count int = randRange(source, 0, 2)
	if count == 0 {
		return nil
	}
	return []protocol.Item{dropOf(itemID, 0, count)}
}

// isPlainBlockDrop 这些方块的 meta 只表示朝向/状态，掉落时要清零，否则捡起来叠不上。
func isPlainBlockDrop(id byte) bool {
	switch id {
	case 23, 25, 27, 28, 29, 33, 50, 53, 54, 58, 61, 65, 66, 67, 69, 70, 72, 76, 77, 81, 85, 86, 91, 96,
		107, 108, 109, 111, 114, 116, 120, 123, 124, 125, 126, 128, 131, 134, 135, 136, 143, 146,
		147, 148, 151, 156, 163, 164, 167, 170, 178, 180, 183, 184, 185, 186, 187, 203, 208:
		return true
	}
	return false
}
