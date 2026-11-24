package world

import (
	"math"
	"math/rand/v2"
)

type caveCarver struct {
	column               *Chunk
	seed, chunkX, chunkZ int32
	shore                [3][3]*Chunk
	// 基础地形只有从海底到 Y=62 的连续水柱。0 表示尚未取样，255 表示干地。
	waterFloor [18][18]byte
}

type cavePoint struct{ east, level, south float64 }

// 一个源区块的路径只取决于它的坐标和世界种子。只把路径在目标区块内的那部分写下来。
func (column *Chunk) fillCaves(seed, chunkX, chunkZ int32) {
	var carver = caveCarver{column: column, seed: seed, chunkX: chunkX, chunkZ: chunkZ}
	carver.shore[1][1] = column
	var east, south int32 = 0, 0
	for east = -8; east <= 8; east++ {
		for south = -8; south <= 8; south++ {
			var random = rand.New(rand.NewPCG(uint64(uint32(seed))<<4|8, chunkKey(chunkX+east, chunkZ+south)))
			if random.IntN(15) != 0 {
				continue
			}
			var count = random.IntN(random.IntN(random.IntN(40)+1) + 1)
			var index = 0
			for index = 0; index < count; index++ {
				var point = cavePoint{float64(east*16) + float64(random.IntN(16)), float64(random.IntN(random.IntN(120) + 8)), float64(south*16) + float64(random.IntN(16))}
				var branches = 1
				if random.IntN(4) == 0 {
					var radius = 2.5 + 6*random.Float64()
					carver.chamber(point, radius, radius*.5)
					branches += random.IntN(4)
				}
				var branch = 0
				for branch = 0; branch < branches; branch++ {
					var yaw, pitch = 2 * math.Pi * random.Float64(), (random.Float64() - .5) / 4
					var width = random.Float64()*2 + random.Float64()
					if random.IntN(10) == 0 {
						width *= 1 + 3*random.Float64()*random.Float64()
					}
					var path = rand.New(rand.NewPCG(random.Uint64(), 0))
					carver.tunnel(path, point, yaw, pitch, width, 0, 112-path.IntN(28))
				}
			}
		}
	}
}

func (carver *caveCarver) tunnel(random *rand.Rand, point cavePoint, yaw, pitch, width float64, start, length int) {
	var fork = random.IntN(length/2) + length/4
	var damping = .7
	if random.IntN(6) == 0 {
		damping = .92
	}
	var yawDrift, pitchDrift = 0.0, 0.0
	var step = 0
	for step = start; step < length; step++ {
		// 每一步最多前进一格；剩余步数连同最大半径都够不到时，不必重演后半条隧道。
		var reach = float64(length-step) + max(width, 1) + 1.5
		if point.east < -reach || point.east > 16+reach || point.south < -reach || point.south > 16+reach {
			return
		}
		var sinYaw, cosYaw = math.Sincos(yaw)
		var sinPitch, cosPitch = math.Sincos(pitch)
		point.east += cosYaw * cosPitch
		point.south += sinYaw * cosPitch
		point.level += sinPitch
		pitch = pitch*damping + pitchDrift*.1
		yaw += yawDrift * .1
		pitchDrift = pitchDrift*.9 + (random.Float64()-random.Float64())*random.Float64()*2
		yawDrift = yawDrift*.75 + (random.Float64()-random.Float64())*random.Float64()*4
		if step == fork && width > 1 {
			var side = -1
			for side = -1; side <= 1; side += 2 {
				var scale = .5 + .5*random.Float64()
				carver.tunnel(rand.New(rand.NewPCG(random.Uint64(), 0)), point, yaw+float64(side)*math.Pi/2, pitch/3, scale, step, length)
			}
			return
		}
		if random.IntN(4) != 0 {
			var radius = 1.5 + math.Sin(math.Pi*float64(step)/float64(length))*width
			carver.chamber(point, radius, radius)
		}
	}
}

// 只查询目标区块外一格的海岸。不会打开邻区存档，也不向世界缓存加入区块。
func (carver *caveCarver) waterBottom(col, row int) int {
	var cached = &carver.waterFloor[col+1][row+1]
	if *cached != 0 {
		return int(*cached)
	}
	var east, south = col >> 4, row >> 4
	var neighbor = carver.shore[east+1][south+1]
	if neighbor == nil {
		neighbor = newChunk()
		neighbor.fillBaseTerrain(carver.seed, carver.chunkX+int32(east), carver.chunkZ+int32(south))
		carver.shore[east+1][south+1] = neighbor
	}
	*cached = 255
	var level = normalSea
	for level = normalSea; level > 0; level-- {
		var sub = neighbor.sub[level>>4]
		if sub == nil || sub.ids[subChunkIDIndex(col&15, level&15, row&15)] != BlockStillWater {
			break
		}
		*cached = byte(level)
	}
	return int(*cached)
}

func (carver *caveCarver) chamber(point cavePoint, radius, height float64) {
	if point.east+radius < .5 || point.east-radius > 15.5 || point.south+radius < .5 || point.south-radius > 15.5 {
		return
	}
	var minX, maxX = max(0, int(math.Ceil(point.east-radius-.5))), min(15, int(math.Floor(point.east+radius-.5)))
	var minZ, maxZ = max(0, int(math.Ceil(point.south-radius-.5))), min(15, int(math.Floor(point.south+radius-.5)))
	var low, high = max(1, int(math.Ceil(point.level-.7*height-.5))), min(119, int(math.Floor(point.level+height-.5)))
	if low > high {
		return
	}
	var col, row, level = 0, 0, 0
	if low-1 <= normalSea {
		for col = minX - 1; col <= maxX+1; col++ {
			for row = minZ - 1; row <= maxZ+1; row++ {
				if carver.waterBottom(col, row) <= high+1 {
					return
				}
			}
		}
	}
	for col = minX; col <= maxX; col++ {
		for row = minZ; row <= maxZ; row++ {
			var east, south = (float64(col) + .5 - point.east) / radius, (float64(row) + .5 - point.south) / radius
			var horizontal = east*east + south*south
			if horizontal >= 1 {
				continue
			}
			for level = high; level >= low; level-- {
				var vertical = (float64(level) + .5 - point.level) / height
				if vertical <= -.7 || horizontal+vertical*vertical >= 1 {
					continue
				}
				var section = carver.column.sub[level>>4]
				if section == nil {
					continue
				}
				var id = section.ids[subChunkIDIndex(col, level&15, row)]
				if id != BlockStone && id != BlockDirt && id != BlockGrass {
					continue
				}
				var material byte = BlockAir
				if level < 10 {
					material = BlockStillLava
				} else if id == BlockGrass {
					var below = carver.column.sub[(level-1)>>4]
					if below != nil && below.ids[subChunkIDIndex(col, (level-1)&15, row)] == BlockDirt {
						below.set(col, (level-1)&15, row, BlockGrass, 0)
					}
				}
				carver.column.setBlockLocked(col, level, row, material, 0)
			}
		}
	}
}
