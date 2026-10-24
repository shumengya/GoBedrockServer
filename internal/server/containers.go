package server

import "maps"
import "slices"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// onBlockPlaced 方块放下后补上它的方块实体。color 是手持物的 meta 低 4 位，只有床用它当颜色。
func (srv *Server) onBlockPlaced(p *Player, x, y, z int32, id byte, color byte) {
	w := srv.playerWorld(p)
	wn := srv.playerWorldName(p)
	switch id {
	case world.BlockChest:
		t := world.NewChestTile(x, y, z)
		w.SetTile(t)
		srv.broadcastTile(wn, t)
	case world.BlockBed:
		t := world.NewBedTile(x, y, z, color)
		w.SetTile(t)
		srv.broadcastTile(wn, t)
	default:
		w.RemoveTile(x, y, z)
	}
}

func (srv *Server) broadcastTile(worldName string, t world.Tile) {
	srv.broadcastInWorld(worldName, &packets.BlockEntityDataPacket{
		X: t.X, Y: t.Y, Z: t.Z,
		NBT: t.SpawnNBT(),
	}, "")
}

// isContainerBlock 点一下会打开物品栏窗口的方块：箱子和熔炉（含烧着的）。
func isContainerBlock(id byte) bool {
	return id == world.BlockChest || id == world.BlockFurnace || id == world.BlockBurningFurnace
}

// containerTileFor 这种方块对应哪种方块实体，不是容器返回空串。
func containerTileFor(id byte) string {
	if id == world.BlockChest {
		return world.TileChest
	}
	if id == world.BlockFurnace || id == world.BlockBurningFurnace {
		return world.TileFurnace
	}
	return ""
}

func containerWindowType(tileID string) byte {
	if tileID == world.TileFurnace {
		return protocol.InventoryTypeFurnace
	}
	return protocol.InventoryTypeContainer
}

func newContainerTile(tileID string, posX int32, posY int32, posZ int32) world.Tile {
	if tileID == world.TileFurnace {
		return world.NewFurnaceTile(posX, posY, posZ)
	}
	return world.NewChestTile(posX, posY, posZ)
}

// openContainer 打开箱子、熔炉这类带物品栏的方块。方块在但方块实体没有（老存档、别处生成的）就当场建一个。
// 同一时刻只开一个窗口，窗口号固定，窗口类型随方块种类变。
func (srv *Server) openContainer(player *Player, posX int32, posY int32, posZ int32) {
	if player == nil || player.Session == nil {
		return
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var tileID string = containerTileFor(blockIDAt(blockWorld, posX, posY, posZ))
	if tileID == "" {
		return
	}
	var tile, found = blockWorld.TileAt(posX, posY, posZ)
	if !found || tile.ID != tileID {
		tile = newContainerTile(tileID, posX, posY, posZ)
		blockWorld.SetTile(tile)
	}
	var windowType byte = containerWindowType(tileID)
	player.mu.Lock()
	player.OpenWindow = protocol.WindowContainer
	player.OpenWindowType = windowType
	player.OpenTX = posX
	player.OpenTY = posY
	player.OpenTZ = posZ
	var runtimeID int64 = player.EntityRuntimeID
	var session *network.Session = player.Session
	player.mu.Unlock()
	session.SendPacket(&packets.ContainerOpenPacket{
		WindowID: protocol.WindowContainer,
		Type:     windowType,
		X:        posX,
		Y:        posY,
		Z:        posZ,
		EntityID: -1,
	})
	session.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.WindowContainer,
		TargetEID: runtimeID,
		Slots:     tile.Items,
	})
}

// openContainerTile 返回玩家当前打开的容器窗口对应的方块实体。窗口没开、方块已经不是容器、
// 方块实体没了、人离得太远，都返回 false：这些服务端自己查，不信客户端。
func (srv *Server) openContainerTile(player *Player) (world.Tile, bool) {
	player.mu.RLock()
	var window byte = player.OpenWindow
	var posX int32 = player.OpenTX
	var posY int32 = player.OpenTY
	var posZ int32 = player.OpenTZ
	player.mu.RUnlock()
	if window != protocol.WindowContainer {
		return world.Tile{}, false
	}
	var blockWorld *world.World = srv.playerWorld(player)
	if !isContainerBlock(blockIDAt(blockWorld, posX, posY, posZ)) {
		return world.Tile{}, false
	}
	var tile, found = blockWorld.TileAt(posX, posY, posZ)
	if !found {
		return world.Tile{}, false
	}
	var eyeX, eyeY, eyeZ = player.Position()
	if !blockInReach(eyeX, eyeY, eyeZ, blockPos{PosX: posX, PosY: posY, PosZ: posZ}, maxInteractDistance) {
		return world.Tile{}, false
	}
	return tile, true
}

// handleContainerWindowSlot 处理客户端对容器窗口某一格的改动。
// 目前容器里的内容还是信客户端报的（背包事务校验在后面单独做），但窗口必须真开着、人必须够得着。
func (srv *Server) handleContainerWindowSlot(player *Player, pkt *packets.ContainerSetSlotPacket) {
	if pkt.WindowID != protocol.WindowContainer {
		return
	}
	var tile, usable = srv.openContainerTile(player)
	if !usable {
		return
	}
	// 槽位是否越界由 SetTileSlot 判断，越界它会拒绝。
	srv.playerWorld(player).SetTileSlot(tile.X, tile.Y, tile.Z, int(pkt.Slot), pkt.Item)
}

func (srv *Server) openWorkbench(p *Player, x, y, z int32) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.Lock()
	p.OpenWindow = protocol.WindowWorkbench
	p.OpenWindowType = protocol.InventoryTypeWorkbench
	p.OpenTX, p.OpenTY, p.OpenTZ = x, y, z
	sess := p.Session
	p.mu.Unlock()
	sess.SendPacket(&packets.ContainerOpenPacket{
		WindowID: protocol.WindowWorkbench,
		Type:     protocol.InventoryTypeWorkbench,
		X:        x, Y: y, Z: z,
		EntityID: -1,
	})
}

func (srv *Server) handleContainerClose(sess *network.Session, pkt *packets.ContainerClosePacket) {
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.OpenWindow == pkt.WindowID || pkt.WindowID == 0 {
		p.OpenWindow = 0
		p.OpenWindowType = 0
	}
	p.mu.Unlock()
}

// containerViewers 此刻开着 (posX,posY,posZ) 这个容器窗口的玩家，按 UUID 排序。
func (srv *Server) containerViewers(worldName string, posX int32, posY int32, posZ int32) []*Player {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var uuids []string = slices.Sorted(maps.Keys(srv.players))
	var viewers []*Player = nil
	var index int = 0
	for index = 0; index < len(uuids); index++ {
		var player *Player = srv.players[uuids[index]]
		player.mu.RLock()
		var watching bool = player.OpenWindow == protocol.WindowContainer &&
			player.OpenTX == posX &&
			player.OpenTY == posY &&
			player.OpenTZ == posZ &&
			sameWorldName(player.WorldName, worldName, srv.world.Name)
		player.mu.RUnlock()
		if watching {
			viewers = append(viewers, player)
		}
	}
	return viewers
}

// releaseTile 方块实体随着方块一起没了：里面的东西掉在地上，正开着它的窗口关掉。
func (srv *Server) releaseTile(worldName string, tile world.Tile) {
	srv.dropTileContents(worldName, tile)
	var viewers []*Player = srv.containerViewers(worldName, tile.X, tile.Y, tile.Z)
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
	}
}

// dropTileContents 把方块实体物品栏里的东西全部掉出来。
func (srv *Server) dropTileContents(worldName string, tile world.Tile) {
	var index int = 0
	for index = 0; index < len(tile.Items); index++ {
		if tile.Items[index].IsAir() {
			continue
		}
		srv.spawnDroppedItem(
			tile.Items[index],
			float32(tile.X)+0.5,
			float32(tile.Y)+0.5,
			float32(tile.Z)+0.5,
			0,
			0.1,
			0,
			10,
			worldName,
		)
	}
}
