package server

import "fmt"
import "maps"
import "slices"

import "go-mc-server/internal/command"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 坐标用 TextPacket 的 TIP 类型显示在快捷栏正上方，也是 PocketMine
// sendTip() 的落点。原版那条 showcoordinates 游戏规则是 1.2 才有的，
// protocol 113 的 StartGame 里规则表恒为 0 条（参考端自己也是 TODO），
// 所以只能服务端定时推。
const coordsRefreshTicks int = 4 // 50ms 一 tick，200ms 刷一次

func (srv *Server) sendCoords(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	var text string = ""
	if player.CoordsEnabled() {
		text = coordsText(player, srv.playerWorldName(player))
	}
	player.Session.SendPacket(&packets.TextPacket{
		Type:    protocol.TextTypeTip,
		Message: text,
	})
}

// coordsText 拼出快捷栏上方那行字；关掉时返回空串，把残留内容清掉。
func coordsText(player *Player, worldName string) string {
	var posX float32 = 0
	var posY float32 = 0
	var posZ float32 = 0
	player.mu.RLock()
	posX, posY, posZ = player.X, player.Y, player.Z
	player.mu.RUnlock()
	var label string = command.WorldDisplayName(worldName, "world")
	return fmt.Sprintf("§7%s §fX: %.1f §7Y: %.1f §7Z: %.1f", label, posX, posY, posZ)
}

// tickCoords 给所有开着坐标显示的玩家刷一次。
func (srv *Server) tickCoords() {
	srv.mu.RLock()
	// range map 只要 value 就得写 _，规范禁止，所以先取出 values 再按下标遍历。
	var list []*Player = slices.Collect(maps.Values(srv.players))
	srv.mu.RUnlock()
	var index int = 0
	for index = 0; index < len(list); index++ {
		var target *Player = list[index]
		if !target.CoordsEnabled() {
			continue
		}
		srv.sendCoords(target)
	}
}
