package server

import "strings"
import "testing"
import "go-mc-server/internal/command"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

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
	var invalid = []string{"speed", "19 -1", "19 107374183", "19 10 256", "19 3 1 maybe", "clear 2"}
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
