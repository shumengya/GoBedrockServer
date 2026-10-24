package access

import "os"
import "path/filepath"
import "strings"
import "testing"
import "time"

// 名字大小写不敏感，增删立即落盘，重开后还在。
func TestNameListPersistsAndIgnoresCase(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "ops.txt")
	var list *NameList = NewNameList(path)
	var added, err = list.Add("  Steve ")
	if err != nil || !added {
		test.Fatalf("首次添加应成功: added=%v err=%v", added, err)
	}
	added, err = list.Add("STEVE")
	if err != nil || added {
		test.Fatalf("大小写不同是同一个人，不应重复添加: added=%v err=%v", added, err)
	}
	if !list.Has("steve") || !list.Has("StEvE") || list.Has("alex") {
		test.Fatal("查询应大小写不敏感")
	}
	var reopened *NameList = NewNameList(path)
	err = reopened.Load()
	if err != nil || !reopened.Has("Steve") {
		test.Fatalf("重开后应读到: err=%v", err)
	}
	var removed bool = false
	removed, err = reopened.Remove("sTeVe")
	if err != nil || !removed || reopened.Has("steve") {
		test.Fatalf("移除失败: removed=%v err=%v", removed, err)
	}
	removed, err = reopened.Remove("steve")
	if err != nil || removed {
		test.Fatal("重复移除应返回 false")
	}
}

// 名字里带换行会把一条写成两条，必须拒绝。
func TestNameListRejectsBadNames(test *testing.T) {
	var list *NameList = NewNameList(filepath.Join(test.TempDir(), "w.txt"))
	var bad []string = []string{"", "   ", "a\nb", "a\rb"}
	var index int = 0
	for index = 0; index < len(bad); index++ {
		var added, err = list.Add(bad[index])
		if err == nil || added {
			test.Errorf("%q 应被拒绝", bad[index])
		}
	}
}

// 兼容参考端格式：每行一个名字，空行忽略，# 开头是注释，带 BOM 的文件也能读。
func TestNameListLoadsReferenceFormat(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "white-list.txt")
	var content string = "\ufeffSteve\r\n\r\n# 注释\r\n  ALEX  \r\n"
	var err error = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	var list *NameList = NewNameList(path)
	err = list.Load()
	if err != nil {
		test.Fatal(err)
	}
	var names []string = list.Names()
	if len(names) != 2 || names[0] != "alex" || names[1] != "steve" {
		test.Fatalf("应读到排好序的 alex、steve: %v", names)
	}
}

func TestNameListMissingFileIsEmpty(test *testing.T) {
	var list *NameList = NewNameList(filepath.Join(test.TempDir(), "none.txt"))
	var err error = list.Load()
	if err != nil || len(list.Names()) != 0 {
		test.Fatalf("文件不存在应视为空名单: err=%v", err)
	}
}

// 保存失败（目录不存在）要报错，并且内存里不能留下「已添加」的假象。
func TestNameListAddRollsBackOnSaveFailure(test *testing.T) {
	var list *NameList = NewNameList(filepath.Join(test.TempDir(), "no-dir", "ops.txt"))
	var added, err = list.Add("Steve")
	if err == nil || added {
		test.Fatal("目录不存在时保存应失败")
	}
	if list.Has("steve") {
		test.Fatal("保存失败后内存名单不该保留这一项，否则重启后 OP 就悄悄没了")
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
}

// 永久封禁：查得到、重复封禁不覆盖原因、解封后查不到。
func TestBanListAddFindRemove(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "banned-players.txt")
	var list *BanList = NewBanList(path)
	var now time.Time = fixedNow()
	var added, err = list.Add("Griefer", "Admin", "拆家", time.Time{}, now)
	if err != nil || !added {
		test.Fatalf("首次封禁应成功: added=%v err=%v", added, err)
	}
	var entry, found = list.Find("GRIEFER", now)
	if !found || entry.Reason != "拆家" || entry.Source != "Admin" {
		test.Fatalf("应查到且大小写不敏感: %+v found=%v", entry, found)
	}
	added, err = list.Add("griefer", "Other", "换个理由", time.Time{}, now)
	if err != nil || added {
		test.Fatal("已封禁的重复封禁应返回 false")
	}
	entry, _ = list.Find("griefer", now)
	if entry.Reason != "拆家" {
		test.Fatalf("重复封禁不该覆盖原来的原因: %q", entry.Reason)
	}
	var removed bool = false
	removed, err = list.Remove("Griefer", now)
	if err != nil || !removed {
		test.Fatalf("解封失败: removed=%v err=%v", removed, err)
	}
	var _, stillBanned = list.Find("griefer", now)
	if stillBanned {
		test.Fatal("解封后不应再查到")
	}
}

// 到期的封禁当作不存在，且落盘时被清掉；到期后可以重新封禁。
func TestBanListExpiry(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "banned-players.txt")
	var list *BanList = NewBanList(path)
	var now time.Time = fixedNow()
	var _, err = list.Add("Temp", "Admin", "", now.Add(time.Hour), now)
	if err != nil {
		test.Fatal(err)
	}
	var _, banned = list.Find("temp", now.Add(30*time.Minute))
	if !banned {
		test.Fatal("到期前应被封")
	}
	var _, stillBanned = list.Find("temp", now.Add(2*time.Hour))
	if stillBanned {
		test.Fatal("到期后应视为未封禁")
	}
	if len(list.Entries(now.Add(2*time.Hour))) != 0 {
		test.Fatal("到期的不该出现在列表里")
	}
	var again bool = false
	again, err = list.Add("Temp", "Admin", "再封", time.Time{}, now.Add(2*time.Hour))
	if err != nil || !again {
		test.Fatalf("到期后应能重新封禁: again=%v err=%v", again, err)
	}
}

// 文件格式和参考端一致：名字|创建时间|来源|到期|原因，永久写 Forever；写出去再读回来不丢信息。
func TestBanListFileFormatRoundTrip(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "banned-ips.txt")
	var list *BanList = NewBanList(path)
	var now time.Time = fixedNow()
	var expires time.Time = now.Add(48 * time.Hour)
	var _, err = list.Add("1.2.3.4", "Admin", "刷屏", time.Time{}, now)
	if err != nil {
		test.Fatal(err)
	}
	_, err = list.Add("5.6.7.8", "Console", "", expires, now)
	if err != nil {
		test.Fatal(err)
	}
	var raw, readErr = os.ReadFile(path)
	if readErr != nil {
		test.Fatal(readErr)
	}
	var text string = string(raw)
	if !strings.Contains(text, "1.2.3.4|2026-05-01 12:00:00 +0000|Admin|Forever|刷屏") {
		test.Fatalf("永久封禁行格式不对:\n%s", text)
	}
	var reopened *BanList = NewBanList(path)
	var skipped, loadErr = reopened.Load()
	if loadErr != nil || len(skipped) != 0 {
		test.Fatalf("重开读取: skipped=%v err=%v", skipped, loadErr)
	}
	var entry, found = reopened.Find("5.6.7.8", now)
	if !found || !entry.Expires.Equal(expires) || entry.Reason != defaultBanReason {
		test.Fatalf("临时封禁往返不一致: %+v found=%v", entry, found)
	}
}

// 读参考端风格的文件：缺省段补默认值，原因里带 | 不能把行拆坏，坏行跳过并报告，好行照常生效。
func TestBanListLoadsReferenceFileAndSkipsBadLines(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "banned-players.txt")
	var content string = "# 头\n" +
		"onlyname\n" +
		"full|2026-01-02 03:04:05 +0800|Admin|Forever|原因里有|竖线\n" +
		"badtime|不是时间|Admin|Forever|x\n" +
		"badexpire|2026-01-02 03:04:05 +0800|Admin|不是时间|x\n" +
		"|2026-01-02 03:04:05 +0800|Admin|Forever|没名字\n"
	var err error = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	var list *BanList = NewBanList(path)
	var skipped []string = nil
	skipped, err = list.Load()
	if err != nil {
		test.Fatal(err)
	}
	if len(skipped) != 3 {
		test.Fatalf("应跳过 3 行坏行（坏时间、坏到期、没名字），实际 %d: %v", len(skipped), skipped)
	}
	var now time.Time = fixedNow()
	var bare, bareFound = list.Find("onlyname", now)
	if !bareFound || bare.Source != defaultBanSource || bare.Reason != defaultBanReason {
		test.Fatalf("只有名字的行应补默认来源和原因: %+v", bare)
	}
	var full, fullFound = list.Find("full", now)
	if !fullFound || full.Reason != "原因里有|竖线" {
		test.Fatalf("原因里的 | 应保留: %+v", full)
	}
}

// 来源和原因里的 | 与换行写入时要清掉，否则一条封禁会变成两行、读回来错位。
func TestBanListSanitizesFields(test *testing.T) {
	var path string = filepath.Join(test.TempDir(), "banned-players.txt")
	var list *BanList = NewBanList(path)
	var now time.Time = fixedNow()
	var _, err = list.Add("Evil", "Ad|min\n", "理由|一\r\n二", time.Time{}, now)
	if err != nil {
		test.Fatal(err)
	}
	var reopened *BanList = NewBanList(path)
	var skipped, loadErr = reopened.Load()
	if loadErr != nil || len(skipped) != 0 {
		test.Fatalf("清洗后文件应能完整读回: skipped=%v err=%v", skipped, loadErr)
	}
	var entry, found = reopened.Find("evil", now)
	if !found || strings.ContainsAny(entry.Source, "|\r\n") || strings.ContainsAny(entry.Reason, "|\r\n") {
		test.Fatalf("字段没清洗干净: %+v", entry)
	}
}

func TestBanListRejectsBadKeys(test *testing.T) {
	var list *BanList = NewBanList(filepath.Join(test.TempDir(), "b.txt"))
	var bad []string = []string{"", "  ", "a|b", "a\nb"}
	var index int = 0
	for index = 0; index < len(bad); index++ {
		var added, err = list.Add(bad[index], "x", "y", time.Time{}, fixedNow())
		if err == nil || added {
			test.Errorf("%q 应被拒绝", bad[index])
		}
	}
}

// 四个文件一次打开；文件都不存在时全是空的。
func TestOpen(test *testing.T) {
	var dir string = test.TempDir()
	var access, warnings, err = Open(dir)
	if err != nil || len(warnings) != 0 {
		test.Fatalf("空目录应能打开: err=%v warnings=%v", err, warnings)
	}
	var added bool = false
	added, err = access.Ops.Add("Boss")
	if err != nil || !added {
		test.Fatal("添加管理员失败")
	}
	_, err = access.BannedNames.Add("Bad", "Boss", "x", time.Time{}, fixedNow())
	if err != nil {
		test.Fatal(err)
	}
	var reopened, _, reopenErr = Open(dir)
	if reopenErr != nil {
		test.Fatal(reopenErr)
	}
	var _, banned = reopened.BannedNames.Find("bad", fixedNow())
	if !reopened.Ops.Has("boss") || !banned {
		test.Fatal("重开后管理员和封禁都应还在")
	}
}

// 封禁文件存在却读不了（用目录冒充）必须报错，不能带着空封禁列表上线。
func TestOpenFailsWhenBanFileUnreadable(test *testing.T) {
	var dir string = test.TempDir()
	var err error = os.Mkdir(filepath.Join(dir, FileBannedNames), 0o755)
	if err != nil {
		test.Fatal(err)
	}
	var _, _, openErr = Open(dir)
	if openErr == nil {
		test.Fatal("封禁文件读不出来时 Open 应失败")
	}
}
