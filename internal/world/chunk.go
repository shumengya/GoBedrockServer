package world

import (
	"encoding/binary"
	"errors"
	"sync"
)

const (
	BlockAir           = 0
	BlockStone         = 1
	BlockGrass         = 2
	BlockDirt          = 3
	BlockCobblestone   = 4
	BlockPlanks        = 5
	BlockBedrock       = 7
	BlockStillWater    = 9 // PE still water
	BlockFlowingWater  = 8
	BlockStillLava     = 11
	BlockFlowingLava   = 10
	BlockSand          = 12
	BlockCactus        = 81
	BlockLog           = 17
	BlockGlass         = 20
	BlockWool          = 35
	BlockBed           = 26
	BlockChest         = 54
	BlockCraftingTable = 58
	BlockFlowerPot     = 140
	BlockSkull         = 144
	BlockHopper        = 154
	BlockItemFrame     = 199
	BlockNetherrack    = 87
	BlockEndStone      = 121

	GeneratorFlat   = "flat"
	GeneratorNormal = "normal"
	GeneratorSky    = "sky"
	GeneratorOcean  = "ocean"
	GeneratorNether = "nether"
	GeneratorEnd    = "end"

	spawnPlatformY = 64

	maxSubChunks = 16
)

var (
	errDiskShort   = errors.New("column disk payload too short")
	errDiskVersion = errors.New("unsupported column disk version")
	errDiskCorrupt = errors.New("corrupt column disk payload")
)

func subChunkIDIndex(x, y, z int) int {
	return (x << 8) | (z << 4) | y
}

func dataIndex(x, y, z int) (byteIdx int, shift uint) {
	byteIdx = (x << 7) | (z << 3) | (y >> 1)
	shift = uint((y & 1) << 2)
	return
}

type SubChunk struct {
	ids        [4096]byte
	data       [2048]byte
	litCells   int
	blockLight [2048]byte
}

func newSubChunk() *SubChunk {
	return &SubChunk{}
}

func (sc *SubChunk) empty() bool {
	for _, id := range sc.ids {
		if id != 0 {
			return false
		}
	}
	return true
}

func (sc *SubChunk) get(x, y, z int) (id, data byte) {
	id = sc.ids[subChunkIDIndex(x, y, z)]
	bi, sh := dataIndex(x, y, z)
	data = (sc.data[bi] >> sh) & 0x0F
	return
}

func (sc *SubChunk) set(x, y, z int, id, data byte) {
	sc.ids[subChunkIDIndex(x, y, z)] = id
	bi, sh := dataIndex(x, y, z)
	sc.data[bi] = (sc.data[bi] &^ (0x0F << sh)) | ((data & 0x0F) << sh)
}

func (sc *SubChunk) networkSerialize(sky *[2048]byte) []byte {
	buf := make([]byte, 0, 1+4096+2048+2048+2048)
	buf = append(buf, 0)
	buf = append(buf, sc.ids[:]...)
	buf = append(buf, sc.data[:]...)
	buf = append(buf, sky[:]...)
	buf = append(buf, sc.blockLight[:]...)
	return buf
}

// Chunk is a mutable 16×256×16 column of sub-chunks.
type Chunk struct {
	mu  sync.Mutex
	sub [maxSubChunks]*SubChunk
	// 只在侧向天光超过本列直射值时存该格，空子区块无需为光创建方块数组。
	sideSky   [maxSubChunks]*[2048]byte
	heightMap [256]uint16
	biomes    [256]byte
	dirty     bool
}

func newChunk() *Chunk {
	c := &Chunk{}
	for i := range c.biomes {
		c.biomes[i] = 1 // plains
	}
	return c
}

func (c *Chunk) subAt(sy int) *SubChunk {
	if c.sub[sy] == nil {
		c.sub[sy] = newSubChunk()
	}
	return c.sub[sy]
}

func (c *Chunk) GetBlock(x, y, z int) (id, data byte) {
	if y < 0 || y > 255 {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sy := y >> 4
	if c.sub[sy] == nil {
		return 0, 0
	}
	return c.sub[sy].get(x, y&15, z)
}

func (c *Chunk) SetBlock(x, y, z int, id, data byte) {
	if y < 0 || y > 255 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setBlockLocked(x, y, z, id, data)
}

func (c *Chunk) setBlockLocked(x, y, z int, id, data byte) {
	sc := c.subAt(y >> 4)
	var previous = sc.ids[subChunkIDIndex(x, y&15, z)]
	sc.set(x, y&15, z, id, data)
	c.dirty = true
	if skyBarrier(previous) == skyBarrier(id) {
		return
	}
	var index = z<<4 | x
	if skyBarrier(id) {
		c.heightMap[index] = max(c.heightMap[index], uint16(y+1))
	} else if c.heightMap[index] == uint16(y+1) {
		c.heightMap[index] = c.skyHeight(x, y-1, z)
	}
}

func (c *Chunk) Dirty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dirty
}

func (c *Chunk) ClearDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = false
}

// MarkDirty 把区块标成待存盘。存盘失败时用它把标记还回去，下一轮重试。
func (c *Chunk) MarkDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = true
}

const diskColumnVersion = 1

// EncodeDisk writes a compact column (non-empty subchunks only, no light).
func (c *Chunk) EncodeDisk() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n byte
	for i := 0; i < maxSubChunks; i++ {
		if c.sub[i] != nil && !c.sub[i].empty() {
			n++
		}
	}
	buf := make([]byte, 0, 2+int(n)*(1+4096+2048)+512+256)
	buf = append(buf, diskColumnVersion, n)
	for i := 0; i < maxSubChunks; i++ {
		sc := c.sub[i]
		if sc == nil || sc.empty() {
			continue
		}
		buf = append(buf, byte(i))
		buf = append(buf, sc.ids[:]...)
		buf = append(buf, sc.data[:]...)
	}
	var hm [512]byte
	for i := 0; i < 256; i++ {
		binary.LittleEndian.PutUint16(hm[i*2:], c.heightMap[i])
	}
	buf = append(buf, hm[:]...)
	buf = append(buf, c.biomes[:]...)
	return buf
}

// 老版本把玻璃等透光方块也计入高度图；读盘时由方块重建。
func DecodeDisk(raw []byte) (*Chunk, error) {
	if len(raw) < 2+512+256 {
		return nil, errDiskShort
	}
	if raw[0] != diskColumnVersion {
		return nil, errDiskVersion
	}
	n := int(raw[1])
	off := 2
	c := newChunk()
	for i := 0; i < n; i++ {
		if off+1+4096+2048 > len(raw) {
			return nil, errDiskShort
		}
		sy := int(raw[off])
		off++
		if sy < 0 || sy >= maxSubChunks {
			return nil, errDiskCorrupt
		}
		sc := newSubChunk()
		copy(sc.ids[:], raw[off:off+4096])
		off += 4096
		copy(sc.data[:], raw[off:off+2048])
		off += 2048
		c.sub[sy] = sc
	}
	if off+512+256 > len(raw) {
		return nil, errDiskShort
	}
	for i := 0; i < 256; i++ {
		c.heightMap[i] = c.skyHeight(i&15, 255, i>>4)
		off += 2
	}
	copy(c.biomes[:], raw[off:off+256])
	return c, nil
}

func (c *Chunk) generate(gen string, seed, cx, cz int32) {
	switch gen {
	case GeneratorNormal:
		c.fillNormal(seed, cx, cz)
		c.fillNormalOres(seed, cx, cz)
	case GeneratorSky:
		c.fillSky(cx, cz)
	case GeneratorOcean:
		c.fillOcean(cx, cz)
	case GeneratorNether:
		c.fillNether(cx, cz)
	case GeneratorEnd:
		c.fillEnd(cx, cz)
	default:
		c.fillFlat()
	}
}

func inOriginChunk(cx, cz int32) bool {
	return cx == 0 && cz == 0
}

// paintOriginIsland fills the entire (0,0) column with a two-layer pad at y=63/64. Caller holds c.mu.
func (c *Chunk) paintOriginIsland(cx, cz int32, top, below byte) {
	if !inOriginChunk(cx, cz) {
		return
	}
	belowY := spawnPlatformY - 1
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			c.subAt(belowY>>4).set(lx, belowY&15, lz, below, 0)
			c.subAt(spawnPlatformY>>4).set(lx, spawnPlatformY&15, lz, top, 0)
			c.heightMap[(lz<<4)|lx] = uint16(spawnPlatformY + 1)
		}
	}
}

func (c *Chunk) fillFlat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	sc := c.subAt(0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			sc.set(x, 0, z, BlockBedrock, 0)
			sc.set(x, 1, z, BlockDirt, 0)
			sc.set(x, 2, z, BlockDirt, 0)
			sc.set(x, 3, z, BlockDirt, 0)
			sc.set(x, 4, z, BlockGrass, 0)
			c.heightMap[(z<<4)|x] = 5
		}
	}
}

func (c *Chunk) fillSky(cx, cz int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paintOriginIsland(cx, cz, BlockGrass, BlockDirt)
}

func (c *Chunk) fillOcean(cx, cz int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for y := 0; y < spawnPlatformY; y++ {
		sc := c.subAt(y >> 4)
		ly := y & 15
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				sc.set(x, ly, z, BlockStillWater, 0)
			}
		}
	}
	for i := range c.heightMap {
		c.heightMap[i] = uint16(spawnPlatformY)
	}
	c.paintOriginIsland(cx, cz, BlockGrass, BlockDirt)
}

func (c *Chunk) fillNether(cx, cz int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paintOriginIsland(cx, cz, BlockNetherrack, BlockNetherrack)
}

func (c *Chunk) fillEnd(cx, cz int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paintOriginIsland(cx, cz, BlockEndStone, BlockEndStone)
}

// NetworkSerialize 单独编码一列时使用主世界天空；世界发包时按维度选择天空光。
func (c *Chunk) NetworkSerialize() []byte { return c.networkPayload(true) }

func (c *Chunk) networkPayload(hasSky bool) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 1
	for i := maxSubChunks - 1; i >= 0; i-- {
		if c.sub[i] != nil && (c.sub[i].litCells > 0 || !c.sub[i].empty()) {
			count = i + 1
			break
		}
	}
	empty := newSubChunk()
	var buf []byte
	buf = append(buf, byte(count))
	for i := 0; i < count; i++ {
		sc := c.sub[i]
		if sc == nil {
			sc = empty
		}
		var sky [2048]byte
		if hasSky {
			c.skySection(i, &sky)
		}
		buf = append(buf, sc.networkSerialize(&sky)...)
	}
	var hm [512]byte
	for i := 0; i < 256; i++ {
		binary.LittleEndian.PutUint16(hm[i*2:], c.heightMap[i])
	}
	buf = append(buf, hm[:]...)
	buf = append(buf, c.biomes[:]...)
	buf = append(buf, 0) // border count
	buf = append(buf, 0) // extraData VarInt 0
	return buf
}

// GenerateFlatChunk returns a freshly generated superflat column payload.
func GenerateFlatChunk() []byte {
	c := newChunk()
	c.fillFlat()
	return c.networkPayload(true)
}

// OffsetByFace returns the neighbour of (x,y,z) for a 1.1.x block face (0-5).
func OffsetByFace(x, y, z, face int32) (nx, ny, nz int32, ok bool) {
	switch face {
	case 0:
		return x, y - 1, z, true
	case 1:
		return x, y + 1, z, true
	case 2:
		return x, y, z - 1, true
	case 3:
		return x, y, z + 1, true
	case 4:
		return x - 1, y, z, true
	case 5:
		return x + 1, y, z, true
	default:
		return x, y, z, false
	}
}
