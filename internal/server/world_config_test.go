package server

import "os"
import "testing"

import "go-mc-server/config"
import "go-mc-server/internal/network"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

// 开关世界跨三次启动：未列出的目录不新建，停用世界的玩家带着原有背包回主世界，重新启用时沿用原图。
func TestConfiguredWorldRestart(test *testing.T) {
	var cfg = config.Default()
	cfg.DataDir, cfg.ServerDir = test.TempDir(), test.TempDir()
	var warnings = cfg.ApplyProperties(config.ParseProperties("level-name=Lobby\nextra-worlds=Mines:nether,Field:flat"))
	if len(warnings) != 0 {
		test.Fatal(warnings)
	}
	var first = New(cfg)
	var mines = first.worlds["Mines"]
	if first.world.Name != "Lobby" || len(first.worlds) != 3 || mines == nil || mines.ProtocolDimension() != 1 || first.worlds["Field"].Generator != world.GeneratorFlat {
		test.Fatalf("期望大厅、下界矿区和超平坦农场，实际 %v", first.worlds)
	}
	var _, err = os.Stat(storage.WorldDir(cfg.DataDir, "end"))
	if !os.IsNotExist(err) {
		test.Fatalf("期望不生成默认末地目录，实际 %v", err)
	}
	mines.SetBlock(12, 65, -7, world.BlockWool, 9)
	if err = first.persistWorld(mines); err != nil {
		test.Fatal(err)
	}
	var session = &network.Session{ClientUUID: "uuid-via-mines", Username: "Alex"}
	err = storage.SavePlayer(cfg.DataDir, storage.PlayerData{UUID: session.ClientUUID, World: "Mines", Dimension: 1, X: 12, Y: 85, Z: -7, Inventory: []storage.SlotData{{ID: 265, Count: 4}}})
	if err != nil {
		test.Fatal(err)
	}
	cfg.ApplyProperties(config.ParseProperties("extra-worlds="))
	var second = New(cfg)
	second.spawnPlayer(session)
	var player = second.players[session.ClientUUID]
	if len(second.worlds) != 1 || player.WorldName != "Lobby" || player.Dimension != 0 || player.X != 0.5 || player.Z != 0.5 || !nearly(player.Y-playerEyeHeight, 5) || player.Inventory[0].ID != 265 || player.Inventory[0].Count != 4 {
		test.Fatalf("期望持有 4 个铁锭回主世界出生，实际 %s / %d / (%f,%f,%f) / %+v", player.WorldName, player.Dimension, player.X, player.Y, player.Z, player.Inventory[0])
	}
	// 模拟拿另一张本服地图的备份作为 Mines：目录名和内部旧名字可以不同。
	var metaPath = storage.WorldMetaPath(cfg.DataDir, "Mines")
	var meta, loadErr = storage.LoadWorldMeta(metaPath, storage.WorldDefaults{})
	if loadErr != nil {
		test.Fatal(loadErr)
	}
	meta.Name = "OldBackup"
	if err = storage.SaveWorldMeta(metaPath, meta); err != nil {
		test.Fatal(err)
	}
	cfg.ApplyProperties(config.ParseProperties("extra-worlds=Mines:sky"))
	var third = New(cfg)
	mines = third.worlds["Mines"]
	if mines == nil || mines.Generator != world.GeneratorNetherTerrain {
		test.Fatalf("期望按目录名打开原来的下界，实际 %v", third.worlds)
	}
	var id, data = mines.GetBlock(12, 65, -7)
	if id != world.BlockWool || data != 9 {
		test.Fatalf("期望重新启用后读到羊毛 35:9，实际 %d:%d", id, data)
	}
}
