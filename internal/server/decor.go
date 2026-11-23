package server

import "math"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

func (srv *Server) onDecorPlaced(player *Player, posX, posY, posZ int32, blockID, skullType byte) {
	var blockWorld = srv.playerWorld(player)
	var tile world.Tile = world.Tile{}
	switch blockID {
	case world.BlockFlowerPot:
		tile = world.NewFlowerPotTile(posX, posY, posZ)
	case world.BlockItemFrame:
		tile = world.NewItemFrameTile(posX, posY, posZ)
	case world.BlockHopper:
		tile = world.NewHopperTile(posX, posY, posZ)
	case world.BlockSkull:
		var rotation byte = 0
		var _, face = blockWorld.GetBlock(posX, posY, posZ)
		if face == byte(blockrule.FaceUp) {
			var yaw, _ = player.Look()
			rotation = byte(int32(math.Floor(float64(yaw)*16/360+0.5)) & 15)
		}
		tile = world.NewSkullTile(posX, posY, posZ, skullType, rotation)
	}
	blockWorld.SetTile(tile)
	srv.broadcastTile(srv.playerWorldName(player), tile)
}

// 跟拆方块共用 blockMu：取出的内容和旋转角都属于点击时还在原位的那个方块。
func (srv *Server) interactDecor(player *Player, pkt *packets.UseItemPacket) bool {
	if player.isSneaking() {
		return false
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld = srv.playerWorld(player)
	var blockID, _, _ = blockWorld.Peek(pkt.X, pkt.Y, pkt.Z)
	if blockID != world.BlockFlowerPot && blockID != world.BlockItemFrame {
		return false
	}
	if player.gameMode() == int32(protocol.GameModeSpectator) || srv.spawnProtected(player, pkt.X, pkt.Z) {
		return true
	}
	var pot = blockID == world.BlockFlowerPot
	var tile, found = blockWorld.TileAt(pkt.X, pkt.Y, pkt.Z)
	if !found {
		if pot {
			tile = world.NewFlowerPotTile(pkt.X, pkt.Y, pkt.Z)
		} else {
			tile = world.NewItemFrameTile(pkt.X, pkt.Y, pkt.Z)
		}
	} else if (pot && tile.ID != world.TileFlowerPot) || (!pot && tile.ID != world.TileItemFrame) {
		return true
	}
	var worldName = srv.playerWorldName(player)
	if !tile.Items[0].IsAir() {
		if !pot {
			tile.Rotation = (tile.Rotation + 1) % 8
			blockWorld.SetTile(tile)
			srv.broadcastTile(worldName, tile)
		}
		return true
	}
	var item, slot, remaining, changed = player.takeDecorItem(pot)
	if item.IsAir() {
		return true
	}
	tile.Items[0] = item
	blockWorld.SetTile(tile)
	if pot {
		srv.applyChangeLocked(worldName, blockWorld, blockrule.Change{Pos: blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}, ID: world.BlockFlowerPot, Meta: 1})
	}
	srv.broadcastTile(worldName, tile)
	if changed {
		srv.sendInventorySlot(player, slot, remaining)
		if remaining.IsAir() {
			srv.broadcastHeldItem(player)
		}
	}
	return true
}

// 在同一把玩家锁下从当前绑定的槽取样并扣除一件，避免读到的物品和实际扣掉的物品不同。
func (player *Player) takeDecorItem(pot bool) (protocol.Item, int, protocol.Item, bool) {
	player.mu.Lock()
	defer player.mu.Unlock()
	var slot = player.heldInventorySlotLocked()
	if slot < 0 || slot >= len(player.Inventory) {
		return protocol.Air(), slot, protocol.Air(), false
	}
	var held = player.Inventory[slot]
	if held.IsAir() || (pot && !blockrule.FlowerPotAccepts(held.ID, held.Meta)) {
		return protocol.Air(), slot, held, false
	}
	held.Count = 1
	var _, remaining, changed = player.consumeHeldLocked(1)
	return held, slot, remaining, changed
}

func (srv *Server) handleItemFrameDrop(sess *network.Session, pkt *packets.ItemFrameDropItemPacket) {
	var player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	var eyeX, eyeY, eyeZ = player.Position()
	if !blockInReach(eyeX, eyeY, eyeZ, blockPos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}, maxInteractDistance) {
		return
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld = srv.playerWorld(player)
	var blockID, _, _ = blockWorld.Peek(pkt.X, pkt.Y, pkt.Z)
	if blockID != world.BlockItemFrame {
		return
	}
	var tile, found = blockWorld.TileAt(pkt.X, pkt.Y, pkt.Z)
	if !found || tile.ID != world.TileItemFrame || tile.Items[0].IsAir() {
		return
	}
	if player.gameMode() == int32(protocol.GameModeSpectator) || srv.spawnProtected(player, pkt.X, pkt.Z) {
		sess.SendPacket(&packets.BlockEntityDataPacket{X: pkt.X, Y: pkt.Y, Z: pkt.Z, NBT: tile.SpawnNBT()})
		return
	}
	var item = tile.Items[0]
	tile.Items[0], tile.Rotation = protocol.Air(), 0
	blockWorld.SetTile(tile)
	var worldName = srv.playerWorldName(player)
	srv.broadcastTile(worldName, tile)
	srv.spawnDroppedItem(item, float32(pkt.X)+0.5, float32(pkt.Y)+0.5, float32(pkt.Z)+0.5, 0, 0.1, 0, 10, worldName)
}
