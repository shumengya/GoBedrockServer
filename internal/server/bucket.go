package server

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// exchangeBucketLocked 只从当前快捷栏映射到的实际槽取桶。满包时让调用方掉落装好的桶。
func (player *Player) exchangeBucketLocked(held protocol.Item, resultMeta int32) (protocol.Item, bool) {
	var slot = player.heldInventorySlotLocked()
	if player.combat.Dead || player.GameMode == protocol.GameModeSpectator || slot < 0 || slot >= len(player.Inventory) {
		return protocol.Air(), false
	}
	var stack = player.Inventory[slot]
	if stack.Count == 0 || !sameStack(stack, held) {
		return protocol.Air(), false
	}
	var result = stack
	result.Meta, result.Count = resultMeta, 1
	if player.GameMode == protocol.GameModeCreative {
		if resultMeta != 0 {
			player.addItemLocked(result)
		}
		return protocol.Air(), true
	}
	if stack.Count == 1 {
		player.Inventory[slot], player.HeldItem = result, result
		return protocol.Air(), true
	}
	stack.Count--
	player.Inventory[slot], player.HeldItem = stack, stack
	var overflow, _ = player.addItemLocked(result)
	return overflow, true
}

func (srv *Server) syncBucket(player *Player, overflow protocol.Item) {
	srv.sendPlayerInventory(player)
	srv.broadcastHeldItem(player)
	if !overflow.IsAir() {
		var posX, posY, posZ = player.Position()
		srv.spawnDroppedItem(overflow, posX, posY, posZ, 0, 0.1, 0, 10, srv.playerWorldName(player))
	}
}

func (srv *Server) useBucketLocked(player *Player, blockWorld *world.World, pkt *packets.UseItemPacket, held protocol.Item) {
	var target = blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	var edit, resultMeta, usable = blockrule.UseBucket(loadingView{target: blockWorld}, target, pkt.Face, held.Meta)
	var posX, posY, posZ = player.Position()
	if !usable || !sameStack(pkt.Item, held) || !blockInReach(posX, posY, posZ, changeCell(edit), srv.placeDistance(player)) ||
		srv.spawnProtected(player, pkt.X, pkt.Z) || srv.spawnProtected(player, edit.Pos.PosX, edit.Pos.PosZ) {
		srv.resyncPlacement(player, blockWorld, target, pkt.Face, nil)
		srv.sendPlayerInventory(player)
		return
	}
	player.mu.Lock()
	var overflow, accepted = player.exchangeBucketLocked(held, resultMeta)
	player.mu.Unlock()
	if !accepted {
		srv.resyncPlacement(player, blockWorld, target, pkt.Face, nil)
		srv.sendPlayerInventory(player)
		return
	}
	if edit.ID == blockrule.BlockFlowingWater && blockWorld.ProtocolDimension() == protocol.DimensionNether {
		srv.resyncBlock(player, blockWorld, edit.Pos)
	} else {
		srv.applyUpdateLocked(blockWorld.Name, blockWorld, blockrule.Update{Changes: []blockrule.Change{edit}})
		var sound byte = packets.SoundBucketFillWater
		switch {
		case edit.ID == blockrule.BlockFlowingWater:
			sound = packets.SoundBucketEmptyWater
		case edit.ID == blockrule.BlockFlowingLava:
			sound = packets.SoundBucketEmptyLava
		case resultMeta == int32(blockrule.BlockFlowingLava):
			sound = packets.SoundBucketFillLava
		}
		srv.broadcastInWorld(blockWorld.Name, &packets.LevelSoundEventPacket{SoundID: sound, X: float32(edit.Pos.PosX) + 0.5, Y: float32(edit.Pos.PosY) + 0.5, Z: float32(edit.Pos.PosZ) + 0.5, ExtraData: -1, Pitch: 1}, "")
	}
	srv.syncBucket(player, overflow)
}

func (srv *Server) milkCow(player *Player, target int64) {
	var held = player.heldSnapshot()
	if held.ID != blockrule.ItemBucket || held.Meta != 0 {
		return
	}
	var worldName = srv.playerWorldName(player)
	srv.mu.RLock()
	var cow = srv.entities[target]
	if cow == nil || cow.WorldName != worldName || (cow.Type.NetworkID != 11 && cow.Type.NetworkID != 16) {
		srv.mu.RUnlock()
		return
	}
	var cowX, cowY, cowZ = cow.X, cow.Y, cow.Z
	srv.mu.RUnlock()
	var posX, posY, posZ = player.Position()
	var diffX, diffY, diffZ = posX - cowX, posY - playerEyeHeight - cowY, posZ - cowZ
	if diffX*diffX+diffY*diffY+diffZ*diffZ > maxInteractDistance*maxInteractDistance {
		return
	}
	player.mu.Lock()
	var overflow, accepted = player.exchangeBucketLocked(held, 1)
	player.mu.Unlock()
	if accepted {
		srv.syncBucket(player, overflow)
		srv.broadcastInWorld(worldName, &packets.LevelSoundEventPacket{SoundID: packets.SoundMilk, X: cowX, Y: cowY, Z: cowZ, ExtraData: -1, Pitch: 1}, "")
	}
}

func (srv *Server) drinkMilk(player *Player) {
	player.mu.Lock()
	var held = player.HeldItem
	if held.ID != blockrule.ItemBucket || held.Meta != 1 {
		player.mu.Unlock()
		return
	}
	var overflow, accepted = player.exchangeBucketLocked(held, 0)
	if !accepted {
		player.mu.Unlock()
		return
	}
	var removed = player.combat.Effects
	var hadAbsorption = player.combat.Absorption > 0
	player.combat.Effects, player.combat.Absorption = nil, 0
	player.mu.Unlock()
	player.sendRemovedEffects(removed, hadAbsorption)
	srv.broadcastInWorld(srv.playerWorldName(player), &packets.EntityEventPacket{EntityRuntimeID: player.EntityRuntimeID, Event: packets.EntityEventUseItem}, "")
	srv.syncBucket(player, overflow)
}
