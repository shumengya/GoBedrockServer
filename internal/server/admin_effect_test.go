package server

import "bytes"
import "strings"
import "testing"
import "go-mc-server/internal/command"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 凋零 II 每 25 刻伤害一次且可致死；饱食 II 每 10 刻只补饥饿，满值后不重复发包。
func TestEffectCommandWitherAndSaturation(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var ctx = &command.Context{IsOp: true, Admin: fixture.admin(), Reply: func(string) {}}
	var updates = 0
	player.Session.SentHook = func(raw []byte) {
		if raw[0] == protocol.IDUpdateAttributes {
			updates++
		}
	}
	player.food.Food, player.food.Saturation = 18, 2
	fixture.srv.cmds.Dispatch(ctx, "effect Alex saturation 2 1")
	var tick = 0
	for tick = 0; tick < 40; tick++ {
		fixture.srv.tickEffects(player)
		var expected float32 = 19
		if tick >= 10 {
			expected = 20
		}
		if player.food.Food != expected || player.food.Saturation != 2 {
			test.Fatalf("期望第 %d 刻饥饿 %v、饱和度 2，实际 %+v", tick+1, expected, player.food)
		}
	}
	if updates != 2 {
		test.Fatalf("期望两次补饥饿属性通知，实际 %d", updates)
	}
	player.combat.Health = 2
	fixture.srv.cmds.Dispatch(ctx, "effect Alex wither 3 1")
	for tick = 0; tick < 35; tick++ {
		fixture.srv.tickEffects(player)
		var expected float32 = 2
		if tick >= 10 {
			expected = 1
		}
		if player.Health() != expected {
			test.Fatalf("期望第 %d 刻血量 %v，实际 %v", tick+1, expected, player.Health())
		}
	}
	fixture.srv.tickEffects(player)
	if !player.IsDead() || player.combat.LastCause != DamageCauseMagic {
		test.Fatalf("期望第二次凋零造成魔法死亡，实际 %+v", player.combat)
	}
}

// 用命令触发真实结算：瞬时药默认一刻，显式时长逐刻生效，普通药仍按秒换算。
func TestEffectCommandPotionDuration(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var reply = ""
	var ctx = &command.Context{IsOp: true, Admin: fixture.admin(), Reply: func(msg string) { reply = msg }}
	player.combat.Health = 8
	fixture.srv.cmds.Dispatch(ctx, "effect Alex 6")
	fixture.srv.tickEffects(player)
	if player.Health() != 12 || len(player.combat.Effects) != 0 || !strings.Contains(reply, "1 刻") {
		test.Fatalf("期望默认一刻治疗后回到 12 点生命并移除，实际 %v / %+v / %s", player.Health(), player.combat.Effects, reply)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex healing 2")
	fixture.srv.tickEffects(player)
	fixture.srv.tickEffects(player)
	if player.Health() != 20 || len(player.combat.Effects) != 0 || !strings.Contains(reply, "2 刻") {
		test.Fatalf("期望显式两刻各治疗一次，实际 %v / %+v / %s", player.Health(), player.combat.Effects, reply)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex harming")
	fixture.srv.tickEffects(player)
	if player.Health() != 14 || len(player.combat.Effects) != 0 {
		test.Fatalf("期望瞬时伤害扣 6 后到期，实际 %v / %+v", player.Health(), player.combat.Effects)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex jump 2")
	if player.combat.Effects[8].Duration != 40 || !strings.Contains(reply, "2 秒") {
		test.Fatalf("期望跳跃别名对应 40 刻，实际 %+v / %s", player.combat.Effects, reply)
	}
}

// 命令添加效果后，当事人须收到移速属性，同世界的其他玩家须收到隐身标志。
func TestEffectCommandPotionSync(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var viewer = fixture.join("Steve", "127.0.0.2", "world")
	var ctx = &command.Context{IsOp: true, Admin: fixture.admin(), Reply: func(string) {}}
	var movement, flags []byte
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
	fixture.srv.cmds.Dispatch(ctx, "effect Alex speed 8 1")
	fixture.srv.cmds.Dispatch(ctx, "effect Alex invisibility")
	var wantSpeed, _ = packets.EncodePacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.MovementAttribute(0.14)}})
	var invisible = packets.DefaultPlayerFlags()&^(uint64(3)<<14) | 32
	var wantFlags, _ = packets.EncodePacket(&packets.SetEntityDataPacket{EntityRuntimeID: player.EntityRuntimeID, Flags: invisible, SendFlags: true})
	if !bytes.Equal(movement, wantSpeed) || !bytes.Equal(flags, wantFlags) {
		test.Fatalf("期望命令同步迅捷 II 和隐身标志，实际 %x / %x", movement, flags)
	}
}

// 从命令分派一直检查到效果包、存档和玩家属性：秒数要换算成刻，清单项不能误删其他效果。
func TestEffectCommandLifecycle(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var reply = ""
	var ctx = &command.Context{IsOp: true, Admin: fixture.admin(), Reply: func(msg string) { reply = msg }}
	var events = []packets.MobEffectPacket{}
	player.Session.SentHook = func(raw []byte) {
		if raw[0] != protocol.IDMobEffect {
			return
		}
		var pkt = packets.MobEffectPacket{}
		var err = pkt.Decode(protocol.NewStream(raw[1:]))
		if err != nil {
			test.Fatal(err)
		}
		events = append(events, pkt)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Al absorption 8 1 true")
	var saved = player.Snapshot("world")
	if len(events) != 1 || events[0].Particles || events[0].Duration != 160 || events[0].Amplifier != 1 || !saved.Effects[22].HideParticles || saved.Absorption != 8 {
		test.Fatalf("期望隐藏粒子的 160 刻吸收 II 并保留 8 点吸收值，实际 %+v / %+v / %s", events, saved.Effects, reply)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex poison")
	fixture.srv.tickEffects(player)
	if len(events) != 2 || !events[1].Particles || events[1].Duration != 300 || player.combat.Effects[19].Duration != 299 {
		test.Fatalf("期望默认 300 刻且正常结算，实际 %+v / %+v", events, player.combat.Effects)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex 22 0")
	if len(events) != 3 || events[2].Event != packets.MobEffectRemove || events[2].EffectID != 22 || player.combat.Absorption != 0 || len(player.combat.Effects) != 1 {
		test.Fatalf("期望仅清除吸收，实际 %+v / %+v", events, player.combat)
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex clear")
	if len(events) != 4 || events[3].EffectID != 19 || events[3].Event != packets.MobEffectRemove || len(player.combat.Effects) != 0 {
		test.Fatalf("期望清除剩余的中毒，实际 %+v", events)
	}
}

// 普通玩家、未知效果和无法表示的秒数不能改变目标；管理员也不能用弱效果覆盖强效果。
func TestEffectCommandRejectsInvalid(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var reply = ""
	var ctx = &command.Context{Admin: fixture.admin(), Reply: func(msg string) { reply = msg }}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex poison")
	if len(player.combat.Effects) != 0 {
		test.Fatalf("期望普通玩家无权操作，实际 %+v", player.combat.Effects)
	}
	ctx.IsOp = true
	var invalid = []string{"unknown", "19 -1", "19 107374183", "6 2147483648", "19 10 256", "19 3 1 maybe", "clear 2"}
	var index = 0
	for index = 0; index < len(invalid); index++ {
		reply = ""
		fixture.srv.cmds.Dispatch(ctx, "effect Alex "+invalid[index])
		if reply == "" || len(player.combat.Effects) != 0 {
			test.Fatalf("期望拒绝 %s，实际 %s / %+v", invalid[index], reply, player.combat.Effects)
		}
	}
	fixture.srv.cmds.Dispatch(ctx, "effect Alex poison 2 2")
	fixture.srv.cmds.Dispatch(ctx, "effect Alex poison 20 1")
	if !strings.Contains(reply, "已有") || player.combat.Effects[19].Amplifier != 2 || player.combat.Effects[19].Duration != 40 {
		test.Fatalf("期望保留较强效果并提示原因，实际 %+v / %s", player.combat.Effects, reply)
	}
}
