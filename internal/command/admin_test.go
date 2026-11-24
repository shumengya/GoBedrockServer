package command

import "errors"
import "fmt"
import "math"
import "strings"
import "testing"

// fakeAdmin 记录收到的调用。内嵌接口本身（留空）：没实现的方法一旦被调用就会 panic，
// 能立刻暴露「本不该走到这里」的命令。
type fakeAdmin struct {
	Admin
	calls      []string
	online     []string
	maxPlayers int
	known      map[string]bool
	positions  map[string][3]float32
	ticks      int32
	whitelist  []string
	worlds     []string
}

func (fake *fakeAdmin) note(format string, args ...any) {
	fake.calls = append(fake.calls, fmt.Sprintf(format, args...))
}

func (fake *fakeAdmin) resolve(name string) (string, error) {
	var found, isKnown = fake.known[strings.ToLower(name)]
	if !isKnown || !found {
		return "", fmt.Errorf("%s: %w", name, ErrPlayerNotFound)
	}
	return name, nil
}

func (fake *fakeAdmin) OnlinePlayers() []string { return fake.online }
func (fake *fakeAdmin) WorldNames() []string    { return fake.worlds }
func (fake *fakeAdmin) MaxPlayers() int         { return fake.maxPlayers }

func (fake *fakeAdmin) Kick(name string, reason string) (string, error) {
	fake.note("kick %s %q", name, reason)
	return fake.resolve(name)
}

func (fake *fakeAdmin) Kill(name string) (string, error) {
	fake.note("kill %s", name)
	return fake.resolve(name)
}

func (fake *fakeAdmin) Ban(name string, source string, reason string) (string, bool, error) {
	fake.note("ban %s by %s %q", name, source, reason)
	return name, name != "Already", nil
}

func (fake *fakeAdmin) SetOp(name string, enable bool) (string, bool, error) {
	fake.note("op %s %v", name, enable)
	return name, name != "Same", nil
}

func (fake *fakeAdmin) SetWhitelist(enabled bool) error {
	fake.note("whitelist %v", enabled)
	return nil
}

func (fake *fakeAdmin) WhitelistAdd(name string) (bool, error) {
	fake.note("wl-add %s", name)
	return true, nil
}

func (fake *fakeAdmin) WhitelistNames() []string { return fake.whitelist }

func (fake *fakeAdmin) PlayerPosition(name string) (float32, float32, float32, bool) {
	var pos, found = fake.positions[strings.ToLower(name)]
	return pos[0], pos[1], pos[2], found
}

func (fake *fakeAdmin) TeleportToPlayer(subject string, target string) (string, string, error) {
	fake.note("tp %s -> %s", subject, target)
	return subject, target, nil
}

func (fake *fakeAdmin) TeleportToPosition(subject string, posX float32, posY float32, posZ float32) (string, error) {
	fake.note("tp %s -> %.1f,%.1f,%.1f", subject, posX, posY, posZ)
	return subject, nil
}

func (fake *fakeAdmin) Broadcast(sender string, message string) {
	fake.note("say [%s] %s", sender, message)
}

func (fake *fakeAdmin) Tell(sender string, target string, message string) (string, error) {
	fake.note("tell %s -> %s: %s", sender, target, message)
	return fake.resolve(target)
}

func (fake *fakeAdmin) RequestStop() { fake.note("stop") }
func (fake *fakeAdmin) SaveAll()     { fake.note("save") }

func (fake *fakeAdmin) WorldTime(worldName string) (int32, bool) {
	return fake.ticks, worldName != "missing"
}
func (fake *fakeAdmin) SetWorldTime(worldName string, ticks int32) {
	fake.note("time-set %s %d", worldName, ticks)
}
func (fake *fakeAdmin) AddWorldTime(worldName string, delta int32) {
	fake.note("time-add %s %d", worldName, delta)
}
func (fake *fakeAdmin) SetWorldTimeStopped(worldName string, stopped bool) {
	fake.note("time-stopped %s %v", worldName, stopped)
}

type adminHarness struct {
	commands *Map
	admin    *fakeAdmin
	replies  []string
}

func newHarness() *adminHarness {
	var harness *adminHarness = &adminHarness{commands: NewMap(), admin: &fakeAdmin{}}
	harness.admin.known = map[string]bool{"steve": true, "alex": true}
	var list []Command = AdminCommands()
	var index int = 0
	for index = 0; index < len(list); index++ {
		harness.commands.Register(list[index])
	}
	harness.commands.Register(HelpCommand{Map: harness.commands})
	return harness
}

func (harness *adminHarness) context(username string, isOp bool, isConsole bool) *Context {
	return &Context{
		Username:     username,
		IsOp:         isOp,
		IsConsole:    isConsole,
		DefaultWorld: "world",
		Reply:        func(msg string) { harness.replies = append(harness.replies, msg) },
		Admin:        harness.admin,
	}
}

func (harness *adminHarness) lastReply() string {
	if len(harness.replies) == 0 {
		return ""
	}
	return harness.replies[len(harness.replies)-1]
}

// 非管理员碰管理命令一律被挡在 Dispatch，连 Admin 都不会被调用；管理员和控制台可以。
func TestAdminCommandsRequireOp(test *testing.T) {
	var harness *adminHarness = newHarness()
	var lines []string = []string{
		"kick Steve", "ban Steve", "ban-ip 1.2.3.4", "pardon Steve", "pardon-ip 1.2.3.4",
		"whitelist on", "op Steve", "deop Steve", "tp Alex", "say hi", "time set day", "stop", "save-all",
	}
	var index int = 0
	for index = 0; index < len(lines); index++ {
		harness.replies = nil
		harness.commands.Dispatch(harness.context("Mallory", false, false), lines[index])
		if len(harness.replies) != 1 || !strings.Contains(harness.replies[0], "没有权限") {
			test.Errorf("非管理员执行 /%s 应被拒绝，回复=%v", lines[index], harness.replies)
		}
	}
	if len(harness.admin.calls) != 0 {
		test.Fatalf("被拒绝的命令不该触达 Admin: %v", harness.admin.calls)
	}
	harness.commands.Dispatch(harness.context("Boss", true, false), "kick Steve")
	harness.commands.Dispatch(harness.context("CONSOLE", false, true), "kick Alex")
	if len(harness.admin.calls) != 2 {
		test.Fatalf("管理员和控制台都应能执行: %v", harness.admin.calls)
	}
}

// 公共命令人人可用。
func TestPublicCommandsOpenToEveryone(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.admin.online = []string{"Zed", "Amy"}
	harness.admin.maxPlayers = 20
	harness.commands.Dispatch(harness.context("Mallory", false, false), "list")
	if harness.lastReply() != "在线玩家 (2/20): Amy, Zed" {
		test.Fatalf("list 应排序并带上限: %q", harness.lastReply())
	}
	harness.commands.Dispatch(harness.context("Mallory", false, false), "tell Steve hello world")
	if harness.lastReply() != "你悄悄地对 Steve 说: hello world" {
		test.Fatalf("tell 回复: %q", harness.lastReply())
	}
	harness.admin.maxPlayers = 0
	harness.commands.Dispatch(harness.context("Mallory", false, false), "list")
	if !strings.Contains(harness.lastReply(), "/不限)") {
		test.Fatalf("max-players 为 0 应显示不限: %q", harness.lastReply())
	}
}

// /kill：自己人人可杀；杀别人要管理员；用自己名字的前缀绕不过去。
func TestKillPermissions(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.commands.Dispatch(harness.context("Steve", false, false), "kill")
	harness.commands.Dispatch(harness.context("Steve", false, false), "kill steve")
	if len(harness.admin.calls) != 2 || harness.admin.calls[0] != "kill Steve" {
		test.Fatalf("杀自己应放行（含大小写不同）: %v", harness.admin.calls)
	}
	harness.admin.calls = nil
	harness.commands.Dispatch(harness.context("Steve", false, false), "kill Alex")
	harness.commands.Dispatch(harness.context("Steve", false, false), "kill Ste")
	if len(harness.admin.calls) != 0 {
		test.Fatalf("普通玩家杀别人 / 用前缀冒充自己，不该触达 Admin: %v", harness.admin.calls)
	}
	harness.commands.Dispatch(harness.context("Boss", true, false), "kill Alex")
	if len(harness.admin.calls) != 1 || harness.admin.calls[0] != "kill Alex" {
		test.Fatalf("管理员可以杀别人: %v", harness.admin.calls)
	}
	harness.admin.calls = nil
	harness.replies = nil
	harness.commands.Dispatch(harness.context("CONSOLE", false, true), "kill")
	if len(harness.admin.calls) != 0 || !strings.Contains(harness.lastReply(), "用法") {
		test.Fatalf("控制台没有「自己」，不带目标必须报用法: calls=%v reply=%q", harness.admin.calls, harness.lastReply())
	}
}

// 玩家不在线时错误链里要能 errors.Is 到 ErrPlayerNotFound，回复里带名字。
func TestAdminErrorsKeepSentinel(test *testing.T) {
	var harness *adminHarness = newHarness()
	var err error = harness.commands.cmds["kick"].Execute(harness.context("Boss", true, false), []string{"Ghost"})
	if !errors.Is(err, ErrPlayerNotFound) {
		test.Fatalf("应能识别「玩家不在线」: %v", err)
	}
	harness.commands.Dispatch(harness.context("Boss", true, false), "kick Ghost")
	if !strings.Contains(harness.lastReply(), "Ghost") {
		test.Fatalf("回复应带上玩家名: %q", harness.lastReply())
	}
}

func TestBanAndOpReplies(test *testing.T) {
	var harness *adminHarness = newHarness()
	var boss *Context = harness.context("Boss", true, false)
	harness.commands.Dispatch(boss, "ban Steve 恶意拆家 多次")
	if harness.admin.calls[0] != "ban Steve by Boss \"恶意拆家 多次\"" || harness.lastReply() != "已封禁 Steve" {
		test.Fatalf("封禁: calls=%v reply=%q", harness.admin.calls, harness.lastReply())
	}
	harness.commands.Dispatch(boss, "ban Already")
	if !strings.Contains(harness.lastReply(), "已经在封禁名单里") {
		test.Fatalf("重复封禁: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "op Steve")
	if harness.lastReply() != "已将 Steve 设为管理员" {
		test.Fatalf("op: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "op Same")
	if harness.lastReply() != "Same 已经是管理员" {
		test.Fatalf("重复 op: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "deop Steve")
	if harness.lastReply() != "已撤销 Steve 的管理员权限" {
		test.Fatalf("deop: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "deop Same")
	if harness.lastReply() != "Same 不是管理员" {
		test.Fatalf("对非管理员 deop: %q", harness.lastReply())
	}
}

func TestWhitelistSubcommands(test *testing.T) {
	var harness *adminHarness = newHarness()
	var boss *Context = harness.context("Boss", true, false)
	harness.commands.Dispatch(boss, "whitelist on")
	harness.commands.Dispatch(boss, "whitelist add Steve")
	harness.commands.Dispatch(boss, "whitelist list")
	if !strings.Contains(harness.lastReply(), "空") {
		test.Fatalf("空白名单: %q", harness.lastReply())
	}
	harness.admin.whitelist = []string{"alex", "steve"}
	harness.commands.Dispatch(boss, "whitelist list")
	if harness.lastReply() != "白名单（2 人）: alex, steve" {
		test.Fatalf("白名单列表: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "whitelist add")
	if !strings.Contains(harness.lastReply(), "用法") {
		test.Fatalf("缺参数应给用法: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "whitelist nonsense")
	if !strings.Contains(harness.lastReply(), "用法") {
		test.Fatalf("未知子命令应给用法: %q", harness.lastReply())
	}
	if len(harness.admin.calls) != 2 || harness.admin.calls[0] != "whitelist true" || harness.admin.calls[1] != "wl-add Steve" {
		test.Fatalf("调用不符: %v", harness.admin.calls)
	}
}

func TestSayLabelsConsole(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.commands.Dispatch(harness.context("CONSOLE", false, true), "say 服务器即将重启")
	harness.commands.Dispatch(harness.context("Boss", true, false), "say hello")
	harness.commands.Dispatch(harness.context("Boss", true, false), "say")
	if len(harness.admin.calls) != 2 || harness.admin.calls[0] != "say [Server] 服务器即将重启" || harness.admin.calls[1] != "say [Boss] hello" {
		test.Fatalf("控制台显示 Server，玩家显示自己的名字，空消息不广播: %v", harness.admin.calls)
	}
}

// 没挂 Admin 的上下文要报错而不是空指针。
func TestAdminCommandWithoutAdmin(test *testing.T) {
	var commands *Map = NewMap()
	var list []Command = AdminCommands()
	var index int = 0
	for index = 0; index < len(list); index++ {
		commands.Register(list[index])
	}
	var replies []string = nil
	var ctx *Context = &Context{
		Username: "Boss",
		IsOp:     true,
		Reply:    func(msg string) { replies = append(replies, msg) },
	}
	commands.Dispatch(ctx, "kick Steve")
	commands.Dispatch(ctx, "list")
	if len(replies) != 2 || !strings.Contains(replies[0], "未就绪") || !strings.Contains(replies[1], "未就绪") {
		test.Fatalf("缺少 Admin 应报未就绪: %v", replies)
	}
}

func TestTeleportForms(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.admin.positions = map[string][3]float32{"steve": {10, 64, 20}, "alex": {0, 70, 0}}
	var boss *Context = harness.context("Steve", true, false)
	harness.commands.Dispatch(boss, "tp Alex")
	harness.commands.Dispatch(boss, "tp Alex Steve")
	harness.commands.Dispatch(boss, "tp 1 65 2")
	harness.commands.Dispatch(boss, "tp Alex ~ ~5 ~-3")
	var want []string = []string{
		"tp Steve -> Alex",
		"tp Alex -> Steve",
		"tp Steve -> 1.0,65.0,2.0",
		"tp Alex -> 0.0,75.0,-3.0",
	}
	if len(harness.admin.calls) != len(want) {
		test.Fatalf("调用数不符: %v", harness.admin.calls)
	}
	var index int = 0
	for index = 0; index < len(want); index++ {
		if harness.admin.calls[index] != want[index] {
			test.Errorf("第 %d 次: got %q want %q", index, harness.admin.calls[index], want[index])
		}
	}
}

// 控制台省略传送对象时没有「自己」可传，必须报错；指定了对象就可以。
func TestTeleportConsoleNeedsSubject(test *testing.T) {
	var harness *adminHarness = newHarness()
	var console *Context = harness.context("CONSOLE", false, true)
	harness.commands.Dispatch(console, "tp Alex")
	harness.commands.Dispatch(console, "tp 1 2 3")
	if len(harness.admin.calls) != 0 {
		test.Fatalf("控制台不指定对象不该传送: %v", harness.admin.calls)
	}
	if !strings.Contains(harness.lastReply(), "控制台必须指定") {
		test.Fatalf("回复: %q", harness.lastReply())
	}
	harness.admin.positions = map[string][3]float32{"alex": {0, 64, 0}}
	harness.commands.Dispatch(console, "tp Alex 5 64 5")
	if len(harness.admin.calls) != 1 {
		test.Fatalf("指定了对象应放行: %v", harness.admin.calls)
	}
}

// NaN、Inf、越界坐标不能传过去，否则玩家位置变成 NaN 后所有距离判定都失效。
func TestTeleportRejectsBadCoordinates(test *testing.T) {
	var harness *adminHarness = newHarness()
	var boss *Context = harness.context("Boss", true, false)
	var bad []string = []string{
		"tp Steve nan 64 0", "tp Steve 0 64 inf", "tp Steve 0 64 -Inf",
		"tp Steve 99999999 64 0", "tp Steve 0 -100 0", "tp Steve 0 9999 0", "tp Steve abc 64 0",
	}
	var index int = 0
	for index = 0; index < len(bad); index++ {
		harness.commands.Dispatch(boss, bad[index])
	}
	if len(harness.admin.calls) != 0 {
		test.Fatalf("非法坐标不该触达 Admin: %v", harness.admin.calls)
	}
	if len(harness.replies) != len(bad) {
		test.Fatalf("每条非法命令都应有一条回复，实际 %d/%d", len(harness.replies), len(bad))
	}
}

func TestValidateTeleportTarget(test *testing.T) {
	if ValidateTeleportTarget(0, 64, 0) != nil || ValidateTeleportTarget(-30000000, -64, 30000000) != nil || ValidateTeleportTarget(0, 512, 0) != nil {
		test.Fatal("边界值应合法")
	}
	if ValidateTeleportTarget(float32(math.NaN()), 64, 0) == nil {
		test.Fatal("NaN 必须拒绝")
	}
	if ValidateTeleportTarget(0, float32(math.Inf(1)), 0) == nil {
		test.Fatal("Inf 必须拒绝")
	}
	// 注意 float32 在 3e7 附近只能表示偶数，30000001 会被舍入成 30000000 而放行，所以用 30000002。
	if ValidateTeleportTarget(30000002, 64, 0) == nil || ValidateTeleportTarget(0, 513, 0) == nil || ValidateTeleportTarget(0, -65, 0) == nil {
		test.Fatal("越界一格必须拒绝")
	}
}

// /time 作用在执行者当前的世界，控制台落到默认世界。
func TestTimeCommandWorldResolution(test *testing.T) {
	var harness *adminHarness = newHarness()
	var inNether *Context = harness.context("Boss", true, false)
	inNether.World = "nether"
	harness.commands.Dispatch(inNether, "time set noon")
	harness.commands.Dispatch(harness.context("CONSOLE", false, true), "time set night")
	if harness.admin.calls[0] != "time-set nether 6000" || harness.admin.calls[1] != "time-set world 13000" {
		test.Fatalf("世界解析不对: %v", harness.admin.calls)
	}
}

func TestTimeCommandSubcommands(test *testing.T) {
	var harness *adminHarness = newHarness()
	var boss *Context = harness.context("Boss", true, false)
	harness.admin.ticks = 24000*2 + 1500
	harness.commands.Dispatch(boss, "time query")
	if harness.lastReply() != "现在是当天的第 1500 tick" {
		test.Fatalf("query daytime: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "time query day")
	if harness.lastReply() != "现在是第 3 天" {
		test.Fatalf("query day: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "time query gametime")
	if harness.lastReply() != "游戏时间累计 49500 tick" {
		test.Fatalf("query gametime: %q", harness.lastReply())
	}
	harness.commands.Dispatch(boss, "time add -300")
	harness.commands.Dispatch(boss, "time set 100")
	harness.commands.Dispatch(boss, "time stop")
	harness.commands.Dispatch(boss, "time start")
	var want []string = []string{"time-add world -300", "time-set world 100", "time-stopped world true", "time-stopped world false"}
	var index int = 0
	for index = 0; index < len(want); index++ {
		if harness.admin.calls[index] != want[index] {
			test.Errorf("第 %d 次: got %q want %q", index, harness.admin.calls[index], want[index])
		}
	}
}

func TestTimeCommandRejectsBadInput(test *testing.T) {
	var harness *adminHarness = newHarness()
	var boss *Context = harness.context("Boss", true, false)
	var bad []string = []string{"time", "time set", "time set -5", "time set abc", "time add", "time add 99999999999", "time query nonsense", "time nonsense"}
	var index int = 0
	for index = 0; index < len(bad); index++ {
		harness.commands.Dispatch(boss, bad[index])
	}
	if len(harness.admin.calls) != 0 {
		test.Fatalf("非法输入不该改时间: %v", harness.admin.calls)
	}
	if len(harness.replies) != len(bad) {
		test.Fatalf("每条非法输入都应有回复，实际 %d/%d: %v", len(harness.replies), len(bad), harness.replies)
	}
	var missing *Context = harness.context("Boss", true, false)
	missing.World = "missing"
	harness.replies = nil
	harness.commands.Dispatch(missing, "time query")
	if !strings.Contains(harness.lastReply(), "找不到世界") {
		test.Fatalf("世界不存在应提示: %q", harness.lastReply())
	}
}

func TestStopAndSaveAll(test *testing.T) {
	var harness *adminHarness = newHarness()
	var console *Context = harness.context("CONSOLE", false, true)
	harness.commands.Dispatch(console, "save-all")
	harness.commands.Dispatch(console, "stop")
	if len(harness.admin.calls) != 2 || harness.admin.calls[0] != "save" || harness.admin.calls[1] != "stop" {
		test.Fatalf("调用不符: %v", harness.admin.calls)
	}
}

// /help 不向普通玩家展示管理员命令，管理员能看到全部。
func TestHelpFiltersByPermission(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.commands.Dispatch(harness.context("Mallory", false, false), "help")
	var plain string = harness.lastReply()
	if strings.Contains(plain, "/kick") || strings.Contains(plain, "/stop") || strings.Contains(plain, "/whitelist") {
		test.Fatalf("普通玩家不该看到管理员命令:\n%s", plain)
	}
	if !strings.Contains(plain, "/list") || !strings.Contains(plain, "/kill") || !strings.Contains(plain, "/tell") {
		test.Fatalf("普通玩家应看到公共命令:\n%s", plain)
	}
	harness.commands.Dispatch(harness.context("Boss", true, false), "help")
	if !strings.Contains(harness.lastReply(), "/kick") || !strings.Contains(harness.lastReply(), "/whitelist") {
		test.Fatalf("管理员应看到全部命令:\n%s", harness.lastReply())
	}
}

// 别名要能走通，且别名也受权限限制。
func TestAdminAliases(test *testing.T) {
	var harness *adminHarness = newHarness()
	harness.commands.Dispatch(harness.context("Mallory", false, false), "teleport Alex")
	harness.commands.Dispatch(harness.context("Mallory", false, false), "unban Steve")
	if len(harness.admin.calls) != 0 {
		test.Fatalf("别名不能绕过权限: %v", harness.admin.calls)
	}
	harness.commands.Dispatch(harness.context("Mallory", false, false), "msg Steve hi")
	harness.commands.Dispatch(harness.context("Mallory", false, false), "w Steve hi")
	if len(harness.admin.calls) != 2 {
		test.Fatalf("tell 的别名 msg / w 应可用: %v", harness.admin.calls)
	}
}
