package server

import "testing"
import "go-mc-server/config"

// 新自然世界在陆地上出生，重启沿用存档；新平坦世界向客户端报告不同的生成器。
func TestWorldTypeOnCreation(test *testing.T) {
	var cfg = config.Default()
	cfg.DataDir = test.TempDir()
	cfg.ServerDir = test.TempDir()
	var warnings = cfg.ApplyProperties(config.ParseProperties("level-type=DEFAULT"))
	var first = New(cfg).world
	var floor, _ = first.GetBlock(first.SpawnX, first.SpawnY-1, first.SpawnZ)
	var feet, _ = first.GetBlock(first.SpawnX, first.SpawnY, first.SpawnZ)
	var head, _ = first.GetBlock(first.SpawnX, first.SpawnY+1, first.SpawnZ)
	if len(warnings) != 0 || first.Generator != "normal" || startGameGenerator(first) != 1 || floor == 0 || floor == 9 || feet != 0 || head != 0 {
		test.Fatalf("期望自然世界在陆地上出生，实际 %s / %d / %d,%d,%d / %v", first.Generator, first.SpawnY, floor, feet, head, warnings)
	}
	warnings = cfg.ApplyProperties(config.ParseProperties("level-type=void"))
	if len(warnings) != 1 || cfg.LevelType != "normal" {
		test.Fatalf("期望未知类型沿用 normal，实际 %s / %v", cfg.LevelType, warnings)
	}
	cfg.ApplyProperties(config.ParseProperties("level-type=flat"))
	var reopened = New(cfg).world
	if reopened.Generator != "normal" || reopened.SpawnY != first.SpawnY {
		test.Fatalf("期望重启保留自然地形和出生高度，实际 %s / %d", reopened.Generator, reopened.SpawnY)
	}
	cfg.WorldName = "second"
	var flat = New(cfg).world
	if flat.Generator != "flat" || startGameGenerator(flat) != 2 || flat.SpawnY != 5 {
		test.Fatalf("期望新超平坦世界在高度 5 出生，实际 %s / %d", flat.Generator, flat.SpawnY)
	}
}

// 改配置后启动两次：同名世界保留首次创建的种子，新世界才采用新值，零也是合法种子。
func TestWorldSeedOnCreation(test *testing.T) {
	var cfg = config.Default()
	cfg.DataDir = test.TempDir()
	cfg.ServerDir = test.TempDir()
	var warnings = cfg.ApplyProperties(config.ParseProperties("level-seed=-794"))
	var first = New(cfg)
	if len(warnings) != 0 || first.world.Seed != -794 {
		test.Fatalf("期望新世界种子 -794，实际 %d / %v", first.world.Seed, warnings)
	}
	warnings = cfg.ApplyProperties(config.ParseProperties("level-seed=0"))
	var reopened = New(cfg)
	if len(warnings) != 0 || reopened.world.Seed != -794 {
		test.Fatalf("期望重启仍为 -794，实际 %d / %v", reopened.world.Seed, warnings)
	}
	cfg.WorldName = "second"
	var second = New(cfg)
	if second.world.Seed != 0 {
		test.Fatalf("期望第二个新世界种子为 0，实际 %d", second.world.Seed)
	}
	warnings = cfg.ApplyProperties(config.ParseProperties("level-seed=2147483648"))
	if len(warnings) != 1 || cfg.LevelSeed != 0 {
		test.Fatalf("期望越界值告警并保留 0，实际 %d / %v", cfg.LevelSeed, warnings)
	}
}
