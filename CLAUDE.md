# Go MC Server

用 Go 实现的 Minecraft Bedrock 1.1.x 服务端。模块名 `go-mc-server`，Go 1.25。结构见 README.md，待办见 TODO.md，方块行为的设计见 docs/方块行为.md。

通用的 Go 代码规范沿用全局 CLAUDE.md，这里只写本项目特有的约定和踩过的坑。这份文件会过时：发现和代码对不上，当场改，不要留着。

## 常用命令

```bash
go build -o go-mc-server.exe .
go test ./...
go test ./internal/world/ -run TestClock -v   # 只跑某个包或某个用例
go run ./scripts/gen_pm_blockprops            # 重新生成方块属性表，在项目根目录运行
```

## 架构约定

- 方块行为的规则在 `internal/blockrule`，是纯逻辑：只通过 `View` 读世界，返回要改的格子，不碰网络、实体、锁。落地、广播、掉落、加锁在 `internal/server`。加新行为的步骤见 docs/方块行为.md。
- 服务端有两种 `View`。`loadingView` 读到没加载的区块会把它加载出来，只给玩家主动的操作用（放置，挖掘时找另一半）；`peekView` 只读内存里已有的区块。邻居更新、定时任务、随机刻、下落、液体这类没人主动触发的逻辑必须用 `peekView`，否则一次更新会把相邻的空区块一片片生成出来。
- `Server.blockMu` 必须最先拿，里面再拿 `mu`、玩家锁、世界锁，反过来会死锁。带 `Locked` 后缀的函数要求调用方已经持有它。
- `Player.Y` 存的是眼睛位置，不是脚底。算距离、脚下踩的是哪一格，都要减眼高 `playerEyeHeight`。
- 测试里的 `Server` 是字面量构造的，不走 `New`。给 `Server` 加字段必须保证零值可用，map 用到时再建。
- `internal/blockrule/props_table.go` 是生成的，别手改，改 `scripts/gen_pm_blockprops` 后重新生成。依赖 meta 的属性写在 `props.go`。

## 移植参考端的行为

- 行为以 `pocketmine服务器参考/` 里的 PHP 为准，但它不全对：注释会和代码不符（朝向的南西北东），个别类的继承是错的（158 号木台阶误继承了台阶的燃烧判断），常量有的写 `self::` 有的写 `Block::`。冲突时以向量、公式和原版实际行为为准，偏离的地方记到 docs/方块行为.md 的「和参考端的差异」。
- 读 PHP 方法要读全文。用关键字过滤着读，会把 `canFlowInto`、`isSolid` 这类判断行一起滤掉，读到的是缺行的假逻辑。
- 测试里的预期值照参考端的公式手算，不要从自己的实现反推。

## 测试约定

### 放哪里

- **单元测试和代码放同一目录、同一个包**，文件名 `xxx_test.go`。这是 Go 的主流做法（标准库、Kubernetes 都这样）。不要为了好看把测试搬进 `test/` 子目录：另一个目录就是另一个包，看不到小写的未导出符号，为此把符号改大写会污染业务代码。
- 文件名尽量和被测源文件对应（`clock.go` 对应 `clock_test.go`）。`.vscode/settings.json` 靠同名把测试折叠到源文件下面，名字对不上就折叠不了。
- 集成或端到端测试（起真服务器、用 RakNet 客户端连）放顶层 `test/`，文件头加 `//go:build integration`，平时 `go test ./...` 不会跑，需要时执行 `go test -tags integration ./test/...`。目录尚未建立，有第一个集成测试时再建。

### 怎么写

- 纯逻辑用表驱动，一个函数一张表。写法参考 `internal/blockrule/helpers_test.go` 的 `runPlaceCases` 和 `place_test.go`。
- 表驱动的用例切片在构造时就全部求值。几条用例共用同一个视图或 map，又在字面量里调 `.set(...)`、`.unload(...)`，会在任何一条运行前就改掉它，用例之间互相污染。每条用例各自建视图。
- 夹具：`blockrule` 用 `helpers_test.go` 里的 `mapView` 搭小世界；要走整条链路用 `internal/server/place_test.go` 的 `placeTestSetup`。平坦世界的地面很低，玩家夹具站得比地面高得多，测试要自己在手够得着的地方垫支撑方块。
- 公共的构造函数和夹具放在该包的 `helpers_test.go`，或像 `internal/server/admin_helpers_test.go` 那样按主题命名，别在多个测试文件里各抄一份。
- 测试用的数据文件放包内 `testdata/`。要写磁盘用 `test.TempDir()`，不要写到 `data/`。
- 新增测试能黑盒就黑盒（`package xxx_test`，只用导出 API）。必须碰少数内部符号时，在同包建 `export_test.go`，只在测试编译时导出。
- 每个测试前一行写中文注释，说清楚它保证了什么行为；失败信息用「期望…实际…」的格式。参数名沿用 `test *testing.T`。

### 新测试一次就过，先别信

加完测试做变异验证：临时改坏被测代码，确认对应测试会失败，再恢复。几个容易做废的地方：

- 要替换的目标文本必须恰好命中一次，命中 0 次或多次都要停下。
- 改坏之后必须能编译。多出一个未使用的变量导致编译失败，不算验证。
- 用 `try/finally` 恢复，做完看 `git status` 和 `git diff`，确认没有残留。
- 没抓到有两种可能：测试有洞，补测试；或者这段代码本来就多余（两处检查重复、状态后面会被重新算出来）。逐条推演清楚再选，别想当然，确认多余就删代码并重新验证。

### 包变大怎么办

目录臃肿的根源是包太大，所以拆包，不要拆测试。`internal/blockrule` 就是从 `server` 里拆出来的先例：规则放进纯逻辑包，测试黑盒。`internal/server` 里还有自成一体的纯逻辑（区块排序、材料统计、食物规则等），改到相关代码时顺手抽出去，不要一次性大重构。

## Windows / PowerShell 的坑

- 字符串里写 `"$name:"` 会被当成带作用域的变量（像 `$env:`），整条命令不执行。要写 `"${name}:"`。
- `-match` 默认不分大小写，要区分用 `-cmatch`，比如筛小写开头的未导出符号。
- 从 PowerShell 写 Go 文件用 `[IO.File]::WriteAllText(路径, 内容, (New-Object System.Text.UTF8Encoding($false)))`，别用 `Out-File`、`Set-Content`，会带 BOM 或 CRLF。here-string 追加内容常常少末尾换行，写完跑 `gofmt`。
- `gofmt -w` 只对自己改过的文件跑。有些文件原本就没按 gofmt 排版，整目录跑会带进无关改动。
- 有的文件在工作区是 CRLF 换行，而 HEAD 里是 LF。按 LF 写的替换串会命中 0 次，先用 `git ls-files --eol 路径` 看换行，再决定怎么匹配。
- 同一批并行发出的工具调用没有先后保证。改文件、`git add`、构建、测试之间有依赖的，要分开发，或者放进同一条命令里顺序执行。

## Go 规范里最容易漏的

全局规则已经写全了，这几条在本项目里实际犯过：

- 标识符至少 3 个字母，`at`、`ok` 这种也不行（`id` 例外）。
- 三段式 for 三段都要写，`for ; cond; post` 不算。遍历 map 先 `slices.Sorted(maps.Keys(...))`。
- 零值也要写出来：`var group sync.WaitGroup = sync.WaitGroup{}`，不能只写 `var group sync.WaitGroup`。
- 提交前 grep 一遍 `:=`、`range` 和两个字母的名字。

## 仓库约定

- `pocketmine服务器参考/` 是只读的参考资料，不要改。它和 `*.exe`、`data/`、`ops.txt` 等运行时文件都已在 `.gitignore` 里。
- 提交按路径 `git add`，别用 `git add .`。
- 勾 TODO 要在 `git add` 之前改完，提交前看一眼 `git diff --cached`，确认勾选和代码文件都在暂存区。
- 没有真实客户端可测。涉及朝向、meta 位布局、动画的改动，提交说明里如实写明只验证到服务端逻辑。
