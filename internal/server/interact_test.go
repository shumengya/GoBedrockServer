package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"

// interactTestDoor 在 (3,63,0) 放一块石头，上面立一扇关着的木门。
func interactTestDoor(test *testing.T, fixture *adminFixture, doorID byte) {
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, doorID, 0)
	placeTestPut(test, fixture, 3, 65, 0, doorID, 8)
}

func interactTestTicks(fixture *adminFixture, count int) {
	var tick int = 0
	for tick = 0; tick < count; tick++ {
		fixture.srv.tickScheduled()
	}
}

// 手里拿着方块点关着的门：门开了，方块没放出去，物品也没扣。
func TestClickDoorOpensInsteadOfPlacing(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	interactTestDoor(test, fixture, blockrule.BlockWoodDoor)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var lowerID, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	var upperID, upperMeta = placeTestBlock(fixture, 3, 65, 0)
	if lowerID != blockrule.BlockWoodDoor || lowerMeta != 4 {
		test.Fatalf("下半期望 木门:4，实际 %d:%d", lowerID, lowerMeta)
	}
	if upperID != blockrule.BlockWoodDoor || upperMeta != 8 {
		test.Fatalf("上半不该变，期望 木门:8，实际 %d:%d", upperID, upperMeta)
	}
	var sideID, _ = placeTestBlock(fixture, 4, 64, 0)
	if sideID != blockrule.BlockAir || placeTestCount(player) != 64 {
		test.Fatalf("不该放出方块也不该扣物品，实际 旁边=%d 剩余=%d", sideID, placeTestCount(player))
	}
}

// 点门的上半一样能开，开关位记在下半。
func TestClickDoorTopHalfOpensLower(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	interactTestDoor(test, fixture, blockrule.BlockWoodDoor)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 65, 0, blockrule.FaceEast, 0.5, held)
	var lowerID, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	if lowerID != blockrule.BlockWoodDoor || lowerMeta != 4 {
		test.Fatalf("下半期望 木门:4，实际 %d:%d", lowerID, lowerMeta)
	}
}

// 潜行时不触发交互，方块放到门旁边。
func TestSneakingClickPlacesBlockNextToDoor(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	interactTestDoor(test, fixture, blockrule.BlockWoodDoor)
	player.setSneaking(true)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	var sideID, _ = placeTestBlock(fixture, 4, 64, 0)
	if lowerMeta != 0 {
		test.Fatalf("潜行时门不该被打开，实际 meta=%d", lowerMeta)
	}
	if sideID != blockrule.BlockStone || placeTestCount(player) != 63 {
		test.Fatalf("方块应放到门旁边，实际 旁边=%d 剩余=%d", sideID, placeTestCount(player))
	}
}

// 铁门手点不开，点击也不会往旁边放方块。
func TestClickIronDoorDoesNothing(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	interactTestDoor(test, fixture, blockrule.BlockIronDoor)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	var sideID, _ = placeTestBlock(fixture, 4, 64, 0)
	if lowerMeta != 0 || sideID != blockrule.BlockAir || placeTestCount(player) != 64 {
		test.Fatalf("铁门应纹丝不动，实际 meta=%d 旁边=%d 剩余=%d", lowerMeta, sideID, placeTestCount(player))
	}
}

// 出生点保护范围内，生存玩家不能开门；创造模式不受限制。
func TestSpawnProtectionBlocksInteraction(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	fixture.srv.cfg.SpawnProtection = 16
	interactTestDoor(test, fixture, blockrule.BlockWoodDoor)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, protectedMeta = placeTestBlock(fixture, 3, 64, 0)
	if protectedMeta != 0 {
		test.Fatalf("保护范围内生存玩家不该能开门，实际 meta=%d", protectedMeta)
	}
	updateTestCreative(player)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, creativeMeta = placeTestBlock(fixture, 3, 64, 0)
	if creativeMeta != 4 {
		test.Fatalf("创造模式不受保护限制，期望 meta=4，实际 %d", creativeMeta)
	}
}

// 按钮按下后 20 个 tick 弹回，早一个 tick 都还是按下的。
func TestStoneButtonPopsAfterTwentyTicks(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockStoneButton, 1)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, pressed = placeTestBlock(fixture, 3, 64, 0)
	if pressed != 9 {
		test.Fatalf("按下后期望 meta=9，实际 %d", pressed)
	}
	interactTestTicks(fixture, 19)
	var _, early = placeTestBlock(fixture, 3, 64, 0)
	if early != 9 {
		test.Fatalf("19 个 tick 时还不该弹回，实际 meta=%d", early)
	}
	interactTestTicks(fixture, 1)
	var _, popped = placeTestBlock(fixture, 3, 64, 0)
	if popped != 1 {
		test.Fatalf("20 个 tick 时应弹回，期望 meta=1，实际 %d", popped)
	}
}

// 按下后又点一次，不会把弹回时间越推越晚。
func TestRepeatedClickDoesNotDelayButton(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockStoneButton, 1)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	interactTestTicks(fixture, 10)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	interactTestTicks(fixture, 10)
	var _, meta = placeTestBlock(fixture, 3, 64, 0)
	if meta != 1 {
		test.Fatalf("第一次按下满 20 个 tick 就该弹回，期望 meta=1，实际 %d", meta)
	}
}

// 停在按下状态、队列里却没有任务的按钮（比如重启丢了队列），点一下就能重新弹回来。
func TestStuckButtonHealsOnClick(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockStoneButton, 9)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	interactTestTicks(fixture, 20)
	var _, meta = placeTestBlock(fixture, 3, 64, 0)
	if meta != 1 {
		test.Fatalf("卡住的按钮应在点击后弹回，期望 meta=1，实际 %d", meta)
	}
}

// 拉杆点一下拨开，再点一下拨回。
func TestClickLeverToggles(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockLever, 5)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, on = placeTestBlock(fixture, 3, 64, 0)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	var _, off = placeTestBlock(fixture, 3, 64, 0)
	if on != 13 || off != 5 {
		test.Fatalf("期望 拨开=13 拨回=5，实际 %d、%d", on, off)
	}
}

// 挖掉拉杆靠着的方块，拉杆跟着掉下来，掉一个拉杆物品。
func TestBreakLeverSupportDropsLever(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockLever, 5)
	updateTestBreak(fixture, player, 3, 63, 0)
	var leverID, _ = placeTestBlock(fixture, 3, 64, 0)
	if leverID != blockrule.BlockAir {
		test.Fatalf("拉杆应掉下来，实际 %d", leverID)
	}
	if updateTestDropped(fixture, 69) != 1 {
		test.Fatalf("应掉 1 个拉杆，实际 %d", updateTestDropped(fixture, 69))
	}
}

// 用活板门物品点石头顶面：朝向跟着玩家偏航角。
func TestPlaceTrapdoorFollowsYaw(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.UpdatePosition(0, 64+playerEyeHeight, 0, 270, 0, true)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 96, 0, 5)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockTrapdoor || meta != 1 {
		test.Fatalf("期望 活板门:1，实际 %d:%d", id, meta)
	}
}

// 同一格重复排队，保留更早的到期时间。
func TestScheduleKeepsEarlierDeadline(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var pos blockrule.Pos = blockrule.Pos{PosX: 3, PosY: 64, PosZ: 0}
	var key scheduledKey = scheduledKey{World: "world", Pos: pos}
	fixture.srv.blockMu.Lock()
	defer fixture.srv.blockMu.Unlock()
	fixture.srv.scheduleLocked("world", pos, 20)
	fixture.srv.scheduleLocked("world", pos, 50)
	if fixture.srv.scheduled[key] != 20 {
		test.Fatalf("后排的更晚的不该覆盖，期望 20，实际 %d", fixture.srv.scheduled[key])
	}
	fixture.srv.scheduleLocked("world", pos, 5)
	if fixture.srv.scheduled[key] != 5 {
		test.Fatalf("更早的应该覆盖，期望 5，实际 %d", fixture.srv.scheduled[key])
	}
}

// 一个 tick 最多处理 scheduledBudget 个到期任务，剩下的顺延，不会一次卡死。
func TestScheduledBudgetPerTick(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var extra int = 10
	fixture.srv.blockMu.Lock()
	var index int = 0
	for index = 0; index < scheduledBudget+extra; index++ {
		fixture.srv.scheduleLocked("world", blockrule.Pos{PosX: int32(index), PosY: 64, PosZ: 0}, 1)
	}
	fixture.srv.blockMu.Unlock()
	fixture.srv.tickScheduled()
	fixture.srv.blockMu.Lock()
	var remaining int = len(fixture.srv.scheduled)
	fixture.srv.blockMu.Unlock()
	if remaining != extra {
		test.Fatalf("第一个 tick 后应剩 %d 个，实际 %d", extra, remaining)
	}
	fixture.srv.tickScheduled()
	fixture.srv.blockMu.Lock()
	remaining = len(fixture.srv.scheduled)
	fixture.srv.blockMu.Unlock()
	if remaining != 0 {
		test.Fatalf("第二个 tick 后应清空，实际剩 %d", remaining)
	}
}
