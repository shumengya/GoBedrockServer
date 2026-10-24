package command

import "fmt"
import "strings"

func runKick(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: /kick <玩家> [原因]")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("踢出玩家: %w", err)
	}
	var name string = ""
	name, err = admin.Kick(args[0], strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("踢出玩家: %w", err)
	}
	ctx.respond(fmt.Sprintf("已踢出 %s", name))
	return nil
}

func runBan(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: /ban <玩家> [原因]")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("封禁玩家: %w", err)
	}
	var name string = ""
	var added bool = false
	name, added, err = admin.Ban(args[0], ctx.Username, strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("封禁玩家: %w", err)
	}
	if !added {
		ctx.respond(fmt.Sprintf("%s 已经在封禁名单里", name))
		return nil
	}
	ctx.respond(fmt.Sprintf("已封禁 %s", name))
	return nil
}

func runBanIP(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: /ban-ip <IP|玩家> [原因]")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("封禁 IP: %w", err)
	}
	var address string = ""
	var added bool = false
	address, added, err = admin.BanIP(args[0], ctx.Username, strings.Join(args[1:], " "))
	if err != nil {
		return fmt.Errorf("封禁 IP: %w", err)
	}
	if !added {
		ctx.respond(fmt.Sprintf("IP %s 已经在封禁名单里", address))
		return nil
	}
	ctx.respond(fmt.Sprintf("已封禁 IP %s", address))
	return nil
}

func runPardon(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: /pardon <玩家>")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("解除封禁: %w", err)
	}
	var removed bool = false
	removed, err = admin.Pardon(args[0])
	if err != nil {
		return fmt.Errorf("解除封禁: %w", err)
	}
	if !removed {
		ctx.respond(fmt.Sprintf("%s 没有被封禁", args[0]))
		return nil
	}
	ctx.respond(fmt.Sprintf("已解除对 %s 的封禁", args[0]))
	return nil
}

func runPardonIP(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: /pardon-ip <IP>")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("解除 IP 封禁: %w", err)
	}
	var removed bool = false
	removed, err = admin.PardonIP(args[0])
	if err != nil {
		return fmt.Errorf("解除 IP 封禁: %w", err)
	}
	if !removed {
		ctx.respond(fmt.Sprintf("IP %s 没有被封禁", args[0]))
		return nil
	}
	ctx.respond(fmt.Sprintf("已解除对 IP %s 的封禁", args[0]))
	return nil
}
