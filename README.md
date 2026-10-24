# Go MC Server

用 Golang 从零实现的 **Minecraft Bedrock Edition 1.1.x** 服务端（MCPE 协议 + RakNet UDP）。

- 版本：Bedrock 1.1.x（例如 1.1.7），协议号 **113**（同时接受 110–113）
- 传输：RakNet（UDP，默认端口 19132）
- 待办与进度见 [TODO.md](TODO.md)

## 运行

```bash
go build -o go-mc-server.exe .
./go-mc-server.exe -port 19132 -motd "My Server"
```

需要更详细的解码日志时设置环境变量 `GO_MC_DEBUG=1`。

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-host` | `0.0.0.0` | 绑定地址 |
| `-port` | `19132` | UDP 端口 |
| `-motd` | `一个由go语言开发的minecraft服务器` | 服务器标题 |
| `-max-players` | `20` | 最大玩家数，超过拒绝登录，0 表示不限 |
| `-spawn-protection` | `16` | 出生点保护半径（方块），0 关闭 |
| `-properties` | `server.properties` | 配置文件路径；不存在会按默认值生成 |

配置优先级：内置默认值 < `server.properties` < 命令行里明确传了的参数。`ops.txt`、`white-list.txt`、`banned-players.txt`、`banned-ips.txt` 放在配置文件同一目录，格式和 PocketMine 一致。

**第一次用**：新服务器没有管理员，`/give`、`/gamemode`、`/summon` 和所有管理命令只对管理员开放。在运行窗口里输入 `op <你的名字>` 即可。离线模式下名字由客户端自报、没有验证，别人用同名登录就等于拿到该名字的权限，公网服务器请谨慎使用。

## 目前支持

**网络与登录**
- RakNet 握手、可靠有序、分片、ACK/NACK；MCPE Batch；离线登录（读取用户名、UUID、皮肤）
- 白名单、封禁（名字 / IP）、人数上限；被拒时客户端能看到原因

**世界**
- 多世界：`world`（超平坦）/ `sky` / `ocean` / `nether` / `end`，`/world` 切换
- 区块按距离排队发送，视距上限可配置（`view-distance`，2–32）
- 昼夜循环：每世界独立时钟，可 `/time` 调整或冻结
- 存档：区块、玩家、世界元数据、方块实体、掉落物

**玩法**
- 放置、破坏（按工具和硬度计时、掉落表、工具耐久）、箱子、工作台、床
- 2×2 背包合成与 3×3 工作台合成
- 生命：摔落/虚空/窒息/溺水/岩浆/火焰/仙人掌伤害，死亡掉落与重生，PvP 与击退，自然回血
- 饥饿：饥饿值/饱和度/疲劳度，吃东西，饿到掉血
- 掉落物：丢弃、捡起、物理、超时消失
- 多人互相可见：移动、挥手、手持物品、加入/离开提示

**防作弊**
- 挖掘时间、放置/破坏/交互距离、背包核实、移动回拉、出生点保护

**命令与管理**
- 游戏内和服务器控制台（stdin）都能执行，管理命令带权限
- `/give` `/summon` `/gamemode` `/world` `/coord` `/help`
- `/kick` `/ban` `/ban-ip` `/pardon` `/pardon-ip` `/whitelist` `/op` `/deop` `/list` `/tp` `/say` `/tell` `/kill` `/time` `/stop` `/save-all`

## 项目结构

```
Go-MC-Server/
├── main.go
├── TODO.md              # 待办与进度
├── config/              # 运行配置 + server.properties 读写
├── docs/                # 各模块的设计说明
├── scripts/             # 从 PocketMine 源码生成 block/item/entity 注册表
└── internal/
    ├── logger/          # 带文件:行、模块、panic stack 的日志
    ├── raknet/          # RakNet：握手、可靠有序、分片、ACK
    ├── protocol/        # MCPE 协议常量、二进制流、Batch、物品槽
    │   ├── nbt/         # 方块实体用的小端 NBT
    │   └── packets/     # 各包编解码 + 注册表
    ├── network/         # MCPE 会话状态机
    ├── server/          # 登录、区块、放置/破坏、容器、掉落物、多世界、饥饿进食、昼夜、合成、管理桥接与控制台
    ├── crafting/        # 配方书（内嵌 recipes.json）与合成格匹配
    ├── access/          # 管理员 / 白名单 / 封禁名单
    ├── world/           # 可变区块列、模板生成器、方块实体（箱子/床）
    ├── storage/         # 区块 region 存档、玩家/世界/方块实体/掉落物 JSON
    ├── command/         # 斜杠命令框架与内置命令
    ├── block/ item/     # 方块/物品注册表（仅 ID+名字，暂无行为）
    └── entity/          # 实体注册表 + 掉落物物理
```

登录与收发包的流程见 [docs/登录与收发包流程.md](docs/登录与收发包流程.md)。
