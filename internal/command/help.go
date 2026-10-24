package command

import (
	"fmt"
	"strings"
)

// HelpCommand lists registered slash commands.
type HelpCommand struct {
	Map *Map
}

func (HelpCommand) Name() string        { return "help" }
func (HelpCommand) Aliases() []string   { return []string{"?"} }
func (HelpCommand) Usage() string       { return "/help" }
func (HelpCommand) Description() string { return "查看指令列表" }

func (c HelpCommand) Execute(ctx *Context, args []string) error {
	if c.Map == nil {
		return fmt.Errorf("命令表未就绪")
	}
	var b strings.Builder
	b.WriteString("指令列表:")
	for _, cmd := range c.Map.Commands() {
		if requiresOp(cmd) && !ctx.HasOp() {
			continue
		}
		b.WriteString("\n")
		b.WriteString(cmd.Usage())
	}
	if ctx.Reply != nil {
		ctx.Reply(b.String())
	}
	return nil
}
