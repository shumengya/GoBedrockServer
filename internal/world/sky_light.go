package world

func (column *Chunk) rawSky(x, y, z int) byte {
	var base = column.directSky(x, y, z)
	var side = column.sideSky[y>>4]
	if base == 15 || side == nil {
		return base
	}
	var index, shift = dataIndex(x, y&15, z)
	return max(base, side[index]>>shift&15)
}

// 新列内只需从有直射光、且相邻格更暗的位置启动传播。
// 扫描每根竖柱的受光部分：高度图以下超过十四格便没有直射光；以上始终为 15。
func (wave *lightWave) skyBoundary(cx, cz int32, column *Chunk, joining bool) {
	var x, z, side int = 0, 0, 0
	for x = 0; x < 16; x++ {
		for z = 0; z < 16; z++ {
			var wx, wz = cx<<4 + int32(x), cz<<4 + int32(z)
			for side = 0; side < 4; side++ {
				var dx, dz = lightSteps[side][0], lightSteps[side][2]
				var nx, nz = wx + dx, wz + dz
				var other = column
				var border = nx>>4 != cx || nz>>4 != cz
				if border {
					other = wave.column(nx, nz)
					if other == nil {
						continue
					}
				} else if !joining || side%2 == 1 {
					continue
				}
				var ax, az = int(nx & 15), int(nz & 15)
				var first = int(column.heightMap[z<<4|x])
				var second = int(other.heightMap[az<<4|ax])
				var low = max(0, min(first, second)-14)
				// 已载邻区带来的侧光可能从任意深度跨界；撤区时只重算保留一侧的侧光。
				if border {
					low = 0
				}
				var y int = low
				for y = low; y < max(first, second); y++ {
					if !joining {
						if other.rawSky(ax, y, az) > other.directSky(ax, y, az) {
							wave.enqueue(nx, int32(y), nz)
						}
						continue
					}
					var left = column.directSky(x, y, z)
					var right = other.rawSky(ax, y, az)
					if left > right+1 {
						wave.enqueue(nx, int32(y), nz)
					}
					if right > left+1 {
						wave.enqueue(wx, int32(y), wz)
					}
				}
			}
		}
	}
	wave.run(true)
}
