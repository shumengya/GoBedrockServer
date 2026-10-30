package server

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// enderChestItems 玩家末影箱当前的 27 格，拷贝一份给调用方。没用过的就是 27 个空格。
func (p *Player) enderChestItems() []protocol.Item {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var items []protocol.Item = make([]protocol.Item, protocol.ChestSize)
	copy(items, p.EnderChest)
	return items
}

// setEnderChestSlot 写玩家末影箱里的一格，越界返回 false。第一次写时才补满 27 格。
func (p *Player) setEnderChestSlot(slot int, item protocol.Item) bool {
	if slot < 0 || slot >= protocol.ChestSize {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.EnderChest) != protocol.ChestSize {
		var padded []protocol.Item = make([]protocol.Item, protocol.ChestSize)
		copy(padded, p.EnderChest)
		p.EnderChest = padded
	}
	p.EnderChest[slot] = item
	return true
}

// windowContents 打开窗口时发给客户端的那一整排格子。末影箱取玩家自己的，别的容器取方块实体里的。
func (srv *Server) windowContents(player *Player, halves []world.Tile) []protocol.Item {
	if halves[0].ID == world.TileEnderChest {
		return player.enderChestItems()
	}
	return windowItems(halves)
}
