package server

import "testing"
import "maps"
import "slices"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

// 两侧各取一个随机值，防止把百分比阈值或闭区间端点改反。
func TestFoodEffectThresholds(test *testing.T) {
	var cases = []struct{ id, threshold, effect, duration int32 }{{365, 70, 17, 600}, {367, 20, 17, 600}, {394, 41, 19, 100}}
	for _, example := range cases {
		var item = protocol.Item{ID: example.id, Count: 1}
		var below = foodEffects(item, &fixedRandom{values: []int{int(example.threshold) - 1}})
		var hit = foodEffects(item, &fixedRandom{values: []int{int(example.threshold)}})
		if len(below) != 0 || !slices.Equal(hit, []storage.EffectData{{ID: example.effect, Duration: example.duration}}) {
			test.Fatalf("期望物品 %d 只在阈值处触发，实际 %v / %v", item.ID, below, hit)
		}
	}
	var puffer = foodEffects(protocol.Item{ID: 349, Meta: 3}, nil)
	if len(puffer) != 3 || !slices.Equal(puffer, foodEffects(protocol.Item{ID: 462}, nil)) || len(foodEffects(protocol.Item{ID: 349, Meta: 1}, nil)) != 0 {
		test.Fatalf("期望仅两种河豚编码对应三项效果，实际 %v", puffer)
	}
}

// 最后一只河豚也按原 meta 判定；持续时间用完通知客户端，不把中毒结算成死亡。
func TestPufferEffectsLifecycle(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	player.food.Food = 10
	player.ApplyInventorySlot(0, protocol.Item{ID: 349, Meta: 3, Count: 1})
	var events []packets.MobEffectPacket = nil
	player.Session.SentHook = func(raw []byte) {
		if raw[0] == protocol.IDMobEffect {
			var pkt packets.MobEffectPacket
			if err := pkt.Decode(protocol.NewStream(raw[1:])); err != nil {
				test.Fatal(err)
			}
			events = append(events, pkt)
		}
	}
	fixture.srv.finishEating(player)
	if len(events) != 3 || events[2].Event != 1 || events[2].Amplifier != 3 || events[2].Duration != 1200 || !player.Inventory[0].IsAir() {
		test.Fatalf("期望三项效果和空槽，实际 %+v / %+v", events, player.Inventory[0])
	}
	fixture.srv.tickEffects(player)
	if player.Health() != 19 || !nearly(player.food.Exhaustion, exhaustDamage+0.075) {
		test.Fatalf("期望 19 血、0.375 疲劳，实际 %v/%v", player.Health(), player.food.Exhaustion)
	}
	for tick := 1; tick < 1200; tick++ {
		fixture.srv.tickEffects(player)
	}
	if player.Health() != 1 || player.IsDead() || len(events) != 6 || events[5].Event != 3 || len(player.combat.Effects) != 0 {
		test.Fatalf("期望半心存活且三项效果均移除，实际血量 %v，事件 %+v", player.Health(), events)
	}
}

// 金苹果恢复、抵伤、刷新及重连后应延续剩下的吸收值，而不是重新发满。
func TestGoldenAppleEffects(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	player.combat.Health = 10
	player.ApplyInventorySlot(0, protocol.Item{ID: 322, Count: 1})
	fixture.srv.finishEating(player)
	for tick := 0; tick < 100; tick++ {
		fixture.srv.tickEffects(player)
	}
	fixture.srv.applyDamage(player, 3, DamageCauseGeneric, nil, true)
	if player.Health() != 15 || player.vitals().Absorption != 1 {
		test.Fatalf("期望 15 血、1 吸收，实际 %+v", player.vitals())
	}
	var saved = player.Snapshot("world")
	if err := storage.SavePlayer(fixture.srv.cfg.DataDir, saved); err != nil {
		test.Fatal(err)
	}
	var loaded, found, err = storage.LoadPlayer(fixture.srv.cfg.DataDir, saved.UUID)
	if !found || err != nil {
		test.Fatalf("期望能读取玩家存档，实际 %v %v", found, err)
	}
	var restored = newTestPlayer(protocol.Air())
	restored.ApplySave(loaded)
	if restored.vitals().Absorption != 1 || !maps.Equal(restored.combat.Effects, player.combat.Effects) {
		test.Fatalf("期望恢复剩余吸收与时长，实际 %+v", restored.combat)
	}
	var enhanced = foodEffects(protocol.Item{ID: 466}, nil)
	for _, effect := range enhanced {
		player.addEffect(effect)
	}
	if fixture.srv.applyDamage(player, 4, DamageCauseLava, nil, true) {
		test.Fatal("期望抗火免疫岩浆，实际受到伤害")
	}
	fixture.srv.applyDamage(player, 10, DamageCauseGeneric, nil, true)
	if player.vitals().Absorption != 8 || player.Health() != 15 {
		test.Fatalf("期望抗性后扣 8 吸收，实际 %+v", player.vitals())
	}
	player.Revive()
	if len(player.combat.Effects) != 0 || player.vitals().Absorption != 0 {
		test.Fatalf("期望重生清除所有效果，实际 %+v", player.combat)
	}
}

// 重复施加时客户端只收到修改；较弱效果或同强度但更短的效果不能缩短已有时长。
func TestEffectRefreshPackets(test *testing.T) {
	var fixture = newAdminFixture(test)
	var player = fixture.join("Alex", "127.0.0.1", "world")
	var changes []byte
	player.Session.SentHook = func(raw []byte) {
		if raw[0] != protocol.IDMobEffect {
			return
		}
		var packet packets.MobEffectPacket
		if err := packet.Decode(protocol.NewStream(raw[1:])); err != nil {
			test.Fatal(err)
		}
		changes = append(changes, packet.Event)
	}
	player.addEffect(storage.EffectData{ID: effectPoison, Amplifier: 1, Duration: 40})
	player.addEffect(storage.EffectData{ID: effectPoison, Duration: 200})
	player.addEffect(storage.EffectData{ID: effectPoison, Amplifier: 1, Duration: 20})
	player.addEffect(storage.EffectData{ID: effectPoison, Amplifier: 1, Duration: 80})
	if !slices.Equal(changes, []byte{packets.MobEffectAdd, packets.MobEffectModify}) || player.combat.Effects[effectPoison].Duration != 80 {
		test.Fatalf("期望新增后仅刷新一次到 80 tick，实际 %v / %+v", changes, player.combat.Effects)
	}
}
