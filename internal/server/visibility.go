package server

import "cmp"

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// playerEyeHeight 是 Human::$baseOffset。客户端 MovePlayer 上报的是眼睛高度，
// 服务端存的 Player.Y 也是这个值；AddPlayer 要的是脚底，所以要减掉。
const playerEyeHeight float32 = 1.62

// 客户端内置翻译键，按客户端语言显示“xxx 加入了游戏”，§e 为黄色。
const msgPlayerJoined string = "§e%multiplayer.player.joined"
const msgPlayerLeft string = "§e%multiplayer.player.left"

func playerListEntry(player *Player) packets.PlayerListEntry {
	var skin []byte = player.Session.SkinData
	if len(skin) == 0 {
		skin = make([]byte, 8192)
	}
	return packets.PlayerListEntry{
		UUID:     uuidToBytes(player.UUID),
		EntityID: player.EntityRuntimeID,
		Name:     player.Username,
		SkinID:   cmp.Or(player.Session.SkinID, "Standard_Custom"),
		SkinData: skin,
	}
}

// sendPlayerListOnJoin 给新玩家发全服 Tab 列表（含自己），并把新玩家加进其他人的列表。
// Tab 列表不分世界；皮肤也只从这里下发，必须早于任何 AddPlayer。
func (srv *Server) sendPlayerListOnJoin(player *Player) {
	var entries []packets.PlayerListEntry = nil
	var key string = ""
	srv.mu.RLock()
	entries = make([]packets.PlayerListEntry, 0, len(srv.players))
	for key = range srv.players {
		entries = append(entries, playerListEntry(srv.players[key]))
	}
	srv.mu.RUnlock()
	player.Session.SendPacket(&packets.PlayerListPacket{
		Action:  protocol.PlayerListAdd,
		Entries: entries,
	})
	srv.broadcast(
		&packets.PlayerListPacket{
			Action:  protocol.PlayerListAdd,
			Entries: []packets.PlayerListEntry{playerListEntry(player)},
		},
		player.UUID,
	)
}

// viewersOf 返回和 player 同世界、已可见的其他玩家。
// 先放开 player.mu 再拿 srv.mu，与 broadcastInWorld 的加锁顺序一致。
func (srv *Server) viewersOf(player *Player) []*Player {
	var worldName string = srv.playerWorldName(player)
	var viewers []*Player = nil
	var key string = ""
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for key = range srv.players {
		var other *Player = srv.players[key]
		if other == player || other.Session == nil {
			continue
		}
		other.mu.RLock()
		var visible bool = other.Visible && sameWorldName(other.WorldName, worldName, srv.world.Name)
		other.mu.RUnlock()
		if visible {
			viewers = append(viewers, other)
		}
	}
	return viewers
}

func sendToPlayers(list []*Player, pkt packets.Packet) {
	if len(list) == 0 {
		return
	}
	var raw, err = packets.EncodePacket(pkt)
	if err != nil {
		logger.Error("server", "编码失败 %T: %v", pkt, err)
		return
	}
	var index int = 0
	for index = 0; index < len(list); index++ {
		list[index].Session.SendBatch(raw)
	}
}

func addPlayerPacket(player *Player) *packets.AddPlayerPacket {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return &packets.AddPlayerPacket{
		UUID:            uuidToBytes(player.UUID),
		Username:        player.Username,
		EntityUniqueID:  player.EntityRuntimeID,
		EntityRuntimeID: player.EntityRuntimeID,
		PosX:            player.X,
		PosY:            player.Y - playerEyeHeight,
		PosZ:            player.Z,
		Pitch:           player.Pitch,
		HeadYaw:         player.Yaw,
		Yaw:             player.Yaw,
		Item:            player.HeldItem,
		Metadata:        packets.PlayerMetadata(player.Username, packets.PlayerFlags(player.combat.Effects[effectInvisible].Duration > 0)),
	}
}

// showPlayerInWorld 在区块加载完、进入 Playing 后调用：
// 与同世界已可见的玩家互发 AddPlayer；首次进服再广播加入提示。
func (srv *Server) showPlayerInWorld(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	// 串行化显示/隐藏，避免两人同时进出时一方漏发 AddPlayer 或 RemoveEntity。
	srv.visMu.Lock()
	defer srv.visMu.Unlock()

	player.mu.Lock()
	if player.Visible {
		player.mu.Unlock()
		return
	}
	player.Visible = true
	var announce bool = !player.JoinAnnounced
	player.JoinAnnounced = true
	player.mu.Unlock()

	var viewers []*Player = srv.viewersOf(player)
	sendToPlayers(viewers, addPlayerPacket(player))
	var index int = 0
	for index = 0; index < len(viewers); index++ {
		player.Session.SendPacket(addPlayerPacket(viewers[index]))
	}
	exchangeArmor(player, viewers)
	if !announce {
		return
	}
	srv.broadcastJoinLeave(msgPlayerJoined, player.Username)
	logger.Info("server", "%s 加入了游戏", player.Username)
}

// hidePlayerInWorld 在切世界或退出时调用：双方互删实体。
// 退出时 player 已从 srv.players 移除，sendSelf=false 不再给它发包。
func (srv *Server) hidePlayerInWorld(player *Player, sendSelf bool) {
	if player == nil {
		return
	}
	srv.visMu.Lock()
	defer srv.visMu.Unlock()

	if !player.isVisible() {
		return
	}
	var viewers []*Player = srv.viewersOf(player)
	player.mu.Lock()
	player.Visible = false
	player.mu.Unlock()

	sendToPlayers(viewers, &packets.RemoveEntityPacket{EntityUniqueID: player.EntityRuntimeID})
	if !sendSelf || player.Session == nil {
		return
	}
	var index int = 0
	for index = 0; index < len(viewers); index++ {
		player.Session.SendPacket(&packets.RemoveEntityPacket{EntityUniqueID: viewers[index].EntityRuntimeID})
	}
}

func (srv *Server) broadcastJoinLeave(key string, name string) {
	srv.broadcast(
		&packets.TextPacket{
			Type:       protocol.TextTypeTranslation,
			Message:    key,
			Parameters: []string{name},
		},
		"",
	)
}

func (player *Player) isVisible() bool {
	player.mu.RLock()
	defer player.mu.RUnlock()
	return player.Visible
}

// broadcastPlayerMove 把位置转发给同世界其他玩家。Y 保持眼睛高度，与参考端 sendPosition 一致。
func (srv *Server) broadcastPlayerMove(player *Player) {
	if !player.isVisible() {
		return
	}
	player.mu.RLock()
	var pkt *packets.MovePlayerPacket = &packets.MovePlayerPacket{
		EntityRuntimeID: player.EntityRuntimeID,
		X:               player.X,
		Y:               player.Y,
		Z:               player.Z,
		Pitch:           player.Pitch,
		Yaw:             player.Yaw,
		BodyYaw:         player.Yaw,
		Mode:            protocol.MovePlayerModeNormal,
		OnGround:        player.OnGround,
	}
	player.mu.RUnlock()
	sendToPlayers(srv.viewersOf(player), pkt)
}

// broadcastHeldItem 让其他玩家看到 player 手里拿的东西。
func (srv *Server) broadcastHeldItem(player *Player) {
	if !player.isVisible() {
		return
	}
	player.mu.RLock()
	var pkt = player.equipmentLocked()
	player.mu.RUnlock()
	sendToPlayers(srv.viewersOf(player), pkt)
}

// handleAnimate 转发挥手等动作；客户端只会发自己的，eid 以服务端为准。
func (srv *Server) handleAnimate(sess *network.Session, pkt *packets.AnimatePacket) {
	srv.mu.RLock()
	var player *Player = srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if player == nil || !player.isVisible() {
		return
	}
	sendToPlayers(
		srv.viewersOf(player),
		&packets.AnimatePacket{
			Action:          pkt.Action,
			EntityRuntimeID: player.EntityRuntimeID,
		},
	)
}
