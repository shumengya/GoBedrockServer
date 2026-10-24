package server

import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

var signTextKeys [4]string = [4]string{"Text1", "Text2", "Text3", "Text4"}

// handleBlockEntityData 处理客户端发来的方块实体数据，目前只有告示牌写字用它。
func (srv *Server) handleBlockEntityData(sess *network.Session, pkt *packets.BlockEntityDataPacket) {
	var player *Player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	srv.editSignLocked(player, pkt)
}

// editSignLocked 玩家给告示牌写字。要同时满足：那一格已经加载并且真是告示牌、方块实体还在、
// 人够得着且不在出生点保护里、不是旁观者、他就是放下这块牌的人而且还没写过、NBT 读得出来且 id 是 Sign。
// 任何一条不满足，都把牌子真实的样子发回给他，他本地多出来的字就会消掉。
func (srv *Server) editSignLocked(player *Player, pkt *packets.BlockEntityDataPacket) {
	var blockWorld *world.World = srv.playerWorld(player)
	// 用 Peek 而不是 GetBlock：客户端可以随便填坐标，不能让一个包把任意远处的区块加载出来。
	// Peek 对没加载的区块返回空气，所以这一条同时挡住了「区块没加载」和「那格不是告示牌」。
	var blockID, _, _ = blockWorld.Peek(pkt.X, pkt.Y, pkt.Z)
	if !isSignBlock(blockID) {
		return
	}
	// 方块是告示牌、方块实体却在不在都不影响后面的写入：UpdateTile 找不到就返回 false。
	var reason string = srv.signEditRejection(player, pkt)
	if reason != "" {
		logger.Debug("server", "%s 改告示牌被拒 (%d,%d,%d): %s", player.Username, pkt.X, pkt.Y, pkt.Z, reason)
		srv.sendSignTo(player, pkt.X, pkt.Y, pkt.Z)
		return
	}
	var compound, readErr = nbt.ReadNetworkCompound(pkt.NBT)
	if readErr != nil || compound.Strings["id"] != nbt.IDSign {
		logger.Debug("server", "%s 改告示牌被拒 (%d,%d,%d): NBT 不对 %v", player.Username, pkt.X, pkt.Y, pkt.Z, readErr)
		srv.sendSignTo(player, pkt.X, pkt.Y, pkt.Z)
		return
	}
	var lines [4]string = [4]string{}
	var index int = 0
	for index = 0; index < len(lines); index++ {
		lines[index] = world.CleanSignLine(compound.Strings[signTextKeys[index]])
	}
	var edited bool = blockWorld.UpdateTile(pkt.X, pkt.Y, pkt.Z, func(live *world.Tile) bool {
		if live.ID != world.TileSign || live.Editor == "" || live.Editor != player.UUID {
			return false
		}
		live.Lines = lines
		live.Editor = ""
		return true
	})
	var current, _ = blockWorld.TileAt(pkt.X, pkt.Y, pkt.Z)
	if !edited {
		logger.Debug("server", "%s 改告示牌被拒 (%d,%d,%d): 不是放牌的人，或者已经写过", player.Username, pkt.X, pkt.Y, pkt.Z)
		srv.sendSignTo(player, pkt.X, pkt.Y, pkt.Z)
		return
	}
	srv.broadcastTile(srv.playerWorldName(player), current)
}

func isSignBlock(id byte) bool {
	return id == world.BlockSignPost || id == world.BlockWallSign
}

// signEditRejection 距离、出生点保护、游戏模式这几关，通过返回空串。
func (srv *Server) signEditRejection(player *Player, pkt *packets.BlockEntityDataPacket) string {
	var eyeX, eyeY, eyeZ = player.Position()
	if !blockInReach(eyeX, eyeY, eyeZ, blockPos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}, maxInteractDistance) {
		return "距离过远"
	}
	if player.gameMode() == int32(protocol.GameModeSpectator) {
		return "旁观模式"
	}
	if srv.spawnProtected(player, pkt.X, pkt.Z) {
		return "出生点保护"
	}
	return ""
}

// sendSignTo 把牌子真实的样子只发给这一个玩家，让他本地多出来的字消掉。
// 方块实体已经没了就什么都不发：零值 Tile 会变成一个描述箱子的假包扔到 (0,0,0)。
func (srv *Server) sendSignTo(player *Player, posX int32, posY int32, posZ int32) {
	if player.Session == nil {
		return
	}
	var tile, found = srv.playerWorld(player).TileAt(posX, posY, posZ)
	if !found {
		return
	}
	player.Session.SendPacket(&packets.BlockEntityDataPacket{X: tile.X, Y: tile.Y, Z: tile.Z, NBT: tile.SpawnNBT()})
}
