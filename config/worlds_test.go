package config

import "reflect"
import "testing"

// 整份列表通过校验才生效；空值与缺省不同，非法或重复的目录不能误加载。
func TestConfiguredWorldNames(test *testing.T) {
	var cfg = Default()
	var original = cfg.ExtraWorlds
	var invalid = []string{"../x:flat", "farm:sky,FARM:normal", "farm:void", "farm", "a:sky,", "nested/name:normal"}
	var index = 0
	for index = 0; index < len(invalid); index++ {
		var warnings = cfg.ApplyProperties(map[string]string{"extra-worlds": invalid[index]})
		if len(warnings) != 1 || !reflect.DeepEqual(cfg.ExtraWorlds, original) {
			test.Fatalf("期望拒绝 %q 并保留默认列表，实际 %v / %v", invalid[index], cfg.ExtraWorlds, warnings)
		}
	}
	var warnings = cfg.ApplyProperties(ParseProperties("level-name=../x\nextra-worlds="))
	if len(warnings) != 1 || cfg.WorldName != "world" || len(cfg.ExtraWorlds) != 0 {
		test.Fatalf("期望拒绝主世界路径但清空额外世界，实际 %+v / %v", cfg, warnings)
	}
}
