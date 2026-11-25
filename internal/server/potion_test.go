package server

import "bytes"
import "testing"

import "go-mc-server/internal/item"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 从客户端完成事件验证快捷栏扣瓶、满饥饿饮用、创造保留以及两种瞬时药的结算量。
func TestDrinkPotion(test *testing.T) {
	var cases = []struct {
		meta, mode               int32
		health                   float32
		held, effect, ticks, amp int32
	}{
		{0, 0, 10, 373, 0, 0, 0}, {4, 0, 10, 374, 0, 0, 0}, {22, 0, 18, 374, 0, 0, 0},
		{23, 0, 4, 374, 0, 0, 0}, {27, 1, 10, 373, 19, 439, 1}, {29, 0, 11, 374, 10, 2399, 0},
		{13, 0, 10, 374, 12, 9599, 0}, {5, 0, 10, 374, 16, 3599, 0}, {12, 3, 10, 373, 0, 0, 0},
		{36, 0, 10, 373, 0, 0, 0},
	}
	var index = 0
	for index = 0; index < len(cases); index++ {
		var example = cases[index]
		var fixture, player = placeTestSetup(test)
		player.GameMode = example.mode
		player.combat.Health = 10
		player.selectHotbar(0, 16)
		player.ApplyInventorySlot(7, protocol.Item{ID: 373, Meta: example.meta, Count: 1})
		var event = &packets.EntityEventPacket{EntityRuntimeID: 999, Event: packets.EntityEventUseItem}
		fixture.srv.handleEntityEvent(player.Session, event)
		fixture.srv.handleEntityEvent(player.Session, event)
		fixture.srv.tickEffects(player)
		var effect = player.combat.Effects[example.effect]
		if player.Health() != example.health || player.Inventory[7].ID != example.held || player.HeldItem.ID != example.held || player.food.Food != 20 || effect.Duration != example.ticks || effect.Amplifier != example.amp || (example.effect == 0 && len(player.combat.Effects) != 0) {
			test.Fatalf("期望药水 %+v，实际生命 %v，手持 %+v，饥饿 %v，效果 %+v", example, player.Health(), player.HeldItem, player.food.Food, player.combat.Effects)
		}
	}
	if item.StackLimit(protocol.Item{ID: 373}) != 1 || item.StackLimit(protocol.Item{ID: 374}) != 64 {
		test.Fatalf("期望药水单瓶、空瓶可堆叠，实际 %d / %d", item.StackLimit(protocol.Item{ID: 373}), item.StackLimit(protocol.Item{ID: 374}))
	}
}

// 验证移速实际发包，隐身影响旁观者和后来加入者，并在清除后恢复。
func TestPotionAttributeAndVisibility(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var viewer = fixture.join("Alex", "127.0.0.2", "world")
	var movement, flags []byte = nil, nil
	player.Session.SentHook = func(raw []byte) {
		if raw[0] == protocol.IDUpdateAttributes {
			movement = bytes.Clone(raw)
		}
	}
	viewer.Session.SentHook = func(raw []byte) {
		if raw[0] == protocol.IDSetEntityData {
			flags = bytes.Clone(raw)
		}
	}
	fixture.srv.applyPotion(player, 16)
	fixture.srv.applyPotion(player, 17)
	var wantSpeed, _ = packets.EncodePacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.MovementAttribute(float32(0.14) * 0.85)}})
	if !bytes.Equal(movement, wantSpeed) {
		test.Fatalf("期望迅捷 II 与迟缓 I 同时得到 0.119 的移动属性，实际 %x", movement)
	}
	fixture.srv.applyPotion(player, 8)
	var invisible = packets.DefaultPlayerFlags()&^(uint64(3)<<14) | 32
	var wantFlags, _ = packets.EncodePacket(&packets.SetEntityDataPacket{EntityRuntimeID: player.EntityRuntimeID, Flags: invisible, SendFlags: true})
	if !bytes.Equal(flags, wantFlags) || uint64(addPlayerPacket(player).Metadata[0].Long) != invisible {
		test.Fatalf("期望现有和新观察者均看到隐身标志，实际 %x / %+v", flags, addPlayerPacket(player).Metadata[0])
	}
	player.ApplyInventorySlot(0, protocol.Item{ID: 325, Meta: 1, Count: 1})
	fixture.srv.drinkMilk(player)
	wantSpeed, _ = packets.EncodePacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.MovementAttribute(0.1)}})
	wantFlags, _ = packets.EncodePacket(&packets.SetEntityDataPacket{EntityRuntimeID: player.EntityRuntimeID, Flags: packets.DefaultPlayerFlags(), SendFlags: true})
	if !bytes.Equal(movement, wantSpeed) || !bytes.Equal(flags, wantFlags) {
		test.Fatalf("期望喝奶复原移速与可见性，实际 %x / %x", movement, flags)
	}
}

// 这三种效果需要服务端参与：跳跃提高安全落差，水下呼吸补氧，力量和虚弱按倍率修改真实攻击。
func TestPotionWorldEffects(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	fixture.srv.applyPotion(player, 11)
	fixture.srv.handleEntityFall(player.Session, &packets.EntityFallPacket{FallDistance: 8})
	if player.Health() != 17 {
		test.Fatalf("期望跳跃 II 让八格落差只扣 3 点，实际血量 %v", player.Health())
	}
	fixture.srv.world.SetBlock(0, 65, 0, 9, 0)
	player.combat.AirTicks = 1
	fixture.srv.applyPotion(player, 19)
	fixture.srv.tickDrowning(player)
	if player.combat.AirTicks != airMax || player.Health() != 17 {
		test.Fatalf("期望水下满氧且不掉血，实际 %v / %v", player.combat.AirTicks, player.Health())
	}
	var other = fixture.join("Alex", "127.0.0.2", "world")
	fixture.srv.applyPotion(player, 33)
	fixture.srv.applyPotion(player, 34)
	placeTestHold(player, 268, 0, 1)
	fixture.srv.handleInteract(player.Session, &packets.InteractPacket{Action: packets.InteractLeftClick, Target: other.EntityRuntimeID})
	if !nearly(other.Health(), 13.6) {
		test.Fatalf("期望木剑 5 × 1.6 × 0.8 造成 6.4 点伤害，实际血量 %v", other.Health())
	}
}
