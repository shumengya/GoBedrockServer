package command

import (
	"fmt"
	"strings"
)

// WorldCommand implements /world <name>.
type WorldCommand struct{}

func (WorldCommand) Name() string        { return "world" }
func (WorldCommand) Aliases() []string   { return nil }
func (WorldCommand) Usage() string       { return "/world <世界名>" }
func (WorldCommand) Description() string { return "传送到指定世界" }

func (c WorldCommand) Execute(ctx *Context, args []string) error {
	if len(args) < 1 {
		if ctx.Reply != nil {
			ctx.Reply("可用世界: overworld, sky, ocean, nether, end")
		}
		return nil
	}
	if ctx.FindPlayer == nil {
		return fmt.Errorf("命令上下文未就绪")
	}
	def := ctx.DefaultWorld
	if def == "" {
		def = "world"
	}
	disk, err := ParseWorldName(args[0], def)
	if err != nil {
		return err
	}
	self, ok := ctx.FindPlayer("@s")
	if !ok {
		return fmt.Errorf("玩家不在线: %s", ctx.Username)
	}
	if !self.TransferWorld(disk) {
		return fmt.Errorf("已经在%s", WorldDisplayName(disk, def))
	}
	if ctx.Reply != nil {
		ctx.Reply(fmt.Sprintf("已传送到%s", WorldDisplayName(disk, def)))
	}
	return nil
}

// ParseWorldName maps command aliases to a disk world name.
func ParseWorldName(s, defaultWorld string) (string, error) {
	if defaultWorld == "" {
		defaultWorld = "world"
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "overworld", "world", "flat", "超平坦", "主世界":
		return defaultWorld, nil
	case "sky", "空岛":
		return "sky", nil
	case "ocean", "海洋":
		return "ocean", nil
	case "nether", "hell", "地狱", "下界":
		return "nether", nil
	case "end", "the_end", "theend", "末地":
		return "end", nil
	default:
		return "", fmt.Errorf("未知世界: %s", s)
	}
}

func WorldDisplayName(disk, defaultWorld string) string {
	switch disk {
	case "sky":
		return "空岛"
	case "ocean":
		return "海洋"
	case "nether":
		return "地狱"
	case "end":
		return "末地"
	default:
		return "主世界"
	}
}
