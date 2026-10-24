package command

import "fmt"
import "strings"

func runOp(ctx *Context, args []string) error {
	return setOperator(ctx, args, true)
}

func runDeop(ctx *Context, args []string) error {
	return setOperator(ctx, args, false)
}

func setOperator(ctx *Context, args []string, enable bool) error {
	if len(args) < 1 {
		if enable {
			return fmt.Errorf("用法: /op <玩家>")
		}
		return fmt.Errorf("用法: /deop <玩家>")
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("修改管理员: %w", err)
	}
	var name string = ""
	var changed bool = false
	name, changed, err = admin.SetOp(args[0], enable)
	if err != nil {
		return fmt.Errorf("修改管理员: %w", err)
	}
	if enable && changed {
		ctx.respond(fmt.Sprintf("已将 %s 设为管理员", name))
		return nil
	}
	if enable {
		ctx.respond(fmt.Sprintf("%s 已经是管理员", name))
		return nil
	}
	if changed {
		ctx.respond(fmt.Sprintf("已撤销 %s 的管理员权限", name))
		return nil
	}
	ctx.respond(fmt.Sprintf("%s 不是管理员", name))
	return nil
}

const whitelistSyntax string = "/whitelist <on|off|add|remove|list|reload> [玩家]"
const whitelistUsage string = "用法: " + whitelistSyntax

func runWhitelist(ctx *Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf(whitelistUsage)
	}
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("白名单: %w", err)
	}
	switch strings.ToLower(args[0]) {
	case "on":
		return whitelistToggle(ctx, admin, true)
	case "off":
		return whitelistToggle(ctx, admin, false)
	case "add":
		return whitelistChange(ctx, admin, args, true)
	case "remove":
		return whitelistChange(ctx, admin, args, false)
	case "list":
		return whitelistList(ctx, admin)
	case "reload":
		return whitelistReload(ctx, admin)
	default:
		return fmt.Errorf(whitelistUsage)
	}
}

func whitelistToggle(ctx *Context, admin Admin, enabled bool) error {
	var err error = admin.SetWhitelist(enabled)
	if err != nil {
		return fmt.Errorf("切换白名单: %w", err)
	}
	if enabled {
		ctx.respond("白名单已开启（管理员不受限制）")
		return nil
	}
	ctx.respond("白名单已关闭")
	return nil
}

// changeWhitelist 按 add 选择加入或移出。拆成独立函数是为了用提前 return 代替 else。
func changeWhitelist(admin Admin, name string, add bool) (bool, error) {
	if add {
		return admin.WhitelistAdd(name)
	}
	return admin.WhitelistRemove(name)
}

func whitelistChange(ctx *Context, admin Admin, args []string, add bool) error {
	if len(args) < 2 {
		return fmt.Errorf(whitelistUsage)
	}
	var changed, err = changeWhitelist(admin, args[1], add)
	if err != nil {
		return fmt.Errorf("修改白名单: %w", err)
	}
	if add && changed {
		ctx.respond(fmt.Sprintf("已将 %s 加入白名单", args[1]))
		return nil
	}
	if add {
		ctx.respond(fmt.Sprintf("%s 已在白名单里", args[1]))
		return nil
	}
	if changed {
		ctx.respond(fmt.Sprintf("已将 %s 移出白名单", args[1]))
		return nil
	}
	ctx.respond(fmt.Sprintf("%s 不在白名单里", args[1]))
	return nil
}

func whitelistList(ctx *Context, admin Admin) error {
	var names []string = admin.WhitelistNames()
	if len(names) == 0 {
		ctx.respond("白名单是空的")
		return nil
	}
	ctx.respond(fmt.Sprintf("白名单（%d 人）: %s", len(names), strings.Join(names, ", ")))
	return nil
}

func whitelistReload(ctx *Context, admin Admin) error {
	var err error = admin.WhitelistReload()
	if err != nil {
		return fmt.Errorf("重新读取白名单: %w", err)
	}
	ctx.respond("已重新读取白名单")
	return nil
}
