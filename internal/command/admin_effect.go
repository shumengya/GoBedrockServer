package command

import "fmt"
import "math"
import "strconv"
import "strings"

const effectSyntax = "/effect <玩家> <效果名|ID|clear> [时长] [增幅] [隐藏粒子]"

// 仅开放已有玩法或客户端表现的效果；没有实现的 ID 不能报告施加成功。
var effectNames = [...]struct {
	id          int32
	name, alias string
}{
	{1, "speed", ""}, {2, "slowness", ""}, {5, "strength", ""}, {6, "healing", ""}, {7, "harming", ""},
	{8, "jump_boost", "jump"}, {9, "nausea", "confusion"}, {10, "regeneration", ""}, {11, "resistance", "damage_resistance"},
	{12, "fire_resistance", ""}, {13, "water_breathing", ""}, {14, "invisibility", ""}, {15, "blindness", ""}, {16, "night_vision", ""},
	{17, "hunger", ""}, {18, "weakness", ""}, {19, "poison", ""}, {20, "wither", ""}, {22, "absorption", ""}, {23, "saturation", ""},
}

func runEffect(ctx *Context, args []string) error {
	if len(args) < 2 || len(args) > 5 {
		return fmt.Errorf("用法: %s", effectSyntax)
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return err
	}
	var key = strings.ToLower(args[1])
	var id int32 = 0
	var ticks int32 = 300
	var ticksPerUnit int64 = 20
	var unit = "秒"
	var amplifier int64 = 0
	var hidden = false
	if key == "clear" {
		if len(args) != 2 {
			return fmt.Errorf("用法: /effect <玩家> clear")
		}
		ticks = 0
	} else {
		var number, parseErr = strconv.ParseInt(key, 10, 32)
		var index = 0
		for index = 0; index < len(effectNames); index++ {
			var entry = effectNames[index]
			if key == entry.name || (entry.alias != "" && key == entry.alias) || (parseErr == nil && number == int64(entry.id)) {
				id = entry.id
				break
			}
		}
		if id == 0 {
			return fmt.Errorf("不支持的效果: %s", args[1])
		}
		if id == 6 || id == 7 {
			ticks, ticksPerUnit, unit = 1, 1, "刻"
		}
		if len(args) > 2 {
			var duration, parseErr = strconv.ParseInt(args[2], 10, 32)
			if parseErr != nil || duration < 0 || duration > math.MaxInt32/ticksPerUnit {
				return fmt.Errorf("无效的效果时长（%s）: %s", unit, args[2])
			}
			ticks = int32(duration * ticksPerUnit)
		}
		if len(args) > 3 {
			amplifier, err = strconv.ParseInt(args[3], 10, 32)
			if err != nil || amplifier < 0 || amplifier > 255 {
				return fmt.Errorf("增幅应为 0 到 255")
			}
		}
		if len(args) > 4 {
			switch strings.ToLower(args[4]) {
			case "true", "on", "1", "t":
				hidden = true
			case "false", "off", "0", "f":
			default:
				return fmt.Errorf("无法识别隐藏粒子参数: %s", args[4])
			}
		}
	}
	var player, applyErr = admin.SetEffect(args[0], id, ticks, int32(amplifier), hidden)
	if applyErr != nil {
		return applyErr
	}
	if key == "clear" {
		ctx.respond("已清除 " + player + " 的全部效果")
	} else if ticks == 0 {
		ctx.respond(fmt.Sprintf("已清除 %s 的效果 %d", player, id))
	} else {
		ctx.respond(fmt.Sprintf("已给予 %s 效果 %d，等级 %d，持续 %d %s", player, id, amplifier+1, int64(ticks)/ticksPerUnit, unit))
	}
	return nil
}
