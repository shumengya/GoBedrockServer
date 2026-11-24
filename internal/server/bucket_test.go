package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/item"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

// 从四种液体源装桶后可倒回相邻格；重复的旧请求不能再次改变世界。
func TestBucketLiquidRoundTrip(test *testing.T) {
	var sources = [...]byte{8, 9, 10, 11}
	var index = 0
	for index = 0; index < len(sources); index++ {
		var fixture, player = placeTestSetup(test)
		var source = sources[index]
		placeTestPut(test, fixture, 2, 63, 0, 1, 0)
		placeTestPut(test, fixture, 2, 64, 0, source, 0)
		var empty = placeTestHold(player, 325, 0, 1)
		placeTestUse(fixture, player, 2, 64, 0, blockrule.FaceDown, 0, empty)
		var actual, _ = placeTestBlock(fixture, 2, 64, 0)
		var full = player.heldSnapshot()
		if actual != 0 || full.ID != 325 || full.Meta != int32(source&^1) || full.Count != 1 {
			test.Fatalf("期望取走液体 %d 并得到满桶，实际方块 %d 手持 %+v", source, actual, full)
		}
		placeTestUse(fixture, player, 2, 63, 0, blockrule.FaceUp, 1, empty)
		if player.heldSnapshot().Meta != full.Meta {
			test.Fatalf("期望旧请求保留满桶，实际 %+v", player.heldSnapshot())
		}
		placeTestUse(fixture, player, 2, 63, 0, blockrule.FaceUp, 1, full)
		var poured, level = placeTestBlock(fixture, 2, 64, 0)
		if poured != source&^1 || level != 0 || !sameStack(player.heldSnapshot(), empty) {
			test.Fatalf("期望倒出源方块并收回空桶，实际 %d:%d、%+v", poured, level, player.heldSnapshot())
		}
	}
}

// 非源液体不能装桶；成组空桶只扣一个，新满桶单独占格。
func TestBucketStackedSources(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	clear(player.Inventory)
	var empty = placeTestHold(player, 325, 0, 16)
	placeTestPut(test, fixture, 2, 64, 0, 8, 2)
	placeTestUse(fixture, player, 2, 64, 0, 1, 1, empty)
	var actual, level = placeTestBlock(fixture, 2, 64, 0)
	if actual != 8 || level != 2 || player.HeldItem.Count != 16 {
		test.Fatalf("期望保留流水和 16 个桶，实际 %d:%d、%+v", actual, level, player.HeldItem)
	}
	placeTestPut(test, fixture, 2, 64, 0, 8, 0)
	placeTestUse(fixture, player, 2, 64, 0, 1, 1, empty)
	var filled = player.Inventory[1]
	if player.HeldItem.Count != 15 || filled.ID != 325 || filled.Meta != 8 || filled.Count != 1 || item.StackLimit(empty) != 16 || item.StackLimit(filled) != 1 {
		test.Fatalf("期望 15 个空桶与单格水桶，实际 %+v、%+v，上限 %d/%d", player.HeldItem, filled, item.StackLimit(empty), item.StackLimit(filled))
	}
}

// 下界倒水不覆盖原格，生存扣水桶，创造保留；岩浆仍可放置。
func TestBucketNether(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("builder", "10.0.0.1", "nether")
	var nether = fixture.srv.playerWorld(player)
	nether.SetBlock(3, 64, 0, 1, 0)
	nether.SetBlock(3, 65, 0, 0, 0)
	var cases = [...][4]int32{{0, 8, 0, 0}, {1, 8, 0, 8}, {0, 10, 10, 0}}
	var index = 0
	for index = 0; index < len(cases); index++ {
		var entry = cases[index]
		player.GameMode = entry[0]
		var held = placeTestHold(player, 325, entry[1], 1)
		placeTestUse(fixture, player, 3, 64, 0, 1, 1, held)
		var actual, _ = nether.GetBlock(3, 65, 0)
		if int32(actual) != entry[2] || player.HeldItem.Meta != entry[3] {
			test.Fatalf("期望下界方块 %d 桶 %d，实际 %d、%+v", entry[2], entry[3], actual, player.HeldItem)
		}
	}
}

// 对异世界或太远的牛不能挤奶；挤到的奶在满饥饿时也能清除中毒和吸收。
func TestBucketMilkInteraction(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var kind, _ = entity.ByID(11)
	var cow = &entity.Instance{RuntimeID: 45, Type: kind, X: 2, Y: 64, WorldName: "nether"}
	fixture.srv.entities[cow.RuntimeID] = cow
	var interact = &packets.InteractPacket{Action: packets.InteractRightClick, Target: cow.RuntimeID}
	placeTestHold(player, 325, 0, 1)
	fixture.srv.handleInteract(player.Session, interact)
	cow.WorldName, cow.X = "world", 30
	fixture.srv.handleInteract(player.Session, interact)
	if player.HeldItem.Meta != 0 {
		test.Fatalf("期望远处或异世界的牛保留空桶，实际 %+v", player.HeldItem)
	}
	cow.X = 2
	fixture.srv.handleInteract(player.Session, interact)
	if player.HeldItem.Meta != 1 {
		test.Fatalf("期望挤出牛奶，实际 %+v", player.HeldItem)
	}
	player.addEffect(storage.EffectData{ID: effectPoison, Duration: 100})
	player.addEffect(storage.EffectData{ID: effectAbsorption, Duration: 120})
	var foodBefore = player.food
	fixture.srv.handlePlayerAction(player.Session, &packets.PlayerActionPacket{Action: packets.ActionReleaseItem})
	if len(player.combat.Effects) != 0 || player.combat.Absorption != 0 || player.HeldItem.ID != 325 || player.HeldItem.Meta != 0 || player.food != foodBefore {
		test.Fatalf("期望清除效果、退空桶且饥饿不变，实际 %+v、%+v、%+v", player.combat, player.HeldItem, player.food)
	}
}
