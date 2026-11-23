package world

import (
	"bytes"
	"testing"
)

// 负坐标区块必须可独立重建；改变种子必须改变地形，所有列以基岩封底。
func TestNormalRebuild(test *testing.T) {
	var first, repeated, different = newChunk(), newChunk(), newChunk()
	first.generate(GeneratorNormal, -794, -11, 7)
	different.generate(GeneratorNormal, 0, -11, 7)
	repeated.generate(GeneratorNormal, -794, -11, 7)
	if !bytes.Equal(first.EncodeDisk(), repeated.EncodeDisk()) {
		test.Fatal("期望同种子重建一致，实际列数据不同")
	}
	if bytes.Equal(first.EncodeDisk(), different.EncodeDisk()) {
		test.Fatal("期望不同种子产生不同地形，实际列数据相同")
	}
	var col, row = 0, 0
	for col = 0; col < 16; col++ {
		for row = 0; row < 16; row++ {
			var id, _ = first.GetBlock(col, 0, row)
			if id != BlockBedrock {
				test.Fatalf("期望列 (%d,%d) 底部为基岩，实际 %d", col, row, id)
			}
		}
	}
}

// 跨多个气候区抽样：高度应有起伏，海底在水下，水面统一在 62，陆地有表土。
func TestNormalSurface(test *testing.T) {
	var biomes = map[byte]bool{}
	var heights = map[uint16]bool{}
	var water, land = 0, 0
	var sample = 0
	for sample = 0; sample < 48; sample++ {
		var column = newChunk()
		column.fillNormal(12345, int32(19*sample-500), int32(37*sample-500))
		var id, _ = column.GetBlock(8, normalSea, 8)
		var height = column.heightMap[8*16+8]
		biomes[column.biomes[8*16+8]] = true
		heights[height] = true
		if id == BlockStillWater {
			water++
			var above, _ = column.GetBlock(8, normalSea+1, 8)
			if above != BlockAir || height != normalSea+1 {
				test.Fatalf("期望海面上方为空气且高度图为 63，实际 %d / %d", above, height)
			}
		} else {
			var top, _ = column.GetBlock(8, int(height)-1, 8)
			if top == BlockGrass || top == BlockSand {
				land++
			}
		}
	}
	if water == 0 || land == 0 || len(heights) < 3 || len(biomes) < 3 {
		test.Fatalf("期望海洋、陆地和多种高度与群系，实际水列 %d 陆列 %d 高度 %d 群系 %d", water, land, len(heights), len(biomes))
	}
}
