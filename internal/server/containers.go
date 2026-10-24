package server

import (
	"go-mc-server/internal/network"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/world"
)

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

func (srv *Server) openChest(p *Player, x, y, z int32) {
	if p == nil || p.Session == nil {
		return
	}
	w := srv.playerWorld(p)
	t, ok := w.TileAt(x, y, z)
	if !ok || t.ID != world.TileChest {
		t = world.NewChestTile(x, y, z)
		w.SetTile(t)
	}
	items := t.Items
	if len(items) != protocol.ChestSize {
		padded := make([]protocol.Item, protocol.ChestSize)
		copy(padded, items)
		items = padded
	}
	p.mu.Lock()
	p.OpenWindow = protocol.WindowChest
	p.OpenWindowType = protocol.InventoryTypeContainer
	p.OpenTX, p.OpenTY, p.OpenTZ = x, y, z
	eid := p.EntityRuntimeID
	sess := p.Session
	p.mu.Unlock()
	sess.SendPacket(&packets.ContainerOpenPacket{
		WindowID: protocol.WindowChest,
		Type:     protocol.InventoryTypeContainer,
		X:        x, Y: y, Z: z,
		EntityID: -1,
	})
	sess.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.WindowChest,
		TargetEID: eid,
		Slots:     items,
	})
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

func (srv *Server) dropChestContents(worldName string, t world.Tile) {
	for _, it := range t.Items {
		if it.IsAir() {
			continue
		}
		srv.spawnDroppedItem(it, float32(t.X)+0.5, float32(t.Y)+0.5, float32(t.Z)+0.5, 0, 0.1, 0, 10, worldName)
	}
}
