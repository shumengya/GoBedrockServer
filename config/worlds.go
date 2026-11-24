package config

import "fmt"
import "path/filepath"
import "strings"

type WorldSpec struct {
	Name      string
	Generator string
}

// 名称直接作为存档目录，不让不同输入经路径清理后指向同一世界。
func validWorldName(name string) bool {
	if !filepath.IsLocal(name) {
		return false
	}
	var index = 0
	for index = 0; index < len(name); index++ {
		var chr = name[index]
		if !(chr >= 'a' && chr <= 'z' || chr >= 'A' && chr <= 'Z' || chr >= '0' && chr <= '9' || chr == '-' || chr == '_') {
			return false
		}
	}
	return true
}

func parseExtraWorlds(value string) ([]WorldSpec, error) {
	var result = []WorldSpec{}
	if value == "" {
		return result, nil
	}
	var entries = strings.Split(value, ",")
	var names = make(map[string]bool)
	var index = 0
	for index = 0; index < len(entries); index++ {
		var name, generator, paired = strings.Cut(entries[index], ":")
		name = strings.TrimSpace(name)
		generator = strings.ToLower(strings.TrimSpace(generator))
		if !paired || !validWorldName(name) {
			return nil, fmt.Errorf("世界条目 %q 应为 目录名:地形", entries[index])
		}
		var key = strings.ToLower(name)
		if names[key] {
			return nil, fmt.Errorf("世界目录名 %q 重复", name)
		}
		switch generator {
		case "flat", "normal", "sky", "ocean", "nether", "end":
		default:
			return nil, fmt.Errorf("世界 %q 的地形 %q 不支持", name, generator)
		}
		names[key] = true
		result = append(result, WorldSpec{Name: name, Generator: generator})
	}
	return result, nil
}
