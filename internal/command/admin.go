package command

import "errors"

// 管理类命令共用的错误。服务端用 %w 包上具体名字，命令层用 errors.Is 判断。
var ErrPlayerNotFound error = errors.New("玩家不在线")
var ErrAmbiguousName error = errors.New("名字有多个匹配")

// Admin 是管理类命令需要的服务器能力。命令包不直接依赖 server 包，由服务端实现并挂到 Context 上。
// 带 name 的参数都允许大小写不敏感，并且在名字唯一时允许只写前缀；返回的是解析后的完整名字。
type Admin interface {
	OnlinePlayers() []string
	MaxPlayers() int

	Kick(name string, reason string) (string, error)
	Kill(name string) (string, error)

	// Ban 和 BanIP 的第二个返回值表示是否新增（已封禁则为 false）。
	// 玩家不在线也能封，按字面名字写进名单。
	Ban(name string, source string, reason string) (string, bool, error)
	BanIP(target string, source string, reason string) (string, bool, error)
	Pardon(name string) (bool, error)
	PardonIP(ip string) (bool, error)

	SetOp(name string, enable bool) (string, bool, error)
	WhitelistEnabled() bool
	SetWhitelist(enabled bool) error
	WhitelistAdd(name string) (bool, error)
	WhitelistRemove(name string) (bool, error)
	WhitelistNames() []string
	WhitelistReload() error

	// PlayerPosition 返回玩家脚下的坐标（不是眼睛高度），玩家不在线时第四个返回值为 false。
	PlayerPosition(name string) (float32, float32, float32, bool)
	TeleportToPlayer(subject string, target string) (string, string, error)
	TeleportToPosition(subject string, posX float32, posY float32, posZ float32) (string, error)

	Broadcast(sender string, message string)
	Tell(sender string, target string, message string) (string, error)

	SaveAll()
	RequestStop()

	// 世界时钟，按世界磁盘名区分。
	WorldTime(worldName string) (int32, bool)
	SetWorldTime(worldName string, ticks int32)
	AddWorldTime(worldName string, delta int32)
	SetWorldTimeStopped(worldName string, stopped bool)
}

// adminCommand 把「名字 + 说明 + 一个执行函数」打包成 Command，
// 省得十几条管理命令各自重复写五个只返回常量的方法。
type adminCommand struct {
	name        string
	aliases     []string
	usage       string
	description string
	opOnly      bool
	run         func(ctx *Context, args []string) error
}

func (cmd adminCommand) Name() string        { return cmd.name }
func (cmd adminCommand) Aliases() []string   { return cmd.aliases }
func (cmd adminCommand) Usage() string       { return cmd.usage }
func (cmd adminCommand) Description() string { return cmd.description }
func (cmd adminCommand) RequiresOp() bool    { return cmd.opOnly }

func (cmd adminCommand) Execute(ctx *Context, args []string) error {
	return cmd.run(ctx, args)
}

// senderLabel 是广播消息里显示的发送者：控制台显示 Server。
func senderLabel(ctx *Context) string {
	if ctx.IsConsole {
		return "Server"
	}
	return ctx.Username
}

// worldOf 是命令作用的世界：执行者所在世界，控制台落到默认世界。
func worldOf(ctx *Context) string {
	if ctx.World != "" {
		return ctx.World
	}
	if ctx.DefaultWorld != "" {
		return ctx.DefaultWorld
	}
	return "world"
}
