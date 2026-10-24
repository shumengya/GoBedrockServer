package server

import "maps"
import "slices"
import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// updateTestCreative 把玩家切成创造模式：挖方块立刻成功，不受挖掘时间校验限制，
// 也不产生被挖方块自己的掉落，测试里只看级联出来的掉落。
func updateTestCreative(player *Player) {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.GameMode = int32(protocol.GameModeCreative)
}

func updateTestBreak(fixture *adminFixture, player *Player, posX int32, posY int32, posZ int32) {
	fixture.srv.handleRemoveBlock(player.Session, &packets.RemoveBlockPacket{X: posX, Y: posY, Z: posZ})
}

// updateTestDropped 统计地上某种掉落物的总数量。
func updateTestDropped(fixture *adminFixture, itemID int32) int {
	fixture.srv.mu.RLock()
	defer fixture.srv.mu.RUnlock()
	var total int = 0
	var ids []int64 = slices.Sorted(maps.Keys(fixture.srv.entities))
	var index int = 0
	for index = 0; index < len(ids); index++ {
		var instance *entity.Instance = fixture.srv.entities[ids[index]]
		if instance.IsDroppedItem() && instance.Item.ID == itemID {
			total += int(instance.Item.Count)
		}
	}
	return total
}

// 挖掉火把脚下的方块，火把跟着掉下来变成一个掉落物。
func TestBreakSupportDropsTorch(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockTorch, 5)
	updateTestBreak(fixture, player, 3, 63, 0)
	var stoneID, _ = placeTestBlock(fixture, 3, 63, 0)
	var torchID, _ = placeTestBlock(fixture, 3, 64, 0)
	if stoneID != blockrule.BlockAir || torchID != blockrule.BlockAir {
		test.Fatalf("石头和火把都应消失，实际 石头=%d 火把=%d", stoneID, torchID)
	}
	if updateTestDropped(fixture, 50) != 1 {
		test.Fatalf("应掉落 1 个火把，实际 %d", updateTestDropped(fixture, 50))
	}
}

// 一整根三格高的甘蔗，挖掉脚下的沙子，三格依次掉下来，每格一个掉落物。
func TestBreakSupportCascadesThroughSugarcane(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockSand, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockSugarcane, 0)
	placeTestPut(test, fixture, 3, 65, 0, blockrule.BlockSugarcane, 0)
	placeTestPut(test, fixture, 3, 66, 0, blockrule.BlockSugarcane, 0)
	updateTestBreak(fixture, player, 3, 63, 0)
	var height int32 = 0
	for height = 63; height <= 66; height++ {
		var id, _ = placeTestBlock(fixture, 3, height, 0)
		if id != blockrule.BlockAir {
			test.Fatalf("y=%d 应已消失，实际 %d", height, id)
		}
	}
	if updateTestDropped(fixture, 338) != 3 {
		test.Fatalf("应掉落 3 根甘蔗，实际 %d", updateTestDropped(fixture, 338))
	}
}

// 旁边的方块被挖掉不会影响不依附于它的方块。
func TestBreakUnrelatedNeighborKeepsBlocks(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockGrass, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockPoppy, 0)
	placeTestPut(test, fixture, 4, 63, 0, blockrule.BlockStone, 0)
	updateTestBreak(fixture, player, 4, 63, 0)
	var flowerID, _ = placeTestBlock(fixture, 3, 64, 0)
	if flowerID != blockrule.BlockPoppy {
		test.Fatalf("花脚下没动，不该掉，实际 %d", flowerID)
	}
	if updateTestDropped(fixture, 38) != 0 {
		test.Fatalf("不该有花的掉落物，实际 %d", updateTestDropped(fixture, 38))
	}
}

// 级联到没加载的区块时不能把它加载（生成）出来：区块边上的方块被挖，邻居区块仍保持未加载。
func TestBreakAtChunkEdgeDoesNotLoadNeighbors(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 15, 63, 0, blockrule.BlockStone, 0)
	updateTestBreak(fixture, player, 15, 63, 0)
	var _, _, eastLoaded = fixture.srv.world.Peek(16, 63, 0)
	if eastLoaded {
		test.Fatal("东边的区块不该因为级联被加载")
	}
	var _, _, northLoaded = fixture.srv.world.Peek(15, 63, -1)
	if northLoaded {
		test.Fatal("北边的区块不该因为级联被加载")
	}
}

// 花必须种在草、泥土或耕地上：点石头顶面放不下，点草顶面放得下。
func TestPlaceFlowerNeedsSoil(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 5, 63, 0, blockrule.BlockGrass, 0)
	var held protocol.Item = placeTestHold(player, 38, 0, 10)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var stoneTop, _ = placeTestBlock(fixture, 3, 64, 0)
	if stoneTop != blockrule.BlockAir || placeTestCount(player) != 10 {
		test.Fatalf("石头上不能种花，实际 方块=%d 剩余=%d", stoneTop, placeTestCount(player))
	}
	placeTestUse(fixture, player, 5, 63, 0, blockrule.FaceUp, 1, held)
	var grassTop, _ = placeTestBlock(fixture, 5, 64, 0)
	if grassTop != blockrule.BlockPoppy || placeTestCount(player) != 9 {
		test.Fatalf("草上应种下红花，实际 方块=%d 剩余=%d", grassTop, placeTestCount(player))
	}
}

// 小麦种子种在耕地上：物品 ID 295 要查表成小麦方块 59。
func TestPlaceSeedsOnFarmland(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockFarmland, 0)
	var held protocol.Item = placeTestHold(player, 295, 0, 8)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockWheat || meta != 0 {
		test.Fatalf("期望 小麦:0，实际 %d:%d", id, meta)
	}
	if placeTestCount(player) != 7 {
		test.Fatalf("背包期望剩 7，实际 %d", placeTestCount(player))
	}
}
