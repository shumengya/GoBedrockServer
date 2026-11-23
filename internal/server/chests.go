package server

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// onChestPlaced 箱子放下后建方块实体，并和旁边同朝向、还没配对的箱子配成大箱子。
// 配对以后两边窗口的格数变了，之前开着任何一半的窗口都要关掉，否则玩家的 27 格窗口会错位地写进 54 格里。
func (srv *Server) onChestPlaced(worldName string, blockWorld *world.World, posX int32, posY int32, posZ int32) {
	var tile world.Tile = world.NewChestTile(posX, posY, posZ)
	blockWorld.SetTile(tile)
	var _, meta = blockWorld.GetBlock(posX, posY, posZ)
	var candidates []blockrule.Pos = blockrule.ChestPairCandidates(blockrule.Pos{PosX: posX, PosY: posY, PosZ: posZ}, meta)
	var index int = 0
	for index = 0; index < len(candidates); index++ {
		var side blockrule.Pos = candidates[index]
		var sideID, sideMeta = blockWorld.GetBlock(side.PosX, side.PosY, side.PosZ)
		if sideID != blockrule.BlockChest || sideMeta != meta {
			continue
		}
		if !blockWorld.PairChests(posX, posY, posZ, side.PosX, side.PosZ) {
			continue
		}
		srv.closeContainerViewers(worldName, side.PosX, side.PosY, side.PosZ)
		srv.broadcastChestAndPartner(worldName, blockWorld, posX, posY, posZ)
		return
	}
	srv.broadcastTile(worldName, tile)
}

// broadcastChestAndPartner 把一个箱子和它的另一半（如果有）的方块实体数据都发给同世界的玩家，
// 客户端靠这个把两个箱子画成一个。
func (srv *Server) broadcastChestAndPartner(worldName string, blockWorld *world.World, posX int32, posY int32, posZ int32) {
	var tile, found = blockWorld.TileAt(posX, posY, posZ)
	if !found {
		return
	}
	srv.broadcastTile(worldName, tile)
	var partner, paired = blockWorld.ChestPartner(posX, posY, posZ)
	if paired {
		srv.broadcastTile(worldName, partner)
	}
}

// chestHalves 一个容器窗口背后的方块实体，按窗口里的顺序排。单个箱子和熔炉只有它自己；
// 大箱子是两半，顺序只取决于两个箱子的位置，点哪一半都一样。
func chestHalves(blockWorld *world.World, tile world.Tile) []world.Tile {
	if tile.ID != world.TileChest {
		return []world.Tile{tile}
	}
	var partner, paired = blockWorld.ChestPartner(tile.X, tile.Y, tile.Z)
	if !paired {
		return []world.Tile{tile}
	}
	var front, back = world.OrderChestHalves(tile, partner)
	return []world.Tile{front, back}
}

// windowItems 把窗口背后各个方块实体的物品栏按顺序接成客户端看到的那一整排格子。
func windowItems(halves []world.Tile) []protocol.Item {
	var items []protocol.Item = nil
	var index int = 0
	for index = 0; index < len(halves); index++ {
		items = append(items, halves[index].Items...)
	}
	return items
}

// windowSlotTarget 把窗口里的格子编号落到具体哪个方块实体的哪一格。越界返回 false。
func windowSlotTarget(halves []world.Tile, slot int) (world.Tile, int, bool) {
	if slot < 0 {
		return world.Tile{}, 0, false
	}
	var remaining int = slot
	var index int = 0
	for index = 0; index < len(halves); index++ {
		if remaining < len(halves[index].Items) {
			return halves[index], remaining, true
		}
		remaining -= len(halves[index].Items)
	}
	return world.Tile{}, 0, false
}

// repairStalePair 箱子记着配对、另一半却不认账（存档残缺、另一半被别的东西换掉）时把配对解掉，
// 并把现在的样子重新发出去，免得客户端还把它画成半个大箱子。返回最新的方块实体。
func (srv *Server) repairStalePair(worldName string, blockWorld *world.World, tile world.Tile) world.Tile {
	if tile.ID != world.TileChest || !tile.Paired {
		return tile
	}
	var _, paired = blockWorld.ChestPartner(tile.X, tile.Y, tile.Z)
	if paired {
		return tile
	}
	blockWorld.UnpairChest(tile.X, tile.Y, tile.Z)
	var fresh, found = blockWorld.TileAt(tile.X, tile.Y, tile.Z)
	if !found {
		return tile
	}
	srv.broadcastTile(worldName, fresh)
	return fresh
}

// closeContainerViewers 关掉正开着 (posX,posY,posZ) 这个容器的所有玩家的窗口。
func (srv *Server) closeContainerViewers(worldName string, posX int32, posY int32, posZ int32) {
	var viewers []*Player = srv.containerViewers(worldName, posX, posY, posZ)
	var index int = 0
	for index = 0; index < len(viewers); index++ {
		viewers[index].mu.Lock()
		viewers[index].OpenWindow = 0
		viewers[index].OpenWindowType = 0
		var session *network.Session = viewers[index].Session
		viewers[index].mu.Unlock()
		if session != nil {
			session.SendPacket(&packets.ContainerClosePacket{WindowID: protocol.WindowContainer})
		}
		srv.returnFloating(viewers[index])
	}
}

// redrawChest 把一个箱子现在的样子重新发给同世界的玩家。大箱子拆掉一半以后，剩下那半要重画成单箱。
func (srv *Server) redrawChest(worldName string, posX int32, posY int32, posZ int32) {
	var tile, found = srv.worldByName(worldName).TileAt(posX, posY, posZ)
	if !found {
		return
	}
	srv.broadcastTile(worldName, tile)
}
