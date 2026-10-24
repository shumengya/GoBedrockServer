package server

import "errors"
import "strings"
import "testing"
import "time"

import "go-mc-server/internal/command"

// 在线玩家被封：写进名单并立刻踢下线，踢出提示带上原因。前缀能命中唯一的人。
func TestBanOnlinePlayerKicksAndRecords(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	fixture.join("Alex", "1.1.1.2", "world")
	var name, added, err = fixture.admin().Ban("ste", "Boss", "恶意拆家")
	if err != nil || !added || name != "Steve" {
		test.Fatalf("封禁结果: name=%q added=%v err=%v", name, added, err)
	}
	if len(fixture.kicks) != 1 || fixture.kicks[0].Name != "Steve" || !strings.Contains(fixture.kicks[0].Message, "恶意拆家") {
		test.Fatalf("应只踢 Steve 且带原因: %+v", fixture.kicks)
	}
	var entry, banned = fixture.srv.access.BannedNames.Find("steve", time.Now())
	if !banned || entry.Source != "Boss" {
		test.Fatalf("名单里应有 Steve，来源 Boss: %+v", entry)
	}
}

// 玩家不在线也能封，按字面名字写进名单，之后登录被拦。
func TestBanOfflineNameBlocksLogin(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var name, added, err = fixture.admin().Ban("  Griefer ", "Boss", "")
	if err != nil || !added || name != "Griefer" {
		test.Fatalf("离线封禁: name=%q added=%v err=%v", name, added, err)
	}
	if len(fixture.kicks) != 0 {
		test.Fatal("离线玩家不需要踢人")
	}
	var reason string = fixture.srv.loginDenyReason("GRIEFER", "9.9.9.9", time.Now())
	if !strings.Contains(reason, defaultBanText) {
		test.Fatalf("未写原因应使用默认原因，实际 %q", reason)
	}
	var _, again, _ = fixture.admin().Ban("griefer", "Boss", "")
	if again {
		test.Fatal("重复封禁应返回 added=false")
	}
}

// 前缀有歧义时拒绝封禁，不能挑一个封掉。
func TestBanAmbiguousPrefixRefused(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	fixture.join("Stella", "1.1.1.2", "world")
	var _, _, err = fixture.admin().Ban("Ste", "Boss", "x")
	if !errors.Is(err, command.ErrAmbiguousName) {
		test.Fatalf("应报歧义: %v", err)
	}
	if len(fixture.kicks) != 0 || len(fixture.srv.access.BannedNames.Entries(time.Now())) != 0 {
		test.Fatal("歧义时不能踢人也不能写名单")
	}
}

// 按在线玩家封 IP：同 IP 的所有人一起踢，别的 IP 不受影响。
func TestBanIPByPlayerKicksEveryoneOnThatAddress(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "5.5.5.5", "world")
	fixture.join("Alt", "5.5.5.5", "world")
	fixture.join("Alex", "6.6.6.6", "world")
	var address, added, err = fixture.admin().BanIP("steve", "Boss", "多开刷屏")
	if err != nil || !added || address != "5.5.5.5" {
		test.Fatalf("封 IP: address=%q added=%v err=%v", address, added, err)
	}
	var kicked map[string]bool = map[string]bool{}
	var names []string = fixture.kickedNames()
	var index int = 0
	for index = 0; index < len(names); index++ {
		kicked[names[index]] = true
	}
	if len(kicked) != 2 || !kicked["Steve"] || !kicked["Alt"] || kicked["Alex"] {
		test.Fatalf("应踢 Steve 和 Alt，不踢 Alex: %v", names)
	}
	var reason string = fixture.srv.loginDenyReason("Newcomer", "5.5.5.5", time.Now())
	if !strings.Contains(reason, "多开刷屏") {
		test.Fatalf("该 IP 之后登录应被拒: %q", reason)
	}
	if fixture.srv.loginDenyReason("Newcomer", "6.6.6.6", time.Now()) != "" {
		test.Fatal("别的 IP 不受影响")
	}
}

// 直接写 IP；写法不同的同一个 IPv6 要归一化成一个，否则封了 A 写法，用 B 写法照样进得来。
func TestBanIPLiteralAndNormalization(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var admin serverAdmin = fixture.admin()
	var address, _, err = admin.BanIP("2001:DB8:0:0:0:0:0:1", "Boss", "")
	if err != nil || address != "2001:db8::1" {
		test.Fatalf("IPv6 应归一化: %q %v", address, err)
	}
	if fixture.srv.loginDenyReason("X", "2001:db8::1", time.Now()) == "" {
		test.Fatal("规范写法应被拦")
	}
	var _, _, repeatErr = admin.BanIP("2001:db8::0001", "Boss", "")
	if repeatErr != nil {
		test.Fatalf("再用别的写法封同一个: %v", repeatErr)
	}
	if len(fixture.srv.access.BannedIPs.Entries(time.Now())) != 1 {
		test.Fatal("不同写法的同一个 IP 只该有一条封禁")
	}
	var _, _, badErr = admin.BanIP("not-an-ip-or-player", "Boss", "")
	if badErr == nil {
		test.Fatal("既不是 IP 也不是在线玩家应报错")
	}
}

// 玩家名恰好长得像 IP 前缀时，输入完整 IP 必须当 IP 处理，不能被前缀匹配成玩家。
func TestBanIPPrefersLiteralAddressOverPlayerName(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("10", "7.7.7.7", "world")
	var address, _, err = fixture.admin().BanIP("10.0.0.1", "Boss", "")
	if err != nil || address != "10.0.0.1" {
		test.Fatalf("应按字面 IP 封: %q %v", address, err)
	}
	if len(fixture.kicks) != 0 {
		test.Fatalf("没人在这个 IP 上，不该踢任何人: %v", fixture.kickedNames())
	}
}

func TestPardonAndPardonIP(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var admin serverAdmin = fixture.admin()
	var _, _, banErr = admin.Ban("Bad", "Boss", "x")
	if banErr != nil {
		test.Fatal(banErr)
	}
	var _, _, ipErr = admin.BanIP("3.3.3.3", "Boss", "x")
	if ipErr != nil {
		test.Fatal(ipErr)
	}
	var removed, err = admin.Pardon("BAD")
	if err != nil || !removed {
		test.Fatalf("解封玩家: removed=%v err=%v", removed, err)
	}
	removed, err = admin.Pardon("bad")
	if err != nil || removed {
		test.Fatal("重复解封应返回 false")
	}
	removed, err = admin.PardonIP(" 3.3.3.3 ")
	if err != nil || !removed {
		test.Fatalf("解封 IP: removed=%v err=%v", removed, err)
	}
	if fixture.srv.loginDenyReason("Bad", "3.3.3.3", time.Now()) != "" {
		test.Fatal("解封后应能登录")
	}
}

// 访问控制没挂上时（理论上不会发生）要报错，而不是空指针。
func TestAdminWithoutAccessReportsError(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.srv.access = nil
	var _, _, err = fixture.admin().Ban("X", "Boss", "")
	if err == nil {
		test.Fatal("缺访问控制应报错")
	}
	if fixture.srv.isOp("anyone") {
		test.Fatal("缺访问控制时不能有任何人是管理员")
	}
	if fixture.srv.loginDenyReason("X", "1.1.1.1", time.Now()) != "" {
		test.Fatal("缺访问控制时登录检查应放行而不是崩溃")
	}
}
