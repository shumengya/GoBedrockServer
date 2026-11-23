package world

import (
	"bytes"
	"testing"

	"go-mc-server/internal/blockrule"
)

// 在海平面以上找平原湖，逐格检查水平水面、空气、实心湖床和四面不漏水的岸壁。
func TestNormalLakeBasin(test *testing.T) {
	var sample = 0
	for sample = 0; sample < 600; sample++ {
		var column = newChunk()
		column.fillNormal(12345, int32(sample%30-15), int32(sample/30-10))
		if column.biomes[7*16+7] != 1 {
			continue
		}
		var surface, area, maxDepth = 0, 0, 0
		var col, row = 0, 0
		for col = 0; col < 16; col++ {
			for row = 0; row < 16; row++ {
				var top = int(column.heightMap[row*16+col]) - 1
				var id, meta = column.GetBlock(col, top, row)
				if id != BlockStillWater || top <= normalSea {
					continue
				}
				if col == 0 || row == 0 || col == 15 || row == 15 {
					test.Fatalf("期望湖岸留在区块内，实际水面位于 (%d,%d)", col, row)
				}
				if area == 0 {
					surface = top
				}
				var above, _ = column.GetBlock(col, top+1, row)
				if top != surface || meta != 0 || above != BlockAir {
					test.Fatalf("期望高度 %d 的露天静水源，实际 %d / %d / %d", surface, top, meta, above)
				}
				area++
				var level = top
				for level = top; id == BlockStillWater; level-- {
					var dirs = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
					var side = 0
					for side = 0; side < len(dirs); side++ {
						var neighbor, _ = column.GetBlock(col+dirs[side][0], level, row+dirs[side][1])
						if neighbor != BlockStillWater && !blockrule.IsSolid(neighbor) {
							test.Fatalf("期望湖壁封闭，实际 (%d,%d,%d) 旁为 %d", col, level, row, neighbor)
						}
					}
					id, _ = column.GetBlock(col, level-1, row)
				}
				if !blockrule.IsSolid(id) {
					test.Fatalf("期望湖底为实心方块，实际 %d", id)
				}
				maxDepth = max(maxDepth, top-level)
			}
		}
		if area > 0 {
			if area < 20 || maxDepth < 3 {
				test.Fatalf("期望多格湖盆含深水，实际面积 %d 最大深度 %d", area, maxDepth)
			}
			return
		}
	}
	test.Fatal("期望平原生成独立的高地湖，实际未找到")
}

// 同一个可生成的湖在湖床开一格缺口后必须整座取消，不能在其他列留下半个湖。
func TestNormalLakeRejectsHole(test *testing.T) {
	var flat = newChunk()
	var col, row, level = 0, 0, 0
	for col = 0; col < 16; col++ {
		for row = 0; row < 16; row++ {
			for level = 58; level <= 68; level++ {
				flat.subAt(level>>4).set(col, level&15, row, BlockDirt, 0)
			}
			flat.heightMap[row*16+col] = 69
		}
	}
	var original = flat.EncodeDisk()
	var seed int32 = 0
	for seed = 0; seed < 128; seed++ {
		var trial, err = DecodeDisk(original)
		if err != nil {
			test.Fatal(err)
		}
		trial.fillLake(seed, 0, 0)
		for col = 1; col < 15; col++ {
			for row = 1; row < 15; row++ {
				// 选四格深的湖心，湖床不会同时充当相邻更深水格的侧壁。
				var id, _ = trial.GetBlock(col, 64, row)
				if id != BlockStillWater {
					continue
				}
				for level = 66; level >= 58; level-- {
					id, _ = trial.GetBlock(col, level, row)
					if id != BlockStillWater {
						break
					}
				}
				flat.subAt(level>>4).set(col, level&15, row, BlockAir, 0)
				var before = flat.EncodeDisk()
				flat.fillLake(seed, 0, 0)
				if !bytes.Equal(before, flat.EncodeDisk()) {
					test.Fatal("期望湖床破损时整块不变，实际仍写入湖水或挖掉地面")
				}
				return
			}
		}
	}
	test.Fatal("期望在封闭地面上找到可生成的对照湖，实际没有")
}
