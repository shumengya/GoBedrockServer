package storage

import "os"
import "path/filepath"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// 一个世界 = 一个文件夹：元数据、方块实体、区域文件都在 worlds/<世界名>/ 同一层，
// 不允许再出现 overworld / nether / the_end 这类维度子目录。
func TestWorldIsSingleFolder(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded *world.World = nil
	var err error = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{
		Name:   "test",
		Seed:   1,
		SpawnX: 0,
		SpawnY: 5,
		SpawnZ: 0,
	})
	if err != nil {
		test.Fatal(err)
	}
	if !loaded.SetBlock(20, 6, -4, world.BlockCobblestone, 0) {
		test.Fatal("set cobble")
	}
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var reloaded *world.World = nil
	reloaded, err = OpenWorld(dataDir, WorldDefaults{Name: "test", SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	var blockID byte = 0
	var blockData byte = 0
	blockID, blockData = reloaded.GetBlock(20, 6, -4)
	if blockID != world.BlockCobblestone || blockData != 0 {
		test.Fatalf("loaded id=%d data=%d want cobble", blockID, blockData)
	}
	blockID, blockData = reloaded.GetBlock(0, 4, 0)
	if blockID != world.BlockGrass || blockData != 0 {
		test.Fatalf("unmodified grass got %d/%d", blockID, blockData)
	}
	var worldDir string = filepath.Join(dataDir, "worlds", "test")
	var matches []string = nil
	matches, err = filepath.Glob(filepath.Join(worldDir, "*.gmr"))
	if err != nil {
		test.Fatal(err)
	}
	if len(matches) == 0 {
		test.Fatal("区域文件必须直接放在世界文件夹里")
	}
	var dims []string = []string{"overworld", "nether", "the_end"}
	var index int = 0
	for index = 0; index < len(dims); index++ {
		var statErr error = nil
		var info os.FileInfo = nil
		info, statErr = os.Stat(filepath.Join(worldDir, dims[index]))
		if info != nil {
			test.Fatalf("%s 不该存在于世界文件夹内", dims[index])
		}
		if !os.IsNotExist(statErr) {
			test.Fatalf("%s 检查失败: %v", dims[index], statErr)
		}
	}
}

// 旧版把区块放在 worlds/<世界名>/overworld/ 里，打开世界时要自动搬到根上。
func TestOpenWorldMigratesLegacyDimensionFolders(test *testing.T) {
	var dataDir string = test.TempDir()
	var sourceDir string = filepath.Join(dataDir, "worlds", "source")
	var legacyDir string = filepath.Join(dataDir, "worlds", "legacy", "overworld")
	var err error = os.MkdirAll(legacyDir, 0o755)
	if err != nil {
		test.Fatal(err)
	}
	// 造一个带标记方块的区块，迁移后必须能从新位置读回来。
	var source *world.World = nil
	source, err = OpenWorld(dataDir, WorldDefaults{Name: "source", Seed: 1, SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	if !source.SetBlock(3, 6, 3, world.BlockCobblestone, 0) {
		test.Fatal("set cobble")
	}
	err = SaveWorldFrom(dataDir, source)
	if err != nil {
		test.Fatal(err)
	}
	var saved []string = nil
	saved, err = filepath.Glob(filepath.Join(sourceDir, "*.gmr"))
	if err != nil {
		test.Fatal(err)
	}
	var index int = 0
	for index = 0; index < len(saved); index++ {
		var target string = filepath.Join(legacyDir, filepath.Base(saved[index]))
		err = os.Rename(saved[index], target)
		if err != nil {
			test.Fatal(err)
		}
	}

	var loaded *world.World = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{Name: "legacy", Seed: 1, SpawnX: 8, SpawnY: 5, SpawnZ: 8})
	if err != nil {
		test.Fatal(err)
	}
	var blockID byte = 0
	blockID, _ = loaded.GetBlock(3, 6, 3)
	if blockID != world.BlockCobblestone {
		test.Fatalf("迁移后旧区块没读出来 id=%d", blockID)
	}
	var info os.FileInfo = nil
	info, err = os.Stat(legacyDir)
	if info != nil {
		test.Fatal("空掉的旧维度目录应被删掉")
	}
	if !os.IsNotExist(err) {
		test.Fatalf("检查旧目录失败: %v", err)
	}
}

// 旧目录里还有非 .gmr 文件时，宁可留着也不要替用户删。
func TestMigrateKeepsNonEmptyLegacyFolder(test *testing.T) {
	var dataDir string = test.TempDir()
	var legacyDir string = filepath.Join(dataDir, "worlds", "keep", "nether")
	var err error = os.MkdirAll(legacyDir, 0o755)
	if err != nil {
		test.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(legacyDir, "note.txt"), []byte("x"), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	var moved int = 0
	moved, err = MigrateLegacyDimensions(filepath.Join(dataDir, "worlds", "keep"))
	if err != nil {
		test.Fatal(err)
	}
	if moved != 0 {
		test.Fatalf("没东西可搬却动了 %d 个文件", moved)
	}
	var info os.FileInfo = nil
	info, err = os.Stat(legacyDir)
	if info == nil {
		test.Fatalf("非空的旧目录不该被删: %v", err)
	}
}

func TestPlayerJSONRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var input PlayerData = PlayerData{
		UUID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Name:      "Steve",
		XUID:      "1",
		World:     "world",
		Dimension: 0,
		X:         1.5,
		Y:         18.0,
		Z:         2.5,
		Yaw:       90,
		Pitch:     10,
		GameMode:  0,
		HeldSlot:  2,
		Inventory: SlotsFromItems([]protocol.Item{
			{ID: 1, Count: 64},
			{ID: 4, Meta: 0, Count: 32},
		}),
	}
	var err error = SavePlayer(dataDir, input)
	if err != nil {
		test.Fatal(err)
	}
	var output PlayerData = PlayerData{}
	var found bool = false
	output, found, err = LoadPlayer(dataDir, input.UUID)
	if err != nil || !found {
		test.Fatalf("load ok=%v err=%v", found, err)
	}
	if output.Name != "Steve" || output.X != 1.5 || output.HeldSlot != 2 {
		test.Fatalf("meta mismatch %+v", output)
	}
	var items []protocol.Item = ItemsFromSlots(output.Inventory, 36)
	if items[0].ID != 1 || items[0].Count != 64 {
		test.Fatalf("slot0 %+v", items[0])
	}
	if items[1].ID != 4 || items[1].Count != 32 {
		test.Fatalf("slot1 %+v", items[1])
	}
}

func TestEmptyUUIDDoesNotCollideWithWorldName(test *testing.T) {
	var path string = PlayerPath("data", "")
	if filepath.Base(path) == "world.json" {
		test.Fatalf("empty uuid must not land on %s", path)
	}
}

func TestUnmodifiedFlatNotRequired(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded *world.World = nil
	var err error = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{Name: "test", SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	var blockID byte = 0
	blockID, _ = loaded.GetBlock(0, 4, 0)
	if blockID != world.BlockGrass {
		test.Fatalf("flat grass id=%d", blockID)
	}
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var matches []string = nil
	matches, err = filepath.Glob(filepath.Join(dataDir, "worlds", "test", "*.gmr"))
	if err != nil {
		test.Fatal(err)
	}
	if len(matches) != 0 {
		test.Fatalf("unmodified flat should not write region files: %v", matches)
	}
}

func TestOpenSkyWorld(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded *world.World = nil
	var err error = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{
		Name:      "sky",
		Seed:      1,
		SpawnX:    0,
		SpawnY:    64,
		SpawnZ:    0,
		Generator: world.GeneratorSky,
	})
	if err != nil {
		test.Fatal(err)
	}
	if loaded.Generator != world.GeneratorSky {
		test.Fatalf("generator=%s", loaded.Generator)
	}
	var blockID byte = 0
	blockID, _ = loaded.GetBlock(0, 64, 0)
	if blockID != world.BlockGrass {
		test.Fatalf("sky grass id=%d", blockID)
	}
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var reloaded *world.World = nil
	reloaded, err = OpenWorld(dataDir, WorldDefaults{Name: "sky", Generator: world.GeneratorSky, SpawnY: 64})
	if err != nil {
		test.Fatal(err)
	}
	if reloaded.Generator != world.GeneratorSky {
		test.Fatalf("reloaded generator=%s", reloaded.Generator)
	}
}

// 地狱是平级世界，区块直接进 worlds/nether/，不再套一层 nether/。
func TestOpenNetherWorldUsesPeerFolder(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded *world.World = nil
	var err error = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{
		Name:      "nether",
		Seed:      1,
		SpawnX:    8,
		SpawnY:    64,
		SpawnZ:    8,
		Generator: world.GeneratorNether,
	})
	if err != nil {
		test.Fatal(err)
	}
	if loaded.ProtocolDimension() != 1 {
		test.Fatalf("dim=%d", loaded.ProtocolDimension())
	}
	if !loaded.SetBlock(1, 6, 1, world.BlockCobblestone, 0) {
		test.Fatal("set cobble")
	}
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var matches []string = nil
	matches, err = filepath.Glob(filepath.Join(dataDir, "worlds", "nether", "*.gmr"))
	if err != nil {
		test.Fatal(err)
	}
	if len(matches) == 0 {
		test.Fatal("地狱的区块必须直接放在 worlds/nether/ 里")
	}
	var info os.FileInfo = nil
	info, err = os.Stat(filepath.Join(dataDir, "worlds", "nether", "nether"))
	if info != nil {
		test.Fatal("nether/nether 不该存在")
	}
	if !os.IsNotExist(err) {
		test.Fatalf("检查 nether/nether 失败: %v", err)
	}
}

func TestTilesJSONRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded *world.World = nil
	var err error = nil
	loaded, err = OpenWorld(dataDir, WorldDefaults{Name: "test", SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	var chest world.Tile = world.NewChestTile(4, 5, 6)
	chest.Items[0] = protocol.Item{ID: 4, Count: 12}
	loaded.SetTile(chest)
	loaded.SetTile(world.NewBedTile(7, 5, 6, 14))
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var reloaded *world.World = nil
	reloaded, err = OpenWorld(dataDir, WorldDefaults{Name: "test", SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	var chestBack world.Tile = world.Tile{}
	var chestExists bool = false
	chestBack, chestExists = reloaded.TileAt(4, 5, 6)
	if !chestExists || chestBack.ID != world.TileChest || chestBack.Items[0].ID != 4 || chestBack.Items[0].Count != 12 {
		test.Fatalf("chest %+v exists=%v", chestBack, chestExists)
	}
	var bedBack world.Tile = world.Tile{}
	var bedExists bool = false
	bedBack, bedExists = reloaded.TileAt(7, 5, 6)
	if !bedExists || bedBack.ID != world.TileBed || bedBack.Color != 14 {
		test.Fatalf("bed %+v exists=%v", bedBack, bedExists)
	}
}

func TestDroppedItemsJSONRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var input []DroppedItemData = []DroppedItemData{{
		ID: 1, Count: 8,
		X: 1.5, Y: 5.2, Z: -3,
		SpeedY:      0.1,
		PickupDelay: 10,
		Age:         3,
	}}
	var err error = SaveDroppedItems(dataDir, "test", input)
	if err != nil {
		test.Fatal(err)
	}
	var output []DroppedItemData = nil
	output, err = LoadDroppedItems(dataDir, "test")
	if err != nil {
		test.Fatal(err)
	}
	if len(output) != 1 || output[0].ID != 1 || output[0].Count != 8 || output[0].Y != 5.2 || output[0].Age != 3 {
		test.Fatalf("%+v", output)
	}
	var missing []DroppedItemData = nil
	missing, err = LoadDroppedItems(dataDir, "empty-world")
	if err != nil || missing != nil {
		test.Fatalf("missing file should be empty: %v %v", missing, err)
	}
}
