package config

import "errors"
import "fmt"
import "os"
import "strconv"
import "strings"

// ParseProperties 把 key=value 文本解析成映射。
// # 和 ! 开头是注释；值里可以有 =，只按第一个切；同一个键出现多次以后面的为准。
// 带 BOM 的文件（Windows 记事本另存）要先去掉，否则第一个键名会变成 "\ufeffmotd"。
func ParseProperties(text string) map[string]string {
	var values map[string]string = make(map[string]string)
	var clean string = strings.ReplaceAll(strings.TrimPrefix(text, "\ufeff"), "\r\n", "\n")
	var lines []string = strings.Split(clean, "\n")
	var index int = 0
	for index = 0; index < len(lines); index++ {
		var line string = strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		var cut int = strings.Index(line, "=")
		if cut <= 0 {
			continue
		}
		values[strings.TrimSpace(line[:cut])] = strings.TrimSpace(line[cut+1:])
	}
	return values
}

func appendIf(list []string, message string) []string {
	if message == "" {
		return list
	}
	return append(list, message)
}

// applyInt 读取整数项；缺省、非法、越界都保持原值，非法和越界返回一条警告。
func applyInt(values map[string]string, key string, low int, high int, target *int) string {
	var text, found = values[key]
	if !found {
		return ""
	}
	var number, err = strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return fmt.Sprintf("%s=%q 不是整数，沿用 %d", key, text, *target)
	}
	if number < low || number > high {
		return fmt.Sprintf("%s=%d 超出范围 %d–%d，沿用 %d", key, number, low, high, *target)
	}
	*target = number
	return ""
}

// parseBool 认 on/off、true/false、yes/no、1/0，大小写不敏感。
func parseBool(text string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "on", "true", "yes", "1":
		return true, true
	case "off", "false", "no", "0":
		return false, true
	default:
		return false, false
	}
}

func applyBool(values map[string]string, key string, target *bool) string {
	var text, found = values[key]
	if !found {
		return ""
	}
	var flag, valid = parseBool(text)
	if !valid {
		return fmt.Sprintf("%s=%q 不是 on/off，沿用 %s", key, text, OnOff(*target))
	}
	*target = flag
	return ""
}

// parseGameMode 这版只支持生存和创造，冒险 / 旁观先不开放（和 /gamemode 一致）。
func parseGameMode(text string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "0", "survival", "s":
		return 0, true
	case "1", "creative", "c":
		return 1, true
	default:
		return 0, false
	}
}

func parseDifficulty(text string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "0", "peaceful", "p":
		return 0, true
	case "1", "easy", "e":
		return 1, true
	case "2", "normal", "n":
		return 2, true
	case "3", "hard", "h":
		return 3, true
	default:
		return 0, false
	}
}

func applyChoice(values map[string]string, key string, parse func(string) (int, bool), target *int) string {
	var text, found = values[key]
	if !found {
		return ""
	}
	var number, valid = parse(text)
	if !valid {
		return fmt.Sprintf("%s=%q 不支持，沿用 %d", key, text, *target)
	}
	*target = number
	return ""
}

// ApplyProperties 把解析出的键值套到配置上，返回警告。
// 非法值只警告并保持原值：配置文件手滑写错一项，不该让整个服务器起不来。
func (c *Config) ApplyProperties(values map[string]string) []string {
	var warnings []string = nil
	warnings = appendIf(warnings, applyInt(values, "server-port", 1, 65535, &c.Port))
	warnings = appendIf(warnings, applyInt(values, "max-players", 0, 10000, &c.MaxPlayers))
	warnings = appendIf(warnings, applyInt(values, "spawn-protection", 0, 100000, &c.SpawnProtection))
	warnings = appendIf(warnings, applyInt(values, "view-distance", MinViewDistance, MaxViewDistance, &c.ViewDistance))
	warnings = appendIf(warnings, applyBool(values, "pvp", &c.PVP))
	warnings = appendIf(warnings, applyBool(values, "white-list", &c.Whitelist))
	warnings = appendIf(warnings, applyChoice(values, "gamemode", parseGameMode, &c.GameMode))
	warnings = appendIf(warnings, applyChoice(values, "difficulty", parseDifficulty, &c.Difficulty))
	var address, hasAddress = values["server-ip"]
	if hasAddress && strings.TrimSpace(address) != "" {
		c.Host = strings.TrimSpace(address)
	}
	var motd, hasMOTD = values["motd"]
	if hasMOTD && strings.TrimSpace(motd) != "" {
		// MCPE 的服务器列表应答用 ; 分字段，motd 里带分号会把后面的字段全挤歪。
		c.MOTD = strings.ReplaceAll(motd, ";", ",")
	}
	return warnings
}

// OnOff 把布尔值写成 server.properties 惯用的 on/off。
func OnOff(flag bool) string {
	if flag {
		return "on"
	}
	return "off"
}

func writeProperty(builder *strings.Builder, comment string, key string, value string) {
	builder.WriteString("# " + comment + "\n")
	builder.WriteString(key + "=" + value + "\n")
}

// PropertiesText 按当前配置生成带注释的 server.properties。
func (c *Config) PropertiesText() string {
	var builder strings.Builder = strings.Builder{}
	builder.WriteString("# Go MC Server 配置文件\n")
	builder.WriteString("# 修改后重启生效；命令行参数（如 -port）优先于本文件。\n\n")
	writeProperty(&builder, "服务器名称，显示在服务器列表里（不要写分号）", "motd", c.MOTD)
	writeProperty(&builder, "UDP 端口", "server-port", strconv.Itoa(c.Port))
	writeProperty(&builder, "绑定地址，0.0.0.0 表示所有网卡", "server-ip", c.Host)
	writeProperty(&builder, "最大玩家数，0 表示不限制", "max-players", strconv.Itoa(c.MaxPlayers))
	writeProperty(&builder, "新玩家的默认游戏模式：0 / survival 生存，1 / creative 创造", "gamemode", strconv.Itoa(c.GameMode))
	writeProperty(&builder, "难度：0 和平，1 简单，2 普通，3 困难", "difficulty", strconv.Itoa(c.Difficulty))
	writeProperty(&builder, "客户端视距上限（区块），范围 2–32；越大越吃 CPU 和流量", "view-distance", strconv.Itoa(c.ViewDistance))
	writeProperty(&builder, "玩家之间能否互相攻击", "pvp", OnOff(c.PVP))
	writeProperty(&builder, "是否开启白名单（名单见 white-list.txt，管理员不受限）", "white-list", OnOff(c.Whitelist))
	writeProperty(&builder, "出生点保护半径（方块），0 关闭", "spawn-protection", strconv.Itoa(c.SpawnProtection))
	return builder.String()
}

func createProperties(path string, c *Config) []string {
	var err error = os.WriteFile(path, []byte(c.PropertiesText()), 0o644)
	if err != nil {
		return []string{fmt.Sprintf("无法创建 %s，使用内置默认值: %v", path, err)}
	}
	return []string{fmt.Sprintf("%s 不存在，已按默认值生成", path)}
}

// LoadProperties 读取并套用 server.properties；文件不存在就按当前配置生成一份。
// 文件存在却读不出来是错误：悄悄用默认值上线，可能把本来开着的白名单变成关着的。
func LoadProperties(path string, c *Config) ([]string, error) {
	var raw, readErr = os.ReadFile(path)
	if errors.Is(readErr, os.ErrNotExist) {
		return createProperties(path, c), nil
	}
	if readErr != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, readErr)
	}
	return c.ApplyProperties(ParseProperties(string(raw))), nil
}

// SetProperty 改写（或追加）一项并保留文件里其它内容和注释。/whitelist on|off 靠它持久化。
func SetProperty(path string, key string, value string) error {
	var raw, readErr = os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("读取 %s: %w", path, readErr)
	}
	var lines []string = strings.Split(string(raw), "\n")
	var replaced bool = false
	var index int = 0
	for index = 0; index < len(lines) && !replaced; index++ {
		var trimmed string = strings.TrimSpace(lines[index])
		var cut int = strings.Index(trimmed, "=")
		if strings.HasPrefix(trimmed, "#") || cut <= 0 || strings.TrimSpace(trimmed[:cut]) != key {
			continue
		}
		var suffix string = ""
		if strings.HasSuffix(lines[index], "\r") {
			suffix = "\r"
		}
		lines[index] = key + "=" + value + suffix
		replaced = true
	}
	var text string = strings.Join(lines, "\n")
	if !replaced {
		text = strings.TrimRight(text, "\r\n")
		if text != "" {
			text += "\n"
		}
		text += key + "=" + value + "\n"
	}
	var tmp string = path + ".tmp"
	var writeErr error = os.WriteFile(tmp, []byte(text), 0o644)
	if writeErr != nil {
		return fmt.Errorf("写入 %s: %w", tmp, writeErr)
	}
	var renameErr error = os.Rename(tmp, path)
	if renameErr != nil {
		return fmt.Errorf("替换 %s: %w", path, renameErr)
	}
	return nil
}
