# PocketMine 1.1.x：从 level.dat 到区块列

查游戏客户端实际收到的地图数据，见 [0x3A FullChunkData](PocketMine协议线格式.md#3a-区块列里的光照)。本文讲磁盘上的世界：`level.dat` 描述整张地图，`region/r.<rx>.<rz>.<扩展名>` 才保存各区块里的方块、实体和容器。本服目前使用[自定义 GMR 存档](持久化存储.md)，不能通过修改扩展名来转换地图。

## 仓库里实际有哪几张图

逐个解压并读取随附的三个 `level.dat` 后，结果如下。三份样本都以 `1F 8B 08` 开头（gzip），解压后是**大端、固定宽度**的 NBT，路径是未命名根 Compound → `Data`。

| 世界目录 | 压缩文件字节数 | LevelName（String） | generatorName（String） | RandomSeed（Long） | SpawnX/Y/Z（Int） |
|---|---:|---|---|---:|---|
| [world](../pocketmine服务器参考/worlds/world/level.dat) | 231 | world | normal | 1777430934 | 256 / 70 / 256 |
| [nether](../pocketmine服务器参考/worlds/nether/level.dat) | 228 | nether | hell | 1777430934 | 256 / 70 / 256 |
| [ender](../pocketmine服务器参考/worlds/ender/level.dat) | 224 | ender | ender | 1777430934 | 256 / 70 / 256 |

样本中 `version` 均为 -1，`Time` 和 `DayTime` 均为 0。[BaseLevelProvider](../pocketmine服务器参考/src/pocketmine/level/format/io/BaseLevelProvider.php) 读取世界时钟用的是 **Time**（Long）。磁盘种子也是 Long；即使这三个值都能放进有符号 32 位，也不能据此把其他地图的种子读成 Int。

这三个目录均没有区域文件。它们可以用来核对名称、种子、出生点和生成器；没有已保存的方块可供比对，也不能凭它们验证箱子内容、实体或玩家建筑是否被完整导入。

## 区域文件怎样定位一列

[RegionLoader.php](../pocketmine服务器参考/src/pocketmine/level/format/io/region/RegionLoader.php) 的一个区域覆盖 32 × 32 个**区块**；每个区块横向是 16 × 16 个方块。每扇区 4096 字节。前两扇区是文件头，列数据从第 2 号扇区起（从 0 编号）。

1. 区块坐标 `cx,cz` 各自向下除以 32 得到区域名里的 `rx,rz`；区域内槽号是 `(cx & 31) | ((cz & 31) << 5)`。例如区块 **(-1,32)** 在区域 **(-1,1)**，槽号 **31**。
2. 第一个扇区含 1024 个大端 32 位位置项（每槽 4 字节）。高 24 位为起始扇区，低 8 位为扇区数；全零表示没有这列。第二个扇区按同样顺序存时间戳。
3. 在起始扇区读大端 32 位长度 `L`，再读一字节压缩类型。**L 包含类型字节**，从这里再取 `L-1` 个字节就是压缩负载；扇区尾部的补零不属于负载。类型 1 为 gzip，2 为 zlib。解压后，区块字段在未命名根 Compound 的 `Level` 中；这与 `level.dat` 的 `Data` 路径不同。

例如位置项 `00 00 02 01` 表示列占用第 2 号扇区，共 1 个扇区，列记录从文件字节偏移 8192 起。不要把大端长度作为 MCPE 网络 VarInt 解码。

## 扩展名决定方块数组的轴顺序

下列规格来自本地参考端的读写方法，不是对仓库样本区块的实测。公式中的 x、z 都是列内坐标 0–15。

| 格式与参考代码 | 方块 ID 的存储顺序 | 元数据和高度图 |
|---|---|---|
| [McRegion .mcr](../pocketmine服务器参考/src/pocketmine/level/format/io/region/McRegion.php) | `Level.Blocks` 是 32768 字节；y 为 0–127，索引 `(x << 11) + (z << 7) + y`。没有 Sections 列表 | Data、SkyLight、BlockLight 各 16384 字节，每格 4 位；HeightMap 为 256 个字节 |
| [Anvil .mca](../pocketmine服务器参考/src/pocketmine/level/format/io/region/Anvil.php) | `Level.Sections` 是 Compound 列表；每节的 Y 是节号。令 ly = y & 15，Blocks 为 4096 字节，索引 `(ly << 8) + (z << 4) + x` | 各节的三个半字节数组分别有 2048 字节；HeightMap 为 256 个 Int |
| [PMAnvil .mcapm](../pocketmine服务器参考/src/pocketmine/level/format/io/region/PMAnvil.php) | 同样按 Sections 分节，但直接存参考端内存顺序：`(x << 8) + (z << 4) + ly` | 同样使用每节三个半字节数组和 Int 高度图；半字节也按 PM 的轴顺序排列 |

**最容易漏的差别：** 取 `(x,ly,z)=(2,5,3)`，标准 Anvil 的方块 ID 在节内偏移 **1330**，PMAnvil 在 **565**。前者相邻半字节对应相邻 x，后者对应相邻 ly。只重排 Blocks、不重排 Data，普通石头看不出问题，但有颜色或朝向的方块会读错。[ChunkUtils.php](../pocketmine服务器参考/src/pocketmine/level/format/io/ChunkUtils.php) 分别实现了字节与半字节的重排。

各格式还在 `Level.Entities` 和 `Level.TileEntities` 两个列表保存普通实体与方块实体。仅读写方块 ID 和 meta 不足以还原世界：箱子的方块还在，并不代表箱子里的物品也在。

## 导入到本服

停服后在项目根目录运行以下命令。`-source` 指向含 `level.dat` 的目录，`-dest` 是**尚不存在**的新世界目录；示例假设 `data/worlds` 已在首次启动时创建：

```powershell
go run ./cmd/import-world -source "./pocketmine服务器参考/worlds/world" -dest "./data/worlds/Archive"
```

将 `Archive:normal` 加入现有 `extra-worlds` 后重启，即可用 `/world Archive` 进入。命令不会修改源图或覆盖旧目标。转换先写系统临时目录，再复制到目标；有大量区域文件时，请为临时目录和目标卷留足空间。最终复制阶段遇到磁盘错误会明确报告目标不完整。

三个随附样本导入成功后会报告 **0 列**，因为样本只提供元数据。含 `region` 的世界则逐列导入已有方块、朝向或颜色 meta、群系（包括旧 BiomeColors）。本服重算高度和光照，未保存的地形按 Go 生成器继续生成，不保证新旧边界的地形相同。

当前遇到非空实体、方块实体或待处理刻列表会报告文件与区块并停止，不会只导入箱子外壳。自定义生成器预设、超出 32 位范围的种子、超过 255 的方块 ID 和 LevelDB 也不支持。本工具只做 PocketMine → Go 的转换。
