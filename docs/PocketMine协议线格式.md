# PocketMine 1.1.x：从 UDP 到游戏字段

与[游戏包索引](PocketMine协议索引.md)配套。下面的字段顺序取自仓库随附的 PHP 分支；方向是所列场景中的发送方，不是说反方向一定没有编码器。C = 客户端，S = 服务端。整套流程见[登录与收发包流程](登录与收发包流程.md)。

## 先分清三层 ID

| 层 | 识别方式 | 处理什么 |
|---|---|---|
| RakNet 离线报文 | UDP 首字节，例如 Ping `0x01`、Pong `0x1C` | 服务器列表显示、握手和 MTU 协商；不是游戏包 |
| RakNet 已连接报文 | 数据报 `0x80–0x8F`；另有 ACK `0xC0`、NACK `0xA0` | 顺序、可靠传输和分片；同一数据报可有多个封装帧 |
| MCPE | 游戏负载 `0xFE` 为 Batch；解压后每个子包有自己的单字节 ID | 例如游戏层 `0x01` 是 Login，`0x1C` 是 EntityEvent |

RakNet 版本 **8** 与游戏协议号 **113** 位于不同层。离线握手依次为 C→S OpenConnectionRequest1 (`05`)、S→C Reply1 (`06`)、C→S Request2 (`07`)、S→C Reply2 (`08`)。建立会话后还有 ConnectionRequest (`09`)、ConnectionRequestAccepted (`10`)、NewIncomingConnection (`13`)；这里的包号均为十六进制。实现：[离线握手](../internal/raknet/offline.go)、[会话](../internal/raknet/session.go)。

### 已连接数据怎么拆

1. [数据报](../internal/raknet/datagram.go)：类型后是 3 字节小端序号，然后是一个或多个帧。ACK/NACK 确认的是数据报序号，不是游戏包 ID。
2. [帧](../internal/raknet/encapsulated.go)：flags 的高 3 位是可靠性模式，位 `0x10` 标记分片；接下来是大端 16 位的**位数**。可靠、顺序、有序模式按需附加消息号、顺序号、排序号和通道；索引是 3 字节小端。分片头依次是总片数（BE32）、分组 ID（BE16）、片号（BE32）。重组后再读游戏负载。
3. [Batch](../internal/protocol/batch.go)：跳过 `FE`，对其余内容做 **zlib** 解压。重复读取 `UVarInt 子包字节数 | 游戏包 ID | 正文`，直到流结束。长度含 ID，不含长度字段自身；没有包数量前缀。

**例：客户端收到正午时间。** `SetTime(6000)` 的游戏包是 `0A E0 5D`：`0A` 是 ID，6000 经 ZigZag 得 12000，再按变长整数编码为 `E0 5D`。若 Batch 只装这一包，其**解压后**的内容为 `03 0A E0 5D`。前导 `03` 是三字节子包的长度，不是握手包号；其中没有冻结标志。

## 字段记法

| 记法 | 线上表示 |
|---|---|
| Byte / Bool | 一个字节；写布尔值时 0 为假、1 为真 |
| VarInt / UVarInt | 每字节 7 个数据位、高位表示继续；前者对有符号数先做 ZigZag，后者不做 |
| LE16 / LE32 / LE64；BE32 | 固定宽度的整数，小端或大端；不可与 VarInt 混用 |
| F32 / Vec3 | 小端单精度；Vec3 依次为 X、Y、Z 三个 F32 |
| String | UVarInt **字节数**，然后是对应字节；也可装二进制，不一定是文本 |
| BlockPos | X: VarInt，Y: **UVarInt**，Z: VarInt；显式 signed 坐标方法的 Y 不同 |
| EntityId | 本分支 PHP 的 `getEntityId/putEntityId` 调用 VarInt；不要按新版协议或字段的英文名字猜编码 |
| UUID | 16 字节；高 64 位在前、低 64 位在后，两段各自小端，不是带连字符的文本 |
| Slot | VarInt 物品 ID；空气到此为止。非空再读 VarInt aux（低 8 位数量、高位 meta）、LE16 NBT 字节数及内容，最后是可放置、可破坏的两组方块名（各自 VarInt 数量 + String 列表） |

来源：[BinaryStream.php](../pocketmine服务器参考/src/pocketmine/utils/BinaryStream.php)、[DataPacket.php](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/DataPacket.php)。特别注意：实体元数据中的坐标类型使用三个有符号 VarInt，不等于上表的 BlockPos。

## 常用包正文（已去掉游戏包 ID）

| ID / 名称 | 场景方向 | 字段，按线上先后顺序 |
|---|---|---|
| 01 Login | C→S | 协议 BE32、版本类别 Byte、登录资料 String。资料内先是 LE32 长度 + 身份链 JSON，再是 LE32 长度 + 客户端 JWT。能解析名字不等于通过正版认证 |
| 02 PlayStatus | S→C | 状态 BE32；登录成功与允许出生是两个不同阶段 |
| 08 ResourcePackClientResponse | C→S | 状态 Byte、条目数 LE16、每个包 ID 为 String |
| 0A SetTime | S→C | 时间 VarInt，以 tick 为单位 |
| 0B StartGame | S→C | 世界部分先写种子、维度、生成器、世界游戏模式、难度（均 VarInt），再写出生点 BlockPos。生成器 1 表示无限地形、2 表示超平坦；维度是独立字段。[完整编码顺序](../internal/protocol/packets/start_game.go) |
| 13 MovePlayer | C↔S | EntityId、位置 Vec3、pitch/yaw/bodyYaw 各 F32、模式 Byte、着地 Bool、骑乘实体 EntityId；传送模式才追加原因 LE32、物品 LE32 |
| 16 UpdateBlock | S→C | BlockPos、方块 ID UVarInt、组合值 UVarInt（flags 左移 4 位，低 4 位为 meta） |
| 19 LevelSoundEvent | C↔S | 声音编号 Byte、位置 Vec3、extraData VarInt、pitch VarInt、两个 Bool。点火用十进制 47（线上 `2F`），extraData=-1、pitch=1、两个标志均为假，所以位置之后是 `01 02 00 00`。客户端播放声音和地图上出现火焰是两件事，方块仍靠 `16 UpdateBlock` 同步 |
| 1C EntityEvent | C↔S | EntityId、事件 Byte、附加值 VarInt；例如进食完成是事件 9，不是游戏包 09 |
| 1D MobEffect | S→C | EntityId、操作 Byte（1 新增、2 修改、3 移除）、效果 ID VarInt、强度索引 VarInt、显示粒子 Bool、持续 tick 数 VarInt |
| 1F MobEquipment | C↔S | EntityId、手持 Slot、背包网络槽 Byte、选中的快捷键 Byte、窗口 ID Byte。主手窗口为 0；网络槽 9–44 对应背包 0–35，255 解除绑定。例：网络槽 33、快捷键 3 表示第 4 个快捷键使用背包下标 24，物品仍在原格；切换时服务端从该格读取实际物品。|
| 23 UseItem | C→S | BlockPos、目标方块 ID UVarInt、面 VarInt、点击偏移 Vec3、玩家位置 Vec3、槽位 VarInt、手持 Slot |
| 30 ContainerOpen | S→C | 窗口 ID Byte、窗口类型 Byte、BlockPos、关联实体 EntityId |
| 32 ContainerSetSlot | C↔S | 窗口 ID Byte、槽位 VarInt、快捷栏槽位 VarInt、Slot、选择标志 Byte |
| 36 CraftingEvent | C→S | 窗口 ID Byte、类型 VarInt、配方 UUID；先输入再输出，每组均为 UVarInt 数量 + Slot 列表。桌面客户端可发长度为零的输入列表；材料来自此前的移槽记录 |
| 38 BlockEntityData | C↔S | BlockPos，然后**剩余全部字节**为方块实体 NBT，没有额外 String 长度 |
| 3A FullChunkData | S→C | 区块 X VarInt、区块 Z VarInt、列数据 String；这里的 X/Z 是区块坐标 |
| 47 ItemFrameDropItem | C→S | 只有 BlockPos，没有 Slot。例如 `47 01 40 04` 请求取出 (-1, 64, 2) 的框中物品。成功后服务端用 `38 BlockEntityData` 同步空框，用 `0F AddItemEntity` 生成掉落物。 |
| 53 ResourcePackDataInfo | S→C | 包 ID String、单块上限 LE32、块数 LE32、压缩包大小 LE64、SHA-256 字节串 String |
| 55 ResourcePackChunkRequest | C→S | 包 ID String、块索引 LE32 |
| 54 ResourcePackChunkData | S→C | 包 ID String、块索引 LE32、进度 LE64、**数据长度 LE32**、数据字节；这里不要用 String 读块数据 |

**两种“区块”不同。** 地图列走 `3A` 和视距协商 `45/46`；资源包文件走 `53 → 55 → 54`。Go 能用空资源包列表完成登录，不等于已支持分块下载。要查其他包的完整字段和条件分支，从[索引](PocketMine协议索引.md)打开对应 PHP 包类，服务端发送看 `encode()`，服务端接收看 `decode()`；空方法不能证明该方向没有字段。

### 3A 区块列里的光照

`FullChunkData` 的列数据从一个子区块数量字节开始，按 Y 从低到高排列。每个版本 0 的子区块占 **10,241 字节**：

| 子区块内字节偏移 | 字段 |
| --- | --- |
| 0 | 版本字节 `00` |
| 1–4096 | 4,096 个方块 ID；索引为 `(x << 8) + (z << 4) + y` |
| 4097–6144 | 方块 meta，每格四位 |
| 6145–8192 | 原始天空光，每格四位，含直射和从侧面传入的光，不扣除当前时刻 |
| 8193–10240 | 火把等光源产生的方块光，每格四位 |

这里的 x、y、z 都是 0–15 的局部坐标。三份四位数组的字节索引都是 `(x << 7) + (z << 3) + (y >> 1)`，偶数 y 在低四位。例如方块光字节 `CD` 表示下格亮度 13、上格 12。列尾依次为高度图（256 个小端 uint16，索引 `z*16+x`）、256 字节群系、边界与额外数据、方块实体网络 NBT。Go 会保留虽无方块但有方块光的子区块；单格更新 `16 UpdateBlock` 本身不带光照数组。

参考：[SubChunk::networkSerialize](../pocketmine服务器参考/src/pocketmine/level/format/SubChunk.php)、[Chunk::networkSerialize](../pocketmine服务器参考/src/pocketmine/level/format/Chunk.php)。

### 从食物到 MobEffect

普通金苹果结算成功后，Go 服务端分别发送再生（效果 ID 10，强度索引 1，100 tick）和吸收（ID 22，强度索引 0，2400 tick）的新增事件。强度从 0 算起，100 tick 是 5 秒。再次施加同种效果时使用修改事件，到期使用移除事件；剩余吸收血量另走 `UpdateAttributes` 的 `minecraft:absorption` 属性，所以效果还在不代表还有满额的额外血量。

### 漏斗搬运时客户端收到什么

漏斗用 `30 ContainerOpen`（窗口类型 8），接着用 `34 ContainerSetContent` 发五格初始内容。自动搬运时无需客户端请求，服务端只向正在看相关容器的玩家发 `32 ContainerSetSlot`；双箱后半的第 2 格对应窗口槽号 29（从零计）。漏斗吸走掉落物用 `0E RemoveEntity` 通知同世界的玩家，没有玩家拾取者。

### 桌面端如何合成四根木棍

1. 玩家把两块木板从背包移进合成格。客户端发 `ContainerSetSlot`，报告原背包槽**移动后的内容**。服务端比较原槽与新槽，把取出的两块木板记在该玩家的悬浮背包中。
2. 玩家拿起合成结果。客户端发 `CraftingEvent`，输入列表为空，输出是木棍。服务端优先查包里的 UUID；若客户端发了另一条已知配方的 UUID，再查具有相同产物、材料足够且当前工作台允许的配方。扣两块木板，把配方规定的四根木棍放入悬浮背包。
3. 玩家把木棍放回背包。新的 `ContainerSetSlot` 增加目标格的数量，服务端同时扣掉悬浮背包中的四根木棍。若直接关窗，Go 端把剩余物品还回背包，放不下的掉在玩家所在位置。

参考入口：[移槽差额](../pocketmine服务器参考/src/pocketmine/inventory/BaseTransaction.php)、[桌面合成与配方纠正](../pocketmine服务器参考/src/pocketmine/Player.php)、[Go 端悬浮背包](../internal/server/floating.go)。带输入格列表的合成仍走原有的格局匹配与背包扣料路径。

## 不经过游戏 Batch 的管理协议

| 协议 | 传输与用途 | 参考端入口 |
|---|---|---|
| Query | UDP 服务器信息查询。握手类型 9 返回带 NUL 终止的挑战值；统计请求类型 0 带会话 ID 和挑战值，可取简略或完整信息。与服务器列表的 RakNet Pong 不同 | [QueryHandler.php](../pocketmine服务器参考/src/pocketmine/network/query/QueryHandler.php) |
| RCON | TCP 远程控制台。帧为 LE32 正文长度、LE32 请求 ID、LE32 类型、负载、两个 NUL；认证和命令响应都使用此帧格式 | [RCONInstance.php](../pocketmine服务器参考/src/pocketmine/network/rcon/RCONInstance.php) |

Go 已支持 Query；RCON 的进度见 [TODO 第 28 项](../TODO.md)。Query 与游戏使用同一个 UDP 端口，没有 RakNet 封装。多字节整数的字节序容易混淆：

| 步骤 | 请求（客户端 → 服务端） | 响应（服务端 → 客户端） |
|---|---|---|
| 获取挑战 | `FE FD 09` + 4 字节会话标识 | `09` + 原会话标识 + 十进制有符号挑战值文本 + NUL |
| 简略统计 | `FE FD 00` + 会话标识 + 挑战值 **BE32** | `00` + 会话标识；接着是 MOTD、SMP/CMP、世界名、在线人数、人数上限（各以 NUL 结尾），然后是端口 **LE16** 和以 NUL 结尾的地址 |
| 完整统计 | 简略请求末尾再加 4 个零字节 | 相同响应头；其后是以 NUL 分隔的键值对，从 `splitnum`、单字节 `80` 开始。最后一个值的终止符之后接 `00 01 player_ 00 00`、逐个以 NUL 终止的玩家名和最后一个 NUL |

例如挑战文本 `-2` 在线上是 `2D 32 00`，回传时必须变成 `FF FF FF FE`。挑战绑定来源 IP；过期后重新握手。完整统计里的 `hostport` 是十进制文本，只有简略统计的端口采用小端整数。
