package world

// 波前只访问已加载的列。世界写锁稳定区块集合，按需锁住触及的列，区块包不会读到传播一半的光数组。
type lightWave struct {
	world   *World
	locked  map[*Chunk]bool
	columns []*Chunk
	queue   [][3]int32
	pending map[[3]int32]bool
}

var lightSteps = [6][3]int32{{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0}}

func (wave *lightWave) column(x, z int32) *Chunk {
	var column = wave.world.chunks[chunkKey(x>>4, z>>4)]
	if column != nil && !wave.locked[column] {
		column.mu.Lock()
		if wave.locked == nil {
			wave.locked = make(map[*Chunk]bool)
		}
		wave.locked[column] = true
		wave.columns = append(wave.columns, column)
	}
	return column
}

func (wave *lightWave) unlock() {
	var index int = 0
	for index = 0; index < len(wave.columns); index++ {
		wave.columns[index].mu.Unlock()
	}
}

func (wave *lightWave) enqueue(x, y, z int32) {
	if y < 0 || y > 255 {
		return
	}
	var pos = [3]int32{x, y, z}
	if wave.pending[pos] {
		return
	}
	if wave.pending == nil {
		wave.pending = make(map[[3]int32]bool)
	}
	wave.pending[pos] = true
	wave.queue = append(wave.queue, pos)
}

func (column *Chunk) emittedAt(x, y, z int32) byte {
	var section = column.sub[y>>4]
	if section == nil {
		return 0
	}
	var index, shift = dataIndex(int(x&15), int(y&15), int(z&15))
	return section.blockLight[index] >> shift & 15
}

func (wave *lightWave) run() {
	var head int = 0
	for head = 0; head < len(wave.queue); head++ {
		var pos = wave.queue[head]
		delete(wave.pending, pos)
		var x, y, z = pos[0], pos[1], pos[2]
		var column = wave.column(x, z)
		if column == nil {
			continue
		}
		var section = column.sub[y>>4]
		var id byte = 0
		if section != nil {
			id = section.ids[subChunkIDIndex(int(x&15), int(y&15), int(z&15))]
		}
		var light = optics[id].emit
		var cost = optics[id].filter
		var idx int = 0
		if light+cost < 16 {
			for idx = 0; idx < len(lightSteps); idx++ {
				var off = lightSteps[idx]
				var sideY = y + off[1]
				if sideY < 0 || sideY > 255 {
					continue
				}
				var neighbor = wave.column(x+off[0], z+off[2])
				if neighbor == nil {
					continue
				}
				var value = neighbor.emittedAt(x+off[0], sideY, z+off[2])
				if value > cost {
					light = max(light, value-cost)
				}
			}
		}
		var old = column.emittedAt(x, y, z)
		if old == light {
			continue
		}
		section = column.subAt(int(y >> 4))
		var index, shift = dataIndex(int(x&15), int(y&15), int(z&15))
		section.blockLight[index] = (section.blockLight[index] &^ (15 << shift)) | light<<shift
		if old == 0 {
			section.litCells++
		} else if light == 0 {
			section.litCells--
		}
		for idx = 0; idx < len(lightSteps); idx++ {
			var off = lightSteps[idx]
			wave.enqueue(x+off[0], y+off[1], z+off[2])
		}
	}
}

// 列载入时用盘里的发光方块作种子。边界另一侧已加载时，沿边界接入邻列的已有亮度；
// 卸载则反过来，让保留的邻列重新计算失去的那一面。
func (w *World) lightBoundary(cx, cz int32, column *Chunk, joining bool) {
	var wave = lightWave{world: w}
	defer wave.unlock()
	var sectionIndex, idx int = 0, 0
	if joining {
		wave.column(cx<<4, cz<<4)
		for sectionIndex = 0; sectionIndex < 16; sectionIndex++ {
			var section = column.sub[sectionIndex]
			if section == nil {
				continue
			}
			for idx = 0; idx < len(section.ids); idx++ {
				if optics[section.ids[idx]].emit != 0 {
					wave.enqueue(cx<<4+int32(idx>>8), int32(sectionIndex<<4|idx&15), cz<<4+int32(idx>>4&15))
				}
			}
		}
	}
	var side int = 0
	for side = 0; side < 4; side++ {
		var dx, dz = lightSteps[side][0], lightSteps[side][2]
		var neighbor = wave.column((cx+dx)<<4, (cz+dz)<<4)
		if neighbor == nil {
			continue
		}
		var source = column
		if joining {
			source = neighbor
		}
		for sectionIndex = 0; sectionIndex < 16; sectionIndex++ {
			var section = source.sub[sectionIndex]
			if section == nil || section.litCells == 0 {
				continue
			}
			var n int = 0
			for n = 0; n < 256; n++ {
				var x, z = cx << 4, cz << 4
				if dx == 0 {
					x += int32(n >> 4)
					if dz > 0 {
						z += 15
					}
				} else {
					z += int32(n >> 4)
					if dx > 0 {
						x += 15
					}
				}
				var y = int32(sectionIndex<<4 | n&15)
				var sx, sz, tx, tz = x, z, x + dx, z + dz
				if joining {
					sx, sz, tx, tz = tx, tz, sx, sz
				}
				if source.emittedAt(sx, y, sz) > 1 {
					wave.enqueue(tx, y, tz)
				}
			}
		}
	}
	wave.run()
}
