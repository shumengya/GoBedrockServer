package world

import (
	"math"
	"math/rand/v2"
)

type oreDeposit struct {
	id, meta          byte
	count, size, maxY int
}

// 参数依次为每区块尝试数、矿脉规模和起始高度上限；末端可略高于起点。
var normalDeposits = [...]oreDeposit{
	{16, 0, 20, 16, 128}, {15, 0, 20, 8, 64}, {73, 0, 8, 7, 16},
	{21, 0, 1, 6, 32}, {14, 0, 2, 8, 32}, {56, 0, 1, 7, 16},
	{BlockDirt, 0, 20, 32, 128}, {BlockStone, 1, 20, 32, 128},
	{BlockStone, 3, 20, 32, 128}, {BlockStone, 5, 20, 32, 128}, {13, 0, 10, 16, 128},
}

// 矿脉中心有 +8 的偏移，最大规模 32，其包围盒只会进入源区块及其东、南、东南邻区。
// 从目标反查四个来源，各自用固定随机流重建路径，不读写邻区。
func (column *Chunk) fillNormalOres(seed, chunkX, chunkZ int32) {
	var deltaX, deltaZ, kind, attempt = 0, 0, 0, 0
	for deltaX = -1; deltaX <= 0; deltaX++ {
		for deltaZ = -1; deltaZ <= 0; deltaZ++ {
			var sourceKey = chunkKey(chunkX+int32(deltaX), chunkZ+int32(deltaZ))
			var random = rand.New(rand.NewPCG(uint64(uint32(seed)), sourceKey))
			for kind = 0; kind < len(normalDeposits); kind++ {
				var deposit = normalDeposits[kind]
				for attempt = 0; attempt < deposit.count; attempt++ {
					var posX = float64(16*deltaX + 8 + random.IntN(16))
					var posY = float64(random.IntN(deposit.maxY + 1))
					var posZ = float64(16*deltaZ + 8 + random.IntN(16))
					column.paintVein(random, deposit, posX, posY, posZ)
				}
			}
		}
	}
}

func (column *Chunk) paintVein(random *rand.Rand, deposit oreDeposit, posX, posY, posZ float64) {
	var angle = random.Float64() * math.Pi
	var sine, cosine = math.Sincos(angle)
	var span = float64(deposit.size) / 8
	var startY, endY = posY + 2 + float64(random.IntN(3)), posY + 2 + float64(random.IntN(3))
	var step = 0
	for step = 0; step <= deposit.size; step++ {
		var progress = float64(step) / float64(deposit.size)
		var centerX = posX + (1-2*progress)*cosine*span
		var centerZ = posZ + (1-2*progress)*sine*span
		var centerY = terrainMix(startY, endY, progress)
		var radius = ((math.Sin(math.Pi*progress)+1)*random.Float64()*float64(deposit.size)/16 + 1) / 2
		if centerX+radius < .5 || centerZ+radius < .5 || centerX-radius > 15.5 || centerZ-radius > 15.5 {
			continue
		}
		var square = radius * radius
		var col, row, level = 0, 0, 0
		var lastX, lastZ = int(min(15, math.Floor(centerX+radius-.5))), int(min(15, math.Floor(centerZ+radius-.5)))
		var lastY = int(math.Floor(centerY + radius - .5))
		for col = max(0, int(math.Ceil(centerX-radius-.5))); col <= lastX; col++ {
			var offX = float64(col) + .5 - centerX
			for row = max(0, int(math.Ceil(centerZ-radius-.5))); row <= lastZ; row++ {
				var offZ = float64(row) + .5 - centerZ
				var rest = square - offX*offX - offZ*offZ
				if rest <= 0 {
					continue
				}
				for level = max(1, int(math.Ceil(centerY-radius-.5))); level <= lastY; level++ {
					var offY = float64(level) + .5 - centerY
					var section = column.sub[level>>4]
					if offY*offY >= rest || section == nil {
						continue
					}
					var id, meta = section.get(col, level&15, row)
					if id == BlockStone && meta == 0 {
						section.set(col, level&15, row, deposit.id, deposit.meta)
					}
				}
			}
		}
	}
}
