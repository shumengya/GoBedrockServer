package server

import "cmp"
import "maps"
import "math"
import "slices"
import "strings"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/world"

// 随机刻只作用在玩家附近：每个玩家每 tick 在周围 randomTickRadius 个区块的范围里随机抽
// randomTickChunksPerPlayer 个区块，每个区块里每个有数据的子区块再随机抽 randomTickPicksPerSubChunk 个位置。
// 数量对齐参考端的量级，作物大约要几分钟才会长一个阶段，不会瞬间长满。
const randomTickRadius int = 4
const randomTickChunksPerPlayer int = 12
const randomTickPicksPerSubChunk int = 3

type randomChunk struct {
	World  string
	ChunkX int32
	ChunkZ int32
}

func compareRandomChunk(left randomChunk, right randomChunk) int {
	return cmp.Or(
		strings.Compare(left.World, right.World),
		cmp.Compare(left.ChunkX, right.ChunkX),
		cmp.Compare(left.ChunkZ, right.ChunkZ),
	)
}

func (srv *Server) tickRandom() {
	srv.tickRandomWith(defaultDropRandom{})
}

// tickRandomWith 把随机源作为参数，测试里换成固定序列。
func (srv *Server) tickRandomWith(random dropRandom) {
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var chunks map[randomChunk]bool = srv.randomChunksNearPlayers(random)
	if len(chunks) == 0 {
		return
	}
	var ordered []randomChunk = slices.SortedFunc(maps.Keys(chunks), compareRandomChunk)
	var index int = 0
	for index = 0; index < len(ordered); index++ {
		srv.randomTickChunkLocked(ordered[index], random)
	}
}

// randomChunksNearPlayers 给每个在线玩家抽 randomTickChunksPerPlayer 个周围的区块，去重后返回。
func (srv *Server) randomChunksNearPlayers(random dropRandom) map[randomChunk]bool {
	var chunks map[randomChunk]bool = map[randomChunk]bool{}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var uuids []string = slices.Sorted(maps.Keys(srv.players))
	var index int = 0
	for index = 0; index < len(uuids); index++ {
		var player *Player = srv.players[uuids[index]]
		if player.Session == nil || player.IsDead() {
			continue
		}
		var posX, _, posZ = player.Position()
		var centerX int32 = int32(math.Floor(float64(posX) / 16))
		var centerZ int32 = int32(math.Floor(float64(posZ) / 16))
		var worldName string = srv.playerWorldName(player)
		var sample int = 0
		for sample = 0; sample < randomTickChunksPerPlayer; sample++ {
			var offsetX int32 = int32(random.IntN(2*randomTickRadius+1) - randomTickRadius)
			var offsetZ int32 = int32(random.IntN(2*randomTickRadius+1) - randomTickRadius)
			chunks[randomChunk{World: worldName, ChunkX: centerX + offsetX, ChunkZ: centerZ + offsetZ}] = true
		}
	}
	return chunks
}

// randomTickChunkLocked 在一个区块里抽查位置并评估。区块没加载就跳过，不会把它加载出来。
func (srv *Server) randomTickChunkLocked(chunk randomChunk, random dropRandom) {
	var blockWorld *world.World = srv.worldByName(chunk.World)
	var subChunks []int = blockWorld.LoadedSubChunks(chunk.ChunkX, chunk.ChunkZ)
	var subIndex int = 0
	for subIndex = 0; subIndex < len(subChunks); subIndex++ {
		var pick int = 0
		for pick = 0; pick < randomTickPicksPerSubChunk; pick++ {
			var spot blockrule.Pos = blockrule.Pos{
				PosX: chunk.ChunkX*16 + int32(random.IntN(16)),
				PosY: int32(subChunks[subIndex])*16 + int32(random.IntN(16)),
				PosZ: chunk.ChunkZ*16 + int32(random.IntN(16)),
			}
			var id, _, loaded = blockWorld.Peek(spot.PosX, spot.PosY, spot.PosZ)
			if !loaded || !blockrule.IsRandomTicked(id) {
				continue
			}
			var update blockrule.Update = blockrule.OnRandomTick(peekView{target: blockWorld}, spot, random)
			srv.applyUpdateLocked(chunk.World, blockWorld, update)
		}
	}
}
