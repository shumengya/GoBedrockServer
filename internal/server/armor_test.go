package server

import "maps"
import "slices"
import "testing"

import "go-mc-server/internal/entity"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

// armorTestDuel 两个站在一起的玩家：attacker 是builder（1 号），victim 是被打的（2 号），互相可见，PVP 开着。
func armorTestDuel(test *testing.T) (*adminFixture, *Player, *Player) {
	var fixture, attacker = placeTestSetup(test)
	var victim *Player = fixture.join("victim", "10.0.0.2", "world")
	attacker.Visible = true
	victim.Visible = true
	return fixture, attacker, victim
}

// armorTestEquip 给玩家穿一整套：first 是头盔的物品号，后面三件依次 +1（皮革 298、锁链 302、铁 306、钻石 310、金 314）。
func armorTestEquip(player *Player, first int32) {
	var slot int = 0
	for slot = 0; slot < protocol.ArmorSize; slot++ {
		player.setArmorSlot(slot, protocol.Item{ID: first + int32(slot), Count: 1})
	}
}

// armorTestSwing attacker 手里拿 weaponID（0 是空手）左键打 victim 一下。
func armorTestSwing(fixture *adminFixture, attacker *Player, victim *Player, weaponID int32) {
	var weapon protocol.Item = protocol.Air()
	if weaponID > 0 {
		weapon = protocol.Item{ID: weaponID, Count: 1}
	}
	attacker.mu.Lock()
	attacker.Inventory[0] = weapon
	attacker.HeldSlot = 0
	attacker.HeldItem = weapon
	attacker.mu.Unlock()
	fixture.srv.handleInteract(attacker.Session, &packets.InteractPacket{
		Action: packets.InteractLeftClick,
		Target: victim.EntityRuntimeID,
	})
}

func armorTestSame(left protocol.Item, right protocol.Item) bool {
	return left.ID == right.ID && left.Meta == right.Meta && left.Count == right.Count
}

type armorHitCase struct {
	Name   string
	Weapon int32
	Set    int32
	Expect float32
}

// 真实攻击链路上的减伤。预期血量照公式手算（满血 20）：
// 铁剑 7 打铁套 15 点：7×0.4=2.8→3，剩 17；钻石剑 8 打钻石套 20 点：8×0.2=1.6→2，剩 18；
// 木剑 5 打皮革套 7 点：5×0.72=3.6→4，剩 16；石剑 6 打锁链套 12 点：6×0.52=3.12→3，剩 17；
// 金剑 5 打金套 11 点：5×0.56=2.8→3，剩 17；没穿甲：铁剑 7，剩 13。
func TestArmorReducesPlayerAttack(test *testing.T) {
	var cases []armorHitCase = []armorHitCase{
		{Name: "铁剑打铁套", Weapon: itemIronSword, Set: 306, Expect: 17},
		{Name: "钻石剑打钻石套", Weapon: itemDiamondSword, Set: 310, Expect: 18},
		{Name: "木剑打皮革套", Weapon: itemWoodenSword, Set: 298, Expect: 16},
		{Name: "石剑打锁链套", Weapon: itemStoneSword, Set: 302, Expect: 17},
		{Name: "金剑打金套", Weapon: itemGoldSword, Set: 314, Expect: 17},
		{Name: "铁剑打没穿甲的", Weapon: itemIronSword, Set: 0, Expect: 13},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item armorHitCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, attacker, victim = armorTestDuel(sub)
			if item.Set > 0 {
				armorTestEquip(victim, item.Set)
			}
			armorTestSwing(fixture, attacker, victim, item.Weapon)
			if victim.Health() != item.Expect {
				sub.Fatalf("期望剩 %v 血，实际 %v", item.Expect, victim.Health())
			}
		})
	}
}

// 被打中一次，每件盔甲磨损 1 点。受害者自己走盔甲窗口收到新状态，旁人走 MobArmorEquipment，
// 受害者不会收到关于自己的 MobArmorEquipment。
func TestArmorWearsOnHitAndSyncs(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	armorTestEquip(victim, 306)
	var victimLog *sentLog = captureSent(victim)
	var attackerLog *sentLog = captureSent(attacker)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	var armor [protocol.ArmorSize]protocol.Item = victim.armorSnapshot()
	var slot int = 0
	for slot = 0; slot < protocol.ArmorSize; slot++ {
		if armor[slot].Meta != 1 || armor[slot].ID != 306+int32(slot) {
			test.Fatalf("第 %d 件应是 %d 损耗 1，实际 %+v", slot, 306+slot, armor[slot])
		}
	}
	var own, ownSent = victimLog.lastContainerContent(test, protocol.ContainerArmor)
	if !ownSent || len(own) != protocol.ArmorSize || own[0].Meta != 1 || own[3].Meta != 1 {
		test.Fatalf("受害者应收到盔甲窗口的新内容，实际 发过=%v %+v", ownSent, own)
	}
	var seen, seenSent = attackerLog.lastArmorPacket(test, victim.EntityRuntimeID)
	if !seenSent || seen.Slots[0].Meta != 1 || seen.Slots[2].ID != 308 {
		test.Fatalf("旁人应收到受害者的穿戴，实际 发过=%v %+v", seenSent, seen.Slots)
	}
	var _, selfSent = victimLog.lastArmorPacket(test, victim.EntityRuntimeID)
	if selfSent {
		test.Fatal("受害者自己不该收到关于自己的 MobArmorEquipment")
	}
}

// 磨满的那件消失：靴子最大耐久 196，损耗 195 再挨一下就没了，别的件只是各加 1。
// 受害者和旁人收到的新状态里靴子位都是空的。
func TestArmorBreaksWhenWornOut(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	armorTestEquip(victim, 306)
	victim.setArmorSlot(armorSlotBoots, protocol.Item{ID: 309, Meta: 195, Count: 1})
	var victimLog *sentLog = captureSent(victim)
	var attackerLog *sentLog = captureSent(attacker)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	var armor [protocol.ArmorSize]protocol.Item = victim.armorSnapshot()
	if !armor[armorSlotBoots].IsAir() || armor[armorSlotBoots].ID != 0 {
		test.Fatalf("磨满的靴子应变成空气，实际 %+v", armor[armorSlotBoots])
	}
	if armor[armorSlotHelmet].Meta != 1 || armor[armorSlotLeggings].Meta != 1 {
		test.Fatalf("没磨满的两件应各加 1，实际 %+v / %+v", armor[armorSlotHelmet], armor[armorSlotLeggings])
	}
	var own, _ = victimLog.lastContainerContent(test, protocol.ContainerArmor)
	if len(own) != protocol.ArmorSize || !own[armorSlotBoots].IsAir() {
		test.Fatalf("受害者收到的靴子位应为空，实际 %+v", own)
	}
	var seen, _ = attackerLog.lastArmorPacket(test, victim.EntityRuntimeID)
	if !seen.Slots[armorSlotBoots].IsAir() {
		test.Fatalf("旁人看到的靴子位应为空，实际 %+v", seen.Slots[armorSlotBoots])
	}
}

// 磨损的边界：损耗 194 的靴子挨一下变 195 还在；再挨一下满 196 才消失。没有盔甲就什么都不改、返回 false。
func TestWearArmorBoundary(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	if player.wearArmor() {
		test.Fatal("没穿盔甲不该有改动")
	}
	player.setArmorSlot(armorSlotBoots, protocol.Item{ID: 309, Meta: 194, Count: 1})
	if !player.wearArmor() || player.armorSnapshot()[armorSlotBoots].Meta != 195 {
		test.Fatalf("194 磨一下应是 195 且还在，实际 %+v", player.armorSnapshot()[armorSlotBoots])
	}
	if !player.wearArmor() || !player.armorSnapshot()[armorSlotBoots].IsAir() {
		test.Fatalf("195 再磨一下满 196 应消失，实际 %+v", player.armorSnapshot()[armorSlotBoots])
	}
}

// 没穿盔甲的人挨打，不会收到任何盔甲包，旁人也不会：没变化就别发。
func TestNoArmorNoSyncNoise(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	var victimLog *sentLog = captureSent(victim)
	var attackerLog *sentLog = captureSent(attacker)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	if victimLog.countContent(test, protocol.ContainerArmor) != 0 || attackerLog.count(protocol.IDMobArmorEquipment) != 0 {
		test.Fatalf("没穿盔甲不该发盔甲包，实际 受害者=%d 旁人=%d",
			victimLog.countContent(test, protocol.ContainerArmor), attackerLog.count(protocol.IDMobArmorEquipment))
	}
}

// 空手打穿钻石套的人：1 点伤害 ×0.2 四舍五入成 0，这一下被完全挡住——
// 不扣血、不磨损、不进无敌时间、不击退、没有受伤动画。紧接着一剑照样能打中，证明没有进无敌时间。
func TestFullyBlockedHitHasNoSideEffects(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	armorTestEquip(victim, 310)
	var victimLog *sentLog = captureSent(victim)
	armorTestSwing(fixture, attacker, victim, 0)
	if victim.Health() != maxPlayerHealth {
		test.Fatalf("被完全挡住不该掉血，实际 %v", victim.Health())
	}
	var armor [protocol.ArmorSize]protocol.Item = victim.armorSnapshot()
	var slot int = 0
	for slot = 0; slot < protocol.ArmorSize; slot++ {
		if armor[slot].Meta != 0 {
			test.Fatalf("被完全挡住不该磨损，第 %d 件 %+v", slot, armor[slot])
		}
	}
	if !victim.combat.LastDamaged.IsZero() {
		test.Fatal("被完全挡住不该进无敌时间")
	}
	if victimLog.count(protocol.IDSetEntityMotion) != 0 || victimLog.count(protocol.IDEntityEvent) != 0 {
		test.Fatalf("被完全挡住不该击退、不该有受伤动画，实际 击退=%d 动画=%d",
			victimLog.count(protocol.IDSetEntityMotion), victimLog.count(protocol.IDEntityEvent))
	}
	armorTestSwing(fixture, attacker, victim, itemDiamondSword)
	if victim.Health() != 18 {
		test.Fatalf("紧接着的钻石剑 8×0.2=1.6→2 应打中，剩 18，实际 %v", victim.Health())
	}
}

type armorDamageCase struct {
	Name       string
	Cause      DamageCause
	ExpectHeal float32
	ExpectMeta int32
}

// 满钻石套（20 点）挨 10 点伤害：吃减伤的（玩家攻击、仙人掌、岩浆）剩 18，其余原样剩 10；
// 磨损只有玩家攻击才有，仙人掌和岩浆减伤却不磨损。
func TestArmorAppliesPerDamageCause(test *testing.T) {
	var cases []armorDamageCase = []armorDamageCase{
		{Name: "玩家攻击", Cause: DamageCausePlayer, ExpectHeal: 18, ExpectMeta: 1},
		{Name: "仙人掌", Cause: DamageCauseCactus, ExpectHeal: 18, ExpectMeta: 0},
		{Name: "岩浆", Cause: DamageCauseLava, ExpectHeal: 18, ExpectMeta: 0},
		{Name: "着火", Cause: DamageCauseFire, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "摔落", Cause: DamageCauseFall, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "虚空", Cause: DamageCauseVoid, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "窒息", Cause: DamageCauseSuffocation, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "溺水", Cause: DamageCauseDrowning, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "饥饿", Cause: DamageCauseStarvation, ExpectHeal: 10, ExpectMeta: 0},
		{Name: "未知", Cause: DamageCauseGeneric, ExpectHeal: 10, ExpectMeta: 0},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item armorDamageCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, _, victim = armorTestDuel(sub)
			armorTestEquip(victim, 310)
			fixture.srv.applyDamage(victim, 10, item.Cause, nil, true)
			if victim.Health() != item.ExpectHeal {
				sub.Fatalf("期望剩 %v 血，实际 %v", item.ExpectHeal, victim.Health())
			}
			if victim.armorSnapshot()[armorSlotChestplate].Meta != item.ExpectMeta {
				sub.Fatalf("胸甲损耗期望 %d，实际 %d", item.ExpectMeta, victim.armorSnapshot()[armorSlotChestplate].Meta)
			}
		})
	}
}

// 创造模式的人不受伤，也就不磨损盔甲。
func TestCreativeVictimDoesNotWearArmor(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	updateTestCreative(victim)
	armorTestEquip(victim, 306)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	if victim.Health() != maxPlayerHealth || victim.armorSnapshot()[armorSlotHelmet].Meta != 0 {
		test.Fatalf("创造模式不受伤不磨损，实际 血=%v 头盔=%+v", victim.Health(), victim.armorSnapshot()[armorSlotHelmet])
	}
}

// 无敌时间里的第二下被拒绝，只算一次磨损、只扣一次血。
func TestHitDuringCooldownWearsArmorOnce(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	armorTestEquip(victim, 306)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	if victim.Health() != 17 || victim.armorSnapshot()[armorSlotChestplate].Meta != 1 {
		test.Fatalf("无敌时间里的第二下不算，期望 血=17 损耗=1，实际 血=%v 损耗=%d",
			victim.Health(), victim.armorSnapshot()[armorSlotChestplate].Meta)
	}
}

func armorTestDroppedMetas(fixture *adminFixture, first int32, last int32) []int32 {
	fixture.srv.mu.RLock()
	defer fixture.srv.mu.RUnlock()
	var metas []int32 = nil
	var keys []int64 = slices.Sorted(maps.Keys(fixture.srv.entities))
	var index int = 0
	for index = 0; index < len(keys); index++ {
		var dropped *entity.Instance = fixture.srv.entities[keys[index]]
		if dropped.IsDroppedItem() && dropped.Item.ID >= first && dropped.Item.ID <= last {
			metas = append(metas, dropped.Item.Meta)
		}
	}
	return metas
}

// 致命一击：这一下不磨损（人已经死了），盔甲连同背包一起掉在地上，身上清空，
// 自己和旁人都收到空的盔甲状态。
func TestLethalHitDropsArmorAndClearsIt(test *testing.T) {
	var fixture, attacker, victim = armorTestDuel(test)
	armorTestEquip(victim, 306)
	victim.combat.Health = 2
	var victimLog *sentLog = captureSent(victim)
	var attackerLog *sentLog = captureSent(attacker)
	armorTestSwing(fixture, attacker, victim, itemIronSword)
	if !victim.IsDead() {
		test.Fatal("夹具：受害者应已死亡")
	}
	var metas []int32 = armorTestDroppedMetas(fixture, 306, 309)
	if len(metas) != 4 {
		test.Fatalf("4 件盔甲都该掉出来，实际 %d 件", len(metas))
	}
	var index int = 0
	for index = 0; index < len(metas); index++ {
		if metas[index] != 0 {
			test.Fatalf("致命一击不磨损，掉出的盔甲损耗应为 0，实际 %v", metas)
		}
	}
	if !armorEmpty(victim.armorSnapshot()) {
		test.Fatalf("身上应清空，实际 %+v", victim.armorSnapshot())
	}
	var own, ownSent = victimLog.lastContainerContent(test, protocol.ContainerArmor)
	if !ownSent || len(own) != protocol.ArmorSize || !armorEmpty([protocol.ArmorSize]protocol.Item{own[0], own[1], own[2], own[3]}) {
		test.Fatalf("死者应收到空的盔甲窗口，实际 发过=%v %+v", ownSent, own)
	}
	var seen, seenSent = attackerLog.lastArmorPacket(test, victim.EntityRuntimeID)
	if !seenSent || !armorEmpty(seen.Slots) {
		test.Fatalf("旁人看到的应是空的，实际 发过=%v %+v", seenSent, seen.Slots)
	}
}

// 死亡掉落要把盔甲也搬走。
func TestTakeAllItemsIncludesArmor(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	player.Inventory[5] = protocol.Item{ID: 1, Count: 64}
	player.setArmorSlot(armorSlotHelmet, protocol.Item{ID: 310, Meta: 7, Count: 1})
	player.setArmorSlot(armorSlotBoots, protocol.Item{ID: 313, Count: 1})
	var dumped []protocol.Item = player.takeAllItems()
	if len(dumped) != 3 {
		test.Fatalf("应掉出石头加 2 件盔甲共 3 组，实际 %d", len(dumped))
	}
	if !armorEmpty(player.armorSnapshot()) {
		test.Fatalf("盔甲应清空，实际 %+v", player.armorSnapshot())
	}
}

// 盔甲不叠加：两件头盔占两格，每格 1 件，不会合成一格 2 件。
func TestArmorDoesNotStackInInventory(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	player.AddItem(protocol.Item{ID: 306, Count: 1})
	player.AddItem(protocol.Item{ID: 306, Count: 1})
	if player.Inventory[0].ID != 306 || player.Inventory[0].Count != 1 || player.Inventory[1].ID != 306 || player.Inventory[1].Count != 1 {
		test.Fatalf("两件头盔应各占一格，实际 %+v / %+v", player.Inventory[0], player.Inventory[1])
	}
}

type armorSlotCase struct {
	Name   string
	Slot   int32
	Item   protocol.Item
	Accept bool
}

// 客户端往盔甲槽放东西。夹具里头盔槽原本是一件钻石头盔。
// 收下的：对的盔甲、取下（空）、数量 0 的（当空）；不收的：放错槽、石头、叠了两件、损耗值是负数——
// 不收就把真实的这一格发回去，旁人不受打扰。
func TestClientArmorSlotChange(test *testing.T) {
	var cases []armorSlotCase = []armorSlotCase{
		{Name: "铁胸甲放胸甲槽", Slot: 1, Item: protocol.Item{ID: 307, Count: 1}, Accept: true},
		{Name: "铁头盔换下钻石头盔", Slot: 0, Item: protocol.Item{ID: 306, Count: 1}, Accept: true},
		{Name: "取下头盔", Slot: 0, Item: protocol.Air(), Accept: true},
		{Name: "数量 0 的当空物品", Slot: 0, Item: protocol.Item{ID: 306, Count: 0}, Accept: true},
		{Name: "头盔放进胸甲槽", Slot: 1, Item: protocol.Item{ID: 306, Count: 1}, Accept: false},
		{Name: "石头放进头盔槽", Slot: 0, Item: protocol.Item{ID: 1, Count: 1}, Accept: false},
		{Name: "叠了两件头盔", Slot: 0, Item: protocol.Item{ID: 306, Count: 2}, Accept: false},
		{Name: "损耗值是负数的靴子", Slot: 3, Item: protocol.Item{ID: 313, Meta: -5, Count: 1}, Accept: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item armorSlotCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, viewer, wearer = armorTestDuel(sub)
			var initial protocol.Item = protocol.Item{ID: 310, Count: 1}
			wearer.setArmorSlot(armorSlotHelmet, initial)
			var wearerLog *sentLog = captureSent(wearer)
			var viewerLog *sentLog = captureSent(viewer)
			fixture.srv.handleContainerSetSlot(wearer.Session, &packets.ContainerSetSlotPacket{
				WindowID: protocol.ContainerArmor,
				Slot:     item.Slot,
				Item:     item.Item,
			})
			var now protocol.Item = wearer.armorSnapshot()[item.Slot]
			if item.Accept {
				var want protocol.Item = item.Item
				if want.IsAir() {
					want = protocol.Air()
				}
				if !armorTestSame(now, want) || now.ID != want.ID {
					sub.Fatalf("应收下 %+v，实际 %+v", want, now)
				}
				var seen, seenSent = viewerLog.lastArmorPacket(sub, wearer.EntityRuntimeID)
				if !seenSent || !armorTestSame(seen.Slots[item.Slot], want) || seen.Slots[item.Slot].ID != want.ID {
					sub.Fatalf("旁人应看到新的穿戴，实际 发过=%v %+v", seenSent, seen.Slots[item.Slot])
				}
				var _, resent = wearerLog.lastContainerSlot(sub, protocol.ContainerArmor)
				if resent {
					sub.Fatal("收下的不该把格子再发回去")
				}
				return
			}
			var expected protocol.Item = protocol.Air()
			if item.Slot == int32(armorSlotHelmet) {
				expected = initial
			}
			if !armorTestSame(now, expected) {
				sub.Fatalf("不该收下，这一格应保持 %+v，实际 %+v", expected, now)
			}
			var back, backSent = wearerLog.lastContainerSlot(sub, protocol.ContainerArmor)
			if !backSent || back.Slot != item.Slot || !armorTestSame(back.Item, expected) {
				sub.Fatalf("应把真实的这一格发回去，实际 发过=%v %+v", backSent, back)
			}
			var _, seenSent = viewerLog.lastArmorPacket(sub, wearer.EntityRuntimeID)
			if seenSent {
				sub.Fatal("没收下的不该惊动旁人")
			}
		})
	}
}

// 槽位号越界、死人的改动：一律忽略，不改不发。
func TestClientArmorSlotIgnored(test *testing.T) {
	var fixture, viewer, wearer = armorTestDuel(test)
	var wearerLog *sentLog = captureSent(wearer)
	var viewerLog *sentLog = captureSent(viewer)
	var slots []int32 = []int32{-1, 4, 100}
	var index int = 0
	for index = 0; index < len(slots); index++ {
		fixture.srv.handleContainerSetSlot(wearer.Session, &packets.ContainerSetSlotPacket{
			WindowID: protocol.ContainerArmor,
			Slot:     slots[index],
			Item:     protocol.Item{ID: 306, Count: 1},
		})
	}
	wearer.combat.Dead = true
	fixture.srv.handleContainerSetSlot(wearer.Session, &packets.ContainerSetSlotPacket{
		WindowID: protocol.ContainerArmor,
		Slot:     1,
		Item:     protocol.Item{ID: 307, Count: 1},
	})
	if !armorEmpty(wearer.armorSnapshot()) {
		test.Fatalf("越界和死人的改动都不该生效，实际 %+v", wearer.armorSnapshot())
	}
	var _, backSent = wearerLog.lastContainerSlot(test, protocol.ContainerArmor)
	if backSent || viewerLog.count(protocol.IDMobArmorEquipment) != 0 {
		test.Fatal("被忽略的改动不该发任何包")
	}
}

// 盔甲带着损耗值随存档往返，经过真正的 JSON 文件也一样；没穿盔甲的玩家快照里没有这个字段。
func TestArmorSurvivesSnapshotAndDisk(test *testing.T) {
	var fixture, source = placeTestSetup(test)
	var target *Player = fixture.join("target", "10.0.0.2", "world")
	var empty storage.PlayerData = source.Snapshot("world")
	if empty.Armor != nil {
		test.Fatalf("没穿盔甲的快照不该带这个字段，实际 %d 格", len(empty.Armor))
	}
	source.setArmorSlot(armorSlotHelmet, protocol.Item{ID: 306, Meta: 12, Count: 1})
	source.setArmorSlot(armorSlotBoots, protocol.Item{ID: 313, Meta: 400, Count: 1})
	var directory string = test.TempDir()
	var saveErr error = storage.SavePlayer(directory, source.Snapshot("world"))
	if saveErr != nil {
		test.Fatalf("存盘失败: %v", saveErr)
	}
	var loaded, found, loadErr = storage.LoadPlayer(directory, source.UUID)
	if loadErr != nil || !found {
		test.Fatalf("读盘失败 found=%v err=%v", found, loadErr)
	}
	target.ApplySave(loaded)
	var armor [protocol.ArmorSize]protocol.Item = target.armorSnapshot()
	if !armorTestSame(armor[armorSlotHelmet], protocol.Item{ID: 306, Meta: 12, Count: 1}) {
		test.Fatalf("头盔期望 306:12，实际 %+v", armor[armorSlotHelmet])
	}
	if !armorTestSame(armor[armorSlotBoots], protocol.Item{ID: 313, Meta: 400, Count: 1}) {
		test.Fatalf("靴子期望 313:400，实际 %+v", armor[armorSlotBoots])
	}
	if !armor[armorSlotChestplate].IsAir() || !armor[armorSlotLeggings].IsAir() {
		test.Fatalf("没穿的两件应是空的，实际 %+v / %+v", armor[armorSlotChestplate], armor[armorSlotLeggings])
	}
}

type tamperCase struct {
	Name   string
	Slots  []storage.SlotData
	Expect [protocol.ArmorSize]bool
}

// 读档不信存档文件：放错槽、数量不对、不是盔甲、损耗值越界的一律丢成空，合规的原样保留。
func TestArmorFromSlotsRejectsTampering(test *testing.T) {
	var cases []tamperCase = []tamperCase{
		{
			Name: "各种非法",
			Slots: []storage.SlotData{
				{ID: 313, Count: 1},
				{ID: 307, Count: 2},
				{ID: 1, Count: 1},
				{ID: 309, Meta: 196, Count: 1},
			},
			Expect: [protocol.ArmorSize]bool{false, false, false, false},
		},
		{
			Name: "全部合规",
			Slots: []storage.SlotData{
				{ID: 306, Meta: 165, Count: 1},
				{ID: 307, Count: 1},
				{ID: 308, Count: 1},
				{ID: 309, Meta: 195, Count: 1},
			},
			Expect: [protocol.ArmorSize]bool{true, true, true, true},
		},
		{
			Name:   "没有这个字段的老存档",
			Slots:  nil,
			Expect: [protocol.ArmorSize]bool{false, false, false, false},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item tamperCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var armor [protocol.ArmorSize]protocol.Item = armorFromSlots(item.Slots)
			var slot int = 0
			for slot = 0; slot < protocol.ArmorSize; slot++ {
				if !armor[slot].IsAir() != item.Expect[slot] {
					sub.Fatalf("第 %d 格应保留=%v，实际 %+v", slot, item.Expect[slot], armor[slot])
				}
			}
		})
	}
}

// 新玩家出现在别人眼前：先 AddPlayer，再互相告知穿戴（AddPlayer 之前发，客户端还没有这个实体，会丢掉）。
func TestShowPlayerSendsArmorAfterAddPlayer(test *testing.T) {
	var fixture, viewer, newcomer = armorTestDuel(test)
	newcomer.Visible = false
	armorTestEquip(newcomer, 306)
	viewer.setArmorSlot(armorSlotBoots, protocol.Item{ID: 313, Count: 1})
	var viewerLog *sentLog = captureSent(viewer)
	var newcomerLog *sentLog = captureSent(newcomer)
	fixture.srv.showPlayerInWorld(newcomer)
	var addIndex int = viewerLog.firstIndexOf(protocol.IDAddPlayer)
	var armorIndex int = viewerLog.firstIndexOf(protocol.IDMobArmorEquipment)
	if addIndex < 0 || armorIndex < 0 || armorIndex < addIndex {
		test.Fatalf("旁观者应先收到 AddPlayer 再收到盔甲，实际 AddPlayer=%d 盔甲=%d", addIndex, armorIndex)
	}
	var seen, seenSent = viewerLog.lastArmorPacket(test, newcomer.EntityRuntimeID)
	if !seenSent || seen.Slots[armorSlotHelmet].ID != 306 {
		test.Fatalf("旁观者应看到新来的穿着铁头盔，实际 发过=%v %+v", seenSent, seen.Slots)
	}
	var back, backSent = newcomerLog.lastArmorPacket(test, viewer.EntityRuntimeID)
	if !backSent || back.Slots[armorSlotBoots].ID != 313 {
		test.Fatalf("新来的应看到旁观者穿着钻石靴子，实际 发过=%v %+v", backSent, back.Slots)
	}
}

// 两边都没穿盔甲，就不发盔甲包。
func TestShowPlayerWithoutArmorSendsNoArmorPackets(test *testing.T) {
	var fixture, viewer, newcomer = armorTestDuel(test)
	newcomer.Visible = false
	var viewerLog *sentLog = captureSent(viewer)
	var newcomerLog *sentLog = captureSent(newcomer)
	fixture.srv.showPlayerInWorld(newcomer)
	if viewerLog.count(protocol.IDMobArmorEquipment) != 0 || newcomerLog.count(protocol.IDMobArmorEquipment) != 0 {
		test.Fatal("都没穿盔甲不该发盔甲包")
	}
}

// 自己的盔甲走盔甲窗口的 ContainerSetContent，4 格，内容对得上。
func TestSendOwnArmorLayout(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.setArmorSlot(armorSlotHelmet, protocol.Item{ID: 306, Count: 1})
	player.setArmorSlot(armorSlotBoots, protocol.Item{ID: 313, Meta: 9, Count: 1})
	var log *sentLog = captureSent(player)
	fixture.srv.sendOwnArmor(player)
	var slots, sent = log.lastContainerContent(test, protocol.ContainerArmor)
	if !sent || len(slots) != protocol.ArmorSize {
		test.Fatalf("应发 4 格的盔甲窗口，实际 发过=%v 格数=%d", sent, len(slots))
	}
	if slots[0].ID != 306 || !slots[1].IsAir() || !slots[2].IsAir() || slots[3].ID != 313 || slots[3].Meta != 9 {
		test.Fatalf("内容不对: %+v", slots)
	}
}

// 重生时重发一遍盔甲（死亡时清空了，客户端那边也要对上）。
func TestRespawnResendsArmor(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.combat.Dead = true
	var log *sentLog = captureSent(player)
	fixture.srv.respawnPlayer(player)
	if log.countContent(test, protocol.ContainerArmor) != 1 {
		test.Fatalf("重生应发 1 次盔甲窗口，实际 %d", log.countContent(test, protocol.ContainerArmor))
	}
}

// 进服时把盔甲发给玩家自己。
func TestJoinSendsOwnArmor(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.Session.SetState(network.StateSpawning)
	armorTestEquip(player, 310)
	var log *sentLog = captureSent(player)
	fixture.srv.sendChunksAndSpawn(player.Session, 4)
	var slots, sent = log.lastContainerContent(test, protocol.ContainerArmor)
	if !sent || len(slots) != protocol.ArmorSize || slots[0].ID != 310 || slots[3].ID != 313 {
		test.Fatalf("进服应把盔甲发给自己，实际 发过=%v %+v", sent, slots)
	}
}
