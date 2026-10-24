package command

import "fmt"
import "strings"

// CoordCommand implements /coord [on|off]：切换快捷栏上方的实时坐标显示。
type CoordCommand struct{}

func (CoordCommand) Name() string        { return "coord" }
func (CoordCommand) Aliases() []string   { return []string{"coords", "pos"} }
func (CoordCommand) Usage() string       { return "/coord [on|off]" }
func (CoordCommand) Description() string { return "开关当前位置显示" }

func (coord CoordCommand) Execute(ctx *Context, args []string) error {
	if ctx.FindPlayer == nil {
		return fmt.Errorf("命令上下文未就绪")
	}
	var self, found = ctx.FindPlayer("@s")
	if !found {
		return fmt.Errorf("玩家不在线: %s", ctx.Username)
	}
	// 不带参数就是取反，方便快速开关。
	var enable bool = false
	var err error = nil
	if len(args) < 1 {
		enable = !self.CoordsEnabled()
	} else {
		enable, err = parseCoordToggle(args[0])
		if err != nil {
			return fmt.Errorf("解析坐标开关 %s: %w", args[0], err)
		}
	}
	self.SetCoordsEnabled(enable)
	if enable {
		ctx.Reply("已开启坐标显示")
		return nil
	}
	ctx.Reply("已关闭坐标显示")
	return nil
}

func parseCoordToggle(text string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "on", "true", "1", "yes", "开", "开启":
		return true, nil
	case "off", "false", "0", "no", "关", "关闭":
		return false, nil
	default:
		return false, fmt.Errorf("用法: /coord [on|off]")
	}
}
