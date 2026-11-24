package world

import "testing"

// 椭球横跨 Y=9/10 时，已挖部分分别是静止岩浆和空气；床岩与不可挖材料保持原样。
func TestCaveChamberMaterials(test *testing.T) {
	var column = newChunk()
	var level = 0
	for level = 0; level <= 14; level++ {
		column.subAt(level>>4).set(8, level&15, 8, BlockStone, 0)
	}
	column.subAt(0).set(8, 0, 8, BlockBedrock, 0)
	column.subAt(0).set(8, 8, 8, BlockSand, 0)
	column.subAt(0).set(8, 13, 8, BlockDirt, 0)
	column.subAt(0).set(8, 14, 8, BlockGrass, 0)
	column.heightMap[8*16+8] = 15
	var carver = caveCarver{column: column}
	carver.shore[1][1] = column
	carver.chamber(cavePoint{8.5, 10.5, 8.5}, 2, 16)
	var checks = [...][2]int{{0, BlockBedrock}, {1, BlockStillLava}, {8, BlockSand}, {9, BlockStillLava}, {10, BlockAir}, {14, BlockAir}}
	var index = 0
	for index = 0; index < len(checks); index++ {
		var check = checks[index]
		var id, meta = column.GetBlock(8, check[0], 8)
		if int(id) != check[1] || meta != 0 {
			test.Fatalf("期望 Y=%d 为 %d:0，实际 %d:%d", check[0], check[1], id, meta)
		}
	}
	if column.heightMap[8*16+8] != 9 {
		test.Fatalf("期望开洞后最高遮光方块是 Y=8 的沙，实际高度图 %d", column.heightMap[8*16+8])
	}
}

// 岸边同一段洞在邻区是水时保留石壁，干地时可挖通，不要求邻区已进入世界缓存。
func TestCaveShoreAcrossEdge(test *testing.T) {
	var scenario = 0
	for scenario = 0; scenario < 2; scenario++ {
		var column, neighbor = newChunk(), newChunk()
		column.subAt(3).set(15, 12, 8, BlockStone, 0)
		if scenario == 1 {
			var level = 0
			for level = 60; level <= normalSea; level++ {
				neighbor.subAt(level>>4).set(0, level&15, 8, BlockStillWater, 0)
			}
		}
		var carver = caveCarver{column: column}
		carver.shore[1][1], carver.shore[2][1] = column, neighbor
		carver.chamber(cavePoint{15.5, 60.5, 8.5}, 2, 2)
		var id, _ = column.GetBlock(15, 60, 8)
		var expected byte = BlockAir
		if scenario == 1 {
			expected = BlockStone
		}
		if id != expected {
			test.Fatalf("期望岸壁场景 %d 得到 %d，实际 %d", scenario, expected, id)
		}
	}
}

// 通过完整地形入口生成相邻区块，确认原先实心的边界两侧出现同一通道，且低层有新岩浆。
func TestNormalCaveSeam(test *testing.T) {
	const seed = -794
	var joined, molten = false, false
	var chunkX int32 = 0
	for chunkX = -12; chunkX < 0 && (!joined || !molten); chunkX++ {
		var left, right, rawLeft, rawRight = newChunk(), newChunk(), newChunk(), newChunk()
		right.fillNormal(seed, chunkX+1, -2)
		left.fillNormal(seed, chunkX, -2)
		rawLeft.fillBaseTerrain(seed, chunkX, -2)
		rawRight.fillBaseTerrain(seed, chunkX+1, -2)
		var row, level = 0, 0
		for row = 0; row < 16; row++ {
			for level = 1; level < 45; level++ {
				var beforeLeft, _ = rawLeft.GetBlock(15, level, row)
				var beforeRight, _ = rawRight.GetBlock(0, level, row)
				var afterLeft, _ = left.GetBlock(15, level, row)
				var afterRight, _ = right.GetBlock(0, level, row)
				if beforeLeft == BlockStone && beforeRight == BlockStone && afterLeft == BlockAir && afterRight == BlockAir && level >= 10 {
					joined = true
				}
				if beforeLeft == BlockStone && afterLeft == BlockStillLava && level < 10 {
					molten = true
				}
			}
		}
	}
	if !joined || !molten {
		test.Fatalf("期望跨区块的新通道和深层岩浆，实际 %t / %t", joined, molten)
	}
}
