package server

import "bytes"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

func hopperTestTile(level *world.World, tile world.Tile, blockID, face byte) {
	level.SetBlock(tile.X, tile.Y, tile.Z, blockID, face)
	level.SetTile(tile)
}

// 同一轮先从上方箱子取一件再送入侧面双箱；窗口同步到后半箱，随后整整八次不搬。
func TestHopperChainAndWindow(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var level = fixture.srv.world
	var top = world.NewChestTile(0, 65, 0)
	top.Items[4] = protocol.Item{ID: 35, Meta: 7, Count: 3, NBT: []byte{10, 0, 0, 0}}
	hopperTestTile(level, top, world.BlockChest, 0)
	hopperTestTile(level, world.NewHopperTile(0, 64, 0), world.BlockHopper, 5)
	var back = world.NewChestTile(1, 64, 0)
	back.Items[2] = top.Items[4]
	hopperTestTile(level, back, world.BlockChest, 0)
	hopperTestTile(level, world.NewChestTile(2, 64, 0), world.BlockChest, 0)
	level.PairChests(1, 64, 0, 2, 0)
	fixture.srv.openContainer(player, 1, 64, 0)
	var log = captureSent(player)
	fixture.srv.tickHoppers()
	var update, found = log.lastContainerSlot(test, protocol.WindowContainer)
	if !found || update.Slot != 29 || update.Item.Count != 4 || !bytes.Equal(update.Item.NBT, top.Items[4].NBT) {
		test.Fatalf("期望双箱第 29 槽收到四件带标签的羊毛，实际 %+v", update)
	}
	var tick int = 0
	for tick = 0; tick < 8; tick++ {
		fixture.srv.tickHoppers()
	}
	var src, _ = level.TileAt(0, 65, 0)
	var hopper, _ = level.TileAt(0, 64, 0)
	if src.Items[4].Count != 2 || !hopper.Items[0].IsAir() || hopper.TransferCooldown != 0 {
		test.Fatalf("期望冷却八次、上方余两件且漏斗为空，实际 %+v / %+v", src, hopper)
	}
	fixture.srv.tickHoppers()
	var dest, _ = level.TileAt(1, 64, 0)
	if dest.Items[2].Count != 5 {
		test.Fatalf("期望冷却结束后双箱有五件，实际 %+v", dest.Items[2])
	}
}

// 拾取可穿过冷却：必须整栈放得下，不能碰到隔壁区块、不同世界或刚好在上边界的物品。
func TestHopperCollectWholeDrops(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var level = fixture.srv.world
	var hopper = world.NewHopperTile(15, 64, 0)
	hopper.TransferCooldown = 6
	var stack = protocol.Item{ID: 35, Meta: 4, Count: 2, NBT: []byte{10, 0, 0, 0}}
	var idx int = 0
	for idx = 0; idx < 5; idx++ {
		hopper.Items[idx] = stack
		hopper.Items[idx].Count = 64
	}
	hopper.Items[1].Count = 63
	hopperTestTile(level, hopper, world.BlockHopper, 0)
	fixture.srv.spawnDroppedItem(stack, 15.5, 65.9, 0.5, 0, 0, 0, 100, "world")
	fixture.srv.spawnDroppedItem(stack, 16.01, 65, 0.5, 0, 0, 0, 100, "world")
	fixture.srv.spawnDroppedItem(stack, 15.5, 66, 0.5, 0, 0, 0, 100, "world")
	fixture.srv.spawnDroppedItem(stack, 15.5, 65, 0.5, 0, 0, 0, 100, "nether")
	var log = captureSent(player)
	fixture.srv.tickHoppers()
	var first, _ = level.TileAt(15, 64, 0)
	if len(fixture.srv.entities) != 4 || first.Items[1].Count != 63 {
		test.Fatalf("期望空间不足时四个实体都还在、槽中 63 件，实际 %d / %+v", len(fixture.srv.entities), first.Items[1])
	}
	first.Items[2].Count = 63
	level.SetTile(first)
	fixture.srv.tickHoppers()
	var after, _ = level.TileAt(15, 64, 0)
	if len(fixture.srv.entities) != 3 || log.count(protocol.IDRemoveEntity) != 1 || after.Items[1].Count != 64 || after.Items[2].Count != 64 || after.TransferCooldown != 4 {
		test.Fatalf("期望冷却中跨两格收下一整栈并移除一个实体，实际 %d / %+v", len(fixture.srv.entities), after)
	}
}

// 取熔炉产物不动输入和燃料；侧面只送燃料，碰到不能投送的第一格时继续找。
func TestHopperFurnaceSides(test *testing.T) {
	var fixture = newAdminFixture(test)
	var level = fixture.srv.world
	var top = world.NewFurnaceTile(0, 65, 0)
	top.Items = []protocol.Item{{ID: 15, Count: 2}, {ID: 263, Count: 8}, {ID: 265, Count: 3}}
	hopperTestTile(level, top, world.BlockFurnace, 0)
	var hopper = world.NewHopperTile(0, 64, 0)
	hopper.Items[1] = protocol.Item{ID: 263, Count: 2}
	hopperTestTile(level, hopper, world.BlockHopper, 5)
	hopperTestTile(level, world.NewFurnaceTile(1, 64, 0), world.BlockFurnace, 0)
	fixture.srv.tickHoppers()
	var source, _ = level.TileAt(0, 65, 0)
	var middle, _ = level.TileAt(0, 64, 0)
	var dest, _ = level.TileAt(1, 64, 0)
	if source.Items[0].Count != 2 || source.Items[1].Count != 8 || source.Items[2].Count != 2 || middle.Items[0].ID != 265 || middle.Items[1].Count != 1 || dest.Items[1].ID != 263 || !dest.Items[2].IsAir() {
		test.Fatalf("期望取铁锭、侧面送煤，实际 %+v / %+v / %+v", source.Items, middle.Items, dest.Items)
	}
}

// 顶部只能投到原料格；岩浆烧完的空桶能从底部取出，经满的燃料格也不会塞进成品格。
func TestHopperFurnaceTopAndBucket(test *testing.T) {
	var fixture = newAdminFixture(test)
	var level = fixture.srv.world
	var upper = world.NewHopperTile(1, 65, 0)
	upper.Items[0] = protocol.Item{ID: 15, Count: 2}
	hopperTestTile(level, upper, world.BlockHopper, 0)
	var furnace = world.NewFurnaceTile(1, 64, 0)
	furnace.Items[1] = protocol.Item{ID: 325, Count: 1}
	hopperTestTile(level, furnace, world.BlockFurnace, 0)
	hopperTestTile(level, world.NewHopperTile(1, 63, 0), world.BlockHopper, 0)
	fixture.srv.tickHoppers()
	var low, _ = level.TileAt(1, 63, 0)
	var mid, _ = level.TileAt(1, 64, 0)
	if low.Items[0].ID != 325 || mid.Items[0].ID != 15 || mid.Items[0].Count != 1 || !mid.Items[1].IsAir() || !mid.Items[2].IsAir() {
		test.Fatalf("期望顶部一份矿石、底部一个空桶，实际 %+v / %+v", mid.Items, low.Items)
	}
}

// 漏斗在未加载的区块暂停，跨边界也不会唤醒邻居；上下叠放时只让下方漏斗主动拉取。
func TestHopperSleepsAndPullsFromAbove(test *testing.T) {
	var fixture = newAdminFixture(test)
	var level = fixture.srv.world
	var top = world.NewHopperTile(15, 65, 0)
	top.Items[0] = protocol.Item{ID: 264, Count: 4}
	hopperTestTile(level, top, world.BlockHopper, 5)
	var sleeping = world.NewHopperTile(16, 65, 0)
	sleeping.TransferCooldown = 3
	level.SetTile(sleeping)
	var bottom = world.NewHopperTile(15, 64, 0)
	bottom.TransferCooldown = 1
	hopperTestTile(level, bottom, world.BlockHopper, 0)
	fixture.srv.tickHoppers()
	var after, _ = level.TileAt(15, 65, 0)
	var cold, _ = level.TileAt(16, 65, 0)
	var _, _, loaded = level.Peek(16, 65, 0)
	if after.Items[0].Count != 4 || cold.TransferCooldown != 3 || loaded {
		test.Fatalf("期望叠放等待下方且邻区仍卸载，实际 %+v / %+v / %v", after, cold, loaded)
	}
	fixture.srv.tickHoppers()
	var low, _ = level.TileAt(15, 64, 0)
	after, _ = level.TileAt(15, 65, 0)
	if low.Items[0].Count != 1 || after.Items[0].Count != 3 || low.TransferCooldown != 8 || after.TransferCooldown != 7 {
		test.Fatalf("期望上下只搬一件且双方冷却，实际 %+v / %+v", low, after)
	}
}

type hopperSaveStore struct {
	world.ColumnStore
	duringSave func()
}

func (store hopperSaveStore) SaveColumn(chunkX, chunkZ int32, column *world.Chunk) error {
	store.duringSave()
	return store.ColumnStore.SaveColumn(chunkX, chunkZ, column)
}

// 在后台写区块的准确时刻吸入掉落物；重读磁盘时，带标签的物品必须只在存档的一侧出现。
func TestHopperPickupDuringSave(test *testing.T) {
	var fixture = newAdminFixture(test)
	var level = fixture.srv.world
	hopperTestTile(level, world.NewHopperTile(2, 64, 0), world.BlockHopper, 0)
	var stack = protocol.Item{ID: 35, Meta: 2, Count: 3, NBT: []byte{10, 0, 0, 0}}
	fixture.srv.spawnDroppedItem(stack, 2.5, 65, 0.5, 0, 0, 0, 10, "world")
	var folder = fixture.srv.cfg.DataDir
	var layoutErr = storage.EnsureLayout(folder, level.Name)
	if layoutErr != nil {
		test.Fatal(layoutErr)
	}
	level.SetStore(hopperSaveStore{storage.NewRegionStore(storage.WorldDir(folder, level.Name)), fixture.srv.tickHoppers})
	var err = fixture.srv.persistWorld(level)
	if err != nil {
		test.Fatal(err)
	}
	var tile, _ = level.TileAt(2, 64, 0)
	if tile.Items[0].Count != 3 || len(fixture.srv.entities) != 0 {
		test.Fatal("期望存盘时已吸入掉落物，实际未触发")
	}
	var drops, dropErr = storage.LoadDroppedItems(folder, level.Name)
	var tiles, tileErr = storage.LoadTiles(folder, level.Name)
	if dropErr != nil || tileErr != nil || len(tiles) != 1 || len(drops) != 1 || drops[0].Count != 3 || !bytes.Equal(storage.ItemFromDrop(drops[0]).NBT, stack.NBT) || !storage.WorldTilesFromRecords(tiles)[0].Items[0].IsAir() {
		test.Fatalf("期望磁盘仍为拾取前的完整一份，实际 %+v / %+v / %v / %v", drops, tiles, dropErr, tileErr)
	}
}
