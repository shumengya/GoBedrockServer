package server

import "go-mc-server/internal/logger"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

// armorSnapshot 身上盔甲的拷贝。
func (p *Player) armorSnapshot() [protocol.ArmorSize]protocol.Item {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Armor
}

// armorPoints 身上盔甲的护甲点数之和。
func (p *Player) armorPoints() int32 {
	return armorPointsOf(p.armorSnapshot())
}

// setArmorSlot 写一个盔甲槽。下标越界返回 false。
func (p *Player) setArmorSlot(slot int, item protocol.Item) bool {
	if slot < 0 || slot >= protocol.ArmorSize {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Armor[slot] = item
	return true
}

// wearArmor 被打中一次，身上每件盔甲磨损 1 点；磨满的那件消失。创造和旁观不会走到这里（它们不受伤）。
// 返回是否有盔甲被改动。
func (p *Player) wearArmor() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	var changed bool = false
	var slot int = 0
	for slot = 0; slot < len(p.Armor); slot++ {
		var info, found = armorInfoFor(p.Armor[slot])
		if !found {
			continue
		}
		var worn protocol.Item = p.Armor[slot]
		worn.Meta = max(worn.Meta, 0) + 1
		if worn.Meta >= info.MaxDurability {
			worn = protocol.Air()
		}
		p.Armor[slot] = worn
		changed = true
	}
	return changed
}

// armorEmpty 四个槽都是空的。
func armorEmpty(armor [protocol.ArmorSize]protocol.Item) bool {
	var slot int = 0
	for slot = 0; slot < len(armor); slot++ {
		if !armor[slot].IsAir() {
			return false
		}
	}
	return true
}

// armorToSlots 存档用。全空就不写，省得每个存档带 4 个空格。
func armorToSlots(armor [protocol.ArmorSize]protocol.Item) []storage.SlotData {
	if armorEmpty(armor) {
		return nil
	}
	return storage.SlotsFromItems(armor[:])
}

// armorFromSlots 读档用。放错槽、数量不对、损耗值越界的一律丢掉，不信存档文件。
func armorFromSlots(slots []storage.SlotData) [protocol.ArmorSize]protocol.Item {
	var armor [protocol.ArmorSize]protocol.Item
	var items []protocol.Item = storage.ItemsFromSlots(slots, protocol.ArmorSize)
	var slot int = 0
	for slot = 0; slot < len(armor); slot++ {
		if armorAcceptable(slot, items[slot]) {
			armor[slot] = items[slot]
		}
	}
	return armor
}

// armorPacketFor 其他玩家看到的这个人的穿戴。
func armorPacketFor(player *Player) *packets.MobArmorEquipmentPacket {
	return &packets.MobArmorEquipmentPacket{
		EntityRuntimeID: player.EntityRuntimeID,
		Slots:           player.armorSnapshot(),
	}
}

// sendOwnArmor 把盔甲发给玩家自己：走盔甲窗口的 ContainerSetContent，不是 MobArmorEquipment。
func (srv *Server) sendOwnArmor(player *Player) {
	if player == nil || player.Session == nil {
		return
	}
	var armor [protocol.ArmorSize]protocol.Item = player.armorSnapshot()
	player.Session.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.ContainerArmor,
		TargetEID: player.EntityRuntimeID,
		Slots:     armor[:],
	})
}

// broadcastArmor 让同世界看得见这个玩家的人更新他的穿戴。
func (srv *Server) broadcastArmor(player *Player) {
	if !player.isVisible() {
		return
	}
	sendToPlayers(srv.viewersOf(player), armorPacketFor(player))
}

// syncArmor 盔甲变了：自己和旁人都更新。
func (srv *Server) syncArmor(player *Player) {
	srv.sendOwnArmor(player)
	srv.broadcastArmor(player)
}

// exchangeArmor 玩家刚出现在 viewers 眼前：互相告知身上穿着什么。
// AddPlayer 之后才能发，实体还没建出来时客户端会丢掉这个包。全空的不用发。
func exchangeArmor(player *Player, viewers []*Player) {
	if !armorEmpty(player.armorSnapshot()) {
		sendToPlayers(viewers, armorPacketFor(player))
	}
	if player.Session == nil {
		return
	}
	var index int = 0
	for index = 0; index < len(viewers); index++ {
		if !armorEmpty(viewers[index].armorSnapshot()) {
			player.Session.SendPacket(armorPacketFor(viewers[index]))
		}
	}
}

// wearArmorAfterHit 被打中后磨损盔甲并同步。
func (srv *Server) wearArmorAfterHit(target *Player) {
	if target.wearArmor() {
		srv.syncArmor(target)
	}
}

// handleArmorSlot 客户端往盔甲槽里放东西（或拿走）。放得进去才收：必须是这个槽该穿的盔甲，
// 数量 1，损耗值在范围里；否则把服务端真实的这一格发回去，它本地多画出来的会消掉。
func (srv *Server) handleArmorSlot(player *Player, pkt *packets.ContainerSetSlotPacket) {
	if player.IsDead() || pkt.Slot < 0 || pkt.Slot >= protocol.ArmorSize {
		return
	}
	var slot int = int(pkt.Slot)
	player.mu.Lock()
	var current = player.Armor[slot]
	var valid = armorAcceptable(slot,pkt.Item) && player.exchangeSlotLocked(current,pkt.Item)
	if valid {
		player.Armor[slot] = pkt.Item
		if pkt.Item.IsAir() { player.Armor[slot] = protocol.Air() }
	}
	player.mu.Unlock()
	if !valid {
		logger.Debug("server", "%s 盔甲槽 %d 拒绝了 %d:%d x%d", player.Username, slot, pkt.Item.ID, pkt.Item.Meta, pkt.Item.Count)
		if player.Session != nil {
			player.Session.SendPacket(&packets.ContainerSetSlotPacket{
				WindowID: protocol.ContainerArmor,
				Slot:     pkt.Slot,
				Item:     current,
			})
		}
		return
	}
	srv.broadcastArmor(player)
}
