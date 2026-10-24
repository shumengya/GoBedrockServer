package world

import (
	"math"
	"sync"
)

// ColumnStore persists chunk columns. Missing columns return (nil, nil).
type ColumnStore interface {
	LoadColumn(cx, cz int32) (*Chunk, error)
	SaveColumn(cx, cz int32, c *Chunk) error
}

// World holds spawn metadata and an in-memory map of mutable chunk columns.
type World struct {
	Name      string
	Generator string
	SpawnX    int32
	SpawnY    int32
	SpawnZ    int32
	Seed      int32

	clock clock

	mu     sync.RWMutex
	chunks map[uint64]*Chunk
	store  ColumnStore

	tileMu     sync.RWMutex
	tiles      map[string]Tile
	tilesDirty bool
}

// New returns a World with sensible defaults.
func New(name string) *World {
	return &World{
		Name:      name,
		Generator: GeneratorFlat,
		SpawnX:    0,
		SpawnY:    5,
		SpawnZ:    0,
		Seed:      12345,
		chunks:    make(map[uint64]*Chunk),
		tiles:     make(map[string]Tile),
	}
}

func chunkKey(cx, cz int32) uint64 {
	return uint64(uint32(cx))<<32 | uint64(uint32(cz))
}

func splitChunkKey(key uint64) (cx, cz int32) {
	return int32(key >> 32), int32(key)
}

func (w *World) SetStore(s ColumnStore) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.store = s
}

func (w *World) chunk(cx, cz int32) *Chunk {
	key := chunkKey(cx, cz)
	w.mu.Lock()
	if c, ok := w.chunks[key]; ok {
		w.mu.Unlock()
		return c
	}
	store := w.store
	w.mu.Unlock()

	var loaded *Chunk
	if store != nil {
		c, err := store.LoadColumn(cx, cz)
		if err == nil {
			loaded = c
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if c, ok := w.chunks[key]; ok {
		return c
	}
	if loaded != nil {
		w.chunks[key] = loaded
		return loaded
	}
	c := newChunk()
	c.generate(w.Generator, cx, cz)
	w.chunks[key] = c
	return c
}

// SaveDirty writes modified columns to the store.
func (w *World) SaveDirty() error {
	w.mu.RLock()
	store := w.store
	type col struct {
		cx, cz int32
		c      *Chunk
	}
	var dirty []col
	for key, c := range w.chunks {
		if c.Dirty() {
			cx, cz := splitChunkKey(key)
			dirty = append(dirty, col{cx, cz, c})
		}
	}
	w.mu.RUnlock()
	if store == nil {
		return nil
	}
	for _, d := range dirty {
		if err := store.SaveColumn(d.cx, d.cz, d.c); err != nil {
			return err
		}
		d.c.ClearDirty()
	}
	return nil
}

// GetBlock returns the block id/data at world coordinates.
func (w *World) GetBlock(x, y, z int32) (id, data byte) {
	if y < 0 || y > 255 {
		return BlockAir, 0
	}
	lx := int(x & 15)
	lz := int(z & 15)
	if x < 0 {
		lx = int((x%16 + 16) % 16)
	}
	if z < 0 {
		lz = int((z%16 + 16) % 16)
	}
	cx := int32(math.Floor(float64(x) / 16.0))
	cz := int32(math.Floor(float64(z) / 16.0))
	return w.chunk(cx, cz).GetBlock(lx, int(y), lz)
}

// BlockID 只取方块 id，忽略 data。判定用不上元数据时用它，省得每次接两个返回值。
func (w *World) BlockID(posX int32, posY int32, posZ int32) byte {
	var id, data = w.GetBlock(posX, posY, posZ)
	_ = data
	return id
}

// SetBlock writes a block. Existing bedrock at y=0 cannot be replaced. Returns false if ignored.
func (w *World) SetBlock(x, y, z int32, id, data byte) bool {
	if y < 0 || y > 255 {
		return false
	}
	if y == 0 {
		cur, _ := w.GetBlock(x, y, z)
		if cur == BlockBedrock && id != BlockBedrock {
			return false
		}
	}
	lx := int((x%16 + 16) % 16)
	lz := int((z%16 + 16) % 16)
	cx := int32(math.Floor(float64(x) / 16.0))
	cz := int32(math.Floor(float64(z) / 16.0))
	w.chunk(cx, cz).SetBlock(lx, int(y), lz, id, data)
	return true
}

// NetworkSerialize returns FullChunkData payload for column (cx, cz).
func (w *World) NetworkSerialize(cx, cz int32) []byte {
	buf := w.chunk(cx, cz).NetworkSerialize()
	return append(buf, w.columnTileNBT(cx, cz)...)
}

// ProtocolDimension is the vanilla dimension id sent in StartGame / ChangeDimension.
// Nether = 1, the End = 2, everything else (overworld / sky / ocean) = 0.
func (w *World) ProtocolDimension() int32 {
	if w == nil {
		return 0
	}
	switch w.Generator {
	case GeneratorNether:
		return 1
	case GeneratorEnd:
		return 2
	default:
		return 0
	}
}

// ChunksAround returns the (chunkX, chunkZ) pairs within radius of a block position.
func ChunksAround(worldX, worldZ, radius int32) [][2]int32 {
	cx := worldX >> 4
	cz := worldZ >> 4
	return ChunksAroundChunk(cx, cz, radius)
}

// BlockToChunk converts a block coordinate (float) to chunk coordinates.
func BlockToChunk(block float32) int32 {
	return int32(math.Floor(float64(block) / 16.0))
}

// ChunksAroundChunk lists chunk column coordinates within radius of (cx, cz).
func ChunksAroundChunk(cx, cz, radius int32) [][2]int32 {
	var chunks [][2]int32
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			chunks = append(chunks, [2]int32{cx + dx, cz + dz})
		}
	}
	return chunks
}
