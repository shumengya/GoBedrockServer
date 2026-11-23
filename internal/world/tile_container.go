package world

import "cmp"
import "slices"

import "go-mc-server/internal/protocol"

const BlockFurnace byte = 61
const BlockBurningFurnace byte = 62
const BlockEnderChest byte = 130

// SlotCount 这种方块实体有几格物品栏，没有物品栏的返回 0。
func SlotCount(id string) int {
	switch id {
	case TileChest:
		return ChestSize
	case TileFurnace:
		return FurnaceSize
	case TileHopper:
		return HopperSize
	case TileFlowerPot, TileItemFrame:
		// 花盆和展示框都只装一件东西，借用一格物品栏存它，配对、存盘、掉落都走现成的路。
		return 1
	}
	return 0
}

// KnownTile 读档时认得的方块实体种类，别的一律丢弃。
func KnownTile(id string) bool {
	return SlotCount(id) > 0 || id == TileBed || id == TileSign || id == TileEnderChest || id == TileSkull
}

func padItems(items []protocol.Item, size int) []protocol.Item {
	var padded []protocol.Item = make([]protocol.Item, size)
	copy(padded, items)
	return padded
}

// UpdateTile 在持有方块实体锁的情况下原地修改一个方块实体。
// change 返回 true 才算真的改了，才会标脏；方块实体不存在也返回 false。
func (w *World) UpdateTile(posX int32, posY int32, posZ int32, change func(tile *Tile) bool) bool {
	w.tileMu.Lock()
	defer w.tileMu.Unlock()
	var key string = tileKey(posX, posY, posZ)
	var tile, found = w.tiles[key]
	if !found {
		return false
	}
	tile = tile.clone()
	if !change(&tile) {
		return false
	}
	w.tiles[key] = tile
	w.tilesDirty = true
	return true
}

// SetTileSlot 写方块实体物品栏里的一格，槽位越界或方块实体不存在返回 false。
func (w *World) SetTileSlot(posX int32, posY int32, posZ int32, slot int, item protocol.Item) bool {
	return w.UpdateTile(posX, posY, posZ, func(tile *Tile) bool {
		if slot < 0 || slot >= len(tile.Items) {
			return false
		}
		tile.Items[slot] = item
		return true
	})
}

// TilePositions 返回某种方块实体的所有坐标，按 X、Y、Z 排序，遍历顺序是确定的。
func (w *World) TilePositions(id string) [][3]int32 {
	w.tileMu.RLock()
	defer w.tileMu.RUnlock()
	var positions [][3]int32 = nil
	for _, tile := range w.tiles {
		if tile.ID == id {
			positions = append(positions, [3]int32{tile.X, tile.Y, tile.Z})
		}
	}
	slices.SortFunc(positions, func(left [3]int32, right [3]int32) int {
		return cmp.Or(cmp.Compare(left[0], right[0]), cmp.Compare(left[1], right[1]), cmp.Compare(left[2], right[2]))
	})
	return positions
}

// NewFurnaceTile 一个空的熔炉：输入、燃料、产物三格都是空的，没在烧。
func NewFurnaceTile(posX int32, posY int32, posZ int32) Tile {
	return Tile{ID: TileFurnace, X: posX, Y: posY, Z: posZ, Items: make([]protocol.Item, FurnaceSize)}
}

// NewEnderChestTile 末影箱的方块实体只负责让客户端把箱子画出来，没有物品栏：
// 里面的东西属于玩家，不属于这个方块。
func NewEnderChestTile(posX int32, posY int32, posZ int32) Tile {
	return Tile{ID: TileEnderChest, X: posX, Y: posY, Z: posZ}
}

// NewFlowerPotTile 一个空花盆。种进去的东西占物品栏第 0 格。
func NewFlowerPotTile(posX int32, posY int32, posZ int32) Tile {
	return Tile{ID: TileFlowerPot, X: posX, Y: posY, Z: posZ, Items: make([]protocol.Item, 1)}
}

// NewSkullTile 一颗头颅。skullType 是种类，rotation 只在立着放时有用。
func NewSkullTile(posX int32, posY int32, posZ int32, skullType byte, rotation byte) Tile {
	return Tile{
		ID:        TileSkull,
		X:         posX,
		Y:         posY,
		Z:         posZ,
		SkullType: skullType,
		Rotation:  rotation,
	}
}

// NewItemFrameTile 一个空展示框。
func NewItemFrameTile(posX int32, posY int32, posZ int32) Tile {
	return Tile{ID: TileItemFrame, X: posX, Y: posY, Z: posZ, Items: make([]protocol.Item, 1)}
}

// 漏斗的五格物品随方块实体存档。
func NewHopperTile(posX int32, posY int32, posZ int32) Tile {
	return Tile{ID: TileHopper, X: posX, Y: posY, Z: posZ, Items: make([]protocol.Item, HopperSize)}
}
