package world

// WithChunkForTest 把 withChunk 导出给黑盒测试：act 就是「拿到区块之后、写进去之前」的插入点，
// 测试在里面触发卸载，就能确定地复现这个只有几条指令宽的竞态窗口。只在测试编译时存在。
func (w *World) WithChunkForTest(chunkX int32, chunkZ int32, act func(column *Chunk)) bool {
	return w.withChunk(chunkX, chunkZ, act)
}
