package world

import (
	"bytes"
	"testing"
)

// 检查地狱基岩顶底、熔岩海、可通行洞层和洞顶萤石，固定种子的岩层还须可重建。
func TestNetherTerrain(test *testing.T) {
	var lava, glow, open = false, false, false
	var chunkX, col, row, level = 0, 0, 0, 0
	for chunkX = -6; chunkX <= 6; chunkX++ {
		var column = newChunk()
		column.generate(GeneratorNetherTerrain, -794, int32(chunkX), 2)
		for col = 0; col < 16; col++ {
			for row = 0; row < 16; row++ {
				var floor, _ = column.GetBlock(col, 0, row)
				var roof, _ = column.GetBlock(col, 127, row)
				if floor != BlockBedrock || roof != BlockBedrock || column.biomes[row*16+col] != 8 {
					test.Fatalf("期望地狱顶底基岩和群系 8，实际 %d / %d / %d", floor, roof, column.biomes[row*16+col])
				}
				for level = 1; level < 127; level++ {
					var id, _ = column.GetBlock(col, level, row)
					if id == BlockStillLava {
						if level > 32 {
							test.Fatalf("期望岩浆海不高于 32，实际 %d", level)
						}
						if level == 32 {
							lava = true
						}
					}
					if id == BlockAir && level == 64 {
						open = true
					}
					if id == dimensionGlowstone {
						var above, _ = column.GetBlock(col, level+1, row)
						if above == BlockNetherrack {
							glow = true
						}
					}
				}
			}
		}
		if chunkX == 0 {
			var repeat, different = newChunk(), newChunk()
			repeat.generate(GeneratorNetherTerrain, -794, 0, 2)
			different.generate(GeneratorNetherTerrain, 0, 0, 2)
			if !bytes.Equal(column.EncodeDisk(), repeat.EncodeDisk()) || bytes.Equal(column.EncodeDisk(), different.EncodeDisk()) {
				test.Fatal("期望同种子重建且异种子地形不同，实际不符")
			}
		}
	}
	if !lava || !glow || !open {
		test.Fatalf("期望岩浆海、洞顶萤石和洞层，实际 %t / %t / %t", lava, glow, open)
	}
}

// 末地必须有中央岛、环形虚空和外岛，中央与外岛都不能铺满整个维度。
func TestEndTerrain(test *testing.T) {
	var inner, outer, pillar = false, false, false
	var chunkX int32 = 0
	for chunkX = -4; chunkX < 150; chunkX++ {
		var column = newChunk()
		column.generate(GeneratorEndTerrain, -794, chunkX, 0)
		if column.biomes[0] != 9 {
			test.Fatalf("期望末地群系 9，实际 %d", column.biomes[0])
		}
		var index = 0
		for index = 0; index < 256; index++ {
			var height = column.heightMap[index]
			if chunkX >= 8 && chunkX <= 22 && height != 0 {
				test.Fatalf("期望虚空带无岛，实际区块 %d 高度 %d", chunkX, height)
			}
			if height == 0 {
				continue
			}
			var id, _ = column.GetBlock(index&15, int(height)-1, index>>4)
			if id == BlockEndStone {
				if chunkX >= 26 {
					outer = true
				} else if chunkX >= -4 && chunkX <= 4 {
					inner = true
				}
			}
			if chunkX != 0 && id == dimensionObsidian {
				var below, _ = column.GetBlock(index&15, int(height)-20, index>>4)
				if below == dimensionObsidian {
					pillar = true
				}
			}
		}
	}
	if !inner || !outer || !pillar {
		test.Fatalf("期望中央岛、外岛和黑曜石柱，实际 %t / %t / %t", inner, outer, pillar)
	}
}
