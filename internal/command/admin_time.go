package command

import "fmt"
import "strconv"
import "strings"

import "go-mc-server/internal/world"

const timeUsage string = "用法: /time <set|add|query|start|stop> [值]"

// timeNames 是 /time set 认的时段名，数值对齐参考端 Level::TIME_*。
var timeNames map[string]int32 = map[string]int32{
	"day":      world.TimeDay,
	"noon":     world.TimeNoon,
	"sunset":   world.TimeSunset,
	"night":    world.TimeNight,
	"midnight": world.TimeMidnight,
	"sunrise":  world.TimeSunrise,
}

func parseTimeValue(text string) (int32, error) {
	var named, found = timeNames[strings.ToLower(strings.TrimSpace(text))]
	if found {
		return named, nil
	}
	var number, err = strconv.ParseInt(strings.TrimSpace(text), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("无效的时间: %s", text)
	}
	return int32(number), nil
}

func runTime(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf(timeUsage)
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("时间: %w", err)
	}
	var worldName string = worldOf(ctx)
	switch strings.ToLower(args[0]) {
	case "start":
		admin.SetWorldTimeStopped(worldName, false)
		ctx.respond("时间已恢复流动")
		return nil
	case "stop":
		admin.SetWorldTimeStopped(worldName, true)
		ctx.respond("时间已停止")
		return nil
	case "query":
		return timeQuery(ctx, admin, worldName, args)
	case "set":
		return timeSet(ctx, admin, worldName, args)
	case "add":
		return timeAdd(ctx, admin, worldName, args)
	default:
		return fmt.Errorf(timeUsage)
	}
}

func timeQuery(ctx *Context, admin Admin, worldName string, args []string) error {
	var ticks, known = admin.WorldTime(worldName)
	if !known {
		return fmt.Errorf("时间: 找不到世界 %s", worldName)
	}
	var kind string = "daytime"
	if len(args) > 1 {
		kind = strings.ToLower(args[1])
	}
	switch kind {
	case "daytime":
		ctx.respond(fmt.Sprintf("现在是当天的第 %d tick", ticks%world.TimeFull))
	case "gametime":
		ctx.respond(fmt.Sprintf("游戏时间累计 %d tick", ticks))
	case "day":
		ctx.respond(fmt.Sprintf("现在是第 %d 天", ticks/world.TimeFull+1))
	default:
		return fmt.Errorf("用法: /time query <daytime|gametime|day>")
	}
	return nil
}

func timeSet(ctx *Context, admin Admin, worldName string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: /time set <day|noon|sunset|night|midnight|sunrise|tick数>")
	}
	var value, err = parseTimeValue(args[1])
	if err != nil {
		return err
	}
	if value < 0 {
		return fmt.Errorf("/time set 不接受负数")
	}
	admin.SetWorldTime(worldName, value)
	ctx.respond(fmt.Sprintf("已将时间设为 %d", value))
	return nil
}

func timeAdd(ctx *Context, admin Admin, worldName string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: /time add <tick数>")
	}
	var value, err = parseTimeValue(args[1])
	if err != nil {
		return err
	}
	admin.AddWorldTime(worldName, value)
	ctx.respond(fmt.Sprintf("已将时间增加 %d", value))
	return nil
}
