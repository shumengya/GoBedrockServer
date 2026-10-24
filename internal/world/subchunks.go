package world

// LoadedSubChunks 返回已经在内存里的区块中有数据的子区块编号(0~15)；区块没加载返回 nil。
// 不会触发生成或读盘，随机刻靠它挑要抽查的位置，不能因此把没人去的区块生成出来。
func (w *World) LoadedSubChunks(chunkX int32, chunkZ int32) []int {
	var key uint64 = chunkKey(chunkX, chunkZ)
	w.mu.RLock()
	var column *Chunk = w.chunks[key]
	w.mu.RUnlock()
	if column == nil {
		return nil
	}
	return column.filledSubChunks()
}

func (c *Chunk) filledSubChunks() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	var indices []int = nil
	var index int = 0
	for index = 0; index < maxSubChunks; index++ {
		if c.sub[index] != nil {
			indices = append(indices, index)
		}
	}
	return indices
}
