package world

import (
	"math"
	"math/rand/v2"
)

// 独立的平原小湖：整个盆地和一格湖岸落在源区块内，不依赖邻区是否加载。
func (column *Chunk) fillLake(seed, chunkX, chunkZ int32) {
	if column.biomes[7*16+7] != 1 {
		return
	}
	var random = rand.New(rand.NewPCG(uint64(uint32(seed))<<3|4, chunkKey(chunkX, chunkZ)))
	if random.IntN(8) != 0 {
		return
	}
	var centerX, centerZ = 6.5 + float64(random.IntN(4)), 6.5 + float64(random.IntN(4))
	var angle = random.Float64() * 2 * math.Pi
	var sine, cosine = math.Sincos(angle)
	var major, minor = 3.5 + random.Float64()*1.5, 2.5 + random.Float64()
	var depth = [16][16]int{}
	var waterY, highest = 128, 0
	var col, row = 0, 0
	for col = 1; col < 15; col++ {
		for row = 1; row < 15; row++ {
			var east, south = float64(col) + .5 - centerX, float64(row) + .5 - centerZ
			var axisX, axisZ = (east*cosine + south*sine) / major, (south*cosine - east*sine) / minor
			var distance = axisX*axisX + axisZ*axisZ
			if distance >= 1 {
				continue
			}
			depth[col][row] = 1 + int(3*(1-distance))
			var top = int(column.heightMap[row*16+col]) - 1
			waterY = min(waterY, top-1)
			highest = max(highest, top)
		}
	}
	if waterY <= normalSea || highest-waterY > 4 {
		return
	}
	// 填水前先检查整圈湖岸和湖底。坡面缺一格或底下是洞，则整湖不落地。
	var directions = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	var level = 0
	for col = 1; col < 15; col++ {
		for row = 1; row < 15; row++ {
			var layers = depth[col][row]
			if layers == 0 {
				continue
			}
			var top = int(column.heightMap[row*16+col]) - 1
			for level = waterY - layers; level <= top; level++ {
				var id, _ = column.GetBlock(col, level, row)
				if id != BlockStone && id != BlockDirt && id != BlockGrass {
					return
				}
			}
			var side = 0
			for side = 0; side < 4; side++ {
				var nearX, nearZ = col + directions[side][0], row + directions[side][1]
				for level = waterY - layers + 1; level <= waterY-depth[nearX][nearZ]; level++ {
					var id, _ = column.GetBlock(nearX, level, nearZ)
					if id != BlockStone && id != BlockDirt && id != BlockGrass {
						return
					}
				}
			}
		}
	}
	for col = 1; col < 15; col++ {
		for row = 1; row < 15; row++ {
			var layers = depth[col][row]
			if layers == 0 {
				continue
			}
			var top = int(column.heightMap[row*16+col]) - 1
			for level = waterY - layers + 1; level <= top; level++ {
				var id byte = BlockStillWater
				if level > waterY {
					id = BlockAir
				}
				column.subAt(level>>4).set(col, level&15, row, id, 0)
			}
			column.heightMap[row*16+col] = uint16(waterY + 1)
		}
	}
}
