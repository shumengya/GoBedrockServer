package command

import "fmt"

func runStop(ctx *Context, args []string) error {
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("停止服务器: %w", err)
	}
	ctx.respond("正在停止服务器…")
	admin.RequestStop()
	return nil
}

func runSaveAll(ctx *Context, args []string) error {
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("保存: %w", err)
	}
	ctx.respond("正在保存…")
	admin.SaveAll()
	ctx.respond("保存完成")
	return nil
}

// AdminCommands 返回全部管理类命令，调用方逐个 Register。
// opOnly 为假的几条人人可用；/kill 自己在命令里区分「杀自己」和「杀别人」。
func AdminCommands() []Command {
	return []Command{
		adminCommand{name: "kick", usage: "/kick <玩家> [原因]", description: "踢出玩家", opOnly: true, run: runKick},
		adminCommand{name: "ban", usage: "/ban <玩家> [原因]", description: "封禁玩家", opOnly: true, run: runBan},
		adminCommand{name: "ban-ip", aliases: []string{"banip"}, usage: "/ban-ip <IP|玩家> [原因]", description: "封禁 IP", opOnly: true, run: runBanIP},
		adminCommand{name: "pardon", aliases: []string{"unban"}, usage: "/pardon <玩家>", description: "解除玩家封禁", opOnly: true, run: runPardon},
		adminCommand{name: "pardon-ip", aliases: []string{"unban-ip"}, usage: "/pardon-ip <IP>", description: "解除 IP 封禁", opOnly: true, run: runPardonIP},
		adminCommand{name: "whitelist", usage: whitelistSyntax, description: "管理白名单", opOnly: true, run: runWhitelist},
		adminCommand{name: "op", usage: "/op <玩家>", description: "授予管理员权限", opOnly: true, run: runOp},
		adminCommand{name: "deop", usage: "/deop <玩家>", description: "撤销管理员权限", opOnly: true, run: runDeop},
		adminCommand{name: "list", usage: "/list", description: "查看在线玩家", run: runList},
		adminCommand{name: "tp", aliases: []string{"teleport"}, usage: "/tp [玩家] <目的玩家|x y z>", description: "传送", opOnly: true, run: runTeleport},
		adminCommand{name: "say", usage: "/say <消息>", description: "向全服广播消息", opOnly: true, run: runSay},
		adminCommand{name: "tell", aliases: []string{"msg", "w"}, usage: "/tell <玩家> <消息>", description: "私聊", run: runTell},
		adminCommand{name: "kill", usage: "/kill [玩家]", description: "自杀；管理员可击杀他人", run: runKill},
		adminCommand{name: "effect", usage: effectSyntax, description: "施加或清除玩家状态效果", opOnly: true, run: runEffect},
		adminCommand{name: "time", usage: timeSyntax, description: "查询或调整当前世界或全部世界的时间", opOnly: true, run: runTime},
		adminCommand{name: "stop", usage: "/stop", description: "保存并关闭服务器", opOnly: true, run: runStop},
		adminCommand{name: "save-all", aliases: []string{"saveall"}, usage: "/save-all", description: "立即保存全部数据", opOnly: true, run: runSaveAll},
	}
}
