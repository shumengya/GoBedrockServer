package server

import "errors"
import "os"
import "testing"
import "time"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

// chunkCoords 按发送顺序列出发给这个玩家的所有区块坐标。
func (log *sentLog) chunkCoords(test *testing.T) [][2]int32 {
	var coords [][2]int32 = nil
	var index int = 0
	for index = 0; index < len(log.raws); index++ {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDFullChunkData {
			continue
		}
		var stream *protocol.BinaryStream = protocol.NewStream(log.raws[index][1:])
		var chunkX, xErr = stream.ReadVarInt()
		var chunkZ, zErr = stream.ReadVarInt()
		if xErr != nil || zErr != nil {
			test.Fatalf("解码区块坐标失败: %v %v", xErr, zErr)
		}
		coords = append(coords, [2]int32{chunkX, chunkZ})
	}
	return coords
}

// pipeTestPlayer 一个站在原点、视野里排满了区块的玩家。needSpawn 为真时还处在出生等区块的阶段。
func pipeTestPlayer(test *testing.T, radius int32, needSpawn bool) (*adminFixture, *Player, *sentLog) {
	var fixture, player = placeTestSetup(test)
	player.ViewRadius = radius
	player.NeedSpawnStatus = needSpawn
	var log *sentLog = captureSent(player)
	player.enqueueChunks(chunkSquare(radius))
	return fixture, player, log
}

func chunkSquare(radius int32) [][2]int32 {
	var cells [][2]int32 = nil
	var chunkX int32 = 0
	var chunkZ int32 = 0
	for chunkX = -radius; chunkX <= radius; chunkX++ {
		for chunkZ = -radius; chunkZ <= radius; chunkZ++ {
			cells = append(cells, [2]int32{chunkX, chunkZ})
		}
	}
	return cells
}

// pipeTestManual 装上通道但不起 worker，测试自己决定任务什么时候做完。
func pipeTestManual(fixture *adminFixture, capacity int) {
	fixture.srv.chunkJobs = make(chan chunkJob, capacity)
	fixture.srv.chunkDone = make(chan chunkResult, capacity*4)
}

// pipeTestFinishJobs 把通道里排着的任务全部做完，结果放进 chunkDone，返回做了几个。
func pipeTestFinishJobs(fixture *adminFixture) int {
	var finished int = 0
	for len(fixture.srv.chunkJobs) > 0 {
		var job chunkJob = <-fixture.srv.chunkJobs
		fixture.srv.chunkDone <- prepareChunk(job)
		finished++
	}
	return finished
}

// 没有 worker 池时（字面量构造的服务器、旧行为）当场准备，每 tick 发 chunksPerTick 个，近的先发。
func TestChunkSendWithoutWorkersIsSynchronous(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 2, false)
	fixture.srv.tickChunkSend()
	var coords [][2]int32 = log.chunkCoords(test)
	if len(coords) != chunksPerTick {
		test.Fatalf("一个 tick 应发 %d 个区块，实际 %d", chunksPerTick, len(coords))
	}
	if coords[0] != [2]int32{0, 0} {
		test.Fatalf("第一个应是脚下的 (0,0)，实际 %v", coords[0])
	}
	if len(player.ChunkQueue) != 25-chunksPerTick || player.SpawnChunksSent != chunksPerTick {
		test.Fatalf("队列应剩 %d、已发计数 %d，实际 %d / %d",
			25-chunksPerTick, chunksPerTick, len(player.ChunkQueue), player.SpawnChunksSent)
	}
	if player.chunkInflight != 0 {
		test.Fatalf("同步准备后不该有在途的区块，实际 %d", player.chunkInflight)
	}
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != 2*chunksPerTick {
		test.Fatalf("第二个 tick 应再发 %d 个，共 %d，实际 %d", chunksPerTick, 2*chunksPerTick, len(log.chunkCoords(test)))
	}
}

// 出生阶段：攒够 spawnChunkMin 个区块就放行，放行只发一次。
func TestChunkSendFinishesSpawnOnceAfterEnoughChunks(test *testing.T) {
	var fixture, _, log = pipeTestPlayer(test, 4, true)
	var tick int = 0
	for tick = 0; tick < 3; tick++ {
		fixture.srv.tickChunkSend()
		if log.count(protocol.IDPlayStatus) != 0 {
			test.Fatalf("第 %d 个 tick 后才发了 %d 个区块，不够 %d 个，不该放行", tick+1, (tick+1)*chunksPerTick, spawnChunkMin)
		}
	}
	fixture.srv.tickChunkSend()
	if log.count(protocol.IDPlayStatus) != 1 {
		test.Fatalf("发够 %d 个区块后应放行 1 次，实际 %d", spawnChunkMin, log.count(protocol.IDPlayStatus))
	}
	fixture.srv.tickChunkSend()
	fixture.srv.tickChunkSend()
	if log.count(protocol.IDPlayStatus) != 1 {
		test.Fatalf("放行只能发一次，实际 %d", log.count(protocol.IDPlayStatus))
	}
}

// 视距小、凑不够 spawnChunkMin 个：要发的都发完了就放行，不能一直卡在加载界面。
func TestChunkSendFinishesSpawnWhenQueueDrains(test *testing.T) {
	var fixture, _, log = pipeTestPlayer(test, 1, true)
	fixture.srv.tickChunkSend()
	fixture.srv.tickChunkSend()
	if log.count(protocol.IDPlayStatus) != 0 {
		test.Fatalf("才发了 8 个，队列里还有 1 个，不该放行，实际 %d", log.count(protocol.IDPlayStatus))
	}
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != 9 || log.count(protocol.IDPlayStatus) != 1 {
		test.Fatalf("9 个全发完应放行 1 次，实际 区块=%d 放行=%d", len(log.chunkCoords(test)), log.count(protocol.IDPlayStatus))
	}
}

// 有 worker 池时，每个玩家在途的区块不超过上限；超过之后不再下单，等做好的交付了腾出名额。
func TestChunkSendRespectsInflightCap(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 4, false)
	pipeTestManual(fixture, 64)
	var tick int = 0
	for tick = 0; tick < 5; tick++ {
		fixture.srv.tickChunkSend()
	}
	if player.chunkInflight != chunkInflightMax || len(fixture.srv.chunkJobs) != chunkInflightMax {
		test.Fatalf("在途应卡在上限 %d，实际 在途=%d 通道里=%d", chunkInflightMax, player.chunkInflight, len(fixture.srv.chunkJobs))
	}
	if len(log.chunkCoords(test)) != 0 {
		test.Fatal("worker 还没做完，不该有区块发出去")
	}
	pipeTestFinishJobs(fixture)
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != chunkInflightMax {
		test.Fatalf("做好的 %d 个应在下个 tick 交付，实际 %d", chunkInflightMax, len(log.chunkCoords(test)))
	}
	if player.chunkInflight != chunksPerTick {
		test.Fatalf("交付后腾出名额，应再下 %d 单，实际在途 %d", chunksPerTick, player.chunkInflight)
	}
}

// 出生放行要等在途的区块也到齐：队列空了但还有区块在 worker 手里，不能放行。
func TestChunkSendWaitsForInflightBeforeSpawn(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, true)
	pipeTestManual(fixture, 64)
	// 第 1 个 tick 下 4 单，做完；第 2 个 tick 交付 4 个、再下 4 单；第 3 个 tick 把最后 1 个也下了单。
	// 此时队列空了，但还有 5 个在 worker 手里，已发 4 个，不够 spawnChunkMin，也还没排空。
	fixture.srv.tickChunkSend()
	pipeTestFinishJobs(fixture)
	fixture.srv.tickChunkSend()
	fixture.srv.tickChunkSend()
	if len(player.ChunkQueue) != 0 || player.chunkInflight != 5 || player.SpawnChunksSent != 4 {
		test.Fatalf("夹具：期望 队列=0 在途=5 已发=4，实际 队列=%d 在途=%d 已发=%d",
			len(player.ChunkQueue), player.chunkInflight, player.SpawnChunksSent)
	}
	if log.count(protocol.IDPlayStatus) != 0 {
		test.Fatal("还有区块在 worker 手里，不该放行")
	}
	pipeTestFinishJobs(fixture)
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != 9 || log.count(protocol.IDPlayStatus) != 1 {
		test.Fatalf("全部交付后应放行 1 次，实际 区块=%d 放行=%d", len(log.chunkCoords(test)), log.count(protocol.IDPlayStatus))
	}
}

// worker 队列满了递不出去，区块放回队列最前面，在途数不变，下个 tick 接着发，顺序不乱。
func TestChunkSendRequeuesWhenWorkerQueueIsFull(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, false)
	pipeTestManual(fixture, 2)
	fixture.srv.tickChunkSend()
	if len(fixture.srv.chunkJobs) != 2 || player.chunkInflight != 2 {
		test.Fatalf("通道只装得下 2 个，实际 通道=%d 在途=%d", len(fixture.srv.chunkJobs), player.chunkInflight)
	}
	if len(player.ChunkQueue) != 7 {
		test.Fatalf("9 个里下了 2 单，递不出去的放回队列，应剩 7 个，实际 %d", len(player.ChunkQueue))
	}
	if chunkChebyshev(player.ChunkQueue[0][0], player.ChunkQueue[0][1], 0, 0) != 1 {
		test.Fatalf("放回去的应排在最前面，且是紧挨着脚下的那一圈，实际队首 %v", player.ChunkQueue[0])
	}
	pipeTestFinishJobs(fixture)
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != 2 {
		test.Fatalf("做好的 2 个应交付，实际 %d", len(log.chunkCoords(test)))
	}
}

// 视野重置之后，旧视野里还在 worker 手里的区块包回来要丢掉：不发、不计数、不碰新视野的在途数。
func TestStaleChunkResultIsDiscarded(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, false)
	pipeTestManual(fixture, 64)
	fixture.srv.tickChunkSend()
	var epochBefore uint64 = player.viewEpoch
	fixture.srv.enqueuePlayerView(player)
	if player.viewEpoch == epochBefore || player.chunkInflight != 0 {
		test.Fatalf("重置视野应让代数加一、在途清零，实际 代数 %d→%d 在途=%d", epochBefore, player.viewEpoch, player.chunkInflight)
	}
	var inflightAfterReset int = player.chunkInflight
	pipeTestFinishJobs(fixture)
	fixture.srv.drainChunkResults()
	if len(log.chunkCoords(test)) != 0 || player.SpawnChunksSent != 0 {
		test.Fatalf("旧视野的区块包不该发出去，实际 发出=%d 计数=%d", len(log.chunkCoords(test)), player.SpawnChunksSent)
	}
	if player.chunkInflight != inflightAfterReset {
		test.Fatalf("丢弃旧结果时不该动在途数，期望 %d，实际 %d", inflightAfterReset, player.chunkInflight)
	}
}

// 换世界时，在发 ChangeDimension 的那一刻手里还没交付的旧世界区块包，不能插在它后面发给客户端。
// 这个窗口夹在「改世界名」和「重置视野」之间，所以在 ChangeDimension 发出的瞬间塞一个旧代数的结果进去。
func TestTransferDiscardsOldWorldChunkInFlight(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, false)
	var stale chunkResult = prepareChunk(chunkJob{
		Player: player,
		World:  fixture.srv.world,
		ChunkX: 0,
		ChunkZ: 0,
		Epoch:  player.viewEpoch,
	})
	var injected bool = false
	player.Session.SentHook = func(raw []byte) {
		log.raws = append(log.raws, append([]byte(nil), raw...))
		if len(raw) > 0 && raw[0] == protocol.IDChangeDimension && !injected {
			injected = true
			fixture.srv.deliverChunk(stale)
		}
	}
	if !fixture.srv.transferPlayer(player, "nether") {
		test.Fatal("夹具：应能传送到下界")
	}
	if !injected {
		test.Fatal("夹具：应发出 ChangeDimension")
	}
	if player.SpawnChunksSent != 0 {
		test.Fatalf("旧世界的区块包不该被接受，实际计数 %d", player.SpawnChunksSent)
	}
	var coords [][2]int32 = log.chunkCoords(test)
	if len(coords) != 0 {
		test.Fatalf("ChangeDimension 之后不该再发旧世界的区块，实际 %v", coords)
	}
}

// 做出错的区块包不能算已发送，不然它永远补不上；在途数和排队标记要还回去，下次移动还能重新排。
func TestChunkResultWithErrorIsNotCountedAsSent(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, false)
	pipeTestManual(fixture, 64)
	fixture.srv.tickChunkSend()
	var job chunkJob = <-fixture.srv.chunkJobs
	var key string = chunkCoordKey(job.ChunkX, job.ChunkZ)
	var inflightBefore int = player.chunkInflight
	fixture.srv.deliverChunk(chunkResult{Job: job, Err: errors.New("序列化失败")})
	var _, sent = player.SentChunks[key]
	var _, queued = player.chunkQueued[key]
	if sent || queued {
		test.Fatalf("出错的区块不该记为已发送或仍在排队，实际 已发=%v 排队=%v", sent, queued)
	}
	if player.chunkInflight != inflightBefore-1 {
		test.Fatalf("在途数应减一，期望 %d，实际 %d", inflightBefore-1, player.chunkInflight)
	}
	if len(log.chunkCoords(test)) != 0 {
		test.Fatal("出错的区块不该发出去")
	}
}

// 同一个区块的结果被交付两次（不该发生，但出了就不能重复发、重复计数）。
func TestChunkResultDeliveredTwiceIsSentOnce(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 1, false)
	var result chunkResult = prepareChunk(chunkJob{
		Player: player,
		World:  fixture.srv.world,
		ChunkX: 0,
		ChunkZ: 0,
		Epoch:  player.viewEpoch,
	})
	fixture.srv.deliverChunk(result)
	fixture.srv.deliverChunk(result)
	if len(log.chunkCoords(test)) != 1 || player.SpawnChunksSent != 1 {
		test.Fatalf("应只发 1 次、计数 1，实际 发出=%d 计数=%d", len(log.chunkCoords(test)), player.SpawnChunksSent)
	}
}

// 已经发过的区块，轮到它时直接跳过，不重复序列化、不占在途名额。
func TestChunkSendSkipsAlreadySentChunk(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 0, false)
	player.SentChunks["0,0"] = struct{}{}
	fixture.srv.tickChunkSend()
	if len(log.chunkCoords(test)) != 0 || len(player.ChunkQueue) != 0 || player.chunkInflight != 0 {
		test.Fatalf("已发过的区块应被跳过，实际 发出=%d 队列=%d 在途=%d",
			len(log.chunkCoords(test)), len(player.ChunkQueue), player.chunkInflight)
	}
	var _, queued = player.chunkQueued["0,0"]
	if queued {
		test.Fatal("跳过的区块应清掉排队标记")
	}
}

// prepareChunk 做出的是完整的 Batch：解开后是一个 FullChunkData，坐标对，数据和世界序列化出来的一致。
func TestPrepareChunkProducesDecodableBatch(test *testing.T) {
	var fixture, player, _ = pipeTestPlayer(test, 0, false)
	var result chunkResult = prepareChunk(chunkJob{Player: player, World: fixture.srv.world, ChunkX: 3, ChunkZ: -2})
	if result.Err != nil || len(result.Batch) == 0 || result.Batch[0] != protocol.IDBatch {
		test.Fatalf("应得到 Batch，实际 错误=%v 长度=%d", result.Err, len(result.Batch))
	}
	var inner, decodeErr = protocol.DecodeBatch(result.Batch[1:])
	if decodeErr != nil || len(inner) != 1 || inner[0][0] != protocol.IDFullChunkData {
		test.Fatalf("Batch 里应是 1 个 FullChunkData，实际 错误=%v 个数=%d", decodeErr, len(inner))
	}
	var stream *protocol.BinaryStream = protocol.NewStream(inner[0][1:])
	var chunkX, _ = stream.ReadVarInt()
	var chunkZ, _ = stream.ReadVarInt()
	var payload, payloadErr = stream.ReadString()
	if payloadErr != nil || chunkX != 3 || chunkZ != -2 {
		test.Fatalf("坐标应是 (3,-2)，实际 (%d,%d) 错误=%v", chunkX, chunkZ, payloadErr)
	}
	if payload != string(fixture.srv.world.NetworkSerialize(3, -2)) {
		test.Fatal("区块数据应和世界序列化出来的一致")
	}
}

// 准备区块时崩溃（比如世界是 nil）要变成一次失败的交付，不能让 worker 退出：
// worker 一退出，它手里那个区块的在途数永远减不下来，玩家的区块加载就卡死了。
func TestSafePrepareChunkTurnsPanicIntoError(test *testing.T) {
	var fixture, player, _ = pipeTestPlayer(test, 0, false)
	var broken chunkResult = safePrepareChunk(chunkJob{Player: player, World: nil, ChunkX: 1, ChunkZ: 2})
	if broken.Err == nil || len(broken.Batch) != 0 {
		test.Fatalf("崩溃应变成错误，实际 错误=%v 数据=%d 字节", broken.Err, len(broken.Batch))
	}
	var healthy chunkResult = safePrepareChunk(chunkJob{Player: player, World: fixture.srv.world, ChunkX: 1, ChunkZ: 2})
	if healthy.Err != nil || len(healthy.Batch) == 0 {
		test.Fatalf("正常任务不受影响，实际 错误=%v 数据=%d 字节", healthy.Err, len(healthy.Batch))
	}
}

// panicStore 一读盘就崩溃的存储，用来在没有 worker 池的同步路径里制造「准备区块时崩溃」。
type panicStore struct{}

func (store panicStore) LoadColumn(chunkX int32, chunkZ int32) (*world.Chunk, error) {
	panic("读盘崩溃")
}

func (store panicStore) SaveColumn(chunkX int32, chunkZ int32, column *world.Chunk) error {
	return nil
}

// 同步路径（没有 worker 池）里准备区块崩溃，也只算一次失败的交付：服务器不崩，
// 在途数和排队标记还回去，这个区块没记成已发送，下次移动还能重新排。
func TestSynchronousChunkSendSurvivesPrepareCrash(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 0, false)
	fixture.srv.world.SetStore(panicStore{})
	fixture.srv.tickChunkSend()
	var _, sent = player.SentChunks["0,0"]
	var _, queued = player.chunkQueued["0,0"]
	if sent || queued || player.chunkInflight != 0 {
		test.Fatalf("崩溃的区块不该记为已发送或仍在排队，在途应为 0，实际 已发=%v 排队=%v 在途=%d",
			sent, queued, player.chunkInflight)
	}
	if len(log.chunkCoords(test)) != 0 {
		test.Fatal("崩溃的区块不该发出去")
	}
}

// 一个坏任务不能带走 worker：它后面的好任务照样被做完。
func TestChunkWorkerSurvivesBrokenJob(test *testing.T) {
	var fixture, player, _ = pipeTestPlayer(test, 0, false)
	fixture.srv.startChunkWorkers()
	test.Cleanup(func() { close(fixture.srv.stopCh) })
	var round int = 0
	for round = 0; round < chunkWorkerCount*3; round++ {
		fixture.srv.chunkJobs <- chunkJob{Player: player, World: nil, ChunkX: int32(round), ChunkZ: 0}
	}
	fixture.srv.chunkJobs <- chunkJob{Player: player, World: fixture.srv.world, ChunkX: 100, ChunkZ: 0}
	var deadline time.Time = time.Now().Add(5 * time.Second)
	var healthyDone bool = false
	var failures int = 0
	var result chunkResult = chunkResult{}
	for !healthyDone && time.Now().Before(deadline) {
		select {
		case result = <-fixture.srv.chunkDone:
			if result.Err != nil {
				failures++
			} else if result.Job.ChunkX == 100 {
				healthyDone = true
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !healthyDone {
		test.Fatalf("坏任务之后的好任务应被做完（worker 不能退出），实际收到 %d 个失败结果", failures)
	}
}

// 起真正的 worker：区块在后台线程里做好，tick 里交付，最后一个不少地到齐，在途数回到 0。
func TestChunkWorkersDeliverEveryChunk(test *testing.T) {
	var fixture, player, log = pipeTestPlayer(test, 2, false)
	fixture.srv.startChunkWorkers()
	test.Cleanup(func() { close(fixture.srv.stopCh) })
	var deadline time.Time = time.Now().Add(5 * time.Second)
	for len(log.chunkCoords(test)) < 25 && time.Now().Before(deadline) {
		fixture.srv.tickChunkSend()
		time.Sleep(2 * time.Millisecond)
	}
	var coords [][2]int32 = log.chunkCoords(test)
	if len(coords) != 25 {
		test.Fatalf("25 个区块都该送到，实际 %d", len(coords))
	}
	var seen map[[2]int32]bool = map[[2]int32]bool{}
	var index int = 0
	for index = 0; index < len(coords); index++ {
		seen[coords[index]] = true
	}
	if len(seen) != 25 {
		test.Fatalf("不该重复，25 个坐标应各不相同，实际 %d 种", len(seen))
	}
	if player.chunkInflight != 0 || len(player.ChunkQueue) != 0 {
		test.Fatalf("到齐后在途和队列都应为 0，实际 在途=%d 队列=%d", player.chunkInflight, len(player.ChunkQueue))
	}
}

// 自动存盘在后台跑：上一次没存完不再开新的；存完放开占用，并且世界元数据真的写到了盘上。
func TestStartWorldAutosaveDoesNotOverlapAndSaves(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	fixture.srv.worldSaveBusy.Store(true)
	if fixture.srv.startWorldAutosave() {
		test.Fatal("上一次还在存，不该再开一次")
	}
	fixture.srv.worldSaveBusy.Store(false)
	if !fixture.srv.startWorldAutosave() {
		test.Fatal("空闲时应能开始")
	}
	var deadline time.Time = time.Now().Add(3 * time.Second)
	for fixture.srv.worldSaveBusy.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fixture.srv.worldSaveBusy.Load() {
		test.Fatal("存完应放开占用")
	}
	var metaPath string = storage.WorldMetaPath(fixture.srv.cfg.DataDir, "world")
	var _, statErr = os.Stat(metaPath)
	if statErr != nil {
		test.Fatalf("自动存盘应写出世界元数据 %s: %v", metaPath, statErr)
	}
}
