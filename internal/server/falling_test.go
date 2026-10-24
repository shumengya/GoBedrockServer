package server

import "maps"
import "slices"
import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

func fallTestCount(fixture *adminFixture) int {
	fixture.srv.mu.RLock()
	defer fixture.srv.mu.RUnlock()
	var ids []int64 = slices.Sorted(maps.Keys(fixture.srv.entities))
	var total int = 0
	var index int = 0
	for index = 0; index < len(ids); index++ {
		if fixture.srv.entities[ids[index]].IsFallingBlock() {
			total++
		}
	}
	return total
}

// fallTestSettle 一直推进下落，直到没有下落中的方块或者超过 limit 个 tick，返回用了多少 tick。
func fallTestSettle(fixture *adminFixture, limit int) int {
	var tick int = 0
	for tick = 0; tick < limit && fallTestCount(fixture) > 0; tick++ {
		fixture.srv.tickFallingBlocks()
	}
	return tick
}

// 挖掉沙子脚下的方块，沙子变成下落的实体，落在平坦世界的地面（最高实心层 y=4）上，停在 y=5。
func TestSandFallsWhenSupportBroken(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockSand, 0)
	updateTestBreak(fixture, player, 3, 63, 0)
	var origin, _ = placeTestBlock(fixture, 3, 64, 0)
	if origin != blockrule.BlockAir || fallTestCount(fixture) != 1 {
		test.Fatalf("原位置应变空并出现 1 个下落方块，实际 方块=%d 下落=%d", origin, fallTestCount(fixture))
	}
	var ticks int = fallTestSettle(fixture, 200)
	if fallTestCount(fixture) != 0 {
		test.Fatalf("200 个 tick 内应落地，实际用了 %d 个仍未落地", ticks)
	}
	var landedID, landedMeta = placeTestBlock(fixture, 3, 5, 0)
	if landedID != blockrule.BlockSand || landedMeta != 0 {
		test.Fatalf("期望落在 (3,5,0) 变回沙子，实际 %d:%d", landedID, landedMeta)
	}
	var column, _ = placeTestBlock(fixture, 3, 6, 0)
	if column != blockrule.BlockAir || updateTestDropped(fixture, 12) != 0 {
		test.Fatalf("不该多出方块或掉落物，实际 上方=%d 掉落=%d", column, updateTestDropped(fixture, 12))
	}
}

// 三块沙子叠着，挖掉最下面的支撑，三块依次掉下来，最后重新叠成一根柱子，一块不少。
func TestStackedSandFallsAndRestacks(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	updateTestCreative(player)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockSand, 0)
	placeTestPut(test, fixture, 3, 65, 0, blockrule.BlockSand, 0)
	placeTestPut(test, fixture, 3, 66, 0, blockrule.BlockGravel, 0)
	updateTestBreak(fixture, player, 3, 63, 0)
	if fallTestCount(fixture) != 3 {
		test.Fatalf("应有 3 个下落方块，实际 %d", fallTestCount(fixture))
	}
	fallTestSettle(fixture, 200)
	var expected []byte = []byte{blockrule.BlockSand, blockrule.BlockSand, blockrule.BlockGravel}
	var index int = 0
	for index = 0; index < len(expected); index++ {
		var id, _ = placeTestBlock(fixture, 3, int32(5+index), 0)
		if id != expected[index] {
			test.Fatalf("y=%d 期望 %d，实际 %d", 5+index, expected[index], id)
		}
	}
	for index = 0; index < 63; index++ {
		var leftover, _ = placeTestBlock(fixture, 3, int32(8+index), 0)
		if leftover != blockrule.BlockAir {
			test.Fatalf("y=%d 不该剩东西，实际 %d", 8+index, leftover)
		}
	}
}

// 落点被不能顶替的方块占着(火把)，沙子掉成物品，火把原样保留。
func TestFallingSandDropsItemOnOccupiedLanding(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 3, 5, 0, blockrule.BlockTorch, 5)
	fixture.srv.blockMu.Lock()
	fixture.srv.spawnFallingBlock("world", blockrule.Pos{PosX: 3, PosY: 10, PosZ: 0}, blockrule.BlockSand, 0)
	fixture.srv.blockMu.Unlock()
	fallTestSettle(fixture, 200)
	var landing, _ = placeTestBlock(fixture, 3, 5, 0)
	if landing != blockrule.BlockTorch {
		test.Fatalf("火把应保留，实际 %d", landing)
	}
	if updateTestDropped(fixture, 12) != 1 {
		test.Fatalf("沙子应掉成 1 个物品，实际 %d", updateTestDropped(fixture, 12))
	}
}

// 落到世界底部以下(虚空)直接消失，不放方块也不掉物品。
func TestFallingSandVanishesInVoid(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var voidWorld *world.World = world.New("sky")
	voidWorld.Generator = world.GeneratorSky
	fixture.srv.worlds["sky"] = voidWorld
	voidWorld.GetBlock(1000, 70, 1000)
	fixture.srv.blockMu.Lock()
	fixture.srv.spawnFallingBlock("sky", blockrule.Pos{PosX: 1000, PosY: 70, PosZ: 1000}, blockrule.BlockSand, 0)
	fixture.srv.blockMu.Unlock()
	var ticks int = fallTestSettle(fixture, 200)
	if fallTestCount(fixture) != 0 {
		test.Fatalf("200 个 tick 内应掉出世界，实际用了 %d 个仍在下落", ticks)
	}
	var bottom, _ = voidWorld.GetBlock(1000, 0, 1000)
	if bottom != blockrule.BlockAir || updateTestDropped(fixture, 12) != 0 {
		test.Fatalf("不该放方块或掉物品，实际 底部=%d 掉落=%d", bottom, updateTestDropped(fixture, 12))
	}
}

// 在半空靠着侧面放沙子：放下的瞬间脚下是空的，立刻变成下落的方块，物品照常扣一个。
func TestPlaceSandInMidAirFalls(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 12, 0, 64)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceEast, 0.5, held)
	var placed, _ = placeTestBlock(fixture, 4, 63, 0)
	if placed != blockrule.BlockAir || fallTestCount(fixture) != 1 {
		test.Fatalf("放下的沙子应立刻掉落，实际 原位=%d 下落=%d", placed, fallTestCount(fixture))
	}
	if placeTestCount(player) != 63 {
		test.Fatalf("背包期望剩 63，实际 %d", placeTestCount(player))
	}
}

// 出生包要带 variant：方块 ID 放低 8 位，meta 放上面；别的实体不带。
func TestEntityMetadataCarriesFallingVariant(test *testing.T) {
	var kind, found = entity.ByID(entity.NetworkIDFallingSand)
	if !found {
		test.Fatal("下落方块实体类型没有注册")
	}
	var falling *entity.Instance = &entity.Instance{Type: kind, BlockID: 12, BlockData: 3}
	var variant int32 = -1
	var metadata []packets.MetadataEntry = addEntityPacket(falling).Metadata
	var index int = 0
	for index = 0; index < len(metadata); index++ {
		if metadata[index].Key == packets.EntityDataVariant {
			if metadata[index].Type != packets.EntityDataTypeInt {
				test.Fatalf("variant 应是 int 类型，实际 %d", metadata[index].Type)
			}
			variant = metadata[index].Int
		}
	}
	if variant != 12|3<<8 {
		test.Fatalf("variant 期望 %d，实际 %d", 12|3<<8, variant)
	}
	var plain []packets.MetadataEntry = entityMetadata(&entity.Instance{Type: kind})
	for index = 0; index < len(plain); index++ {
		if plain[index].Key == packets.EntityDataVariant {
			test.Fatal("没带方块的实体不该有 variant")
		}
	}
}
