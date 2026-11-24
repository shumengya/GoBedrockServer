package world

import (
	"math"
	"math/rand/v2"
)

const (
	dimensionGlowstone byte = 89
	dimensionObsidian  byte = 49
)

// 维度出生点取实际生成的入口高度，与玩家第一次进入时收到的区块一致。
func DimensionSpawn(generator string, seed int32) [3]int32 {
	var column = newChunk()
	return [3]int32{8, int32(column.fillDimension(generator, seed, 0, 0)), 8}
}

func (column *Chunk) fillDimension(generator string, seed, chunkX, chunkZ int32) int {
	var nether = generator == GeneratorNetherTerrain
	var biome byte = 9
	if nether {
		biome = 8
	}
	var noise = newSimplex(rand.New(rand.NewPCG(uint64(uint32(seed)), uint64(biome))))
	var baseX, baseZ = float64(chunkX) * 16, float64(chunkZ) * 16
	var grid = sampleTerrainGrid(&noise, baseX, baseZ, .5)
	var col, row, level = 0, 0, 0
	for col = 0; col < 16; col++ {
		for row = 0; row < 16; row++ {
			var cell = row*16 + col
			column.biomes[cell] = biome
			var heights = grid.profile(col, row)
			var posX, posZ = baseX + float64(col), baseZ + float64(row)
			var horizontal = posX*posX + posZ*posZ
			for level = 0; level < 128; level++ {
				var sample = terrainMix(heights[level/8], heights[level/8+1], float64(level%8)/8)
				var id byte = BlockAir
				if nether {
					switch {
					case level == 0 || level == 127:
						id = BlockBedrock
					case math.Abs(64-float64(level))/64-sample > .5:
						id = BlockNetherrack
					case level <= 32:
						id = BlockStillLava
					}
				} else {
					var vertical = float64(level)/1.3 - 64
					var distance = horizontal + vertical*vertical
					var density = math.Abs(32-float64(level))/32 - sample - .4
					if (distance < 100*100 && density < 0) || (distance > 400*400 && density < -.2) {
						id = BlockEndStone
					}
				}
				if id != BlockAir {
					column.subAt(level>>4).set(col, level&15, row, id, 0)
					column.heightMap[cell] = uint16(level + 1)
				}
			}
		}
	}
	var random = rand.New(rand.NewPCG(uint64(uint32(seed))<<8|uint64(biome), chunkKey(chunkX, chunkZ)))
	if nether {
		column.hangGlowstone(random)
	} else if (chunkX != 0 || chunkZ != 0) && random.IntN(101) < 10 && random.IntN(2) == 1 {
		column.endPillar(random)
	}
	if chunkX != 0 || chunkZ != 0 {
		return 0
	}
	return column.dimensionLanding(nether)
}

// 从空腔顶面向下生长；每块新萤石贴着上一块，整簇不会悬空。
func (column *Chunk) hangGlowstone(random *rand.Rand) {
	var col, row = 4 + random.IntN(8), 4 + random.IntN(8)
	var level = 0
	for level = 125; level > 32; level-- {
		var ceiling, _ = column.GetBlock(col, level+1, row)
		var current, _ = column.GetBlock(col, level, row)
		if ceiling != BlockNetherrack || current != BlockAir {
			continue
		}
		var cells = [20][3]int{{col, level, row}}
		column.subAt(level>>4).set(col, level&15, row, dimensionGlowstone, 0)
		var count = 1
		var steps = [5][3]int{{0, -1, 0}, {1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}}
		var attempt = 0
		for attempt = 0; attempt < 60 && count < len(cells); attempt++ {
			var parent = cells[random.IntN(count)]
			var step = steps[random.IntN(len(steps))]
			var posX, posY, posZ = parent[0] + step[0], parent[1] + step[1], parent[2] + step[2]
			if posX < 0 || posX >= 16 || posZ < 0 || posZ >= 16 || posY <= 32 {
				continue
			}
			var old, _ = column.GetBlock(posX, posY, posZ)
			if old != BlockAir {
				continue
			}
			column.subAt(posY>>4).set(posX, posY&15, posZ, dimensionGlowstone, 0)
			cells[count] = [3]int{posX, posY, posZ}
			count++
		}
		return
	}
}

func (column *Chunk) endPillar(random *rand.Rand) {
	var centerX, centerZ = 4 + random.IntN(8), 4 + random.IntN(8)
	var base = int(column.heightMap[centerZ*16+centerX])
	if base == 0 {
		return
	}
	var top = base + 27 + random.IntN(23)
	var radius = 2 + random.IntN(3)
	var col, row, level = 0, 0, 0
	for col = centerX - radius; col <= centerX+radius; col++ {
		for row = centerZ - radius; row <= centerZ+radius; row++ {
			var east, south = col - centerX, row - centerZ
			if east*east+south*south > radius*radius {
				continue
			}
			var cell = row*16 + col
			var bottom = int(column.heightMap[cell])
			if bottom == 0 || bottom > top {
				continue
			}
			for level = bottom; level <= top; level++ {
				column.subAt(level>>4).set(col, level&15, row, dimensionObsidian, 0)
			}
			column.heightMap[cell] = uint16(top + 1)
		}
	}
}

// 地狱优先找岩浆海上方有两格净空的落脚点；末地选择岛面，中心无岛时保留抵达平台。
func (column *Chunk) dimensionLanding(nether bool) int {
	var floor = int(column.heightMap[8*16+8]) - 1
	var material = dimensionObsidian
	if nether {
		material = BlockNetherrack
		floor = 63
		var best = 128
		var level = 0
		for level = 33; level <= 123; level++ {
			var support, _ = column.GetBlock(8, level, 8)
			var feet, _ = column.GetBlock(8, level+1, 8)
			var head, _ = column.GetBlock(8, level+2, 8)
			var distance = int(math.Abs(float64(level - 63)))
			if support == BlockNetherrack && feet == BlockAir && head == BlockAir && distance < best {
				floor, best = level, distance
			}
		}
	} else if floor < 0 {
		floor = 48
	}
	var col, row, offset = 0, 0, 0
	for col = 7; col <= 9; col++ {
		for row = 7; row <= 9; row++ {
			column.subAt(floor>>4).set(col, floor&15, row, material, 0)
			for offset = 1; offset <= 3; offset++ {
				var level = floor + offset
				if column.sub[level>>4] != nil {
					column.sub[level>>4].set(col, level&15, row, BlockAir, 0)
				}
			}
			column.heightMap[row*16+col] = column.skyHeight(col, max(floor+3, int(column.heightMap[row*16+col])), row)
		}
	}
	return floor + 1
}
