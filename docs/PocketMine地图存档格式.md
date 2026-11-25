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

区块中的 `TileEntities` 按以下规则转换到本服的 `tiles.json`。坐标是方块的世界坐标；箱子、熔炉和漏斗的 `Items` 是稀疏列表，要读每条记录的 `Slot`，不能拿列表下标当槽号。

| 方块实体 | 导入后保留的内容 |
|---|---|
| 箱子、双箱 | 各半箱自己的 27 格及 `pairx`、`pairz`；物品保留 ID、数量、Damage 和嵌套 tag |
| 熔炉、漏斗 | 熔炉三格、剩余燃烧时间、烹饪进度和燃料总时长；漏斗五格及传输冷却 |
| 告示牌、床、头颅 | 四行原文（含空行）、床色（未写为红色，0 是白色）、头颅类型和旋转 |
| 花盆、物品展示框 | 盆栽 ID 和 meta；框内物品及旋转。仅支持默认掉落概率 1 |
| 末影箱 | 箱体位置；私人物品属于玩家存档，不在区块中，不随世界导入 |

物品的 `tag` 在磁盘上是大端 NBT；进入背包时改为小端、根名变为空串，嵌套列表保留元素类型及顺序。可对照 [Item::nbtDeserialize](../pocketmine服务器参考/src/pocketmine/item/Item.php) 与[背包协议](PocketMine背包与合成协议.md)。

区块中 `Entities` 列表里，`id="Item"` 表示一份掉落物。注意与嵌套的 `Item.id` 区分：前者决定实体种类，后者决定地上的物品是什么。导入器逐条写入本服的 `entities.json`，不合并相同物品。

| 掉落物磁盘字段 | 类型 | 本服读取方式 |
|---|---|---|
| `Pos`、`Motion` | 各 3 个 Double | 世界位置和每刻速度，转成本服运行时的 float32；未写速度时为零 |
| `Rotation`、`OnGround` | 2 个 Float、1 个 Byte | 依次为 yaw、pitch；另存是否着地 |
| `Item` | Compound | 物品 ID、meta、数量和附加 NBT，转换规则与箱内物品相同 |
| `Age`、`PickupDelay` | Short | 已存活的刻数、玩家可拾取前的剩余刻数。32767 表示永久不可由玩家拾取 |

参考端的[掉落物实现](../pocketmine服务器参考/src/pocketmine/entity/Item.php)不承受跌落伤害，因此不迁移累计跌落距离。未实现的归属、药水效果、着火或受伤状态等会报错；不会把带这些状态的物品当作普通物品。落地后的运动和寿命规则见[本服存储说明](持久化存储.md)。

遇到不支持的实体、方块实体或扩展字段（例如容器的 `CustomName`、`Lock`），导入器会指出区域、区块和出错记录，不发布新世界。除掉落物外的普通实体、待处理刻、自定义生成器预设、超出 32 位范围的种子、超过 255 的方块 ID 和 LevelDB 仍不支持。本工具只做 PocketMine → Go 的转换。
