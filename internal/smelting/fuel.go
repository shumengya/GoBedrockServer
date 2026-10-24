// Package smelting 是熔炉烧制的纯逻辑：燃料、一个 tick 怎么推进。
// 它不碰方块实体、网络和锁，配方通过 Recipes 接口传入，方便脱离服务器单独测试。
package smelting

import "go-mc-server/internal/protocol"

const itemBucket int32 = 325
const itemLavaBucketMeta int32 = 10

// fuelTicks 每种燃料能烧多少 tick，照参考端 Fuel::$duration。
// 参考端漏了几样原版会烧的木头东西：金合欢/深色橡木原木(162)和这两种木楼梯(163、164)，
// 它们在这里补上，烧的时间和同类一样。木半砖、木头压力板之外的木制品没补。
var fuelTicks map[int32]int32 = map[int32]int32{
	263: 1600,  // 煤和木炭
	173: 16000, // 煤块
	17:  300,   // 原木
	162: 300,   // 金合欢、深色橡木原木(补)
	5:   300,   // 木板
	6:   100,   // 树苗
	271: 200,   // 木斧
	270: 200,   // 木镐
	268: 200,   // 木剑
	269: 200,   // 木锹
	290: 200,   // 木锄
	72:  300,   // 木压力板
	280: 100,   // 木棍
	85:  300,   // 栅栏
	107: 300,   // 栅栏门
	183: 300,
	184: 300,
	185: 300,
	186: 300,
	187: 300,
	53:  300, // 木楼梯
	134: 300,
	135: 300,
	136: 300,
	163: 300, // 金合欢、深色橡木楼梯(补)
	164: 300,
	96:  300,   // 活板门
	58:  300,   // 工作台
	25:  300,   // 音符盒
	47:  300,   // 书架
	54:  300,   // 箱子
	146: 300,   // 陷阱箱
	151: 300,   // 阳光探测器
	325: 20000, // 桶：只有岩浆桶(meta 10)算，见 FuelTicks
	369: 2400,  // 烈焰棒
}

// FuelTicks 这个物品当燃料能烧多少 tick，不是燃料返回 0。
// 桶只有装岩浆的才是燃料。
func FuelTicks(item protocol.Item) int32 {
	if item.IsAir() {
		return 0
	}
	if item.ID == itemBucket && item.Meta != itemLavaBucketMeta {
		return 0
	}
	return fuelTicks[item.ID]
}

// consumeOne 扣掉一个，扣完变空气。
func consumeOne(item protocol.Item) protocol.Item {
	if item.Count <= 1 {
		return protocol.Air()
	}
	item.Count--
	return item
}

// spendFuel 点燃一个燃料之后格子里剩下的东西：岩浆桶烧完剩一个空桶，别的扣一个。
func spendFuel(fuel protocol.Item) protocol.Item {
	if fuel.ID == itemBucket {
		return protocol.Item{ID: itemBucket, Meta: 0, Count: 1}
	}
	return consumeOne(fuel)
}
