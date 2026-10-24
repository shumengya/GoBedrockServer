# Go MC Server

用 Go 实现的 Minecraft Bedrock 1.1.x 服务端。模块名 `go-mc-server`，Go 1.25。结构见 README.md，待办见 TODO.md。

通用的 Go 代码规范沿用全局 CLAUDE.md，这里只写本项目特有的约定。

## 常用命令

```bash
go build -o go-mc-server.exe .
go test ./...
go test ./internal/world/ -run TestClock -v   # 只跑某个包或某个用例
```

## 测试约定

### 放哪里

- **单元测试和代码放同一目录、同一个包**，文件名 `xxx_test.go`。这是 Go 的主流做法（标准库、Kubernetes 都这样）。不要为了好看把测试搬进 `test/` 子目录：另一个目录就是另一个包，看不到小写的未导出符号，为此把符号改大写会污染业务代码。
- 文件名尽量和被测源文件对应（`clock.go` 对应 `clock_test.go`）。`.vscode/settings.json` 靠同名把测试折叠到源文件下面，名字对不上就折叠不了。
- 集成或端到端测试（起真服务器、用 RakNet 客户端连）放顶层 `test/`，文件头加 `//go:build integration`，平时 `go test ./...` 不会跑，需要时执行 `go test -tags integration ./test/...`。目录尚未建立，有第一个集成测试时再建。

### 怎么写

- 纯逻辑用表驱动，一个函数一张表，别为每组数据写一个 `TestXxx`。
- 公共的构造函数和夹具放在该包的 `helpers_test.go`，或像 `internal/server/admin_helpers_test.go` 那样按主题命名，别在多个测试文件里各抄一份。
- 测试用的数据文件放包内 `testdata/`，Go 工具链会自动忽略这个目录名。
- 要写磁盘用 `test.TempDir()`，不要写到 `data/`。
- 新增测试能黑盒就黑盒（`package xxx_test`，只用导出 API）。必须碰少数内部符号时，在同包建 `export_test.go`，只在测试编译时导出，不污染正式 API。
- 每个测试前一行写中文注释，说清楚它保证了什么行为；失败信息用「期望…实际…」的格式。
- 参数名沿用现有的 `test *testing.T`。

表驱动写法（已在本项目跑通）：

```go
// 回绕点以内原样保留，到回绕点归零。
func TestNormalizeTicks(test *testing.T) {
	type tickCase struct {
		Name   string
		Input  int64
		Expect int32
	}
	var cases []tickCase = []tickCase{
		{Name: "未到回绕点原样保留", Input: 100, Expect: 100},
		{Name: "到回绕点归零", Input: int64(TimeFull) * tickWrapDays, Expect: 0},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item tickCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var actual int32 = normalizeTicks(item.Input)
			if actual != item.Expect {
				sub.Fatalf("normalizeTicks(%d) 期望 %d，实际 %d", item.Input, item.Expect, actual)
			}
		})
	}
}
```

### 包变大怎么办

目录臃肿的根源是包太大，所以拆包，不要拆测试。

`internal/server` 是目前最大的包（登录、区块、战斗、饥饿、合成、管理都在里面），它的测试大多依赖未导出符号。改到相关代码时，顺手把自成一体的纯逻辑（区块排序、材料统计、食物规则等）抽成小包。抽出后符号自然导出，测试也能黑盒写。不要一次性大重构。

## 仓库约定

- `pocketmine服务器参考/` 是只读的参考资料（约 174MB），不要改、不要提交。
- `go-mc-server.exe` 是构建产物，不提交。
- `data/`、`ops.txt`、`white-list.txt`、`banned-*.txt` 是运行时数据，已在 `.gitignore` 里。
- 提交时按路径 `git add`，别用 `git add .`，否则会把上面这些带进去。
