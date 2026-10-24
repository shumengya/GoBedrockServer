package server

import "sync"
import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// placeTestSetup 返回一台不联网的服务器和一个站在原点、眼睛高 64+1.62 的生存玩家。
// 平坦世界的地面只到 y=4，测试里要放方块得自己在玩家手够得着的地方垫一块支撑。
func placeTestSetup(test *testing.T) (*adminFixture, *Player) {
	var fixture *adminFixture = newAdminFixture(test)
	var player *Player = fixture.join("builder", "10.0.0.1", "world")
	return fixture, player
}

func placeTestPut(test *testing.T, fixture *adminFixture, posX int32, posY int32, posZ int32, id byte, meta byte) {
	if !fixture.srv.world.SetBlock(posX, posY, posZ, id, meta) {
		test.Fatalf("摆放测试方块失败 (%d,%d,%d)", posX, posY, posZ)
	}
}

// placeTestHold 把第一格设成指定物品并拿在手上。
func placeTestHold(player *Player, id int32, meta int32, count byte) protocol.Item {
	var stack protocol.Item = protocol.Item{ID: id, Meta: meta, Count: count}
	player.mu.Lock()
	defer player.mu.Unlock()
	player.Inventory[0] = stack
	player.HeldSlot = 0
	player.HeldItem = stack
	return stack
}

func placeTestCount(player *Player) byte {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.Inventory[0].Count
}

// placeTestUse 模拟玩家拿着 held 点击 (posX,posY,posZ) 的 face 面，点击点在面中心、高度 clickY。
func placeTestUse(
	fixture *adminFixture,
	player *Player,
	posX int32,
	posY int32,
	posZ int32,
	face int32,
	clickY float32,
	held protocol.Item,
) {
	fixture.srv.handleUseItem(player.Session, &packets.UseItemPacket{
		X:    posX,
		Y:    posY,
		Z:    posZ,
		Face: face,
		FX:   0.5,
		FY:   clickY,
		FZ:   0.5,
		Item: held,
	})
}

func placeTestBlock(fixture *adminFixture, posX int32, posY int32, posZ int32) (byte, byte) {
	return fixture.srv.world.GetBlock(posX, posY, posZ)
}

// 点支撑方块顶面，方块落在它上面，背包扣一个。
func TestPlaceHeldBlockConsumesOne(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockStone || meta != 0 {
		test.Fatalf("落点期望 石头:0，实际 %d:%d", id, meta)
	}
	if placeTestCount(player) != 63 {
		test.Fatalf("背包期望剩 63，实际 %d", placeTestCount(player))
	}
}

// 楼梯的朝向跟着玩家偏航角走：偏航 90 看向西，楼梯往西升（meta 1）。
func TestPlaceStairFollowsPlayerYaw(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.UpdatePosition(0, 64+playerEyeHeight, 0, 90, 0, true)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 53, 0, 5)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockWoodStairs || meta != 1 {
		test.Fatalf("落点期望 楼梯:1，实际 %d:%d", id, meta)
	}
}

// 点击的方块在交互距离(8)内，但落点超出放置距离(7)，要被拒绝，背包不扣。
func TestPlaceBeyondPlaceDistanceRejected(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 7, 64, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 7, 64, 0, blockrule.FaceEast, 0.5, held)
	var id, _ = placeTestBlock(fixture, 8, 64, 0)
	if id != blockrule.BlockAir {
		test.Fatalf("落点超出放置距离，应保持空气，实际 %d", id)
	}
	if placeTestCount(player) != 64 {
		test.Fatalf("被拒绝不该扣背包，实际剩 %d", placeTestCount(player))
	}
}

// 点到高草，直接把高草那一格换成方块，而不是放到高草旁边。
func TestPlaceReplacesTallGrass(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockTallGrass, 1)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 1, held)
	var id, _ = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockStone {
		test.Fatalf("高草那格期望换成石头，实际 %d", id)
	}
	var above, _ = placeTestBlock(fixture, 3, 65, 0)
	if above != blockrule.BlockAir {
		test.Fatalf("方块不该放到高草上方，实际 %d", above)
	}
	if placeTestCount(player) != 63 {
		test.Fatalf("背包期望剩 63，实际 %d", placeTestCount(player))
	}
}

// 实心方块不能放进玩家自己的脚底那一格；火把这类不实心的可以。
func TestPlaceIntoOwnBodyColumn(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 0, 63, 0, blockrule.BlockStone, 0)
	var stone protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 0, 63, 0, blockrule.FaceUp, 1, stone)
	var solidID, _ = placeTestBlock(fixture, 0, 64, 0)
	if solidID != blockrule.BlockAir {
		test.Fatalf("实心方块不该放进自己脚底，实际 %d", solidID)
	}
	if placeTestCount(player) != 64 {
		test.Fatalf("被拒绝不该扣背包，实际剩 %d", placeTestCount(player))
	}
	var torch protocol.Item = placeTestHold(player, 50, 0, 10)
	placeTestUse(fixture, player, 0, 63, 0, blockrule.FaceUp, 1, torch)
	var torchID, torchMeta = placeTestBlock(fixture, 0, 64, 0)
	if torchID != blockrule.BlockTorch || torchMeta != 5 {
		test.Fatalf("火把期望 火把:5，实际 %d:%d", torchID, torchMeta)
	}
	if placeTestCount(player) != 9 {
		test.Fatalf("背包期望剩 9，实际 %d", placeTestCount(player))
	}
}

// 铁剑不是方块，点击没有任何效果；种子本阶段还没有放置规则，同样不放也不扣。
func TestPlaceNonBlockItemsDoNothing(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var sword protocol.Item = placeTestHold(player, 267, 0, 1)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, sword)
	var seeds protocol.Item = placeTestHold(player, 295, 0, 8)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, seeds)
	var id, _ = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockAir {
		test.Fatalf("落点应保持空气，实际 %d", id)
	}
	if placeTestCount(player) != 8 {
		test.Fatalf("种子不该被扣，期望 8，实际 %d", placeTestCount(player))
	}
}

// 服务端手里的方块背包里其实没有，要被拒绝，不能凭空放出方块。
func TestPlaceWithoutItemInInventoryRejected(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	player.mu.Lock()
	player.Inventory[0] = protocol.Air()
	player.mu.Unlock()
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, _ = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockAir {
		test.Fatalf("背包里没有该方块，不该放出来，实际 %d", id)
	}
}

// 点下半砖顶面，两块半砖合成一整块，只扣一个。
func TestPlaceSlabMergesIntoDoubleSlab(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockSlab, 0)
	var held protocol.Item = placeTestHold(player, 44, 0, 10)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockDoubleSlab || meta != 0 {
		test.Fatalf("期望合成 双层半砖:0，实际 %d:%d", id, meta)
	}
	if placeTestCount(player) != 9 {
		test.Fatalf("背包期望剩 9，实际 %d", placeTestCount(player))
	}
}

// 两个玩家同时往同一格放方块，只能成功一个，两人合计只扣一个物品。
func TestPlaceSameCellOnlyOnce(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var first *Player = fixture.join("first", "10.0.0.1", "world")
	var second *Player = fixture.join("second", "10.0.0.2", "world")
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var firstHeld protocol.Item = placeTestHold(first, 1, 0, 64)
	var secondHeld protocol.Item = placeTestHold(second, 1, 0, 64)
	var group sync.WaitGroup = sync.WaitGroup{}
	group.Go(func() {
		placeTestUse(fixture, first, 3, 63, 0, blockrule.FaceUp, 1, firstHeld)
	})
	group.Go(func() {
		placeTestUse(fixture, second, 3, 63, 0, blockrule.FaceUp, 1, secondHeld)
	})
	group.Wait()
	var consumed int = int(64-placeTestCount(first)) + int(64-placeTestCount(second))
	if consumed != 1 {
		test.Fatalf("两人合计应只扣 1 个，实际扣了 %d", consumed)
	}
}
