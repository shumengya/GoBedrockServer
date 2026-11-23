package world

import (
	"testing"

	"go-mc-server/internal/blockrule"
)

// 在真实生成的区块中验证树种与树根，以及平原和沼泽各自的花种；从树根选出生点必须避开树干。
func TestNormalBiomePlants(test *testing.T) {
	const seed = -794
	var _, temp, rain = terrainNoise(seed)
	var found = map[byte]bool{}
	var spawned = false
	var sample = 0
	for sample = 0; sample < 1000 && len(found) < 5; sample++ {
		var chunkX, chunkZ = int32(37*sample - 500), int32(19*sample - 500)
		var posX, posZ = float64(chunkX)*16 + 7, float64(chunkZ)*16 + 7
		var biome = selectClimate(temp.climate(posX, posZ), rain.climate(posX, posZ)).id
		if found[biome] || (biome != 4 && biome != 5 && biome != 27 && biome != 1 && biome != 6) {
			continue
		}
		var column = newChunk()
		column.generate(GeneratorNormal, seed, chunkX, chunkZ)
		var logs, grass, flowers = 0, 0, 0
		var col, row, level = 0, 0, 0
		for col = 0; col < 16; col++ {
			for row = 0; row < 16; row++ {
				for level = normalSea - 2; level <= 140; level++ {
					var id, meta = column.GetBlock(col, level, row)
					switch id {
					case BlockLog:
						var soil, _ = column.GetBlock(col, level-1, row)
						if soil != BlockDirt {
							continue
						}
						var species = map[byte]byte{4: 0, 5: 1, 27: 2}
						var want, woodland = species[biome]
						if !woodland || meta != want {
							test.Fatalf("期望群系 %d 的树干种类 %d，实际 %d", biome, want, meta)
						}
						logs++
						if !spawned {
							var spawn = NormalSpawn(seed, chunkX*16+int32(col), chunkZ*16+int32(row))
							var target = newChunk()
							target.generate(GeneratorNormal, seed, spawn[0]>>4, spawn[2]>>4)
							var feet, _ = target.GetBlock(int(spawn[0]&15), int(spawn[1]), int(spawn[2]&15))
							var head, _ = target.GetBlock(int(spawn[0]&15), int(spawn[1]+1), int(spawn[2]&15))
							if feet != BlockAir || head != BlockAir {
								test.Fatalf("期望树根附近出生点有两格净空，实际 %d / %d", feet, head)
							}
							spawned = true
						}
					case blockrule.BlockDandelion, blockrule.BlockPoppy, blockrule.BlockTallGrass:
						var soil, _ = column.GetBlock(col, level-1, row)
						if soil != BlockGrass {
							test.Fatalf("期望花草扎根草方块，实际 %d", soil)
						}
						if id == blockrule.BlockTallGrass {
							if meta != 1 {
								test.Fatalf("期望高草 meta 1，实际 %d", meta)
							}
							grass++
						} else {
							if (biome == 6 && (id != 38 || meta != 1)) || (biome == 1 && (meta == 1 || meta == 2 || meta > 8 || (id == 37 && meta != 0))) || (biome != 1 && biome != 6) {
								test.Fatalf("期望参考端群系 %d 花种，实际 %d:%d", biome, id, meta)
							}
							flowers++
						}
					}
				}
			}
		}
		if logs > 0 || (biome == 1 && grass > 0 && flowers > 0) || (biome == 6 && flowers > 0) {
			found[biome] = true
		}
	}
	if len(found) != 5 || !spawned {
		test.Fatalf("期望验证三种森林、平原和沼泽，实际 %v", found)
	}
}

// 无树群系不能自己种树；若边界两侧存在相连的树叶，树冠确实从相邻林地伸入了本区块。
func TestNormalCanopyCrossesBiomeEdge(test *testing.T) {
	const seed = 12345
	var _, temp, rain = terrainNoise(seed)
	var east, south int32 = 0, 0
	for east = -32; east <= 32; east++ {
		for south = -32; south <= 32; south++ {
			var posX, posZ = float64(east)*16 + 7, float64(south)*16 + 7
			var biome = selectClimate(temp.climate(posX, posZ), rain.climate(posX, posZ)).id
			var other = selectClimate(temp.climate(posX+16, posZ), rain.climate(posX+16, posZ)).id
			if biome != 1 || (other != 4 && other != 27 && other != 5) {
				continue
			}
			var plain, forest = newChunk(), newChunk()
			plain.generate(GeneratorNormal, seed, east, south)
			forest.generate(GeneratorNormal, seed, east+1, south)
			var row, level = 0, 0
			for row = 0; row < 16; row++ {
				for level = normalSea; level < 140; level++ {
					var left, leftMeta = plain.GetBlock(15, level, row)
					var right, rightMeta = forest.GetBlock(0, level, row)
					if left == blockrule.BlockLeaves && right == left && leftMeta == rightMeta {
						return
					}
				}
			}
		}
	}
	test.Fatal("期望从森林到平原有连续树冠，实际区块边界没有相连的树叶")
}
