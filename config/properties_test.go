package config

import "os"
import "path/filepath"
import "strings"
import "testing"

// 注释、空行、值里的等号、BOM、CRLF 都要处理对。
func TestParseProperties(test *testing.T) {
	var text string = "\ufeff# 注释\r\n! 另一种注释\r\nmotd = 欢迎=来玩 \r\n\r\nserver-port=19133\r\nbroken line\r\n=nokey\r\nserver-port=19134\r\n"
	var values map[string]string = ParseProperties(text)
	if values["motd"] != "欢迎=来玩" {
		test.Fatalf("motd=%q，值里的等号应保留、首尾空白去掉", values["motd"])
	}
	if values["server-port"] != "19134" {
		test.Fatalf("重复键应以后面的为准，实际 %q", values["server-port"])
	}
	if len(values) != 2 {
		test.Fatalf("注释、无等号行、空键都不该进映射: %v", values)
	}
}

func TestApplyPropertiesValid(test *testing.T) {
	var cfg *Config = Default()
	var warnings []string = cfg.ApplyProperties(map[string]string{
		"server-port":      "20000",
		"max-players":      "0",
		"gamemode":         "creative",
		"difficulty":       "hard",
		"view-distance":    "10",
		"pvp":              "off",
		"white-list":       "ON",
		"spawn-protection": "0",
		"server-ip":        "127.0.0.1",
		"motd":             "a;b",
	})
	if len(warnings) != 0 {
		test.Fatalf("合法配置不该有警告: %v", warnings)
	}
	if cfg.Port != 20000 || cfg.MaxPlayers != 0 || cfg.GameMode != 1 || cfg.Difficulty != 3 || cfg.ViewDistance != 10 {
		test.Fatalf("数值没套上: %+v", cfg)
	}
	if cfg.PVP || !cfg.Whitelist || cfg.SpawnProtection != 0 || cfg.Host != "127.0.0.1" {
		test.Fatalf("开关没套上: %+v", cfg)
	}
	if cfg.MOTD != "a,b" {
		test.Fatalf("motd 里的分号会破坏服务器列表应答，应换成逗号，实际 %q", cfg.MOTD)
	}
}

// 非法值只警告并保持原值，不能让一项写错就起不来，也不能把值改成怪东西。
func TestApplyPropertiesInvalidKeepsDefaults(test *testing.T) {
	var cfg *Config = Default()
	var defaults Config = *cfg
	var warnings []string = cfg.ApplyProperties(map[string]string{
		"server-port":   "abc",
		"max-players":   "-3",
		"view-distance": "999",
		"gamemode":      "adventure",
		"difficulty":    "impossible",
		"pvp":           "maybe",
		"white-list":    "2",
	})
	if len(warnings) != 7 {
		test.Fatalf("7 项非法应有 7 条警告，实际 %d: %v", len(warnings), warnings)
	}
	if *cfg != defaults {
		test.Fatalf("非法值不该改动配置: %+v", cfg)
	}
}

func TestApplyPropertiesRangeEdges(test *testing.T) {
	var cfg *Config = Default()
	var warnings []string = cfg.ApplyProperties(map[string]string{"view-distance": "2", "server-port": "65535"})
	if len(warnings) != 0 || cfg.ViewDistance != 2 || cfg.Port != 65535 {
		test.Fatalf("边界值应合法: %v %+v", warnings, cfg)
	}
	warnings = cfg.ApplyProperties(map[string]string{"view-distance": "1", "server-port": "0"})
	if len(warnings) != 2 || cfg.ViewDistance != 2 || cfg.Port != 65535 {
		test.Fatalf("越界应警告并保持原值: %v %+v", warnings, cfg)
	}
	warnings = cfg.ApplyProperties(map[string]string{"view-distance": "33"})
	if len(warnings) != 1 || cfg.ViewDistance != 2 {
		test.Fatalf("视距上限 32，33 应被拒: %v", warnings)
	}
}

// 生成的文件再读回来，配置应该完全一致。
func TestPropertiesTextRoundTrip(test *testing.T) {
	var source *Config = Default()
	source.Port = 12345
	source.MOTD = "往返测试"
	source.MaxPlayers = 7
	source.GameMode = 1
	source.Difficulty = 3
	source.ViewDistance = 12
	source.PVP = false
	source.Whitelist = true
	source.SpawnProtection = 3
	var loaded *Config = Default()
	var warnings []string = loaded.ApplyProperties(ParseProperties(source.PropertiesText()))
	if len(warnings) != 0 {
		test.Fatalf("生成的文件自己读回来不该有警告: %v", warnings)
	}
	if *loaded != *source {
		test.Fatalf("往返不一致:\n got %+v\nwant %+v", loaded, source)
	}
}

func TestLoadPropertiesCreatesMissingFile(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "server.properties")
	var cfg *Config = Default()
	var warnings, err = LoadProperties(path, cfg)
	if err != nil || len(warnings) != 1 {
		test.Fatalf("缺文件应生成并给一条提示: err=%v warnings=%v", err, warnings)
	}
	var raw, readErr = os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(raw), "server-port=19132") {
		test.Fatalf("应生成带默认值的文件: %v %q", readErr, raw)
	}
	var again *Config = Default()
	warnings, err = LoadProperties(path, again)
	if err != nil || len(warnings) != 0 || *again != *cfg {
		test.Fatalf("第二次读取应无警告且配置一致: err=%v warnings=%v", err, warnings)
	}
}

// 目录不存在、没法生成文件时只降级为默认值，不能崩。
func TestLoadPropertiesUnwritableFallsBack(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "no-such-dir", "server.properties")
	var warnings, err = LoadProperties(path, Default())
	if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "无法创建") {
		test.Fatalf("应降级为默认值并提示: err=%v warnings=%v", err, warnings)
	}
}

// 文件存在但读不了（这里用目录冒充）必须报错，不能悄悄用默认值上线。
func TestLoadPropertiesUnreadableIsError(test *testing.T) {
	var _, err = LoadProperties(test.TempDir(), Default())
	if err == nil {
		test.Fatal("路径是目录时读取应失败")
	}
}

// 改一项要保留注释和其它项，CRLF 文件改完仍是 CRLF；没有的键追加到末尾。
func TestSetProperty(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "server.properties")
	var original string = "# 说明\r\nwhite-list=off\r\nmotd=hello\r\n"
	var err error = os.WriteFile(path, []byte(original), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	err = SetProperty(path, "white-list", "on")
	if err != nil {
		test.Fatal(err)
	}
	var raw, _ = os.ReadFile(path)
	if string(raw) != "# 说明\r\nwhite-list=on\r\nmotd=hello\r\n" {
		test.Fatalf("改一项不该动别的行或换行风格: %q", raw)
	}
	err = SetProperty(path, "pvp", "off")
	if err != nil {
		test.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.HasSuffix(string(raw), "pvp=off\n") || !strings.Contains(string(raw), "motd=hello") {
		test.Fatalf("新键应追加到末尾: %q", raw)
	}
	var fresh string = filepath.Join(test.TempDir(), "new.properties")
	err = SetProperty(fresh, "white-list", "on")
	if err != nil {
		test.Fatal(err)
	}
	raw, _ = os.ReadFile(fresh)
	if string(raw) != "white-list=on\n" {
		test.Fatalf("文件不存在时应新建: %q", raw)
	}
}

// 注释里提到同名键不能被当成配置项改掉。
func TestSetPropertyIgnoresComments(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "server.properties")
	var err error = os.WriteFile(path, []byte("# white-list=off\nwhite-list=off\n"), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	err = SetProperty(path, "white-list", "on")
	if err != nil {
		test.Fatal(err)
	}
	var raw, _ = os.ReadFile(path)
	if string(raw) != "# white-list=off\nwhite-list=on\n" {
		test.Fatalf("只应改真正的配置行: %q", raw)
	}
}
