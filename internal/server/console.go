package server

import "bufio"
import "io"
import "strings"

import "go-mc-server/internal/command"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol/packets"

// consoleName 是控制台在日志和命令上下文里的名字。
const consoleName string = "CONSOLE"

// stripColors 去掉 § 颜色码（连同后面那个样式字符）。游戏里的回复带颜色码，
// 终端不认，直接打出来会是一堆乱码。
func stripColors(text string) string {
	var builder strings.Builder = strings.Builder{}
	var runes []rune = []rune(text)
	var index int = 0
	for index = 0; index < len(runes); index++ {
		if runes[index] == '§' {
			index++
			continue
		}
		builder.WriteRune(runes[index])
	}
	return builder.String()
}

// RunConsole 从 input 逐行读命令执行，读到 EOF 返回。没有交互终端（后台运行、重定向）时
// 读到 EOF 是正常的，调用方不该把它当成故障。
func (srv *Server) RunConsole(input io.Reader) error {
	var scanner *bufio.Scanner = bufio.NewScanner(input)
	var more bool = scanner.Scan()
	for more {
		srv.DispatchConsole(scanner.Text())
		more = scanner.Scan()
	}
	var err error = scanner.Err()
	if err != nil {
		return err
	}
	return nil
}

// DispatchConsole 以控制台身份执行一行命令。控制台拥有全部权限，但没有位置和背包：
// 依赖「自己」的写法（@s、省略坐标）会报「玩家不在线」或要求写明坐标。
func (srv *Server) DispatchConsole(line string) {
	var text string = strings.TrimSpace(line)
	text = strings.TrimSpace(strings.TrimPrefix(text, command.Prefix))
	if text == "" {
		return
	}
	logger.Info("command", "%s 执行 /%s", consoleName, text)
	var ctx *command.Context = &command.Context{
		Username:  consoleName,
		IsConsole: true,
		IsOp:      true,
		Reply: func(msg string) {
			var lines []string = strings.Split(stripColors(msg), "\n")
			var index int = 0
			for index = 0; index < len(lines); index++ {
				logger.Info("console", "%s", lines[index])
			}
		},
		FindPlayer: func(name string) (command.InventoryPlayer, bool) {
			return srv.findConsolePlayer(name)
		},
		SpawnEntity: func(networkID int32, posX float32, posY float32, posZ float32) error {
			return srv.spawnEntity(networkID, posX, posY, posZ, srv.world.Name)
		},
		DefaultWorld: srv.world.Name,
		World:        srv.world.Name,
		Admin:        serverAdmin{srv: srv},
	}
	srv.cmds.Dispatch(ctx, text)
}

// findConsolePlayer 控制台按名字找在线玩家（支持唯一前缀）；它自己不是玩家，所以没有 @s。
func (srv *Server) findConsolePlayer(name string) (command.InventoryPlayer, bool) {
	var player, err = (serverAdmin{srv: srv}).lookupOnline(name)
	if err != nil {
		return nil, false
	}
	return &commandPlayer{srv: srv, p: player}, true
}

// sendAvailableCommands 按玩家当前是否管理员重发命令列表。
func (srv *Server) sendAvailableCommands(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.AvailableCommandsPacket{
		CommandsJSON: srv.cmds.AvailableCommandsJSON(srv.isOp(player.Username)),
	})
}

// refreshPermissions 在玩家被 /op 或 /deop 后立刻让客户端的命令补全和权限标志跟上，
// 不用等他重新登录。
func (srv *Server) refreshPermissions(player *Player, nowOp bool) {
	if player == nil || player.Session == nil || player.Session.State() != network.StatePlaying {
		return
	}
	srv.sendGameModeAndSettings(player)
	srv.sendAvailableCommands(player)
	if nowOp {
		srv.sendRaw(player.Session, "§e你已被设为管理员")
		return
	}
	srv.sendRaw(player.Session, "§e你的管理员权限已被撤销")
}
