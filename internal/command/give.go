package command

import (
	"fmt"
	"strconv"

	"go-mc-server/internal/protocol"
)

// GiveCommand implements /give <player> <item[:meta]|name> [count].
type GiveCommand struct{}

func (GiveCommand) Name() string        { return "give" }
func (GiveCommand) Aliases() []string   { return nil }
func (GiveCommand) Usage() string       { return "/give <玩家> <物品id[:meta]|名称> [数量]" }
func (GiveCommand) Description() string { return "给予玩家物品" }

func (c GiveCommand) Execute(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: %s", c.Usage())
	}
	if ctx.FindPlayer == nil {
		return fmt.Errorf("命令上下文未就绪")
	}

	target, ok := ctx.FindPlayer(args[0])
	if !ok {
		return fmt.Errorf("玩家不在线: %s", args[0])
	}

	item, err := ParseItem(args[1])
	if err != nil {
		return err
	}

	var count byte = byte(defaultGiveCount(item))
	if len(args) >= 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 {
			return fmt.Errorf("无效数量: %s", args[2])
		}
		if n > MaxStack {
			n = MaxStack
		}
		count = byte(n)
	}
	item.Count = count

	leftover, added := target.AddItem(item)
	if !added {
		return fmt.Errorf("%s 的背包已满", target.Name())
	}
	target.SyncInventory()

	given := int(item.Count)
	if !leftover.IsAir() {
		given = int(item.Count) - int(leftover.Count)
	}
	spec := formatItem(item)
	if ctx.Reply != nil {
		ctx.Reply(fmt.Sprintf("已给予 %s %s ×%d", target.Name(), spec, given))
	}
	if target.Name() != ctx.Username {
		target.Reply(fmt.Sprintf("你收到来自 %s 的 %s ×%d", ctx.Username, spec, given))
	}
	return nil
}

func formatItem(it protocol.Item) string {
	if it.Meta != 0 {
		return fmt.Sprintf("%d:%d", it.ID, it.Meta)
	}
	return fmt.Sprintf("%d", it.ID)
}
