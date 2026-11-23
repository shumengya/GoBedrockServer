package storage_test

import "os"
import "bytes"
import "strings"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

func sameItem(left protocol.Item, right protocol.Item) bool {
	return left.ID == right.ID && left.Meta == right.Meta && left.Count == right.Count && bytes.Equal(left.NBT, right.NBT)
}

// 熔炉、配对的箱子、告示牌、床存盘再读回来，每个字段都要原样回来。
func TestTilesRoundTrip(test *testing.T) {
	var directory string = test.TempDir()
	var furnaceItems []protocol.Item = []protocol.Item{{ID: 4, Count: 8}, {ID: 263, Count: 3}, {ID: 1, Count: 2}}
	var chestItems []protocol.Item = make([]protocol.Item, 27)
	chestItems[0] = protocol.Item{ID: 1, Meta: 2, Count: 5}
	var saved []world.Tile = []world.Tile{
		{ID: world.TileFurnace, X: 1, Y: 5, Z: 2, Items: furnaceItems, BurnTime: 120, CookTime: 40, MaxTime: 300},
		{ID: world.TileChest, X: 2, Y: 5, Z: 2, Items: chestItems, Paired: true, PairX: 3, PairZ: 2},
		{ID: world.TileSign, X: 4, Y: 5, Z: 2, Lines: [4]string{"第一行", "", "第三行", ""}},
		{ID: world.TileBed, X: 5, Y: 5, Z: 2, Color: 14},
		world.NewSkullTile(6, 5, 2, 4, 13),
		{ID: world.TileItemFrame, X: 7, Y: 5, Z: 2, Rotation: 7, Items: []protocol.Item{{ID: 267, Meta: 17, Count: 1, NBT: []byte{10, 0, 0, 0}}}},
		{ID: world.TileFlowerPot, X: 8, Y: 5, Z: 2, Items: []protocol.Item{{ID: 6, Meta: 2, Count: 1}}},
		world.NewHopperTile(9, 5, 2),
	}
	var saveErr error = storage.SaveTiles(directory, "w", storage.TilesFromWorld(saved))
	if saveErr != nil {
		test.Fatalf("存盘失败: %v", saveErr)
	}
	var records, loadErr = storage.LoadTiles(directory, "w")
	if loadErr != nil {
		test.Fatalf("读盘失败: %v", loadErr)
	}
	var loaded []world.Tile = storage.WorldTilesFromRecords(records)
	if len(loaded) != len(saved) {
		test.Fatalf("期望 %d 个方块实体，实际 %d", len(saved), len(loaded))
	}
	var index int = 0
	for index = 0; index < len(saved); index++ {
		var want world.Tile = saved[index]
		var got world.Tile = loaded[index]
		if got.Rotation != want.Rotation || got.SkullType != want.SkullType || len(got.Items) != len(want.Items) {
			test.Fatalf("期望种类/角度/格数 %d/%d/%d，实际 %d/%d/%d", want.SkullType, want.Rotation, len(want.Items), got.SkullType, got.Rotation, len(got.Items))
		}
		if got.ID != want.ID || got.X != want.X || got.Y != want.Y || got.Z != want.Z || got.Color != want.Color {
			test.Fatalf("第 %d 个基本字段不对，期望 %+v，实际 %+v", index, want, got)
		}
		if got.BurnTime != want.BurnTime || got.CookTime != want.CookTime || got.MaxTime != want.MaxTime {
			test.Fatalf("第 %d 个烧制进度不对，期望 %d/%d/%d，实际 %d/%d/%d", index, want.BurnTime, want.CookTime, want.MaxTime, got.BurnTime, got.CookTime, got.MaxTime)
		}
		if got.Paired != want.Paired || got.PairX != want.PairX || got.PairZ != want.PairZ {
			test.Fatalf("第 %d 个配对信息不对，期望 %v(%d,%d)，实际 %v(%d,%d)", index, want.Paired, want.PairX, want.PairZ, got.Paired, got.PairX, got.PairZ)
		}
		if got.Lines != want.Lines {
			test.Fatalf("第 %d 个告示牌文字不对，期望 %q，实际 %q", index, want.Lines, got.Lines)
		}
		var slot int = 0
		for slot = 0; slot < len(want.Items); slot++ {
			if !sameItem(got.Items[slot], want.Items[slot]) {
				test.Fatalf("第 %d 个的第 %d 格不对，期望 %+v，实际 %+v", index, slot, want.Items[slot], got.Items[slot])
			}
		}
	}
}

// 没用到的新字段不写进 JSON，老存档和新存档里的箱子、床长得一样，空告示牌也不会存四个空串。
func TestTilesOmitUnusedFields(test *testing.T) {
	var directory string = test.TempDir()
	var records []storage.TileRecord = storage.TilesFromWorld([]world.Tile{
		{ID: world.TileChest, X: 1, Y: 5, Z: 2},
		{ID: world.TileBed, X: 2, Y: 5, Z: 2, Color: 3},
		{ID: world.TileSign, X: 3, Y: 5, Z: 2},
	})
	var saveErr error = storage.SaveTiles(directory, "w", records)
	if saveErr != nil {
		test.Fatalf("存盘失败: %v", saveErr)
	}
	var raw, readErr = os.ReadFile(storage.TilesPath(directory, "w"))
	if readErr != nil {
		test.Fatalf("读文件失败: %v", readErr)
	}
	var text string = string(raw)
	var absent []string = []string{"burnTime", "cookTime", "maxTime", "paired", "pairX", "pairZ", "lines"}
	var index int = 0
	for index = 0; index < len(absent); index++ {
		if strings.Contains(text, absent[index]) {
			test.Fatalf("没用到的字段 %s 不该写进文件: %s", absent[index], text)
		}
	}
}

// 末影箱的方块实体存盘再读回来还是末影箱，而且没有物品栏：它的东西在玩家身上，不在方块里。
func TestEnderChestTileRoundTrip(test *testing.T) {
	var directory string = test.TempDir()
	var saved []world.Tile = []world.Tile{world.NewEnderChestTile(7, 64, -3)}
	var saveErr error = storage.SaveTiles(directory, "w", storage.TilesFromWorld(saved))
	if saveErr != nil {
		test.Fatalf("存盘失败: %v", saveErr)
	}
	var records, loadErr = storage.LoadTiles(directory, "w")
	if loadErr != nil {
		test.Fatalf("读盘失败: %v", loadErr)
	}
	var target *world.World = world.New("w")
	target.LoadTiles(storage.WorldTilesFromRecords(records))
	var loaded, found = target.TileAt(7, 64, -3)
	if !found || loaded.ID != world.TileEnderChest || len(loaded.Items) != 0 {
		test.Fatalf("末影箱应原样回来且没有物品栏，实际 找到=%v %+v", found, loaded)
	}
}
