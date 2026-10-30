package world

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
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

	// persistMu 把「存盘」和「卸载」串起来：卸载是先存盘再从内存移除，
	// 如果它和另一次存盘交错，区块可能在新数据落盘之前就被重新读回内存，读到旧数据。
	// 加锁顺序：persistMu → mu → 区块自己的锁。
	persistMu sync.Mutex

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

// chunk 返回一个区块列，不在内存里就读盘，盘上没有就生成。
// 读盘和生成都在世界锁外面做：生成一个区块要几毫秒，持着锁做会让所有人读写方块都排队。
// 两个协程同时要同一个区块时各做一遍，后到的丢掉自己那份，用先放进去的。
func (w *World) chunk(cx, cz int32) *Chunk {
	var key uint64 = chunkKey(cx, cz)
	w.mu.RLock()
	var found *Chunk = w.chunks[key]
	var store ColumnStore = w.store
	var generator string = w.Generator
	w.mu.RUnlock()
	if found != nil {
		return found
	}
	var created *Chunk = nil
	if store != nil {
		var loaded, loadErr = store.LoadColumn(cx, cz)
		if loadErr == nil {
			created = loaded
		}
	}
	if created == nil {
		created = newChunk()
		created.generate(generator, cx, cz)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var existing *Chunk = w.chunks[key]
	if existing != nil {
		return existing
	}
	w.chunks[key] = created
	return created
}

// dirtyColumn 一个待存盘的区块列。
type dirtyColumn struct {
	ChunkX int32
	ChunkZ int32
	Column *Chunk
}

// SaveDirty 把改过的区块列写进存储。每个区块先清脏标记再存：存的过程中又有人改它，
// 脏标记会重新立起来，下一轮再存；反过来先存后清的话，夹在中间的修改会被一起清掉，永远不落盘。
// 存失败的区块把脏标记还回去，别的区块照常继续。
func (w *World) SaveDirty() error {
	w.persistMu.Lock()
	defer w.persistMu.Unlock()
	w.mu.RLock()
	var store ColumnStore = w.store
	var keys []uint64 = slices.Sorted(maps.Keys(w.chunks))
	var dirty []dirtyColumn = nil
	var index int = 0
	for index = 0; index < len(keys); index++ {
		var column *Chunk = w.chunks[keys[index]]
		if column.Dirty() {
			var chunkX, chunkZ = splitChunkKey(keys[index])
			dirty = append(dirty, dirtyColumn{ChunkX: chunkX, ChunkZ: chunkZ, Column: column})
		}
	}
	w.mu.RUnlock()
	if store == nil {
		return nil
	}
	var failures []error = nil
	for index = 0; index < len(dirty); index++ {
		dirty[index].Column.ClearDirty()
		var saveErr error = store.SaveColumn(dirty[index].ChunkX, dirty[index].ChunkZ, dirty[index].Column)
		if saveErr != nil {
			dirty[index].Column.MarkDirty()
			failures = append(failures, fmt.Errorf("保存区块 (%d,%d): %w", dirty[index].ChunkX, dirty[index].ChunkZ, saveErr))
		}
	}
	return errors.Join(failures...)
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

// Peek 只读已经在内存里的区块：区块没加载就返回 loaded=false，不会触发生成或读盘。
// 邻居更新、随机刻靠它判断边界，免得一次放方块把相邻的整片空区块都生成出来。
// 超出 0~255 高度的位置按已加载的空气处理，这样世界顶底的支撑判断才有确定结果。
func (w *World) Peek(posX int32, posY int32, posZ int32) (id byte, data byte, loaded bool) {
	if posY < 0 || posY > 255 {
		return BlockAir, 0, true
	}
	var key uint64 = chunkKey(posX>>4, posZ>>4)
	w.mu.RLock()
	var column *Chunk = w.chunks[key]
	w.mu.RUnlock()
	if column == nil {
		return BlockAir, 0, false
	}
	var blockID, blockData = column.GetBlock(int(posX&15), int(posY), int(posZ&15))
	return blockID, blockData, true
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
	// 写之前拿到的区块可能正好被卸载线程存盘后移走，写进去就丢了，withChunk 会在这种情况下重做。
	return w.withChunk(cx, cz, func(column *Chunk) {
		column.SetBlock(lx, int(y), lz, id, data)
	})
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
