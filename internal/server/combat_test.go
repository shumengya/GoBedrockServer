package server

import "testing"
import "time"

import "go-mc-server/internal/protocol"

func newTestPlayer(held protocol.Item) *Player {
	var player *Player = &Player{
		Inventory: make([]protocol.Item, protocol.PlayerInventorySize),
		combat:    combatState{Health: maxPlayerHealth},
	}
	player.Inventory[0] = held
	player.HeldItem = held
	return player
}

func TestDamageHeldToolBreaksAtMax(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: itemGoldPickaxe, Meta: 31, Count: 1})
	var slot, item, changed = player.DamageHeldTool(1)
	if !changed || slot != 0 || item.Meta != 32 {
		test.Fatalf("金镐第 32 次应剩 1 点, got slot=%d item=%+v changed=%v", slot, item, changed)
	}
	slot, item, changed = player.DamageHeldTool(1)
	if !changed || !item.IsAir() || !player.HeldItem.IsAir() || !player.Inventory[0].IsAir() {
		test.Fatalf("金镐达到 33 次应损坏, got item=%+v held=%+v", item, player.HeldItem)
	}
}

func TestDamageHeldToolSkips(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: 4, Count: 64})
	var slot, item, changed = player.DamageHeldTool(1)
	if changed || slot != 0 || item.Meta != 0 {
		test.Fatalf("圆石不是工具，不应扣耐久, got %+v", item)
	}
	player = newTestPlayer(protocol.Item{ID: itemIronPickaxe, Count: 1})
	player.GameMode = int32(protocol.GameModeCreative)
	slot, item, changed = player.DamageHeldTool(1)
	if changed || slot != 0 || item.Meta != 0 {
		test.Fatalf("创造模式不应扣耐久, got %+v", item)
	}
}

func TestToolWearAmounts(test *testing.T) {
	var stone blockInfo = miningInfoFor(1)
	var flower blockInfo = miningInfoFor(38)
	if miningWear(itemOf(itemIronPickaxe), stone) != 1 {
		test.Fatal("镐子挖石头扣 1")
	}
	if miningWear(itemOf(itemIronSword), stone) != 2 {
		test.Fatal("剑挖方块扣 2")
	}
	if miningWear(itemOf(itemIronPickaxe), flower) != 0 {
		test.Fatal("硬度 0 的方块不扣耐久")
	}
	if attackWear(itemOf(itemIronSword)) != 1 {
		test.Fatal("剑打人扣 1")
	}
	if attackWear(itemOf(itemIronAxe)) != 2 {
		test.Fatal("斧子打人扣 2")
	}
	if attackWear(protocol.Air()) != 0 {
		test.Fatal("空手打人不扣")
	}
}

func TestToolsDoNotStack(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: itemIronPickaxe, Meta: 5, Count: 1})
	var leftover, added = player.AddItem(protocol.Item{ID: itemIronPickaxe, Meta: 5, Count: 1})
	if !added || !leftover.IsAir() {
		test.Fatalf("应放进空格, leftover=%+v", leftover)
	}
	if player.Inventory[0].Count != 1 || player.Inventory[1].Count != 1 {
		test.Fatalf("工具不应叠加, slot0=%+v slot1=%+v", player.Inventory[0], player.Inventory[1])
	}
}

func TestTakeDamageCooldownAndDeath(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	var start time.Time = time.Unix(1000, 0)
	var applied, health, died = player.takeDamage(8, DamageCauseGeneric, start, false)
	if !applied || health != 12 || died {
		test.Fatalf("第一次应扣到 12, got applied=%v health=%v died=%v", applied, health, died)
	}
	applied, health, _ = player.takeDamage(8, DamageCauseGeneric, start.Add(100*time.Millisecond), false)
	if applied || health != 12 {
		test.Fatalf("冷却中不应扣血, got applied=%v health=%v", applied, health)
	}
	applied, health, died = player.takeDamage(8, DamageCauseGeneric, start.Add(600*time.Millisecond), false)
	if !applied || health != 4 || died {
		test.Fatalf("冷却后应扣到 4, got %v %v %v", applied, health, died)
	}
	applied, health, died = player.takeDamage(8, DamageCauseGeneric, start.Add(1200*time.Millisecond), false)
	if !applied || health != 0 || !died || !player.IsDead() {
		test.Fatalf("应死亡, got %v %v %v", applied, health, died)
	}
	applied, _, _ = player.takeDamage(8, DamageCauseGeneric, start.Add(2000*time.Millisecond), false)
	if applied {
		test.Fatal("死亡后不应再受伤")
	}
	player.Revive()
	if player.IsDead() || player.Health() != maxPlayerHealth {
		test.Fatal("重生后应回满血")
	}
}

func TestTakeDamageCreativeImmune(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	player.GameMode = int32(protocol.GameModeCreative)
	var applied, health, died = player.takeDamage(5, DamageCauseGeneric, time.Now(), false)
	if applied || died || health != maxPlayerHealth {
		test.Fatal("创造模式不应受伤")
	}
}

func TestBreakRejectReason(test *testing.T) {
	var start time.Time = time.Unix(2000, 0)
	var stone blockInfo = miningInfoFor(1)
	var target blockPos = blockPos{PosX: 1, PosY: 2, PosZ: 3}
	var elsewhere blockPos = blockPos{PosX: 9, PosY: 2, PosZ: 3}
	var state miningState = miningState{
		Active:  true,
		BlockX:  1,
		BlockY:  2,
		BlockZ:  3,
		BlockID: 1,
		Started: start,
		Ticks:   23,
	}
	var survival int32 = int32(protocol.GameModeSurvival)
	if breakRejectReason(survival, stone, state, target, 1, start.Add(100*time.Millisecond)) == "" {
		test.Fatal("挖 0.1 秒就破坏应被拒")
	}
	if breakRejectReason(survival, stone, state, target, 1, start.Add(1150*time.Millisecond)) != "" {
		test.Fatal("挖够时间应放行")
	}
	if breakRejectReason(survival, stone, miningState{}, target, 1, start) == "" {
		test.Fatal("没发 START_BREAK 应被拒")
	}
	if breakRejectReason(survival, stone, state, elsewhere, 1, start.Add(5*time.Second)) == "" {
		test.Fatal("坐标不一致应被拒")
	}
	if breakRejectReason(int32(protocol.GameModeCreative), stone, miningState{}, target, 1, start) != "" {
		test.Fatal("创造模式秒破应放行")
	}
	if breakRejectReason(survival, miningInfoFor(38), miningState{}, target, 38, start) != "" {
		test.Fatal("硬度 0 的花应能直接破坏")
	}
	if breakRejectReason(survival, miningInfoFor(7), state, target, 7, start.Add(time.Hour)) == "" {
		test.Fatal("基岩永远不可破坏")
	}
}

func TestBlockInReach(test *testing.T) {
	if !blockInReach(0.5, 64, 0.5, blockPos{PosX: 0, PosY: 63, PosZ: 3}, maxBreakDistanceSurvival) {
		test.Fatal("脚下附近的方块应在范围内")
	}
	if blockInReach(0.5, 64, 0.5, blockPos{PosX: 0, PosY: 64, PosZ: 20}, maxBreakDistanceSurvival) {
		test.Fatal("20 格外应超出范围")
	}
}
