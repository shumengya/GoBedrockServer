package world

import "testing"

// 对比装饰前后的真实区块，矿脉只能替换普通石头，六种矿均可生成且贵金属只在低层。
func TestNormalOres(test *testing.T) {
	var found = map[byte]int{}
	var cornerOres = 0
	var bound = map[byte]int{16: 134, 15: 70, 73: 22, 21: 38, 14: 38, 56: 22}
	var chunkX, col, row, level = 0, 0, 0, 0
	for chunkX = -3; chunkX <= 3; chunkX++ {
		var base, decorated = newChunk(), newChunk()
		base.fillNormal(-794, int32(chunkX), -1)
		decorated.generate(GeneratorNormal, -794, int32(chunkX), -1)
		for col = 0; col < 16; col++ {
			for row = 0; row < 16; row++ {
				for level = 0; level <= 135; level++ {
					var old, oldMeta = base.GetBlock(col, level, row)
					var id, meta = decorated.GetBlock(col, level, row)
					if id == old && meta == oldMeta {
						continue
					}
					if old != BlockStone || oldMeta != 0 {
						test.Fatalf("期望只替换天然石头，实际被替换 %d:%d", old, oldMeta)
					}
					var ceiling, ore = bound[id]
					if ore {
						found[id]++
						if col == 0 && row == 0 {
							cornerOres++
						}
						if level > ceiling || level == 0 {
							test.Fatalf("期望矿石 %d 在 1..%d，实际 %d", id, ceiling, level)
						}
					}
				}
			}
		}
	}
	if len(found) != len(bound) {
		test.Fatalf("期望六种矿石，实际 %v", found)
	}
	if cornerOres == 0 {
		test.Fatal("期望西北邻区的矿脉能进入本列西北角，实际没有矿石")
	}
}
