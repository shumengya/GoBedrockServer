package server

import "testing"
import "go-mc-server/config"

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
