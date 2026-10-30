package server

import "maps"
import "slices"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/smelting"
import "go-mc-server/internal/world"

// onBlockPlaced 方块放下后补上它的方块实体。color 是手持物的 meta 低 4 位，只有床用它当颜色。
func (srv *Server) onBlockPlaced(p *Player, x, y, z int32, id byte, color byte) {
	w := srv.playerWorld(p)
	wn := srv.playerWorldName(p)
	switch id {
	case world.BlockChest:
		srv.onChestPlaced(wn, w, x, y, z)
	case world.BlockEnderChest:
		var enderTile world.Tile = world.NewEnderChestTile(x, y, z)
		w.SetTile(enderTile)
		srv.broadcastTile(wn, enderTile)
	case world.BlockBed:
		t := world.NewBedTile(x, y, z, color)
		w.SetTile(t)
		srv.broadcastTile(wn, t)
	case world.BlockSignPost, world.BlockWallSign:
		t := world.NewSignTile(x, y, z, p.UUID)
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

// isContainerBlock 点一下会打开物品栏窗口的方块：箱子、末影箱和熔炉（含烧着的）。
func isContainerBlock(id byte) bool {
	return id == world.BlockChest || id == world.BlockEnderChest ||
		id == world.BlockFurnace || id == world.BlockBurningFurnace
}

// containerTileFor 这种方块对应哪种方块实体，不是容器返回空串。
func containerTileFor(id byte) string {
	if id == world.BlockChest {
		return world.TileChest
	}
	if id == world.BlockEnderChest {
		return world.TileEnderChest
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
	if tileID == world.TileEnderChest {
		return world.NewEnderChestTile(posX, posY, posZ)
	}
	return world.NewChestTile(posX, posY, posZ)
}

// openContainer 打开箱子、熔炉这类带物品栏的方块。方块在但方块实体没有（老存档、别处生成的）就当场建一个。
// 同一时刻只开一个窗口，窗口号固定，窗口类型随方块种类变。
// 大箱子开的是拼好的 54 格窗口，窗口位置记在排在前面的那一半上，点哪一半打开的都一样。
func (srv *Server) openContainer(player *Player, posX int32, posY int32, posZ int32) {
	if player == nil || player.Session == nil {
		return
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var worldName string = srv.playerWorldName(player)
	var tileID string = containerTileFor(blockIDAt(blockWorld, posX, posY, posZ))
	if tileID == "" {
		return
	}
	var tile, found = blockWorld.TileAt(posX, posY, posZ)
	if !found || tile.ID != tileID {
		tile = newContainerTile(tileID, posX, posY, posZ)
		blockWorld.SetTile(tile)
	}
	tile = srv.repairStalePair(worldName, blockWorld, tile)
	var halves []world.Tile = chestHalves(blockWorld, tile)
	var anchor world.Tile = halves[0]
	var windowType byte = containerWindowType(tileID)
	player.mu.Lock()
	player.OpenWindow = protocol.WindowContainer
	player.OpenWindowType = windowType
	player.OpenTX = anchor.X
	player.OpenTY = anchor.Y
	player.OpenTZ = anchor.Z
	var runtimeID int64 = player.EntityRuntimeID
	var session *network.Session = player.Session
	player.mu.Unlock()
	session.SendPacket(&packets.ContainerOpenPacket{
		WindowID: protocol.WindowContainer,
		Type:     windowType,
		X:        anchor.X,
		Y:        anchor.Y,
		Z:        anchor.Z,
		EntityID: -1,
	})
	session.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.WindowContainer,
		TargetEID: runtimeID,
		Slots:     srv.windowContents(player, halves),
	})
	if tileID == world.TileFurnace {
		var progress []packets.Packet = furnaceProgressPackets(furnaceStateOf(tile))
		var index int = 0
		for index = 0; index < len(progress); index++ {
			session.SendPacket(progress[index])
		}
	}
}

// openContainerTiles 返回玩家当前打开的容器窗口背后的方块实体，按窗口里的顺序排（大箱子是两半）。
// 窗口没开、方块已经不是容器、方块实体没了、人离得太远，都返回 false：这些服务端自己查，不信客户端。
// 大箱子只要有一半够得着就算够得着。
func (srv *Server) openContainerTiles(player *Player) ([]world.Tile, bool) {
	player.mu.RLock()
	var window byte = player.OpenWindow
	var posX int32 = player.OpenTX
	var posY int32 = player.OpenTY
	var posZ int32 = player.OpenTZ
	player.mu.RUnlock()
	if window != protocol.WindowContainer {
		return nil, false
	}
	var blockWorld *world.World = srv.playerWorld(player)
	if !isContainerBlock(blockIDAt(blockWorld, posX, posY, posZ)) {
		return nil, false
	}
	var tile, found = blockWorld.TileAt(posX, posY, posZ)
	if !found {
		return nil, false
	}
	var halves []world.Tile = chestHalves(blockWorld, tile)
	var eyeX, eyeY, eyeZ = player.Position()
	var index int = 0
	for index = 0; index < len(halves); index++ {
		var cell blockPos = blockPos{PosX: halves[index].X, PosY: halves[index].Y, PosZ: halves[index].Z}
		if blockInReach(eyeX, eyeY, eyeZ, cell, maxInteractDistance) {
			return halves, true
		}
	}
	return nil, false
}

// handleContainerWindowSlot 处理客户端对容器窗口某一格的改动。
// 目前容器里的内容还是信客户端报的（背包事务校验在后面单独做），但窗口必须真开着、人必须够得着。
func (srv *Server) handleContainerWindowSlot(player *Player, pkt *packets.ContainerSetSlotPacket) {
	if pkt.WindowID != protocol.WindowContainer {
		return
	}
	var halves, usable = srv.openContainerTiles(player)
	if !usable {
		return
	}
	// 末影箱的格子在玩家自己身上，方块实体里没有，不走下面按方块实体找格子的路。
	if halves[0].ID == world.TileEnderChest {
		player.setEnderChestSlot(int(pkt.Slot), pkt.Item)
		return
	}
	var target, local, inRange = windowSlotTarget(halves, int(pkt.Slot))
	if !inRange {
		return
	}
	var current protocol.Item = protocol.Air()
	var rejected bool = false
	srv.playerWorld(player).UpdateTile(target.X, target.Y, target.Z, func(live *world.Tile) bool {
		if local >= len(live.Items) {
			return false
		}
		// 熔炉的产物格只能取不能放：不允许凭空多出来，也不允许换成别的东西。
		if live.ID == world.TileFurnace && local == smelting.SlotOutput &&
			!smelting.OutputChangeAllowed(live.Items[local], pkt.Item) {
			current = live.Items[local]
			rejected = true
			return false
		}
		live.Items[local] = pkt.Item
		return true
	})
	if rejected {
		srv.resendContainerSlot(player, int(pkt.Slot), current)
	}
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
// 大箱子拆掉一半：只掉拆掉这半的东西，但两半的窗口都要关（格数变了），剩下那半重画成单箱。
// 调用时 RemoveTile 已经把搭档的配对解掉了，所以要靠 tile 里留着的配对信息找到它。
func (srv *Server) releaseTile(worldName string, tile world.Tile) {
	srv.dropTileContents(worldName, tile)
	srv.closeContainerViewers(worldName, tile.X, tile.Y, tile.Z)
	if tile.ID == world.TileChest && tile.Paired {
		srv.closeContainerViewers(worldName, tile.PairX, tile.Y, tile.PairZ)
		srv.redrawChest(worldName, tile.PairX, tile.Y, tile.PairZ)
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
