package world

import "go-mc-server/internal/blockrule"
import "math"

// 光线每经过一格至少损失一级；直射天空光在遇到第一层遮挡之前例外。
type opticalProperties struct {
	filter, emit byte
	diffuse      bool
}

var optics = func() [256]opticalProperties {
	var table [256]opticalProperties
	var id int = 0
	for id = 0; id < len(table); id++ {
		var props = blockrule.PropsOf(byte(id))
		table[id] = opticalProperties{filter: min(15, props.Opacity+1), emit: props.Emission, diffuse: props.DiffusesSky}
	}
	return table
}()

func skyBarrier(id byte) bool { return optics[id].filter > 1 || optics[id].diffuse }

func (column *Chunk) skyHeight(x, top, z int) uint16 {
	var y int = top
	for y = top; y >= 0; y-- {
		var section = column.sub[y>>4]
		if section == nil {
			y &= ^15
			continue
		}
		if skyBarrier(section.ids[subChunkIDIndex(x, y&15, z)]) {
			return uint16(y + 1)
		}
	}
	return 0
}

func (column *Chunk) directSky(x, y, z int) byte {
	var height = int(column.heightMap[z<<4|x])
	if y >= height {
		return 15
	}
	if height-y >= 15 {
		return 0
	}
	var light byte = 15
	var step int = height - 1
	for step = height - 1; step >= y; step-- {
		var cost byte = 1
		var section = column.sub[step>>4]
		if section != nil {
			cost = optics[section.ids[subChunkIDIndex(x, step&15, z)]].filter
		}
		if cost >= light {
			return 0
		}
		light -= cost
	}
	return light
}

func (column *Chunk) skySection(section int, sky *[2048]byte) {
	var x, z, pair int = 0, 0, 0
	for x = 0; x < 16; x++ {
		for z = 0; z < 16; z++ {
			var height = int(column.heightMap[z<<4|x])
			var base = section << 4
			if height >= base+30 {
				continue
			}
			for pair = 0; pair < 8; pair++ {
				var y = base + 2*pair
				var index = x<<7 | z<<3 | pair
				if y >= height {
					sky[index] = 0xff
					continue
				}
				sky[index] = column.directSky(x, y, z) | column.directSky(x, y+1, z)<<4
			}
		}
	}
}

// LightAt 供随机刻读取，返回按时刻折算的总光以及不随昼夜变化的方块光，不加载邻区。
func (w *World) LightAt(x, y, z int32) (byte, byte) {
	if y < 0 {
		return 0, 0
	}
	var sky, block byte = 0, 0
	if y > 255 {
		if w.ProtocolDimension() == 0 {
			sky = 15
		}
	} else {
		w.mu.RLock()
		var column = w.chunks[chunkKey(x>>4, z>>4)]
		if column != nil {
			column.mu.Lock()
			block = column.emittedAt(x, y, z)
			if w.ProtocolDimension() == 0 {
				sky = column.directSky(int(x&15), int(y), int(z&15))
			}
			column.mu.Unlock()
		}
		w.mu.RUnlock()
	}
	if sky <= block {
		return block, block
	}
	var progress = math.Mod(float64(w.DayTime())/24000+0.75, 1)
	var angle = progress + (0.5-0.5*math.Cos(progress*math.Pi)-progress)/3
	var loss = byte(11 * min(1.0, max(0.0, 0.5-2*math.Cos(angle*2*math.Pi))))
	return max(sky-min(sky, loss), block), block
}
