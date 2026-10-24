package server

import "os"
import "path/filepath"
import "strings"
import "testing"
import "time"

// 登录检查顺序：名字封禁 → IP 封禁 → 白名单；管理员不受白名单限制，但仍受封禁约束。
func TestLoginDenyReasonOrderAndOps(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var now time.Time = time.Now()
	if fixture.srv.loginDenyReason("Anyone", "1.1.1.1", now) != "" {
		test.Fatal("没有任何限制时应放行")
	}
	fixture.srv.whitelistOn.Store(true)
	if fixture.srv.loginDenyReason("Stranger", "1.1.1.1", now) != denyNotWhitelisted {
		test.Fatal("白名单开启后陌生人应被拒")
	}
	var _, _ = fixture.srv.access.Whitelist.Add("Friend")
	if fixture.srv.loginDenyReason("FRIEND", "1.1.1.1", now) != "" {
		test.Fatal("白名单里的人（忽略大小写）应放行")
	}
	var _, _ = fixture.srv.access.Ops.Add("Boss")
	if fixture.srv.loginDenyReason("boss", "1.1.1.1", now) != "" {
		test.Fatal("管理员不受白名单限制")
	}
	var _, banErr = fixture.srv.access.BannedNames.Add("Boss", "x", "违规", time.Time{}, now)
	if banErr != nil {
		test.Fatal(banErr)
	}
	if !strings.Contains(fixture.srv.loginDenyReason("Boss", "1.1.1.1", now), "违规") {
		test.Fatal("管理员被封了照样进不来")
	}
}

// 临时封禁到期后自动放行。
func TestLoginDenyReasonExpiredBan(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var now time.Time = time.Now()
	var _, err = fixture.srv.access.BannedNames.Add("Temp", "x", "冷静一下", now.Add(time.Hour), now)
	if err != nil {
		test.Fatal(err)
	}
	if fixture.srv.loginDenyReason("Temp", "1.1.1.1", now.Add(30*time.Minute)) == "" {
		test.Fatal("到期前应被拒")
	}
	if fixture.srv.loginDenyReason("Temp", "1.1.1.1", now.Add(2*time.Hour)) != "" {
		test.Fatal("到期后应放行")
	}
}

// /op 在线玩家：名单变化、返回规范名字；重复 /op 不算变化；/deop 撤销。
func TestSetOpOnlineAndOffline(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	var admin serverAdmin = fixture.admin()
	var name, changed, err = admin.SetOp("ste", true)
	if err != nil || !changed || name != "Steve" || !fixture.srv.isOp("STEVE") {
		test.Fatalf("op 在线玩家: name=%q changed=%v err=%v", name, changed, err)
	}
	_, changed, err = admin.SetOp("Steve", true)
	if err != nil || changed {
		test.Fatal("重复 op 不应算变化")
	}
	_, changed, err = admin.SetOp("steve", false)
	if err != nil || !changed || fixture.srv.isOp("steve") {
		test.Fatalf("deop: changed=%v err=%v", changed, err)
	}
	name, changed, err = admin.SetOp("Offline", true)
	if err != nil || !changed || name != "Offline" || !fixture.srv.isOp("offline") {
		test.Fatalf("离线玩家也能 op: name=%q changed=%v err=%v", name, changed, err)
	}
}

// 管理员名单落盘，重新打开服务器后还在（防止 /op 只在内存里生效）。
func TestOpsPersistToDisk(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var _, _, err = fixture.admin().SetOp("Boss", true)
	if err != nil {
		test.Fatal(err)
	}
	var reloaded = openAccess(filepath.Dir(fixture.srv.access.Ops.Path()))
	if !reloaded.Ops.Has("boss") {
		test.Fatal("重新读取后管理员应还在")
	}
}

// /whitelist on|off：立即生效并写回 server.properties；写文件失败时开关仍然生效，但报错说明。
func TestSetWhitelistPersists(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var path string = filepath.Join(test.TempDir(), "server.properties")
	var writeErr error = os.WriteFile(path, []byte("# 配置\nwhite-list=off\nmotd=hi\n"), 0o644)
	if writeErr != nil {
		test.Fatal(writeErr)
	}
	fixture.srv.cfg.PropertiesPath = path
	var admin serverAdmin = fixture.admin()
	var err error = admin.SetWhitelist(true)
	if err != nil || !admin.WhitelistEnabled() {
		test.Fatalf("开启白名单: err=%v enabled=%v", err, admin.WhitelistEnabled())
	}
	var raw, _ = os.ReadFile(path)
	if string(raw) != "# 配置\nwhite-list=on\nmotd=hi\n" {
		test.Fatalf("应只改 white-list 一行: %q", raw)
	}
	fixture.srv.cfg.PropertiesPath = filepath.Join(test.TempDir(), "no-dir", "server.properties")
	err = admin.SetWhitelist(false)
	if err == nil || !strings.Contains(err.Error(), "重启后会还原") {
		test.Fatalf("写文件失败应提示重启会还原: %v", err)
	}
	if admin.WhitelistEnabled() {
		test.Fatal("写文件失败时开关本身仍应生效")
	}
	fixture.srv.cfg.PropertiesPath = ""
	if admin.SetWhitelist(true) != nil || !admin.WhitelistEnabled() {
		test.Fatal("没配置路径时只改内存，不报错")
	}
}

func TestWhitelistAddRemoveReload(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var admin serverAdmin = fixture.admin()
	var added, err = admin.WhitelistAdd("Steve")
	if err != nil || !added {
		test.Fatal("添加失败")
	}
	added, err = admin.WhitelistAdd("steve")
	if err != nil || added {
		test.Fatal("重复添加应返回 false")
	}
	var names []string = admin.WhitelistNames()
	if len(names) != 1 || names[0] != "steve" {
		test.Fatalf("名单: %v", names)
	}
	var path string = filepath.Join(filepath.Dir(fixture.srv.access.Whitelist.Path()), "white-list.txt")
	var writeErr error = os.WriteFile(path, []byte("alex\nsteve\n"), 0o644)
	if writeErr != nil {
		test.Fatal(writeErr)
	}
	err = admin.WhitelistReload()
	if err != nil || len(admin.WhitelistNames()) != 2 {
		test.Fatalf("手改文件后 reload 应读到 2 人: %v err=%v", admin.WhitelistNames(), err)
	}
	var removed bool = false
	removed, err = admin.WhitelistRemove("ALEX")
	if err != nil || !removed {
		test.Fatal("移除失败")
	}
}
