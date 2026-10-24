package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// itemTestSlot 读第一格背包里的物品，工具的磨损记在 meta 里。
func itemTestSlot(player *Player) protocol.Item {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.Inventory[0]
}

// 锄头点草方块：变成耕地，工具磨损 1 点，物品不消耗。
func TestHoeTillsGrassAndWearsTool(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockGrass, 0)
	var held protocol.Item = placeTestHold(player, 292, 0, 1)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, _ = placeTestBlock(fixture, 3, 63, 0)
	if id != blockrule.BlockFarmland {
		test.Fatalf("期望耕地，实际 %d", id)
	}
	var slot protocol.Item = itemTestSlot(player)
	if slot.ID != 292 || slot.Meta != 1 || slot.Count != 1 {
		test.Fatalf("锄头应磨损 1 点，期望 292:1 x1，实际 %d:%d x%d", slot.ID, slot.Meta, slot.Count)
	}
}

// 创造模式用锄头照样耕地，但不磨损。
func TestHoeInCreativeDoesNotWear(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockDirt, 0)
	var held protocol.Item = placeTestHold(player, 292, 0, 1)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, _ = placeTestBlock(fixture, 3, 63, 0)
	if id != blockrule.BlockFarmland || itemTestSlot(player).Meta != 0 {
		test.Fatalf("期望耕地且不磨损，实际 方块=%d 磨损=%d", id, itemTestSlot(player).Meta)
	}
}

// 出生点保护范围内，生存玩家不能用锄头耕地，工具也不磨损。
func TestHoeBlockedBySpawnProtection(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	fixture.srv.cfg.SpawnProtection = 16
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockGrass, 0)
	var held protocol.Item = placeTestHold(player, 292, 0, 1)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, _ = placeTestBlock(fixture, 3, 63, 0)
	if id != blockrule.BlockGrass || itemTestSlot(player).Meta != 0 {
		test.Fatalf("保护范围内不该耕地，实际 方块=%d 磨损=%d", id, itemTestSlot(player).Meta)
	}
}

// 骨粉点小麦：加 2~5 个阶段，消耗一个骨粉。
func TestBoneMealGrowsWheat(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockWheat, 0)
	var held protocol.Item = placeTestHold(player, 351, 15, 4)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 0.5, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockWheat || meta < 2 || meta > 5 {
		test.Fatalf("小麦应长到 2~5 阶段，实际 %d:%d", id, meta)
	}
	if placeTestCount(player) != 3 {
		test.Fatalf("骨粉期望剩 3，实际 %d", placeTestCount(player))
	}
}

// 已经成熟的小麦不吃骨粉。
func TestBoneMealKeptOnRipeWheat(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockWheat, 7)
	var held protocol.Item = placeTestHold(player, 351, 15, 4)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 0.5, held)
	if placeTestCount(player) != 4 {
		test.Fatalf("成熟的小麦不该吃骨粉，期望剩 4，实际 %d", placeTestCount(player))
	}
}

// 潜行时不触发，骨粉原样留着。
func TestSneakingBoneMealDoesNothing(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	player.setSneaking(true)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockWheat, 0)
	var held protocol.Item = placeTestHold(player, 351, 15, 4)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 0.5, held)
	var _, meta = placeTestBlock(fixture, 3, 64, 0)
	if meta != 0 || placeTestCount(player) != 4 {
		test.Fatalf("潜行时骨粉不该生效，实际 meta=%d 剩余=%d", meta, placeTestCount(player))
	}
}

// 骨粉撒在草方块上：周围 5×5 全是草，第一次尝试就一定能长出东西，只可能是蒲公英、红花、高草。
func TestBoneMealScattersOnGrass(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var posX int32 = 0
	var posZ int32 = 0
	for posX = 1; posX <= 5; posX++ {
		for posZ = 1; posZ <= 5; posZ++ {
			placeTestPut(test, fixture, posX, 63, posZ, blockrule.BlockGrass, 0)
		}
	}
	var held protocol.Item = placeTestHold(player, 351, 15, 4)
	placeTestUse(fixture, player, 3, 63, 3, blockrule.FaceUp, 1, held)
	var planted int = 0
	for posX = 1; posX <= 5; posX++ {
		for posZ = 1; posZ <= 5; posZ++ {
			var above, _ = placeTestBlock(fixture, posX, 64, posZ)
			if above == blockrule.BlockAir {
				continue
			}
			if above != blockrule.BlockDandelion && above != blockrule.BlockPoppy && above != blockrule.BlockTallGrass {
				test.Fatalf("(%d,64,%d) 只可能长蒲公英、红花、高草，实际 %d", posX, posZ, above)
			}
			planted++
		}
	}
	if planted < 1 {
		test.Fatal("周围全是草，至少应该长出一株")
	}
	if placeTestCount(player) != 3 {
		test.Fatalf("骨粉期望剩 3，实际 %d", placeTestCount(player))
	}
}

// 落地时脚下是耕地：落差小不踩；落差大变成泥土，上面的小麦失去支撑掉下来。
func TestFallTramplesFarmland(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 3, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 3, 64, 3, blockrule.BlockWheat, 0)
	player.UpdatePosition(3.5, 64+playerEyeHeight, 3.5, 0, 0, true)
	fixture.srv.handleEntityFall(player.Session, &packets.EntityFallPacket{EntityRuntimeID: player.EntityRuntimeID, FallDistance: 0.4})
	var smallID, _ = placeTestBlock(fixture, 3, 63, 3)
	if smallID != blockrule.BlockFarmland {
		test.Fatalf("落差 0.4 不该踩坏耕地，实际 %d", smallID)
	}
	fixture.srv.handleEntityFall(player.Session, &packets.EntityFallPacket{EntityRuntimeID: player.EntityRuntimeID, FallDistance: 5})
	var farmID, _ = placeTestBlock(fixture, 3, 63, 3)
	var wheatID, _ = placeTestBlock(fixture, 3, 64, 3)
	if farmID != blockrule.BlockDirt || wheatID != blockrule.BlockAir {
		test.Fatalf("落差 5 应踩成泥土并带走小麦，实际 耕地=%d 小麦=%d", farmID, wheatID)
	}
	if updateTestDropped(fixture, 295) != 1 {
		test.Fatalf("小麦应掉 1 颗种子，实际 %d", updateTestDropped(fixture, 295))
	}
}

// 创造模式落地也会踩坏耕地。
func TestFallTramplesFarmlandInCreative(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 3, blockrule.BlockFarmland, 0)
	player.UpdatePosition(3.5, 64+playerEyeHeight, 3.5, 0, 0, true)
	fixture.srv.handleEntityFall(player.Session, &packets.EntityFallPacket{EntityRuntimeID: player.EntityRuntimeID, FallDistance: 5})
	var id, _ = placeTestBlock(fixture, 3, 63, 3)
	if id != blockrule.BlockDirt {
		test.Fatalf("创造模式也该踩坏，实际 %d", id)
	}
}
