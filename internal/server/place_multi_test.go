package server

import "maps"
import "slices"
import "testing"
import "time"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// multiTestMine 让玩家处在"已经挖够了时间"的状态，生存模式下挖有硬度的方块才不会被判成秒破。
func multiTestMine(player *Player, posX int32, posY int32, posZ int32, id byte) {
	player.startMining(miningState{
		Active:  true,
		BlockX:  posX,
		BlockY:  posY,
		BlockZ:  posZ,
		BlockID: id,
		Started: time.Now().Add(-time.Hour),
		Ticks:   1,
	})
}

// multiTestDropped 统计地上某种物品（ID 和 meta 都要对上）的总数量。
func multiTestDropped(fixture *adminFixture, itemID int32, meta int32) int {
	fixture.srv.mu.RLock()
	defer fixture.srv.mu.RUnlock()
	var total int = 0
	var ids []int64 = slices.Sorted(maps.Keys(fixture.srv.entities))
	var index int = 0
	for index = 0; index < len(ids); index++ {
		var instance *entity.Instance = fixture.srv.entities[ids[index]]
		if instance.IsDroppedItem() && instance.Item.ID == itemID && instance.Item.Meta == meta {
			total += int(instance.Item.Count)
		}
	}
	return total
}

func multiTestPlaceDoor(test *testing.T) (*adminFixture, *Player) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockWoodDoor, 0)
	placeTestPut(test, fixture, 3, 65, 0, blockrule.BlockWoodDoor, 8)
	return fixture, player
}

// 点石头顶面放门，上下两格一起写进世界，只扣一个。
func TestPlaceDoorFillsBothCells(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.UpdatePosition(0, 64+playerEyeHeight, 0, 270, 0, true)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 324, 0, 3)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var lowerID, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	var upperID, upperMeta = placeTestBlock(fixture, 3, 65, 0)
	if lowerID != blockrule.BlockWoodDoor || lowerMeta != 0 {
		test.Fatalf("下半期望 木门:0，实际 %d:%d", lowerID, lowerMeta)
	}
	if upperID != blockrule.BlockWoodDoor || upperMeta != 8 {
		test.Fatalf("上半期望 木门:8，实际 %d:%d", upperID, upperMeta)
	}
	if placeTestCount(player) != 2 {
		test.Fatalf("背包期望剩 2，实际 %d", placeTestCount(player))
	}
}

// 床两格都要建方块实体，颜色取自手持物的 meta，不是写死的红色。
func TestPlaceBedStoresColorOnBothTiles(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.UpdatePosition(0, 64+playerEyeHeight, 0, 270, 0, true)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 4, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 355, 5, 2)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var footID, footMeta = placeTestBlock(fixture, 3, 64, 0)
	var headID, headMeta = placeTestBlock(fixture, 4, 64, 0)
	if footID != blockrule.BlockBed || footMeta != 3 || headID != blockrule.BlockBed || headMeta != 11 {
		test.Fatalf("期望 床脚 26:3 床头 26:11，实际 %d:%d %d:%d", footID, footMeta, headID, headMeta)
	}
	var footTile, footOK = fixture.srv.world.TileAt(3, 64, 0)
	var headTile, headOK = fixture.srv.world.TileAt(4, 64, 0)
	if !footOK || !headOK || footTile.ID != world.TileBed || headTile.ID != world.TileBed {
		test.Fatalf("两格都应有床的方块实体，实际 脚=%v 头=%v", footOK, headOK)
	}
	if footTile.Color != 5 || headTile.Color != 5 {
		test.Fatalf("颜色期望 5，实际 脚=%d 头=%d", footTile.Color, headTile.Color)
	}
	if placeTestCount(player) != 1 {
		test.Fatalf("背包期望剩 1，实际 %d", placeTestCount(player))
	}
}

// 不管先挖门的哪一半，两半都消失，一共只掉一扇门。
func TestBreakEitherDoorHalfDropsOneDoor(test *testing.T) {
	var halves []int32 = []int32{65, 64}
	var index int = 0
	for index = 0; index < len(halves); index++ {
		var posY int32 = halves[index]
		var fixture, player = multiTestPlaceDoor(test)
		multiTestMine(player, 3, posY, 0, blockrule.BlockWoodDoor)
		updateTestBreak(fixture, player, 3, posY, 0)
		var lowerID, _ = placeTestBlock(fixture, 3, 64, 0)
		var upperID, _ = placeTestBlock(fixture, 3, 65, 0)
		if lowerID != blockrule.BlockAir || upperID != blockrule.BlockAir {
			test.Fatalf("挖 y=%d 后两半都应消失，实际 下=%d 上=%d", posY, lowerID, upperID)
		}
		if multiTestDropped(fixture, 324, 0) != 1 {
			test.Fatalf("挖 y=%d 应掉 1 扇门，实际 %d", posY, multiTestDropped(fixture, 324, 0))
		}
	}
}

// 创造模式挖门，另一半一起消失，但不掉东西。
func TestBreakDoorInCreativeDropsNothing(test *testing.T) {
	var fixture, player = multiTestPlaceDoor(test)
	updateTestCreative(player)
	updateTestBreak(fixture, player, 3, 65, 0)
	var lowerID, _ = placeTestBlock(fixture, 3, 64, 0)
	if lowerID != blockrule.BlockAir {
		test.Fatalf("另一半应一起消失，实际 %d", lowerID)
	}
	if multiTestDropped(fixture, 324, 0) != 0 {
		test.Fatalf("创造模式不该掉门，实际 %d", multiTestDropped(fixture, 324, 0))
	}
}

// 挖掉门脚下的方块，门的下半撑不住掉下来，上半又失去配对跟着消失，一共只掉一扇门。
func TestBreakDoorSupportDropsOneDoor(test *testing.T) {
	var fixture, player = multiTestPlaceDoor(test)
	updateTestCreative(player)
	updateTestBreak(fixture, player, 3, 63, 0)
	var lowerID, _ = placeTestBlock(fixture, 3, 64, 0)
	var upperID, _ = placeTestBlock(fixture, 3, 65, 0)
	if lowerID != blockrule.BlockAir || upperID != blockrule.BlockAir {
		test.Fatalf("门应整个消失，实际 下=%d 上=%d", lowerID, upperID)
	}
	if multiTestDropped(fixture, 324, 0) != 1 {
		test.Fatalf("应掉 1 扇门，实际 %d", multiTestDropped(fixture, 324, 0))
	}
}

// 挖床的任意一半，掉的床颜色取自方块实体，而不是写死的红色。
func TestBreakBedDropsTileColor(test *testing.T) {
	var halves []int32 = []int32{3, 4}
	var index int = 0
	for index = 0; index < len(halves); index++ {
		var posX int32 = halves[index]
		var fixture, player = placeTestSetup(test)
		placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
		placeTestPut(test, fixture, 4, 63, 0, blockrule.BlockStone, 0)
		placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockBed, 3)
		placeTestPut(test, fixture, 4, 64, 0, blockrule.BlockBed, 11)
		fixture.srv.world.SetTile(world.NewBedTile(3, 64, 0, 5))
		fixture.srv.world.SetTile(world.NewBedTile(4, 64, 0, 5))
		multiTestMine(player, posX, 64, 0, blockrule.BlockBed)
		updateTestBreak(fixture, player, posX, 64, 0)
		var footID, _ = placeTestBlock(fixture, 3, 64, 0)
		var headID, _ = placeTestBlock(fixture, 4, 64, 0)
		if footID != blockrule.BlockAir || headID != blockrule.BlockAir {
			test.Fatalf("挖 x=%d 后两半都应消失，实际 脚=%d 头=%d", posX, footID, headID)
		}
		if multiTestDropped(fixture, 355, 5) != 1 || multiTestDropped(fixture, 355, 14) != 0 {
			test.Fatalf(
				"挖 x=%d 应掉 1 张颜色 5 的床，实际 颜色5=%d 红色=%d",
				posX,
				multiTestDropped(fixture, 355, 5),
				multiTestDropped(fixture, 355, 14),
			)
		}
		var _, footTileOK = fixture.srv.world.TileAt(3, 64, 0)
		var _, headTileOK = fixture.srv.world.TileAt(4, 64, 0)
		if footTileOK || headTileOK {
			test.Fatalf("两格的方块实体都应被清掉，实际 脚=%v 头=%v", footTileOK, headTileOK)
		}
	}
}

// 放一株向日葵：上下两格一起写进世界。挖掉上半，下半一起消失，一共只掉一株。
func TestPlaceAndBreakDoublePlant(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockGrass, 0)
	var held protocol.Item = placeTestHold(player, 175, 0, 3)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var lowerID, lowerMeta = placeTestBlock(fixture, 3, 64, 0)
	var upperID, upperMeta = placeTestBlock(fixture, 3, 65, 0)
	if lowerID != blockrule.BlockDoublePlant || lowerMeta != 0 || upperID != blockrule.BlockDoublePlant || upperMeta != 8 {
		test.Fatalf("期望 175:0 和 175:8，实际 %d:%d 和 %d:%d", lowerID, lowerMeta, upperID, upperMeta)
	}
	updateTestBreak(fixture, player, 3, 65, 0)
	var afterLower, _ = placeTestBlock(fixture, 3, 64, 0)
	var afterUpper, _ = placeTestBlock(fixture, 3, 65, 0)
	if afterLower != blockrule.BlockAir || afterUpper != blockrule.BlockAir {
		test.Fatalf("两半都应消失，实际 下=%d 上=%d", afterLower, afterUpper)
	}
	if multiTestDropped(fixture, 175, 0) != 1 {
		test.Fatalf("应掉 1 株向日葵，实际 %d", multiTestDropped(fixture, 175, 0))
	}
}
