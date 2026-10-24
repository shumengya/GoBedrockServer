package world_test

import "bytes"
import "slices"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/world"

type slotCountCase struct {
	Name  string
	ID    string
	Slots int
	Known bool
}

// 有物品栏的方块实体返回格数；床和告示牌认得但没有物品栏；没见过的一律不认得。
func TestSlotCountAndKnownTile(test *testing.T) {
	var cases []slotCountCase = []slotCountCase{
		{Name: "箱子", ID: world.TileChest, Slots: 27, Known: true},
		{Name: "熔炉", ID: world.TileFurnace, Slots: 3, Known: true},
		{Name: "床", ID: world.TileBed, Slots: 0, Known: true},
		{Name: "告示牌", ID: world.TileSign, Slots: 0, Known: true},
		{Name: "没见过的旗帜", ID: "Banner", Slots: 0, Known: false},
		{Name: "空串", ID: "", Slots: 0, Known: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item slotCountCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			if world.SlotCount(item.ID) != item.Slots {
				sub.Fatalf("格数期望 %d，实际 %d", item.Slots, world.SlotCount(item.ID))
			}
			if world.KnownTile(item.ID) != item.Known {
				sub.Fatalf("认得期望 %v，实际 %v", item.Known, world.KnownTile(item.ID))
			}
		})
	}
}

// 写进去的方块实体按种类补满格数：熔炉 3 格、箱子 27 格，已有的物品保留；床没有物品栏，不补。
func TestSetTilePadsToSlotCount(test *testing.T) {
	var target *world.World = world.New("tiles")
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 1, Y: 5, Z: 2})
	target.SetTile(world.Tile{ID: world.TileChest, X: 2, Y: 5, Z: 2, Items: []protocol.Item{{ID: 1, Count: 5}}})
	target.SetTile(world.NewBedTile(3, 5, 2, 14))
	var furnace, furnaceFound = target.TileAt(1, 5, 2)
	if !furnaceFound || len(furnace.Items) != 3 {
		test.Fatalf("熔炉应有 3 格，实际 找到=%v 格数=%d", furnaceFound, len(furnace.Items))
	}
	var chest, chestFound = target.TileAt(2, 5, 2)
	if !chestFound || len(chest.Items) != 27 || chest.Items[0].ID != 1 || chest.Items[0].Count != 5 {
		test.Fatalf("箱子应有 27 格且第 0 格保留，实际 找到=%v 格数=%d", chestFound, len(chest.Items))
	}
	var bed, bedFound = target.TileAt(3, 5, 2)
	if !bedFound || len(bed.Items) != 0 {
		test.Fatalf("床没有物品栏，实际 找到=%v 格数=%d", bedFound, len(bed.Items))
	}
}

// 写一格：槽位在范围内才写；方块实体不存在也不写；别的格子不受影响。
func TestSetTileSlot(test *testing.T) {
	var target *world.World = world.New("tiles")
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 1, Y: 5, Z: 2})
	var coal protocol.Item = protocol.Item{ID: 263, Count: 8}
	if !target.SetTileSlot(1, 5, 2, 1, coal) {
		test.Fatal("槽位 1 在范围内，应该写成功")
	}
	if target.SetTileSlot(1, 5, 2, 3, coal) || target.SetTileSlot(1, 5, 2, -1, coal) {
		test.Fatal("槽位 3 和 -1 越界，不该写成功")
	}
	if target.SetTileSlot(9, 9, 9, 0, coal) {
		test.Fatal("方块实体不存在，不该写成功")
	}
	var furnace, _ = target.TileAt(1, 5, 2)
	if furnace.Items[1].ID != 263 || furnace.Items[1].Count != 8 || !furnace.Items[0].IsAir() || !furnace.Items[2].IsAir() {
		test.Fatalf("只有第 1 格应有煤，实际 %+v", furnace.Items)
	}
}

// UpdateTile 在副本上改，返回 true 才算数：返回 false 时对副本做的任何修改（包括物品栏）都不能漏到真数据里。
func TestUpdateTileKeepsOriginalWhenNotChanged(test *testing.T) {
	var target *world.World = world.New("tiles")
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 1, Y: 5, Z: 2})
	var changed bool = target.UpdateTile(1, 5, 2, func(tile *world.Tile) bool {
		tile.BurnTime = 300
		return true
	})
	if !changed {
		test.Fatal("返回 true 应该算改成功")
	}
	var skipped bool = target.UpdateTile(1, 5, 2, func(tile *world.Tile) bool {
		tile.BurnTime = 999
		tile.Items[0] = protocol.Item{ID: 1, Count: 1}
		return false
	})
	if skipped {
		test.Fatal("返回 false 不该算改成功")
	}
	var furnace, _ = target.TileAt(1, 5, 2)
	if furnace.BurnTime != 300 || !furnace.Items[0].IsAir() {
		test.Fatalf("返回 false 的修改不该生效，实际 BurnTime=%d 第0格=%+v", furnace.BurnTime, furnace.Items[0])
	}
	var called bool = false
	var missing bool = target.UpdateTile(9, 9, 9, func(tile *world.Tile) bool {
		called = true
		return true
	})
	if missing || called {
		test.Fatalf("方块实体不存在不该调用回调，实际 结果=%v 调用=%v", missing, called)
	}
}

// 按种类列出坐标，顺序固定：先 X，再 Y，再 Z。
func TestTilePositionsSortedAndFiltered(test *testing.T) {
	var target *world.World = world.New("tiles")
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 5, Y: 1, Z: 1})
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 1, Y: 2, Z: 3})
	target.SetTile(world.Tile{ID: world.TileFurnace, X: 1, Y: 1, Z: 9})
	target.SetTile(world.Tile{ID: world.TileChest, X: 0, Y: 0, Z: 0})
	var expect [][3]int32 = [][3]int32{{1, 1, 9}, {1, 2, 3}, {5, 1, 1}}
	var actual [][3]int32 = target.TilePositions(world.TileFurnace)
	if !slices.Equal(actual, expect) {
		test.Fatalf("期望 %v，实际 %v", expect, actual)
	}
	if len(target.TilePositions("Banner")) != 0 {
		test.Fatal("没有的种类应返回空")
	}
}

// 读档时认得的留下并补满格数，不认得的丢掉，不能让一个没见过的方块实体卡住整个世界。
func TestLoadTilesKeepsKnownAndDropsUnknown(test *testing.T) {
	var target *world.World = world.New("tiles")
	target.LoadTiles([]world.Tile{
		{ID: "Banner", X: 1, Y: 2, Z: 3},
		{ID: world.TileSign, X: 4, Y: 5, Z: 6, Lines: [4]string{"你好"}},
		{ID: world.TileFurnace, X: 7, Y: 8, Z: 9},
	})
	var _, bannerFound = target.TileAt(1, 2, 3)
	if bannerFound {
		test.Fatal("没见过的方块实体应被丢掉")
	}
	var sign, signFound = target.TileAt(4, 5, 6)
	if !signFound || sign.Lines[0] != "你好" {
		test.Fatalf("告示牌应保留，实际 找到=%v 第一行=%q", signFound, sign.Lines[0])
	}
	var furnace, furnaceFound = target.TileAt(7, 8, 9)
	if !furnaceFound || len(furnace.Items) != 3 {
		test.Fatalf("熔炉应保留并补满 3 格，实际 找到=%v 格数=%d", furnaceFound, len(furnace.Items))
	}
}

// 发给客户端的 NBT：熔炉带烧制进度，告示牌带四行字，配成大箱子的箱子带另一半坐标。
func TestTileSpawnNBT(test *testing.T) {
	var furnace world.Tile = world.Tile{ID: world.TileFurnace, X: 1, Y: 5, Z: 2, BurnTime: 300, CookTime: 20}
	if !bytes.Equal(furnace.SpawnNBT(), nbt.SpawnFurnace(1, 5, 2, 300, 20)) {
		test.Fatalf("熔炉的 NBT 不对: %x", furnace.SpawnNBT())
	}
	var sign world.Tile = world.Tile{ID: world.TileSign, X: 1, Y: 5, Z: 2, Lines: [4]string{"甲", "乙", "", ""}}
	if !bytes.Equal(sign.SpawnNBT(), nbt.SpawnSign(1, 5, 2, [4]string{"甲", "乙", "", ""})) {
		test.Fatalf("告示牌的 NBT 不对: %x", sign.SpawnNBT())
	}
	var paired world.Tile = world.Tile{ID: world.TileChest, X: 1, Y: 5, Z: 2, Paired: true, PairX: 3, PairZ: 2}
	var single world.Tile = world.Tile{ID: world.TileChest, X: 1, Y: 5, Z: 2}
	if !bytes.Equal(paired.SpawnNBT(), nbt.SpawnPairedChest(1, 5, 2, 3, 2)) {
		test.Fatalf("配对箱子的 NBT 不对: %x", paired.SpawnNBT())
	}
	if !bytes.Equal(single.SpawnNBT(), nbt.SpawnChest(1, 5, 2)) || bytes.Equal(single.SpawnNBT(), paired.SpawnNBT()) {
		test.Fatalf("没配对的箱子应是普通箱子的 NBT: %x", single.SpawnNBT())
	}
}
