package world

// chestRank 双箱里哪一半排前面的依据：照参考端按 x + (z << 15) 比较。
// 两半水平相邻，这个值不会相等。
func chestRank(tile Tile) int64 {
	return int64(tile.X) + int64(tile.Z)<<15
}

// OrderChestHalves 把双箱的两半按窗口里的顺序排好：窗口前 27 格是返回的第一个，后 27 格是第二个。
// 排序照参考端：rank 大的在前。顺序必须只取决于两个箱子的位置，不取决于点的是哪一半，
// 否则两个人点不同的半边会看到前后颠倒的窗口。
func OrderChestHalves(first Tile, second Tile) (Tile, Tile) {
	if chestRank(second) > chestRank(first) {
		return second, first
	}
	return first, second
}

// PairChests 把水平相邻的两个箱子配成大箱子。两个都得是还没配对的箱子，否则什么都不改，返回 false。
func (w *World) PairChests(firstX int32, posY int32, firstZ int32, secondX int32, secondZ int32) bool {
	var spanX int32 = firstX - secondX
	var spanZ int32 = firstZ - secondZ
	if spanX < 0 {
		spanX = -spanX
	}
	if spanZ < 0 {
		spanZ = -spanZ
	}
	if spanX+spanZ != 1 {
		return false
	}
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	var firstKey string = tileKey(firstX, posY, firstZ)
	var secondKey string = tileKey(secondX, posY, secondZ)
	var first, firstFound = w.tiles[firstKey]
	var second, secondFound = w.tiles[secondKey]
	if !firstFound || !secondFound {
		return false
	}
	if first.ID != TileChest || second.ID != TileChest || first.Paired || second.Paired {
		return false
	}
	first.Paired = true
	first.PairX = secondX
	first.PairZ = secondZ
	second.Paired = true
	second.PairX = firstX
	second.PairZ = firstZ
	w.tiles[firstKey] = first
	w.tiles[secondKey] = second
	w.tilesDirty = true
	return true
}

// ChestPartner 返回大箱子的另一半。两边必须互相指着对方才算数：只有一边记着配对
// （存档残缺、另一半被别的东西覆盖）就当没配对，要不要修由调用方决定。
func (w *World) ChestPartner(posX int32, posY int32, posZ int32) (Tile, bool) {
	w.tileMu.RLock()
	defer w.tileMu.RUnlock()
	var tile, found = w.tiles[tileKey(posX, posY, posZ)]
	if !found || tile.ID != TileChest || !tile.Paired {
		return Tile{}, false
	}
	var partner, partnerFound = w.tiles[tileKey(tile.PairX, posY, tile.PairZ)]
	if !partnerFound || partner.ID != TileChest || !partner.Paired {
		return Tile{}, false
	}
	if partner.PairX != posX || partner.PairZ != posZ {
		return Tile{}, false
	}
	return partner.clone(), true
}

// UnpairChest 解除一个箱子的配对。另一半如果也指着它，一并解除；指着别处的不碰。
// 返回是否真的改了东西。
func (w *World) UnpairChest(posX int32, posY int32, posZ int32) bool {
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	return w.unpairChestLocked(posX, posY, posZ)
}

// unpairChestLocked 是 UnpairChest 的内核，调用方必须已持有 tileMu。
func (w *World) unpairChestLocked(posX int32, posY int32, posZ int32) bool {
	var key string = tileKey(posX, posY, posZ)
	var tile, found = w.tiles[key]
	if !found || tile.ID != TileChest || !tile.Paired {
		return false
	}
	var partnerKey string = tileKey(tile.PairX, posY, tile.PairZ)
	var partner, partnerFound = w.tiles[partnerKey]
	if partnerFound && partner.ID == TileChest && partner.Paired && partner.PairX == posX && partner.PairZ == posZ {
		partner.Paired = false
		partner.PairX = 0
		partner.PairZ = 0
		w.tiles[partnerKey] = partner
	}
	tile.Paired = false
	tile.PairX = 0
	tile.PairZ = 0
	w.tiles[key] = tile
	w.tilesDirty = true
	return true
}
