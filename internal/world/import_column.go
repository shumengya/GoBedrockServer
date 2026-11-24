package world

// ImportColumn 接收已换为 XZY 顺序的节，非空项依次含 4096 个 ID 和 2048 字节 meta。
// 高度按本服遮光规则重建，入口与普通区块共用相同的存盘和加载后光照流程。
func ImportColumn(sections [16][]byte, biomes []byte) *Chunk {
	var column = newChunk()
	copy(column.biomes[:], biomes)
	var index = 0
	for index = 0; index < len(sections); index++ {
		var data = sections[index]
		if data == nil {
			continue
		}
		var section = column.subAt(index)
		copy(section.ids[:], data[:4096])
		copy(section.data[:], data[4096:])
	}
	for index = 0; index < 256; index++ {
		column.heightMap[index] = column.skyHeight(index&15, 255, index>>4)
	}
	return column
}
