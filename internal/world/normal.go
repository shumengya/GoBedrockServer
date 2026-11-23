package world

import (
	"math"
	"math/rand/v2"

	"go-mc-server/internal/blockrule"
)

const normalSea = 62

type terrainBiome struct {
	id        byte
	low, high float64
}

func selectClimate(temp, rain float64) terrainBiome {
	switch {
	case rain < .25:
		if temp < .7 {
			return terrainBiome{0, 46, 58}
		}
		if temp < .85 {
			return terrainBiome{7, 58, 62}
		}
		return terrainBiome{6, 62, 63}
	case rain < .60:
		if temp < .25 {
			return terrainBiome{12, 63, 74}
		}
		if temp < .75 {
			return terrainBiome{1, 63, 68}
		}
		return terrainBiome{2, 63, 74}
	case rain < .80:
		if temp < .25 {
			return terrainBiome{5, 63, 81}
		}
		if temp < .75 {
			return terrainBiome{4, 63, 81}
		}
		return terrainBiome{27, 63, 81}
	default:
		if temp < .20 {
			return terrainBiome{3, 63, 127}
		}
		if temp < .40 {
			return terrainBiome{20, 63, 97}
		}
		return terrainBiome{7, 58, 62}
	}
}

var terrainWeights = func() [5][5]float64 {
	var weights = [5][5]float64{}
	var total = 0.0
	var row, col = 0, 0
	for row = 0; row < 5; row++ {
		for col = 0; col < 5; col++ {
			weights[row][col] = math.Exp(-float64((row-2)*(row-2)+(col-2)*(col-2)) / 8)
			total += weights[row][col]
		}
	}
	for row = 0; row < 5; row++ {
		for col = 0; col < 5; col++ {
			weights[row][col] /= total
		}
	}
	return weights
}()

func terrainMix(left, right, weight float64) float64 { return left + (right-left)*weight }

func groundMaterial(biome byte, depth int, submerged bool) byte {
	switch biome {
	case 2:
		if depth < 4 {
			return BlockSand
		}
		if depth < 9 {
			return 24
		} // 砂岩
	case 0, 7:
		if depth < 5 {
			if biome == 0 {
				return 13
			}
			return BlockDirt
		}
	case 5:
		if depth == 0 {
			return 243
		} // 灰化土
		if depth < 4 {
			return BlockDirt
		}
	default:
		if depth == 0 && !submerged {
			return BlockGrass
		}
		if depth < 5 {
			return BlockDirt
		}
	}
	return BlockStone
}

func terrainNoise(seed int32) (simplex, simplex, simplex) {
	var random = rand.New(rand.NewPCG(uint64(uint32(seed)), 1))
	return newSimplex(random), newSimplex(random), newSimplex(random)
}

// 气候和密度按世界坐标取样；边界网格与相邻区块共享同一组采样点。
func (column *Chunk) fillNormal(seed, chunkX, chunkZ int32) {
	var base, temp, rain = terrainNoise(seed)
	var baseX, baseZ = float64(chunkX) * 16, float64(chunkZ) * 16
	var climate = [20][20]terrainBiome{}
	var grid = [5][5][17]float64{}
	var col, row, level = 0, 0, 0
	for col = 0; col < 20; col++ {
		for row = 0; row < 20; row++ {
			var posX, posZ = baseX + float64(col-2), baseZ + float64(row-2)
			climate[col][row] = selectClimate(temp.climate(posX, posZ), rain.climate(posX, posZ))
		}
	}
	for col = 0; col < 5; col++ {
		for row = 0; row < 5; row++ {
			for level = 0; level < 17; level++ {
				grid[col][row][level] = base.density(baseX+float64(col*4), float64(level*8), baseZ+float64(row*4))
			}
		}
	}
	for col = 0; col < 16; col++ {
		for row = 0; row < 16; row++ {
			var biome = climate[col+2][row+2].id
			column.biomes[row*16+col] = biome
			var low, high = 0.0, 0.0
			var offX, offZ = 0, 0
			for offX = 0; offX < 5; offX++ {
				for offZ = 0; offZ < 5; offZ++ {
					var adjacent = climate[col+offX][row+offZ]
					low += (adjacent.low - 1) * terrainWeights[offX][offZ]
					high += adjacent.high * terrainWeights[offX][offZ]
				}
			}
			var heights = [17]float64{}
			var cellX, cellZ = col / 4, row / 4
			var fracX, fracZ = float64(col%4) / 4, float64(row%4) / 4
			for level = 0; level < 17; level++ {
				heights[level] = terrainMix(terrainMix(grid[cellX][cellZ][level], grid[cellX+1][cellZ][level], fracX), terrainMix(grid[cellX][cellZ+1][level], grid[cellX+1][cellZ+1][level], fracX), fracZ)
			}
			var surface = -1
			for level = 127; level > 0; level-- {
				var adjustment = min(2*(high-float64(level))/(high-low)-1, .4+max(0, float64(level)-low+10)/10)
				var density = terrainMix(heights[level/8], heights[level/8+1], float64(level%8)/8)
				var id byte = BlockAir
				if density+adjustment > 0 {
					if surface < 0 {
						surface = level
						if biome == 12 && level >= normalSea {
							column.subAt((level+1)>>4).set(col, (level+1)&15, row, 78, 0)
						}
					}
					id = groundMaterial(biome, surface-level, surface < normalSea)
				} else if surface < 0 && level <= normalSea {
					id = BlockStillWater
				}
				if id != BlockAir {
					column.subAt(level>>4).set(col, level&15, row, id, 0)
				}
			}
			column.subAt(0).set(col, 0, row, BlockBedrock, 0)
			column.heightMap[row*16+col] = column.skyHeight(col, 128, row)
		}
	}
	column.fillLake(seed, chunkX, chunkZ)
}

// 仅新建主世界时选点；先查配置坐标，再向四周寻找露天陆地。
func NormalSpawn(seed, originX, originZ int32) [3]int32 {
	var fallback = [3]int32{originX, normalSea + 1, originZ}
	var directions = [8][2]int32{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {-1, -1}, {1, -1}}
	var distance int32 = 0
	for distance = 0; distance <= 512; distance = max(16, distance*2) {
		var count = len(directions)
		if distance == 0 {
			count = 1
		}
		var index = 0
		for index = 0; index < count; index++ {
			var posX, posZ = originX + directions[index][0]*distance, originZ + directions[index][1]*distance
			var column = newChunk()
			column.fillNormal(seed, posX>>4, posZ>>4)
			var ground = column.heightMap
			column.fillVegetation(seed, posX>>4, posZ>>4)
			var start = int(posZ&15)*16 + int(posX&15)
			if distance == 0 {
				fallback[1] = int32(column.heightMap[start])
				var above, _ = column.GetBlock(start&15, int(fallback[1]), start>>4)
				if above != BlockAir {
					fallback[1]++
				}
			}
			var offset = 0
			for offset = 0; offset < 256; offset++ {
				var cell = (start + offset) & 255
				var col, row = cell & 15, cell >> 4
				var top = int(ground[cell]) - 1
				var floor, _ = column.GetBlock(col, top, row)
				if top < normalSea || (floor != blockrule.BlockSnowLayer && !blockrule.IsSolid(floor)) || floor == BlockLog {
					continue
				}
				var feet, _ = column.GetBlock(col, top+1, row)
				if feet == blockrule.BlockSnowLayer {
					top++
					feet, _ = column.GetBlock(col, top+1, row)
				}
				var head, _ = column.GetBlock(col, top+2, row)
				if feet == BlockAir && head == BlockAir {
					return [3]int32{posX&^15 + int32(col), int32(top + 1), posZ&^15 + int32(row)}
				}
			}
		}
	}
	return fallback
}
