# PocketMine 1.1.x 游戏包索引

按十六进制 ID 排列。依据本地 [ProtocolInfo.php](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ProtocolInfo.php)：主协议 **113**，接受 **110、111、112、113**。这里描述这份参考端，不是新版 Bedrock 的包表，也不保证四个版本的每处细节相同。

- 看线上字段、字节序和传输层：[协议线格式](PocketMine协议线格式.md)。
- 查物品 Slot 字节示例、窗口号、快捷栏映射及配方条目：[背包与合成协议](PocketMine背包与合成协议.md)。
- 查实体元数据类型、潜行标志的字节示例，以及生成包和更新包不同的属性顺序：[实体协议](PocketMine实体协议.md)。
- 看玩家何时收发这些包：[登录与收发包流程](登录与收发包流程.md)。
- **PHP** 链接指向本机只读参考树；单独克隆 Go 仓库时该目录可能不存在。“仅常量”表示此版本没有对应包类，不据名称推断字段。
- **Go** 链接指向具体包实现。“已注册”表示可从 [包注册表](../internal/protocol/packets/registry.go) 找到构造器，不代表收发双向或玩法已经完整。“未实现”指没有找到该游戏包的 Go 编解码器。

## 登录和资源包协商

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x01` | [Login](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/LoginPacket.php) | 提交游戏版本、身份链和客户端资料 | [已注册](../internal/protocol/packets/login.go) |
| `0x02` | [PlayStatus](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/PlayStatusPacket.php) | 登录结果或允许出生 | [已注册](../internal/protocol/packets/play_status.go) |
| `0x03` | [ServerToClientHandshake](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ServerToClientHandshakePacket.php) | 服务端握手信息 | 未实现 |
| `0x04` | [ClientToServerHandshake](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ClientToServerHandshakePacket.php) | 客户端确认握手 | 未实现 |
| `0x05` | [Disconnect](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/DisconnectPacket.php) | 断开会话及原因 | [已注册](../internal/protocol/packets/disconnect.go) |
| `0x06` | [ResourcePacksInfo](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePacksInfoPacket.php) | 资源包清单和大小 | [已注册](../internal/protocol/packets/resource_packs_info.go) |
| `0x07` | [ResourcePackStack](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePackStackPacket.php) | 资源包应用顺序 | [已注册](../internal/protocol/packets/resource_pack_stack.go) |
| `0x08` | [ResourcePackClientResponse](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePackClientResponsePacket.php) | 资源包协商状态和所需包 ID | [已注册](../internal/protocol/packets/resource_pack_response.go) |

## 文字、世界与实体

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x09` | [Text](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/TextPacket.php) | 聊天、翻译或系统文字 | [已注册](../internal/protocol/packets/text.go) |
| `0x0A` | [SetTime](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetTimePacket.php) | 同步世界时间 | [已注册](../internal/protocol/packets/set_time.go) |
| `0x0B` | [StartGame](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/StartGamePacket.php) | 初始化本局世界、玩家和规则 | [已注册](../internal/protocol/packets/start_game.go) |
| `0x0C` | [AddPlayer](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddPlayerPacket.php) | 使另一名玩家出现在视野中 | [已注册](../internal/protocol/packets/add_player.go) |
| `0x0D` | [AddEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddEntityPacket.php) | 生成非玩家实体 | [已注册](../internal/protocol/packets/add_entity.go) |
| `0x0E` | [RemoveEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/RemoveEntityPacket.php) | 从视野移除实体 | [已注册](../internal/protocol/packets/remove_entity.go) |
| `0x0F` | [AddItemEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddItemEntityPacket.php) | 生成携带物品栈的掉落实体 | [已注册](../internal/protocol/packets/add_item_entity.go) |
| `0x10` | [AddHangingEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddHangingEntityPacket.php) | 生成悬挂实体 | 未实现 |
| `0x11` | [TakeItemEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/TakeItemEntityPacket.php) | 展示物品实体被拾取 | [已注册](../internal/protocol/packets/take_item_entity.go) |
| `0x12` | [MoveEntity](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MoveEntityPacket.php) | 同步非玩家实体位姿 | [已注册](../internal/protocol/packets/move_entity.go) |
| `0x13` | [MovePlayer](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MovePlayerPacket.php) | 玩家移动、回拉或传送 | [已注册](../internal/protocol/packets/move_player.go) |
| `0x14` | [RiderJump](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/RiderJumpPacket.php) | 骑乘跳跃输入 | 未实现 |
| `0x15` | [RemoveBlock](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/RemoveBlockPacket.php) | 请求挖掉方块 | [已注册](../internal/protocol/packets/remove_block.go) |
| `0x16` | [UpdateBlock](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/UpdateBlockPacket.php) | 同步单个方块的 ID、meta 和标志 | [已注册](../internal/protocol/packets/update_block.go) |
| `0x17` | [AddPainting](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddPaintingPacket.php) | 放置画实体 | 未实现 |
| `0x18` | [Explode](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ExplodePacket.php) | 爆炸位置、范围和受影响方块 | 未实现 |
| `0x19` | [LevelSoundEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/LevelSoundEventPacket.php) | 按声音事件编号播放世界音效 | [已注册](../internal/protocol/packets/level_sound_event.go) |
| `0x1A` | [LevelEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/LevelEventPacket.php) | 世界事件，例如挖掘裂纹、粒子或传送音效 | [已注册](../internal/protocol/packets/level_event.go) |
| `0x1B` | [BlockEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/BlockEventPacket.php) | 方块动画事件，如箱盖开合 | 未实现 |
| `0x1C` | [EntityEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/EntityEventPacket.php) | 实体事件，如受伤、死亡和进食 | [已注册](../internal/protocol/packets/combat.go) |
| `0x1D` | [MobEffect](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MobEffectPacket.php) | 增加、更新或移除状态效果 | [已注册](../internal/protocol/packets/mob_effect.go) |
| `0x1E` | [UpdateAttributes](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/UpdateAttributesPacket.php) | 同步生命、饥饿等实体属性 | [已注册](../internal/protocol/packets/update_attributes.go) |

## 装备、交互与玩家状态

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x1F` | [MobEquipment](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MobEquipmentPacket.php) | 同步手持物和槽位 | [已注册](../internal/protocol/packets/mob_equipment.go) |
| `0x20` | [MobArmorEquipment](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MobArmorEquipmentPacket.php) | 同步四个盔甲槽 | [已注册](../internal/protocol/packets/mob_armor_equipment.go) |
| `0x21` | [Interact](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/InteractPacket.php) | 对实体执行交互 | [已注册](../internal/protocol/packets/interact.go) |
| `0x22` | [BlockPickRequest](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/BlockPickRequestPacket.php) | 创造模式选取目标方块 | 未实现 |
| `0x23` | [UseItem](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/UseItemPacket.php) | 对方块或空气使用手持物 | [已注册](../internal/protocol/packets/use_item.go) |
| `0x24` | [PlayerAction](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/PlayerActionPacket.php) | 开始或结束挖掘、冲刺等玩家动作 | [已注册](../internal/protocol/packets/player_action.go) |
| `0x25` | [EntityFall](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/EntityFallPacket.php) | 报告实体摔落 | [已注册](../internal/protocol/packets/entity_fall.go) |
| `0x26` | [HurtArmor](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/HurtArmorPacket.php) | 报告护甲受损 | 未实现 |
| `0x27` | [SetEntityData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetEntityDataPacket.php) | 更新实体元数据 | [已注册](../internal/protocol/packets/set_entity_data.go) |
| `0x28` | [SetEntityMotion](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetEntityMotionPacket.php) | 更新实体速度向量 | [已注册](../internal/protocol/packets/set_entity_motion.go) |
| `0x29` | [SetEntityLink](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetEntityLinkPacket.php) | 建立或解除骑乘关系 | 未实现 |
| `0x2A` | [SetHealth](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetHealthPacket.php) | 设置玩家生命值 | [已注册](../internal/protocol/packets/combat.go) |
| `0x2B` | [SetSpawnPosition](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetSpawnPositionPacket.php) | 设置出生点坐标 | [已注册](../internal/protocol/packets/set_spawn_position.go) |
| `0x2C` | [Animate](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AnimatePacket.php) | 挥手等实体动作动画 | [已注册](../internal/protocol/packets/animate.go) |
| `0x2D` | [Respawn](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/RespawnPacket.php) | 重生坐标 | [已注册](../internal/protocol/packets/combat.go) |
| `0x2E` | [DropItem](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/DropItemPacket.php) | 玩家请求丢弃物品 | [已注册](../internal/protocol/packets/drop_item.go) |
| `0x2F` | [InventoryAction](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/InventoryActionPacket.php) | 背包动作 | 未实现 |

## 容器、合成与区块

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x30` | [ContainerOpen](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerOpenPacket.php) | 打开容器窗口 | [已注册](../internal/protocol/packets/container_open.go) |
| `0x31` | [ContainerClose](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerClosePacket.php) | 关闭容器窗口 | [已注册](../internal/protocol/packets/container_close.go) |
| `0x32` | [ContainerSetSlot](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerSetSlotPacket.php) | 更新窗口中的单个槽位 | [已注册](../internal/protocol/packets/container_set_slot.go) |
| `0x33` | [ContainerSetData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerSetDataPacket.php) | 更新容器属性，如熔炉进度 | [已注册](../internal/protocol/packets/container_set_data.go) |
| `0x34` | [ContainerSetContent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ContainerSetContentPacket.php) | 同步整个窗口及快捷栏映射 | [已注册](../internal/protocol/packets/container_set_content.go) |
| `0x35` | [CraftingData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CraftingDataPacket.php) | 下发合成和烧炼配方 | [发送编码，不经 Lookup](../internal/protocol/packets/crafting.go) |
| `0x36` | [CraftingEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CraftingEventPacket.php) | 提交合成配方、输入和预期产物 | [已注册](../internal/protocol/packets/crafting.go) |
| `0x37` | [AdventureSettings](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AdventureSettingsPacket.php) | 玩家能力和冒险设置 | [已注册](../internal/protocol/packets/adventure_settings.go) |
| `0x38` | [BlockEntityData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/BlockEntityDataPacket.php) | 同步方块实体 NBT | [已注册](../internal/protocol/packets/block_entity_data.go) |
| `0x39` | [PlayerInput](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/PlayerInputPacket.php) | 骑乘等场景的玩家输入 | 未实现 |
| `0x3A` | [FullChunkData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/FullChunkDataPacket.php) | 下发整列区块数据 | [已注册](../internal/protocol/packets/full_chunk_data.go) |

## 世界设置、地图与命令

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x3B` | [SetCommandsEnabled](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetCommandsEnabledPacket.php) | 设置命令启用状态 | 未实现 |
| `0x3C` | [SetDifficulty](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetDifficultyPacket.php) | 同步游戏难度 | 未实现 |
| `0x3D` | [ChangeDimension](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ChangeDimensionPacket.php) | 切换维度和位置 | [已注册](../internal/protocol/packets/change_dimension.go) |
| `0x3E` | [SetPlayerGameType](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetPlayerGameTypePacket.php) | 切换游戏模式 | [已注册](../internal/protocol/packets/set_player_game_type.go) |
| `0x3F` | [PlayerList](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/PlayerListPacket.php) | 增删玩家列表条目 | [已注册](../internal/protocol/packets/player_list.go) |
| `0x40` | `SIMPLE_EVENT_PACKET`（仅常量） | 简单事件（仅声明） | 未实现 |
| `0x41` | `EVENT_PACKET`（仅常量） | 游戏事件（仅声明） | 未实现 |
| `0x42` | [SpawnExperienceOrb](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SpawnExperienceOrbPacket.php) | 生成经验球 | 未实现 |
| `0x43` | [ClientboundMapItemData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ClientboundMapItemDataPacket.php) | 同步地图物品的数据 | 未实现 |
| `0x44` | [MapInfoRequest](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/MapInfoRequestPacket.php) | 请求地图信息 | 未实现 |
| `0x45` | [RequestChunkRadius](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/RequestChunkRadiusPacket.php) | 请求区块加载半径 | [已注册](../internal/protocol/packets/request_chunk_radius.go) |
| `0x46` | [ChunkRadiusUpdated](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ChunkRadiusUpdatedPacket.php) | 回告实际区块半径 | [已注册](../internal/protocol/packets/chunk_radius_updated.go) |
| `0x47` | [ItemFrameDropItem](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ItemFrameDropItemPacket.php) | 请求掉落展示框中的物品 | [已注册](../internal/protocol/packets/item_frame_drop_item.go) |
| `0x48` | [ReplaceItemInSlot](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ReplaceItemInSlotPacket.php) | 替换槽内物品 | 未实现 |
| `0x49` | `GAME_RULES_CHANGED_PACKET`（仅常量） | 游戏规则变更（仅声明） | 未实现 |
| `0x4A` | [Camera](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CameraPacket.php) | 相机事件 | 未实现 |
| `0x4B` | [AddItem](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AddItemPacket.php) | 添加物品 | 未实现 |
| `0x4C` | [BossEvent](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/BossEventPacket.php) | Boss 血条事件 | 未实现 |
| `0x4D` | [ShowCredits](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ShowCreditsPacket.php) | 显示制作人员名单 | 未实现 |
| `0x4E` | [AvailableCommands](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/AvailableCommandsPacket.php) | 下发可用命令及参数描述 | [已注册](../internal/protocol/packets/available_commands.go) |
| `0x4F` | [CommandStep](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CommandStepPacket.php) | 提交命令和参数 | [已注册](../internal/protocol/packets/command_step.go) |
| `0x50` | [CommandBlockUpdate](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/CommandBlockUpdatePacket.php) | 更新命令方块 | 未实现 |
| `0x51` | [UpdateTrade](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/UpdateTradePacket.php) | 更新交易窗口 | 未实现 |
| `0x52` | `UPDATE_EQUIP_PACKET`（仅常量） | 更新装备（仅声明） | 未实现 |

## 资源包分块与其他表现

| ID | PHP 包 / 常量 | 用途 | Go |
|---|---|---|---|
| `0x53` | [ResourcePackDataInfo](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePackDataInfoPacket.php) | 资源包分块元信息 | 未实现 |
| `0x54` | [ResourcePackChunkData](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePackChunkDataPacket.php) | 返回指定块的数据 | 未实现 |
| `0x55` | [ResourcePackChunkRequest](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/ResourcePackChunkRequestPacket.php) | 请求指定资源包块 | 未实现 |
| `0x56` | [Transfer](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/TransferPacket.php) | 转移到另一台服务器 | 未实现 |
| `0x57` | [PlaySound](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/PlaySoundPacket.php) | 按名称播放声音 | 未实现 |
| `0x58` | [StopSound](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/StopSoundPacket.php) | 停止声音 | 未实现 |
| `0x59` | [SetTitle](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/SetTitlePacket.php) | 设置标题、副标题或显示时长 | 未实现 |
| `0x5A` | `ADD_BEHAVIOR_TREE_PACKET`（仅常量） | 添加行为树（仅声明） | 未实现 |
| `0x5B` | `STRUCTURE_BLOCK_UPDATE_PACKET`（仅常量） | 结构方块更新（仅声明） | 未实现 |
| `0x5C` | `SHOW_STORE_OFFER_PACKET`（仅常量） | 展示商店商品（仅声明） | 未实现 |
| `0x5D` | `PURCHASE_RECEIPT_PACKET`（仅常量） | 购买收据（仅声明） | 未实现 |

## Batch 外层

| ID | PHP 包 | 用途 | Go |
|---|---|---|---|
| `0xFE` | [Batch](../pocketmine服务器参考/src/pocketmine/network/mcpe/protocol/BatchPacket.php) | 压缩并封装一个或多个完整游戏包；它的 ID 直接定义在包类中，不在上面的 93 项里 | [独立收发路径](../internal/protocol/batch.go) |
