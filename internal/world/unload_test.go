package world_test

import "errors"
import "fmt"
import "strings"
import "sync"
import "sync/atomic"
import "testing"

import "go-mc-server/internal/world"

// memoryStore 是放在内存里的区块存储，能让存盘失败、能在「编码完、写入前」插一段代码，
// 用来精确制造「存盘期间区块又被改了」这种时序。
type memoryStore struct {
	mu          sync.Mutex
	saved       map[[2]int32][]byte
	failKeys    map[[2]int32]bool
	afterEncode func(chunkX int32, chunkZ int32)
	saveCount   int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{saved: map[[2]int32][]byte{}, failKeys: map[[2]int32]bool{}}
}

func (store *memoryStore) LoadColumn(chunkX int32, chunkZ int32) (*world.Chunk, error) {
	store.mu.Lock()
	var raw []byte = store.saved[[2]int32{chunkX, chunkZ}]
	store.mu.Unlock()
	if raw == nil {
		return nil, nil
	}
	var column, decodeErr = world.DecodeDisk(raw)
	if decodeErr != nil {
		return nil, fmt.Errorf("解码区块: %w", decodeErr)
	}
	return column, nil
}

func (store *memoryStore) SaveColumn(chunkX int32, chunkZ int32, column *world.Chunk) error {
	store.mu.Lock()
	var shouldFail bool = store.failKeys[[2]int32{chunkX, chunkZ}]
	var hook func(chunkX int32, chunkZ int32) = store.afterEncode
	store.mu.Unlock()
	if shouldFail {
		return errors.New("磁盘满了")
	}
	var raw []byte = column.EncodeDisk()
	if hook != nil {
		hook(chunkX, chunkZ)
	}
	store.mu.Lock()
	store.saved[[2]int32{chunkX, chunkZ}] = raw
	store.saveCount++
	store.mu.Unlock()
	return nil
}

func (store *memoryStore) count() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.saveCount
}

var keepNone world.ChunkKeep = func(chunkX int32, chunkZ int32) bool { return false }

func unloadTestWorld(store *memoryStore) *world.World {
	var target *world.World = world.New("unload")
	if store != nil {
		target.SetStore(store)
	}
	return target
}

func unloadTestSet(test *testing.T, target *world.World, posX int32, posY int32, posZ int32, id byte) {
	if !target.SetBlock(posX, posY, posZ, id, 0) {
		test.Fatalf("写方块 (%d,%d,%d) 失败", posX, posY, posZ)
	}
}

// 没人要的区块从内存里移走，要的留下；这些区块都没改过，不需要存盘。
func TestUnloadChunksRemovesOnlyUnwanted(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	var chunkX int32 = 0
	for chunkX = 0; chunkX < 3; chunkX++ {
		target.GetBlock(chunkX*16, 1, 0)
	}
	if target.LoadedChunkCount() != 3 {
		test.Fatalf("夹具：应加载 3 个区块，实际 %d", target.LoadedChunkCount())
	}
	var keepMiddle world.ChunkKeep = func(chunkX int32, chunkZ int32) bool { return chunkX == 1 && chunkZ == 0 }
	var removed, unloadErr = target.UnloadChunks(keepMiddle)
	if unloadErr != nil {
		test.Fatalf("卸载出错: %v", unloadErr)
	}
	if removed != 2 || target.LoadedChunkCount() != 1 {
		test.Fatalf("期望移走 2 个、剩 1 个，实际 移走=%d 剩=%d", removed, target.LoadedChunkCount())
	}
	if store.count() != 0 {
		test.Fatalf("没改过的区块不该存盘，实际存了 %d 次", store.count())
	}
	var _, _, stillLoaded = target.Peek(16, 1, 0)
	if !stillLoaded {
		test.Fatal("要保留的区块不该被移走")
	}
}

// 改过的区块先存盘再移走；再用到它时从盘上读回来，改动还在。
func TestUnloadChunksSavesDirtyChunkFirst(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	var removed, unloadErr = target.UnloadChunks(keepNone)
	if unloadErr != nil || removed != 1 {
		test.Fatalf("期望移走 1 个，实际 移走=%d 错误=%v", removed, unloadErr)
	}
	if store.count() != 1 {
		test.Fatalf("改过的区块应存盘 1 次，实际 %d", store.count())
	}
	var _, _, loaded = target.Peek(5, 10, 5)
	if loaded {
		test.Fatal("移走之后不该还在内存里")
	}
	if target.BlockID(5, 10, 5) != 41 {
		test.Fatalf("重新读回来应是存盘时的 41，实际 %d", target.BlockID(5, 10, 5))
	}
}

// 没有存储时，改过的区块没处可存，不能移走，否则改动就丢了。
func TestUnloadChunksKeepsDirtyChunkWithoutStore(test *testing.T) {
	var target *world.World = unloadTestWorld(nil)
	unloadTestSet(test, target, 5, 10, 5, 41)
	var removed, unloadErr = target.UnloadChunks(keepNone)
	if unloadErr != nil || removed != 0 {
		test.Fatalf("没有存储时不该移走，实际 移走=%d 错误=%v", removed, unloadErr)
	}
	if target.BlockID(5, 10, 5) != 41 {
		test.Fatalf("改动应还在，实际 %d", target.BlockID(5, 10, 5))
	}
}

// 存盘失败的区块留在内存里并且仍然算脏的，别的区块照常卸载；盘恢复后下一轮能存上。
func TestUnloadChunksSaveFailureKeepsChunk(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	unloadTestSet(test, target, 20, 10, 5, 42)
	store.failKeys[[2]int32{0, 0}] = true
	var removed, unloadErr = target.UnloadChunks(keepNone)
	if removed != 1 || target.LoadedChunkCount() != 1 {
		test.Fatalf("另一个区块应照常卸载，实际 移走=%d 剩=%d", removed, target.LoadedChunkCount())
	}
	if unloadErr == nil || !strings.Contains(unloadErr.Error(), "(0,0)") {
		test.Fatalf("应返回带坐标的错误，实际 %v", unloadErr)
	}
	if target.BlockID(5, 10, 5) != 41 {
		test.Fatalf("存失败的区块改动应还在内存里，实际 %d", target.BlockID(5, 10, 5))
	}
	delete(store.failKeys, [2]int32{0, 0})
	removed, unloadErr = target.UnloadChunks(keepNone)
	if unloadErr != nil || removed != 1 {
		test.Fatalf("盘恢复后应能卸载，实际 移走=%d 错误=%v", removed, unloadErr)
	}
	if target.BlockID(5, 10, 5) != 41 {
		test.Fatalf("重试存上之后读回来应是 41，实际 %d：脏标记在失败时被弄丢了", target.BlockID(5, 10, 5))
	}
}

// 存盘的中途区块又被人改了：这次不能移走（刚存的是旧的），下一轮再存再移。
func TestUnloadChunksKeepsChunkModifiedDuringSave(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	var fired atomic.Bool
	store.afterEncode = func(chunkX int32, chunkZ int32) {
		if fired.CompareAndSwap(false, true) {
			target.SetBlock(5, 11, 5, 42, 0)
		}
	}
	var removed, unloadErr = target.UnloadChunks(keepNone)
	if unloadErr != nil || removed != 0 {
		test.Fatalf("存盘期间被改过，这次不该移走，实际 移走=%d 错误=%v", removed, unloadErr)
	}
	removed, unloadErr = target.UnloadChunks(keepNone)
	if unloadErr != nil || removed != 1 {
		test.Fatalf("下一轮应存上并移走，实际 移走=%d 错误=%v", removed, unloadErr)
	}
	if target.BlockID(5, 10, 5) != 41 || target.BlockID(5, 11, 5) != 42 {
		test.Fatalf("两次修改都该在，实际 %d / %d", target.BlockID(5, 10, 5), target.BlockID(5, 11, 5))
	}
}

// SaveDirty 存盘期间区块又被改了，修改不能连同脏标记一起被清掉，否则永远不落盘。
func TestSaveDirtyKeepsWriteMadeDuringSave(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	var fired atomic.Bool
	store.afterEncode = func(chunkX int32, chunkZ int32) {
		if fired.CompareAndSwap(false, true) {
			target.SetBlock(5, 11, 5, 42, 0)
		}
	}
	var firstErr error = target.SaveDirty()
	if firstErr != nil {
		test.Fatalf("第一次存盘出错: %v", firstErr)
	}
	var secondErr error = target.SaveDirty()
	if secondErr != nil {
		test.Fatalf("第二次存盘出错: %v", secondErr)
	}
	if store.count() != 2 {
		test.Fatalf("存盘期间的修改应让区块仍是脏的，第二次要再存一遍，期望共存 2 次，实际 %d", store.count())
	}
	var fresh *world.World = unloadTestWorld(store)
	if fresh.BlockID(5, 11, 5) != 42 {
		test.Fatalf("重新打开世界应看到存盘期间的那次修改，实际 %d", fresh.BlockID(5, 11, 5))
	}
}

// 一个区块存失败不影响别的区块，且失败的那个下一次还会再存。
func TestSaveDirtyFailureKeepsOthersAndRetries(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	unloadTestSet(test, target, 20, 10, 5, 42)
	store.failKeys[[2]int32{0, 0}] = true
	var saveErr error = target.SaveDirty()
	if saveErr == nil || !strings.Contains(saveErr.Error(), "(0,0)") {
		test.Fatalf("应返回带坐标的错误，实际 %v", saveErr)
	}
	if store.count() != 1 {
		test.Fatalf("另一个区块应照常存上，实际存了 %d 次", store.count())
	}
	delete(store.failKeys, [2]int32{0, 0})
	var retryErr error = target.SaveDirty()
	if retryErr != nil {
		test.Fatalf("重试出错: %v", retryErr)
	}
	if store.count() != 2 {
		test.Fatalf("失败的区块应在下一次补存，期望共 2 次，实际 %d", store.count())
	}
}

// 很多协程同时要一个还没加载的区块，最后内存里只有一份，大家看到的内容一致。
func TestConcurrentFirstAccessYieldsOneChunk(test *testing.T) {
	var target *world.World = unloadTestWorld(nil)
	var group sync.WaitGroup = sync.WaitGroup{}
	var results [16]byte
	var worker int = 0
	for worker = 0; worker < len(results); worker++ {
		var slot int = worker // 闭包里用循环变量前先拷贝
		group.Go(func() {
			results[slot] = target.BlockID(3, 4, 3)
		})
	}
	group.Wait()
	if target.LoadedChunkCount() != 1 {
		test.Fatalf("内存里应只有 1 个区块，实际 %d", target.LoadedChunkCount())
	}
	var index int = 0
	for index = 0; index < len(results); index++ {
		if results[index] != world.BlockGrass {
			test.Fatalf("第 %d 个协程读到 %d，期望 %d", index, results[index], world.BlockGrass)
		}
	}
}

// 拿到区块之后、写进去之前它被卸载了：写进旧区块的东西会丢。withChunk 必须发现这件事，
// 从盘上读回区块在新的那份上重做一遍。这个窗口只有几条指令宽，靠压力测试撞不到，所以在插入点里手动触发卸载。
func TestWithChunkRedoesWriteWhenChunkWasUnloaded(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	unloadTestSet(test, target, 5, 10, 5, 41)
	var calls int = 0
	var done bool = target.WithChunkForTest(0, 0, func(column *world.Chunk) {
		calls++
		if calls == 1 {
			var removed, unloadErr = target.UnloadChunks(keepNone)
			if unloadErr != nil || removed != 1 {
				test.Fatalf("夹具：应卸载掉这个区块，实际 移走=%d 错误=%v", removed, unloadErr)
			}
		}
		column.SetBlock(6, 10, 5, 42, 0)
	})
	if !done || calls != 2 {
		test.Fatalf("期望重做一次后成功，实际 成功=%v 执行次数=%d", done, calls)
	}
	if target.BlockID(6, 10, 5) != 42 || target.BlockID(5, 10, 5) != 41 {
		test.Fatalf("新旧两次写入都该在，实际 %d / %d", target.BlockID(6, 10, 5), target.BlockID(5, 10, 5))
	}
	var saveErr error = target.SaveDirty()
	if saveErr != nil {
		test.Fatalf("存盘出错: %v", saveErr)
	}
	var fresh *world.World = unloadTestWorld(store)
	if fresh.BlockID(6, 10, 5) != 42 {
		test.Fatalf("重新打开世界应看到重做的那次写入，实际 %d", fresh.BlockID(6, 10, 5))
	}
}

// 区块一直被卸载时不能死循环：重做有上限，用完返回 false，调用方会把真实状态发回给玩家。
func TestWithChunkGivesUpWhenChunkKeepsUnloading(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	var calls int = 0
	var done bool = target.WithChunkForTest(0, 0, func(column *world.Chunk) {
		calls++
		var _, unloadErr = target.UnloadChunks(keepNone)
		if unloadErr != nil {
			test.Fatalf("夹具：卸载出错: %v", unloadErr)
		}
		column.SetBlock(6, 10, 5, 42, 0)
	})
	if done {
		test.Fatal("每次都被卸载，不该报告成功")
	}
	if calls != 8 {
		test.Fatalf("期望重做到上限 8 次，实际 %d", calls)
	}
}

// 一边疯狂写方块、一边疯狂卸载，写下去的每一个方块最后都得在盘上：不能写进刚被移走的区块里就丢了。
func TestSetBlockSurvivesConcurrentUnload(test *testing.T) {
	var store *memoryStore = newMemoryStore()
	var target *world.World = unloadTestWorld(store)
	var stop atomic.Bool
	var unloadFailure atomic.Value
	var unloader sync.WaitGroup = sync.WaitGroup{}
	unloader.Go(func() {
		for !stop.Load() {
			var _, unloadErr = target.UnloadChunks(keepNone)
			if unloadErr != nil {
				unloadFailure.Store(unloadErr.Error())
			}
		}
	})
	var writers sync.WaitGroup = sync.WaitGroup{}
	var writer int = 0
	for writer = 0; writer < 4; writer++ {
		var lane int32 = int32(writer) // 闭包里用循环变量前先拷贝
		writers.Go(func() {
			var step int32 = 0
			for step = 0; step < 400; step++ {
				target.SetBlock(lane*16+step%16, 10+step/16, 3, byte(1+step%100), 0)
			}
		})
	}
	writers.Wait()
	stop.Store(true)
	unloader.Wait()
	var failure any = unloadFailure.Load()
	if failure != nil {
		test.Fatalf("卸载协程出错: %v", failure)
	}
	var finalErr error = target.SaveDirty()
	if finalErr != nil {
		test.Fatalf("最后存盘出错: %v", finalErr)
	}
	var fresh *world.World = unloadTestWorld(store)
	var lane int32 = 0
	for lane = 0; lane < 4; lane++ {
		var step int32 = 0
		for step = 0; step < 400; step++ {
			var got byte = fresh.BlockID(lane*16+step%16, 10+step/16, 3)
			var want byte = byte(1 + step%100)
			if got != want {
				test.Fatalf("(%d,%d,3) 期望 %d，实际 %d：写入在卸载时丢了", lane*16+step%16, 10+step/16, want, got)
			}
		}
	}
}
