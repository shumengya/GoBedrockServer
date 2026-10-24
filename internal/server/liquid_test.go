package server

import "testing"

import "go-mc-server/internal/blockrule"

// 平坦世界的地面最高一层是 y=4，水和岩浆都放在 y=5 上，区块 (0,0) 覆盖 x、z 在 0~15。

func liquidTestSettle(fixture *adminFixture, posX int32, posY int32, posZ int32) {
	fixture.srv.blockMu.Lock()
	defer fixture.srv.blockMu.Unlock()
	fixture.srv.settleBlocksLocked(
		"world",
		fixture.srv.world,
		[]blockrule.Pos{{PosX: posX, PosY: posY, PosZ: posZ}},
	)
}

func liquidTestIsWater(id byte) bool {
	return id == blockrule.BlockFlowingWater || id == blockrule.BlockStillWater
}

func liquidTestAbs(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

// liquidTestCount 数 y=5 这一层整个区块里有多少格水。
func liquidTestCount(fixture *adminFixture) int {
	var total int = 0
	var posX int32 = 0
	var posZ int32 = 0
	for posX = 0; posX < 16; posX++ {
		for posZ = 0; posZ < 16; posZ++ {
			var id, _ = placeTestBlock(fixture, posX, 5, posZ)
			if liquidTestIsWater(id) {
				total++
			}
		}
	}
	return total
}

// 平地上的一个水源，铺成一个菱形：离源头曼哈顿距离 d 的格子水位是 d，到 7 为止，再远的没有水。
func TestWaterSpreadsInDiamond(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 8, 5, 8)
	interactTestTicks(fixture, 100)
	var offsetX int32 = 0
	var offsetZ int32 = 0
	for offsetX = -7; offsetX <= 7; offsetX++ {
		for offsetZ = -7; offsetZ <= 7; offsetZ++ {
			var distance int32 = liquidTestAbs(offsetX) + liquidTestAbs(offsetZ)
			var id, meta = placeTestBlock(fixture, 8+offsetX, 5, 8+offsetZ)
			if distance <= 7 {
				if !liquidTestIsWater(id) || int32(meta) != distance {
					test.Fatalf("(%d,%d) 距源头 %d 格，期望水位 %d，实际 %d:%d", offsetX, offsetZ, distance, distance, id, meta)
				}
				continue
			}
			if id != blockrule.BlockAir {
				test.Fatalf("(%d,%d) 距源头 %d 格，不该有水，实际 %d", offsetX, offsetZ, distance, id)
			}
		}
	}
}

// 把源头拿掉，铺开的水要自己干掉，不能留下永远不消失的水。
func TestWaterDriesUpWhenSourceRemoved(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 8, 5, 8)
	interactTestTicks(fixture, 60)
	if liquidTestCount(fixture) < 20 {
		test.Fatalf("拿掉源头前应该已经铺开，实际只有 %d 格", liquidTestCount(fixture))
	}
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockAir, 0)
	liquidTestSettle(fixture, 8, 5, 8)
	var tick int = 0
	var remaining int = liquidTestCount(fixture)
	for tick = 0; tick < 3000 && remaining > 0; tick++ {
		interactTestTicks(fixture, 1)
		remaining = liquidTestCount(fixture)
	}
	if remaining != 0 {
		test.Fatalf("3000 个 tick 后还剩 %d 格水没干", remaining)
	}
}

// 四面围住的竖井顶上一个水源：水柱一路掉下去(水位 8)，落到地面再铺开，水位从 1 开始。
func TestWaterFallsThroughShaftAndSpreads(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var walls [][2]int32 = [][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	var height int32 = 0
	var wall int = 0
	for height = 6; height <= 10; height++ {
		for wall = 0; wall < len(walls); wall++ {
			placeTestPut(test, fixture, 8+walls[wall][0], height, 8+walls[wall][1], blockrule.BlockStone, 0)
		}
	}
	placeTestPut(test, fixture, 8, 10, 8, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 8, 10, 8)
	interactTestTicks(fixture, 150)
	for height = 5; height <= 9; height++ {
		var id, meta = placeTestBlock(fixture, 8, height, 8)
		if !liquidTestIsWater(id) || meta != 8 {
			test.Fatalf("y=%d 应是往下掉的水柱(水位 8)，实际 %d:%d", height, id, meta)
		}
	}
	var expectations [][3]int32 = [][3]int32{{9, 8, 1}, {12, 8, 4}, {8, 5, 3}, {11, 11, 6}}
	var index int = 0
	for index = 0; index < len(expectations); index++ {
		var spotX int32 = expectations[index][0]
		var spotZ int32 = expectations[index][1]
		var level int32 = expectations[index][2]
		var id, meta = placeTestBlock(fixture, spotX, 5, spotZ)
		if !liquidTestIsWater(id) || int32(meta) != level {
			test.Fatalf("(%d,5,%d) 期望水位 %d，实际 %d:%d", spotX, spotZ, level, id, meta)
		}
	}
	var farID, _ = placeTestBlock(fixture, 12, 5, 12)
	if farID != blockrule.BlockAir {
		test.Fatalf("(12,5,12) 离落点 8 格，不该有水，实际 %d", farID)
	}
}

// 岩浆源头紧挨着水：岩浆立刻变黑曜石，水继续流，不会流进黑曜石。
func TestLavaSourceBesideWaterBecomesObsidian(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 4, 5, 4, blockrule.BlockStillLava, 0)
	placeTestPut(test, fixture, 5, 5, 4, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 4, 5, 4)
	var hardened, _ = placeTestBlock(fixture, 4, 5, 4)
	if hardened != blockrule.BlockObsidian {
		test.Fatalf("岩浆源头碰到水应立刻变黑曜石，实际 %d", hardened)
	}
	interactTestTicks(fixture, 60)
	var stillObsidian, _ = placeTestBlock(fixture, 4, 5, 4)
	var waterID, _ = placeTestBlock(fixture, 5, 5, 4)
	var spreadID, spreadMeta = placeTestBlock(fixture, 6, 5, 4)
	if stillObsidian != blockrule.BlockObsidian {
		test.Fatalf("黑曜石不该被水冲走，实际 %d", stillObsidian)
	}
	if !liquidTestIsWater(waterID) || !liquidTestIsWater(spreadID) || spreadMeta != 1 {
		test.Fatalf("水应继续向另一侧流，实际 源头=%d 旁边=%d:%d", waterID, spreadID, spreadMeta)
	}
}

// 流动的岩浆(水位 1~4)碰到水变圆石。
func TestFlowingLavaBesideWaterBecomesCobblestone(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 4, 5, 4, blockrule.BlockFlowingLava, 3)
	placeTestPut(test, fixture, 5, 5, 4, blockrule.BlockFlowingWater, 2)
	liquidTestSettle(fixture, 4, 5, 4)
	var id, _ = placeTestBlock(fixture, 4, 5, 4)
	if id != blockrule.BlockCobblestone {
		test.Fatalf("期望圆石，实际 %d", id)
	}
}

// 水冲过火把：火把被冲掉，水占了那一格，掉出一个火把物品。
func TestWaterWashesAwayTorch(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 6, 5, 6, blockrule.BlockTorch, 5)
	placeTestPut(test, fixture, 5, 5, 6, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 5, 5, 6)
	interactTestTicks(fixture, 20)
	var id, meta = placeTestBlock(fixture, 6, 5, 6)
	if !liquidTestIsWater(id) || meta != 1 {
		test.Fatalf("火把那格应变成水位 1 的水，实际 %d:%d", id, meta)
	}
	if updateTestDropped(fixture, 50) != 1 {
		test.Fatalf("应掉出 1 个火把，实际 %d", updateTestDropped(fixture, 50))
	}
}

// 水只在已经加载的区块里流：源头紧挨区块边界，不会把隔壁没加载的区块生成出来。
func TestWaterStopsAtUnloadedChunk(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 14, 5, 8, blockrule.BlockStillWater, 0)
	liquidTestSettle(fixture, 14, 5, 8)
	interactTestTicks(fixture, 100)
	var edgeID, edgeMeta = placeTestBlock(fixture, 15, 5, 8)
	if !liquidTestIsWater(edgeID) || edgeMeta != 1 {
		test.Fatalf("区块里最靠边的一格应有水位 1 的水，实际 %d:%d", edgeID, edgeMeta)
	}
	var _, _, neighborLoaded = fixture.srv.world.Peek(16, 5, 8)
	var _, _, farLoaded = fixture.srv.world.Peek(20, 5, 8)
	if neighborLoaded || farLoaded {
		test.Fatalf("水不该把隔壁区块加载出来，实际 紧邻=%v 更远=%v", neighborLoaded, farLoaded)
	}
}
