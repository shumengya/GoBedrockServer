package world

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// chunkWriteAttempts 写区块遇到它恰好被卸载时最多重做几次。正常一次就够，
// 要连着被卸载多次说明有人在疯狂卸载，放弃并返回 false，调用方会把真实状态发回给玩家。
const chunkWriteAttempts int = 8

// ChunkKeep 回答「这个区块要不要留在内存里」。
type ChunkKeep func(chunkX int32, chunkZ int32) bool

// withChunk 取 (cx,cz) 的区块并对它执行 act，保证 act 做完时这个区块仍是内存里的那一份。
// 拿到区块到 act 做完之间，卸载线程可能已经把它存盘并移走了，act 写进去的东西就丢在脱离世界的旧区块上；
// 发现这种情况就重新取一遍（会从盘上读回刚存的版本）再做一次，所以 act 必须能重复执行。
// 连着失败 chunkWriteAttempts 次返回 false。
func (w *World) withChunk(cx int32, cz int32, act func(column *Chunk)) bool {
	var attempt int = 0
	for attempt = 0; attempt < chunkWriteAttempts; attempt++ {
		var column *Chunk = w.chunk(cx, cz)
		act(column)
		if w.resident(cx, cz, column) {
			return true
		}
	}
	return false
}

// resident 报告 column 现在是不是 (cx,cz) 在内存里的那一份。
func (w *World) resident(cx int32, cz int32, column *Chunk) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.chunks[chunkKey(cx, cz)] == column
}

// LoadedChunkCount 内存里现在有多少个区块列。
func (w *World) LoadedChunkCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.chunks)
}

// UnloadChunks 把 keep 判为不要的区块存盘后从内存移除，返回移除了几个。
// 脏的区块必须先存成功才能移走：存失败、或者根本没有存储，就留在内存里，不丢数据。
// 一个区块出问题不影响别的，错误汇总后一起返回。没改过的区块（纯生成的）直接移走，
// 再要时按坐标重新生成，结果一样。
func (w *World) UnloadChunks(keep ChunkKeep) (int, error) {
	w.persistMu.Lock()
	defer w.persistMu.Unlock()
	w.mu.RLock()
	var keys []uint64 = slices.Sorted(maps.Keys(w.chunks))
	w.mu.RUnlock()
	var removed int = 0
	var failures []error = nil
	var index int = 0
	for index = 0; index < len(keys); index++ {
		var chunkX, chunkZ = splitChunkKey(keys[index])
		if keep(chunkX, chunkZ) {
			continue
		}
		var gone, unloadErr = w.unloadOne(keys[index], chunkX, chunkZ)
		if unloadErr != nil {
			failures = append(failures, unloadErr)
		}
		if gone {
			removed++
		}
	}
	return removed, errors.Join(failures...)
}

// unloadOne 卸载一个区块。返回是否真的移走了。
func (w *World) unloadOne(key uint64, chunkX int32, chunkZ int32) (bool, error) {
	w.mu.RLock()
	var column *Chunk = w.chunks[key]
	var store ColumnStore = w.store
	w.mu.RUnlock()
	if column == nil {
		return false, nil
	}
	if column.Dirty() {
		if store == nil {
			return false, nil
		}
		column.ClearDirty()
		var saveErr error = store.SaveColumn(chunkX, chunkZ, column)
		if saveErr != nil {
			column.MarkDirty()
			return false, fmt.Errorf("卸载前保存区块 (%d,%d): %w", chunkX, chunkZ, saveErr)
		}
	}
	// 存盘到这里之间可能又有人改了它：拿着世界写锁再看一眼，脏了就留下，下一轮再说。
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.chunks[key] != column || column.Dirty() {
		return false, nil
	}
	delete(w.chunks, key)
	column.mu.Lock()
	w.lightBoundary(chunkX, chunkZ, column, false)
	column.mu.Unlock()
	return true, nil
}
