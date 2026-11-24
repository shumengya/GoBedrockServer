package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/item"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 参考端创造物品表中的每种蛋都应在被点击面的邻格生成对应生物，且不消耗创造物品。
func TestSpawnEggCreativeCatalogue(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.GameMode = protocol.GameModeCreative
	placeTestPut(test, fixture, 2, 64, -1, blockrule.BlockStone, 0)
	var catalogue = item.CreativeItems()
	var index, eggs = 0, 0
	for index = 0; index < len(catalogue); index++ {
		var stack = catalogue[index]
		if stack.ID != 383 {
			continue
		}
		var held = placeTestHold(player, 383, stack.Meta, 1)
		placeTestUse(fixture, player, 2, 64, -1, blockrule.FaceWest, 0.5, held)
		eggs++
		var spawned = fixture.srv.entities[int64(eggs)]
		if spawned == nil || spawned.Type.NetworkID != stack.Meta || spawned.X != 1.5 || spawned.Y != 64 || spawned.Z != -0.5 || spawned.WorldName != "world" || spawned.Yaw < 0 || spawned.Yaw >= 360 || placeTestCount(player) != 1 {
			test.Fatalf("期望在 (1.5,64,-0.5) 生成 %d 并保留蛋，实际 %+v、%+v", stack.Meta, spawned, player.HeldItem)
		}
	}
	if eggs != 41 {
		test.Fatalf("期望覆盖参考表的 41 种刷怪蛋，实际 %d", eggs)
	}
}

// 从生存玩家实际手持槽扣一枚，自定义名字来自蛋的 display.Name，并随出生包发送。
func TestSpawnEggNamedSurvival(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 2, 64, 0, blockrule.BlockStone, 0)
	var held = placeTestHold(player, 383, 11, 2)
	held.NBT = []byte{10, 0, 0, 8, 4, 0, 'N', 'a', 'm', 'e', 1, 0, 'X', 10, 7, 0, 'd', 'i', 's', 'p', 'l', 'a', 'y', 8, 4, 0, 'N', 'a', 'm', 'e', 3, 0, 0xe7, 0x89, 0x9b, 0, 0}
	player.Inventory[0], player.HeldItem = held, held
	placeTestUse(fixture, player, 2, 64, 0, blockrule.FaceUp, 1, held)
	var spawned = fixture.srv.entities[1]
	if spawned == nil || spawned.CustomName != "牛" || spawned.Y != 65 || placeTestCount(player) != 1 {
		test.Fatalf("期望消耗一枚蛋并生成名为牛的实体，实际 %+v、%+v", spawned, player.HeldItem)
	}
	var entries = addEntityPacket(spawned).Metadata
	var index, matched = 0, 0
	for index = 0; index < len(entries); index++ {
		if entries[index].Key == packets.EntityDataNametag && entries[index].String == "牛" {
			matched++
		}
	}
	if matched != 1 {
		test.Fatalf("期望出生包含一个正确的名字字段，实际 %+v", entries)
	}
}

// 未知类型、特殊实体、点击刷怪笼、旁观以及客户端报错蛋种时都不能刷出实体或扣蛋。
func TestSpawnEggRejected(test *testing.T) {
	var cases = [...][4]int32{{999, 999, 1, 0}, {entity.NetworkIDItem, entity.NetworkIDItem, 1, 0}, {11, 11, 52, 0}, {11, 11, 1, 3}, {11, 32, 1, 0}}
	var index = 0
	for index = 0; index < len(cases); index++ {
		var fixture, player = placeTestSetup(test)
		var entry = cases[index]
		var held = placeTestHold(player, 383, entry[0], 1)
		held.Meta, player.GameMode = entry[1], entry[3]
		placeTestPut(test, fixture, 2, 64, 0, byte(entry[2]), 0)
		placeTestUse(fixture, player, 2, 64, 0, blockrule.FaceUp, 1, held)
		if len(fixture.srv.entities) != 0 || placeTestCount(player) != 1 {
			test.Fatalf("期望 %v 保留蛋且不生成实体，实际数量 %d/%d", entry, placeTestCount(player), len(fixture.srv.entities))
		}
	}
}
