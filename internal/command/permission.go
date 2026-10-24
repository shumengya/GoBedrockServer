package command

import "fmt"

// OperatorOnly 是可选接口：命令实现了并返回 true，就只有管理员和控制台能用。
// 没实现的命令人人可用。有些命令（比如 /kill）对自己人人可用、对别人才要管理员，
// 这种不实现本接口，在命令里自己查 ctx.HasOp()。
type OperatorOnly interface {
	RequiresOp() bool
}

// requiresOp 判断命令是否只给管理员用。用二值类型断言取可选接口。
func requiresOp(cmd Command) bool {
	var restricted, implements = cmd.(OperatorOnly)
	return implements && restricted.RequiresOp()
}

// HasOp 报告执行者是否有管理员权限：控制台永远有。
func (ctx *Context) HasOp() bool {
	return ctx != nil && (ctx.IsConsole || ctx.IsOp)
}

// respond 回复执行者；没挂回复函数（测试）时什么都不做。
func (ctx *Context) respond(message string) {
	if ctx == nil || ctx.Reply == nil {
		return
	}
	ctx.Reply(message)
}

// requireAdmin 取管理接口；上下文没挂就是服务器没接好，报错而不是空指针。
func (ctx *Context) requireAdmin() (Admin, error) {
	if ctx == nil || ctx.Admin == nil {
		return nil, fmt.Errorf("命令上下文未就绪")
	}
	return ctx.Admin, nil
}

// 这三条是创造力很强的命令（凭空给物品、刷怪、改游戏模式），默认只给管理员。
func (GiveCommand) RequiresOp() bool     { return true }
func (SummonCommand) RequiresOp() bool   { return true }
func (GamemodeCommand) RequiresOp() bool { return true }
