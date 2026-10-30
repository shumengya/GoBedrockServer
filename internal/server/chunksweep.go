package server

import "maps"
import "slices"
import "time"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/world"

// chunkKeepMargin 在视距和随机刻范围之外多留几圈。玩家来回走动时，边缘的区块不会刚出视野就被卸掉、
// 一转身又要重新读盘。
const chunkKeepMargin int32 = 2

// chunkSweepInterval 多久扫一次内存里的区块，卸掉没人要的。
const chunkSweepInterval time.Duration = 10 * time.Second

// wantedChunks 每个世界里还有人要用的区块：每个在线玩家周围「视距和随机刻范围里大的那个，再加边距」的范围。
// key 是世界的真名，玩家记的世界名对不上任何世界时会落到默认世界，这里按落到的那个世界算，
// 免得默认世界把这个玩家脚下的区块当成没人要的卸掉。
func (srv *Server) wantedChunks() map[string]map[[2]int32]bool {
	var wanted map[string]map[[2]int32]bool = map[string]map[[2]int32]bool{}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var uuids []string = slices.Sorted(maps.Keys(srv.players))
	var index int = 0
	for index = 0; index < len(uuids); index++ {
		var player *Player = srv.players[uuids[index]]
		var worldName string = srv.playerWorld(player).Name
		player.mu.RLock()
		var radius int32 = player.ViewRadius
		var posX float32 = player.X
		var posZ float32 = player.Z
		player.mu.RUnlock()
		if radius <= 0 {
			radius = defaultViewRad
		}
		radius = max(radius, int32(randomTickRadius)) + chunkKeepMargin
		var set map[[2]int32]bool = wanted[worldName]
		if set == nil {
			set = map[[2]int32]bool{}
			wanted[worldName] = set
		}
		var around [][2]int32 = world.ChunksAroundChunk(world.BlockToChunk(posX), world.BlockToChunk(posZ), radius)
		var cell int = 0
		for cell = 0; cell < len(around); cell++ {
			set[around[cell]] = true
		}
	}
	return wanted
}

// sweepChunks 把没人要的区块存盘后从内存移走，返回移走了几个。一个人都没有的世界会被清空。
// 脏区块存不成功就留着，下一轮再试，不会丢数据。
func (srv *Server) sweepChunks() int {
	var wanted map[string]map[[2]int32]bool = srv.wantedChunks()
	var names []string = slices.Sorted(maps.Keys(srv.worlds))
	var total int = 0
	var index int = 0
	for index = 0; index < len(names); index++ {
		var set map[[2]int32]bool = wanted[names[index]]
		var removed, unloadErr = srv.worlds[names[index]].UnloadChunks(func(chunkX int32, chunkZ int32) bool {
			return set[[2]int32{chunkX, chunkZ}]
		})
		if unloadErr != nil {
			logger.Error("server", "卸载世界 %s 的区块出错: %v", names[index], unloadErr)
		}
		total += removed
	}
	return total
}

// startChunkSweep 在后台扫一遍。存盘要压缩和写文件，不能占着 tick 协程。
// 上一次还没扫完就不再开新的，返回 false。
func (srv *Server) startChunkSweep() bool {
	if !srv.chunkSweepBusy.CompareAndSwap(false, true) {
		return false
	}
	logger.Go("server", func() {
		defer srv.chunkSweepBusy.Store(false)
		var removed int = srv.sweepChunks()
		if removed > 0 {
			logger.Debug("server", "卸载了 %d 个没人看的区块", removed)
		}
	})
	return true
}
