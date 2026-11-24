# PocketMine 1.1.x：背包、容器与合成

适用仓库附带的 MCPE 113 参考端。包号均为十六进制，槽位、物品 ID 和数量如无特别说明均为十进制。先从[游戏包索引](PocketMine协议索引.md)找包类；VarInt、BlockPos、UUID 的通用读法见[线格式](PocketMine协议线格式.md)。

## 一件物品在包里占几个字节

Slot 长度不固定。先读有符号 VarInt 物品 ID；空气写一个 `00` 就结束。非空气依次读取：

1. VarInt `aux`。数量是 `aux & 255`，meta 为 `aux >> 8`；meta 等于 `0x7FFF` 时，[PHP 解码器](../pocketmine服务器参考/src/pocketmine/utils/BinaryStream.php)将其解释为通配值 -1。
2. LE16 的 NBT **字节数**，然后读对应长度的原始字节。无标签也要保留两个零字节。
3. canPlaceOn、canDestroy 两个列表。各先读 VarInt 条目数，再按数量读 String。参考端发送时两个数量都写 0。

**例：一个普通水桶**（325:8，数量 1）。ID 作 ZigZag 得 650，写作 `8A 05`；aux 是 `(8 << 8) | 1 = 2049`，编码为 `82 20`。这个 Slot 完整是 `8A 05 82 20 00 00 00 00`。末尾四个零依次是两字节的零长度和两个空列表，不能省略。

## 开箱与移槽

玩家打开箱子时，服务端先发 `30 ContainerOpen` 建立窗口，再发 `34 ContainerSetContent` 填入内容。只改变一格时用 `32 ContainerSetSlot`。窗口号标识这一次打开的界面，窗口类型告诉客户端怎样绘制它；相同窗口号在下次打开时可以用在别的容器。

| ID / 名称 | 常见方向 | 字段顺序或用法 |
|---|---|---|
| 30 ContainerOpen | 服务端 → 客户端 | windowId: Byte，type: Byte，BlockPos，关联 EntityId。Go 用类型 0 显示箱子、2 显示熔炉、8 显示漏斗 |
| 31 ContainerClose | 双向 | windowId: Byte。关闭操作不带物品列表 |
| 32 ContainerSetSlot | 双向 | windowId: Byte，slot: VarInt，hotbarSlot: VarInt，物品 Slot，selectSlot: Byte。Slot 是该格更新后的内容，不是要增减的数量 |
| 33 ContainerSetData | 服务端 → 客户端 | windowId: Byte，property: VarInt，value: VarInt。Go 熔炉属性 0 是烤制进度，1 是火焰显示值；物品改动仍走槽位包 |
| 34 ContainerSetContent | 服务端 → 客户端 | windowId: **UVarInt**，目标 EntityId，物品格数 UVarInt，依次写 Slot，末尾写快捷栏映射数 UVarInt 及映射 VarInt。参考端对非背包窗口也写一个零；其解码器只对窗口 0 读取这段尾部 |
| 1F MobEquipment | 双向 | EntityId，手持 Slot，网络背包槽 Byte，选中快捷键 Byte，windowId: Byte。切换手持物看这个包 |
| 20 MobArmorEquipment | 装备外观同步 | EntityId，然后依次为头、胸、腿、脚四个 Slot；**没有**数量前缀。自己移动盔甲时看窗口 0x78 的槽位包 |
| 2E DropItem | 客户端 → 服务端 | type: Byte，丢弃的 Slot；地上的掉落实体另用 0F AddItemEntity 广播 |
| 2F InventoryAction | 参考端定义 | actionId: UVarInt，Slot，附魔 ID 和等级（各 VarInt）。动作 0 和 2 分别命名为 GIVE_ITEM 和 ENCHANT_ITEM；参考端两种动作都编解码后两个字段。Go 实现状态见索引 |

**三个容易混的下标。** 主背包窗口号为 0。Go 的背包全量包发 36 个实格，再补 9 个空气占位；窗口 0 的单格包使用实格下标 0–35。装备包另有网络槽号：例如实际背包下标 24 在 `32` 中写 24，在 `1F` 的网络槽字节及默认约定的映射值中写 33。快捷键本身仍是 0–8；解绑时映射是 VarInt -1，装备包网络槽字节是 255。参见 [Go 槽位映射](../internal/protocol/inventory.go) 与 [PHP 玩家背包](../pocketmine服务器参考/src/pocketmine/inventory/PlayerInventory.php)。

特殊窗口常量在 [ContainerSetContentPacket.php](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerSetContentPacket.php)：0x78 盔甲、0x79 创造物品、0x7A 快捷栏、0x7B 固定物品栏。它们和箱子、漏斗的**类型号**属于不同字段。

## 配方表和一次合成

`35 CraftingData` 是服务端下发的配方目录。先读 UVarInt 配方数，每个条目从 VarInt 类型开始，所有条目之后才是 Bool `cleanRecipes`。下表中的「Slot 列表」都带 UVarInt 数量前缀。

| 条目类型 | 类型字段后面的内容 |
|---|---|
| 0 无序 | 材料 Slot 列表、产物 Slot 列表、配方 UUID |
| 1 有序 | 宽和高（各 VarInt）；逐行读取宽 × 高个 Slot，**不再读材料数**，空格写空气；然后是产物 Slot 列表、配方 UUID |
| 2 熔炉 | 输入物品 ID VarInt，产物 Slot；不带 UUID 或输入 meta |
| 3 指定 meta 的熔炉配方 | 输入物品 ID 和 meta（各 VarInt），产物 Slot |
| 4 多配方 | 解码器只读 UUID |
| 5 潜影盒配方 | 解码器按类型 0 的布局读 |

参考分支的 [CraftingDataPacket.php](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CraftingDataPacket.php) 能分辨类型 4、5，但配方写入方法只专门处理无序、有序和熔炉配方。不要从解码分支推断该端一定会发这两种条目。

客户端选择产物后发 `36 CraftingEvent`：windowId: Byte、类型 VarInt、配方 UUID、输入 Slot 列表、输出 Slot 列表。它不会再发一份配方定义。[Go 合成处理](../internal/server/craft.go) 用 UUID 查本服配方，再核对材料、产物和工作台。输入列表为空时，Go 核对此前移槽留下的物品；空列表不表示零材料。抓包时，先看登录后是否收到 35，再把 36 与它前面的 32 放在一起看。
