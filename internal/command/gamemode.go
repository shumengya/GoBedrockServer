package command

import (
	"fmt"
	"strings"

	"go-mc-server/internal/protocol"
)

// GamemodeCommand implements /gamemode <mode> [player] (alias /gm).
type GamemodeCommand struct{}

func (GamemodeCommand) Name() string        { return "gamemode" }
func (GamemodeCommand) Aliases() []string   { return []string{"gm"} }
func (GamemodeCommand) Usage() string       { return "/gamemode <模式> [玩家]" }
func (GamemodeCommand) Description() string { return "切换生存或创造模式" }

func (c GamemodeCommand) Execute(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: %s", c.Usage())
	}
	gm, err := ParseGameMode(args[0])
	if err != nil {
		return err
	}
	if ctx.FindPlayer == nil {
		return fmt.Errorf("命令上下文未就绪")
	}
	targetName := ctx.Username
	if len(args) >= 2 {
		targetName = args[1]
	} else {
		targetName = "@s"
	}
	target, ok := ctx.FindPlayer(targetName)
	if !ok {
		return fmt.Errorf("玩家不在线: %s", targetName)
	}
	if !target.SetGameMode(gm) {
		return fmt.Errorf("%s 已经是%s模式", target.Name(), GameModeName(gm))
	}
	self := strings.EqualFold(target.Name(), ctx.Username)
	if self {
		if ctx.Reply != nil {
			ctx.Reply(fmt.Sprintf("已将游戏模式设为%s", GameModeName(gm)))
		}
	} else {
		if ctx.Reply != nil {
			ctx.Reply(fmt.Sprintf("已将 %s 的游戏模式设为%s", target.Name(), GameModeName(gm)))
		}
		target.Reply(fmt.Sprintf("你的游戏模式已设为%s", GameModeName(gm)))
	}
	return nil
}

// ParseGameMode accepts PocketMine 0/1 aliases only this round.
func ParseGameMode(s string) (int32, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "survival", "s":
		return protocol.GameModeSurvival, nil
	case "1", "creative", "c":
		return protocol.GameModeCreative, nil
	default:
		return 0, fmt.Errorf("未知游戏模式: %s", s)
	}
}

func GameModeName(gm int32) string {
	if gm == protocol.GameModeCreative {
		return "创造"
	}
	return "生存"
}
