# PocketMine 1.1.x：实体生成、元数据与属性

适用于仓库参考端的游戏协议 113。收到一个实体包时，先按包号决定布局：`0D AddEntity` 创建非玩家实体，`27 SetEntityData` 修改名称、姿态等元数据，`1E UpdateAttributes` 修改血量、吸收量等数值。下文都是去掉 Batch 封装后的游戏包；基础类型见[协议线格式](PocketMine协议线格式.md)。

## 生成实体时怎么读

[AddEntityPacket.php](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddEntityPacket.php) 的正文按以下次序排列：

1. 唯一 ID、运行时 ID（均为 EntityId）。该 PHP 编码器在两个位置写入相同的 `eid`；读包时仍要读两次。
2. 实体类型 UVarInt、位置 Vec3、速度 Vec3、俯仰角 F32、偏航角 F32。
3. 属性条数 UVarInt；逐条读 **名称 String → 最小值 F32 → 当前值 F32 → 最大值 F32**。
4. 一整组实体元数据，格式见下表。
5. 关联条数 UVarInt；每条读两个 EntityId 和一个表示关联类型的 Byte。没有关联也要读条数。

玩家出现走 `0C AddPlayer`，地面物品出现走 `0F AddItemEntity`。查这两种包从[游戏包索引](PocketMine协议索引.md)进入，不要按 `0D` 的头部去读物品栈或玩家名。

## 元数据由键和类型共同决定

`SetEntityData` 的正文是 **EntityId → 条数 UVarInt → 条目**。每条是 **键 UVarInt → 类型 UVarInt → 值**。键表示用途，类型决定接下来消耗多少字节。没有逐条长度，也没有结尾标志；遇到未知类型时，不能假定跳过一个字节就能找到下一条。

| 类型号 | 类型 | 值的线上表示 |
|---|---|---|
| 0 | BYTE | 1 字节 |
| 1 | SHORT | 有符号小端 16 位整数 |
| 2 | INT | 有符号 VarInt |
| 3 | FLOAT | 小端 F32 |
| 4 | STRING | UVarInt 字节数，然后是字符串内容 |
| 5 | SLOT | 一个完整物品栈，空气只有物品 ID |
| 6 | POS | X、Y、Z 各一个有符号 VarInt；这里的 Y 可以为负 |
| 7 | LONG | 有符号 VarLong，使用 ZigZag |
| 8 | VECTOR3F | X、Y、Z 各一个小端 F32 |

**例：实体 1 仅设置潜行标志。** 游戏包为 `27 02 01 00 07 04`：`27` 是包号，`02` 是实体 ID 1，`01` 是一条元数据，`00` 是 FLAGS 键，`07` 表示 LONG。潜行占第 1 位，所以标志值为 2，经 ZigZag 编为 `04`。FLAGS 的值是整组位；如果实体同时在燃烧，应保留第 0 位，值变成 3，线上末字节为 `06`。

依据：[键与标志位](../pocketmine服务器参考/src/pocketmine/entity/Entity.php)、[元数据读写](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/DataPacket.php)、[SetEntityData 包](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetEntityDataPacket.php)。

## 属性更新与生成包的区别

| 包 | 条数前面的字段 | 每条属性（严格按顺序） |
|---|---|---|
| `0D AddEntity` | 实体头部、位置、速度和朝向 | 名称、最小值、**当前值**、最大值 |
| `1E UpdateAttributes` | EntityId | 最小值、**最大值**、当前值、默认值、名称 |

两处条数都是 UVarInt，名称都是 String，数值都是 F32。若生命条读出来异常，先核对实际包号，以及是否把生成包的属性解码器复用到了更新包。金苹果同时涉及两条独立信息：效果剩余时间走 `1D MobEffect`，剩余吸收血量走 `1E UpdateAttributes` 的 `minecraft:absorption`。见[食物效果示例](PocketMine协议线格式.md#从食物到-mobeffect)和[属性更新编码器](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/UpdateAttributesPacket.php)。

**Go 实现范围：** [AddEntity](../internal/protocol/packets/add_entity.go) 发包时写空属性和空关联列表，解码只读到元数据条数；[元数据编码器](../internal/protocol/packets/entity_metadata.go) 支持上表的标量，尚不能写 SLOT、POS 或任意三分量的 VECTOR3F。[UpdateAttributes](../internal/protocol/packets/update_attributes.go) 已实现发送；其空的 Decode 方法不能用于解析抓包。
