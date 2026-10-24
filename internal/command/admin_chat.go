package command

import "fmt"
import "slices"
import "strings"

func runList(ctx *Context, args []string) error {
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("查看在线玩家: %w", err)
	}
	var names []string = slices.Clone(admin.OnlinePlayers())
	slices.Sort(names)
	var limit string = fmt.Sprintf("%d", admin.MaxPlayers())
	if admin.MaxPlayers() <= 0 {
		limit = "不限"
	}
	if len(names) == 0 {
		ctx.respond(fmt.Sprintf("在线玩家 (0/%s)", limit))
		return nil
	}
	ctx.respond(fmt.Sprintf("在线玩家 (%d/%s): %s", len(names), limit, strings.Join(names, ", ")))
	return nil
}

func runSay(ctx *Context, args []string) error {
	var message string = strings.TrimSpace(strings.Join(args, " "))
	if message == "" {
		return fmt.Errorf("用法: /say <消息>")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("广播: %w", err)
	}
	admin.Broadcast(senderLabel(ctx), message)
	return nil
}

func runTell(ctx *Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: /tell <玩家> <消息>")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("私聊: %w", err)
	}
	var message string = strings.Join(args[1:], " ")
	var target string = ""
	target, err = admin.Tell(senderLabel(ctx), args[0], message)
	if err != nil {
		return fmt.Errorf("私聊: %w", err)
	}
	ctx.respond(fmt.Sprintf("你悄悄地对 %s 说: %s", target, message))
	return nil
}

// runKill 不整条限制管理员：杀自己人人可用，杀别人才要管理员。
// 「是不是自己」只认完整名字（忽略大小写），不认前缀，否则普通玩家用前缀就能绕过去杀别人。
func runKill(ctx *Context, args []string) error {
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("击杀: %w", err)
	}
	var target string = ctx.Username
	if len(args) > 0 {
		target = args[0]
	}
	if len(args) == 0 && ctx.IsConsole {
		return fmt.Errorf("用法: /kill <玩家>")
	}
	var self bool = !ctx.IsConsole && strings.EqualFold(target, ctx.Username)
	if !self && !ctx.HasOp() {
		return fmt.Errorf("§c你没有权限击杀其他玩家")
	}
	var name string = ""
	name, err = admin.Kill(target)
	if err != nil {
		return fmt.Errorf("击杀: %w", err)
	}
	ctx.respond(fmt.Sprintf("已击杀 %s", name))
	return nil
}
