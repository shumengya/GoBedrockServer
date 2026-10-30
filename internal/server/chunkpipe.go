package server

import "fmt"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// chunkWorkerCount 并行准备区块包的协程数。区块生成和压缩吃 CPU，但不需要很多，2 个够把 tick 协程腾出来。
const chunkWorkerCount int = 2

// chunkInflightMax 每个玩家同时最多有几个区块包在 worker 手里。
const chunkInflightMax int = 8

// chunkJob 让 worker 准备一个区块包。Epoch 是下单时玩家的视野代数，回来时对不上就说明视野已经重置过
// （换了世界、重新出生），这份区块包是给旧视野的，丢掉。
type chunkJob struct {
	Player *Player
	World  *world.World
	ChunkX int32
	ChunkZ int32
	Epoch  uint64
}

// chunkResult 是 worker 做好的区块包：Batch 已经编码并压缩，tick 协程直接交给连接。
type chunkResult struct {
	Job   chunkJob
	Batch []byte
	Err   error
}

// startChunkWorkers 起 worker 协程。必须在 tick 循环启动之前调用：只有 tick 协程会读 chunkJobs 和 chunkDone，
// 它们建好之后就不再变，tick 协程可以不加锁地读（监听可以先启动，网络协程不碰这两个通道）。
// 没调用过（测试里字面量构造的 Server）时 chunkJobs 是 nil，区块包在 tick 协程里同步准备，行为不变。
func (srv *Server) startChunkWorkers() {
	srv.chunkJobs = make(chan chunkJob, chunkWorkerCount*chunkInflightMax)
	srv.chunkDone = make(chan chunkResult, chunkWorkerCount*chunkInflightMax*2)
	var worker int = 0
	for worker = 0; worker < chunkWorkerCount; worker++ {
		logger.Go("server", srv.chunkWorkerLoop)
	}
}

// chunkWorkerLoop 一个 worker：取任务、生成或读盘、序列化、压缩，把结果放回去。
func (srv *Server) chunkWorkerLoop() {
	var job chunkJob = chunkJob{}
	for {
		select {
		case <-srv.stopCh:
			return
		case job = <-srv.chunkJobs:
			var result chunkResult = safePrepareChunk(job)
			select {
			case srv.chunkDone <- result:
			case <-srv.stopCh:
				return
			}
		}
	}
}

// safePrepareChunk 把 prepareChunk 里的 panic 变成普通错误。worker 一旦因为 panic 退出，
// 它手里这个区块的在途数永远减不下来，那个玩家的区块加载就卡死了，所以必须当成一次失败的交付。
func safePrepareChunk(job chunkJob) chunkResult {
	var result chunkResult = chunkResult{}
	func() {
		defer func() {
			var crashed any = recover()
			if crashed != nil {
				result = chunkResult{Job: job, Err: fmt.Errorf("准备区块 (%d,%d) 时崩溃: %v", job.ChunkX, job.ChunkZ, crashed)}
			}
		}()
		result = prepareChunk(job)
	}()
	return result
}

// prepareChunk 做出一个区块的完整网络包。区块不在内存里时这里会读盘或生成，是最慢的一步。
func prepareChunk(job chunkJob) chunkResult {
	var packet packets.FullChunkDataPacket = packets.FullChunkDataPacket{
		ChunkX: job.ChunkX,
		ChunkZ: job.ChunkZ,
		Data:   job.World.NetworkSerialize(job.ChunkX, job.ChunkZ),
	}
	var raw, encodeErr = packets.EncodePacket(&packet)
	if encodeErr != nil {
		return chunkResult{Job: job, Err: fmt.Errorf("编码区块 (%d,%d): %w", job.ChunkX, job.ChunkZ, encodeErr)}
	}
	var batch, batchErr = protocol.EncodeBatch([][]byte{raw})
	if batchErr != nil {
		return chunkResult{Job: job, Err: fmt.Errorf("压缩区块 (%d,%d): %w", job.ChunkX, job.ChunkZ, batchErr)}
	}
	return chunkResult{Job: job, Batch: batch}
}

// drainChunkResults 取走 worker 已经做好的区块包，逐个交给玩家。只在 tick 协程里调，
// 玩家的区块状态因此始终只有一个协程在改。
func (srv *Server) drainChunkResults() {
	if srv.chunkDone == nil {
		return
	}
	var result chunkResult = chunkResult{}
	for {
		select {
		case result = <-srv.chunkDone:
			srv.deliverChunk(result)
		default:
			return
		}
	}
}

// deliverChunk 把一个做好的区块包交给玩家。视野代数对不上的丢掉，不碰计数。
func (srv *Server) deliverChunk(result chunkResult) {
	var player *Player = result.Job.Player
	var key string = chunkCoordKey(result.Job.ChunkX, result.Job.ChunkZ)
	player.mu.Lock()
	if player.viewEpoch != result.Job.Epoch {
		player.mu.Unlock()
		return
	}
	if player.chunkInflight > 0 {
		player.chunkInflight--
	}
	delete(player.chunkQueued, key)
	var _, alreadySent = player.SentChunks[key]
	var accept bool = result.Err == nil && !alreadySent
	if accept {
		if player.SentChunks == nil {
			player.SentChunks = make(map[string]struct{})
		}
		player.SentChunks[key] = struct{}{}
		player.SpawnChunksSent++
	}
	var session *network.Session = player.Session
	player.mu.Unlock()
	if result.Err != nil {
		logger.Error("server", "%s 准备区块失败: %v", player.Username, result.Err)
	}
	if accept && session != nil {
		session.SendPrepared(result.Batch)
	}
	srv.maybeFinishChunkSpawn(player)
}

// maybeFinishChunkSpawn 出生用的区块够了就放行：近处攒够 spawnChunkMin 个，
// 或者要发的都发完了（视距小的时候凑不够那么多）。只触发一次，触发后 NeedSpawnStatus 清零。
func (srv *Server) maybeFinishChunkSpawn(player *Player) {
	player.mu.RLock()
	var needed bool = player.NeedSpawnStatus
	var sent int = player.SpawnChunksSent
	var drained bool = len(player.ChunkQueue) == 0 && player.chunkInflight == 0
	player.mu.RUnlock()
	if needed && sent > 0 && (sent >= spawnChunkMin || drained) {
		srv.finishChunkSpawn(player)
	}
}

// submitQueuedChunks 从玩家的队列里取区块交给 worker，每 tick 最多 chunksPerTick 个，
// 在 worker 手里的不超过 chunkInflightMax。没有 worker 池（chunkJobs 为 nil）时当场准备并交付。
func (srv *Server) submitQueuedChunks(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var submitted int = 0
	for submitted < chunksPerTick {
		player.mu.Lock()
		if len(player.ChunkQueue) == 0 || (srv.chunkJobs != nil && player.chunkInflight >= chunkInflightMax) {
			player.mu.Unlock()
			break
		}
		// 队列里不会有已经发过的区块：入队时按 SentChunks 和 chunkQueued 去了重，chunkQueued 的标记
		// 保留到交付才删，「弹出了但还没记为已发送」的空档不存在。万一有，deliverChunk 会拦住不重复发。
		var next [2]int32 = player.ChunkQueue[0]
		player.ChunkQueue = player.ChunkQueue[1:]
		player.chunkInflight++
		var job chunkJob = chunkJob{Player: player, World: blockWorld, ChunkX: next[0], ChunkZ: next[1], Epoch: player.viewEpoch}
		player.mu.Unlock()
		submitted++
		if srv.chunkJobs == nil {
			srv.deliverChunk(safePrepareChunk(job))
			continue
		}
		select {
		case srv.chunkJobs <- job:
		default:
			srv.requeueChunk(job)
			return
		}
	}
	srv.maybeFinishChunkSpawn(player)
}

// requeueChunk 工人队列满了没递出去，把区块放回玩家队列最前面，下个 tick 再试。
func (srv *Server) requeueChunk(job chunkJob) {
	job.Player.mu.Lock()
	defer job.Player.mu.Unlock()
	if job.Player.viewEpoch != job.Epoch {
		return
	}
	if job.Player.chunkInflight > 0 {
		job.Player.chunkInflight--
	}
	job.Player.ChunkQueue = append([][2]int32{{job.ChunkX, job.ChunkZ}}, job.Player.ChunkQueue...)
}
