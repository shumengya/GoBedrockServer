package server

import "strings"
import "testing"
import "time"

import "go-mc-server/internal/storage"

func TestStripColors(test *testing.T) {
	if stripColors("§c你没有权限§r使用 §l这个命令") != "你没有权限使用 这个命令" {
		test.Fatalf("应连同样式字符一起去掉: %q", stripColors("§c你没有权限§r使用 §l这个命令"))
	}
	if stripColors("无颜色码") != "无颜色码" || stripColors("") != "" {
		test.Fatal("没有颜色码的文本应原样返回")
	}
	if stripColors("末尾孤立的§") != "末尾孤立的" {
		test.Fatal("末尾孤立的 § 不能越界")
	}
}

// 控制台拥有全部权限：不需要先 op 自己就能执行管理命令。
func TestDispatchConsoleRunsAdminCommands(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	fixture.srv.DispatchConsole("/op Boss")
	fixture.srv.DispatchConsole("ban Griefer 拆家")
	fixture.srv.DispatchConsole("whitelist add Friend")
	fixture.srv.DispatchConsole("   ")
	fixture.srv.DispatchConsole("/")
	if !fixture.srv.isOp("boss") {
		test.Fatal("控制台应能 op 玩家")
	}
	var _, banned = fixture.srv.access.BannedNames.Find("griefer", time.Now())
	if !banned {
		test.Fatal("控制台应能封禁")
	}
	if !fixture.srv.access.Whitelist.Has("friend") {
		test.Fatal("控制台应能改白名单")
	}
	fixture.srv.DispatchConsole("tp Steve 100 70 100")
	fixture.srv.DispatchConsole("time set night")
	fixture.srv.DispatchConsole("kill Steve")
	var steve *Player = fixture.srv.players["uuid-Steve"]
	if posX, _, _ := steve.Position(); posX != 100 {
		test.Fatalf("控制台应能传送玩家: x=%v", posX)
	}
	if fixture.srv.world.TimeTicks() != 13000 {
		test.Fatalf("控制台 /time 作用在默认世界: %d", fixture.srv.world.TimeTicks())
	}
	if !steve.IsDead() {
		test.Fatal("控制台应能击杀玩家")
	}
}

// 控制台没有「自己」：不指定对象的 /kill 不能误杀别人，只能报用法。
func TestConsoleKillWithoutTargetDoesNothing(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	fixture.srv.DispatchConsole("kill")
	if steve.IsDead() {
		test.Fatal("控制台 /kill 不带目标不该杀任何人")
	}
}

func TestRunConsoleReadsEveryLine(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var input string = "op Alpha\r\n/op Beta\n\n  ban Gamma  \nwhitelist add Delta"
	var err error = fixture.srv.RunConsole(strings.NewReader(input))
	if err != nil {
		test.Fatalf("读到 EOF 是正常结束: %v", err)
	}
	if !fixture.srv.isOp("alpha") || !fixture.srv.isOp("beta") {
		test.Fatal("两行 op 都应执行（含 CRLF 与斜杠前缀）")
	}
	var _, banned = fixture.srv.access.BannedNames.Find("gamma", time.Now())
	if !banned || !fixture.srv.access.Whitelist.Has("delta") {
		test.Fatal("缺少结尾换行的最后一行也要执行")
	}
}

// /stop 只发信号，由 main 去走完整的关服流程；重复请求不能 panic。
func TestStopCommandRequestsShutdown(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	select {
	case <-fixture.srv.StopRequested():
		test.Fatal("还没请求时不该触发")
	default:
	}
	fixture.srv.DispatchConsole("stop")
	fixture.srv.DispatchConsole("stop")
	select {
	case <-fixture.srv.StopRequested():
	default:
		test.Fatal("/stop 后应收到停止请求")
	}
	select {
	case <-fixture.srv.stopCh:
		test.Fatal("/stop 只是请求，不该直接结束 tick 循环")
	default:
	}
}

// 关服：先存档，再带提示踢人，等玩家都断开后才返回；重复调用安全。
func TestStopSavesKicksAndReturnsPromptly(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var dataDir string = fixture.srv.cfg.DataDir
	var layoutErr error = storage.EnsureLayout(dataDir, "world")
	if layoutErr != nil {
		test.Fatal(layoutErr)
	}
	layoutErr = storage.EnsureLayout(dataDir, "nether")
	if layoutErr != nil {
		test.Fatal(layoutErr)
	}
	fixture.join("Steve", "1.1.1.1", "world")
	fixture.join("Alex", "1.1.1.2", "world")
	// 模拟真实断开：踢人后玩家从在线表里消失，Stop 才不用等满超时。
	fixture.srv.kickHook = func(player *Player, message string) {
		fixture.kicks = append(fixture.kicks, kickRecord{Name: player.Username, Message: message})
		delete(fixture.srv.players, player.UUID)
	}
	var started time.Time = time.Now()
	fixture.srv.Stop()
	if time.Since(started) > time.Second {
		test.Fatalf("玩家都已断开，Stop 不该等满超时: %v", time.Since(started))
	}
	if len(fixture.kicks) != 2 || fixture.kicks[0].Message != "服务器已关闭" {
		test.Fatalf("应带「服务器已关闭」提示踢出两人: %+v", fixture.kicks)
	}
	var saved, found, err = storage.LoadPlayer(dataDir, "uuid-Steve")
	if err != nil || !found || saved.Name != "Steve" {
		test.Fatalf("关服前应已存下玩家档案: found=%v err=%v", found, err)
	}
	select {
	case <-fixture.srv.stopCh:
	default:
		test.Fatal("Stop 应结束 tick 循环")
	}
	fixture.srv.Stop()
}

// 玩家发的命令走完整的权限链：普通玩家被拒、管理员放行，/kill 只能杀自己。
func TestPlayerCommandPermissionsEndToEnd(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var mallory *Player = fixture.join("Mallory", "1.1.1.1", "world")
	var boss *Player = fixture.join("Boss", "1.1.1.2", "world")
	fixture.join("Steve", "1.1.1.3", "world")
	var _, opErr = fixture.srv.access.Ops.Add("Boss")
	if opErr != nil {
		test.Fatal(opErr)
	}
	fixture.srv.dispatchCommand(mallory.Session, "ban Steve")
	fixture.srv.dispatchCommand(mallory.Session, "op Mallory")
	fixture.srv.dispatchCommand(mallory.Session, "kill Boss")
	if len(fixture.srv.access.BannedNames.Entries(time.Now())) != 0 || fixture.srv.isOp("mallory") || boss.IsDead() {
		test.Fatal("普通玩家的越权命令都应被拒绝")
	}
	fixture.srv.dispatchCommand(boss.Session, "ban Steve 作弊")
	var _, banned = fixture.srv.access.BannedNames.Find("steve", time.Now())
	if !banned || len(fixture.kicks) != 1 {
		test.Fatalf("管理员应能封禁并踢人: banned=%v kicks=%v", banned, fixture.kicks)
	}
	fixture.srv.dispatchCommand(mallory.Session, "kill")
	if !mallory.IsDead() {
		test.Fatal("任何玩家都能 /kill 自己")
	}
}

// /time 作用在执行者所在的世界，别的世界不受影响。
func TestPlayerTimeCommandUsesOwnWorld(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var boss *Player = fixture.join("Boss", "1.1.1.1", "nether")
	var _, opErr = fixture.srv.access.Ops.Add("Boss")
	if opErr != nil {
		test.Fatal(opErr)
	}
	fixture.srv.dispatchCommand(boss.Session, "time set noon")
	if fixture.srv.worlds["nether"].TimeTicks() != 6000 {
		test.Fatalf("地狱时间应为 6000: %d", fixture.srv.worlds["nether"].TimeTicks())
	}
	if fixture.srv.world.TimeTicks() != 0 {
		test.Fatalf("主世界时间不该被改: %d", fixture.srv.world.TimeTicks())
	}
	fixture.srv.dispatchCommand(boss.Session, "time stop")
	if !fixture.srv.worlds["nether"].TimeStopped() || fixture.srv.world.TimeStopped() {
		test.Fatal("只应冻结地狱的时间")
	}
}

func TestPVPSwitch(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var attacker *Player = fixture.join("A", "1.1.1.1", "world")
	var victim *Player = fixture.join("B", "1.1.1.2", "world")
	if fixture.srv.attackRejectReason(attacker, victim) != "" {
		test.Fatal("PVP 开启且距离合适时应放行")
	}
	victim.X = 100
	if fixture.srv.attackRejectReason(attacker, victim) == "" {
		test.Fatal("超出攻击距离应拒绝")
	}
	victim.X = 0
	fixture.srv.cfg.PVP = false
	if fixture.srv.attackRejectReason(attacker, victim) != pvpDisabledReason {
		test.Fatal("PVP 关闭后应拒绝并给出专用原因")
	}
}
