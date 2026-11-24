package server

import "math/rand/v2"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

const itemSpawnEgg int32 = 383
const blockMobSpawner byte = 52

func (srv *Server) useSpawnEggLocked(player *Player, blockWorld *world.World, pkt *packets.UseItemPacket, held protocol.Item) {
	var target = blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	var side, valid = target.Side(pkt.Face)
	var kind, registered = entity.ByID(held.Meta)
	var clicked, _ = blockWorld.GetBlock(pkt.X, pkt.Y, pkt.Z)
	// 刷怪笼有自己的方块实体交互；这里不能把蛋当作点击普通方块消耗。
	if !valid || side.PosY < 0 || side.PosY > 255 || clicked == blockrule.BlockAir || clicked == blockMobSpawner ||
		!registered || !kind.CanSummon || !sameStack(held, pkt.Item) ||
		srv.spawnProtected(player, target.PosX, target.PosZ) || srv.spawnProtected(player, side.PosX, side.PosZ) {
		srv.sendPlayerInventory(player)
		return
	}
	player.mu.Lock()
	var slot = player.heldInventorySlotLocked()
	if player.combat.Dead || player.GameMode == protocol.GameModeSpectator || slot < 0 ||
		slot >= len(player.Inventory) || !sameStack(player.Inventory[slot], held) || player.Inventory[slot].Count == 0 {
		player.mu.Unlock()
		srv.sendPlayerInventory(player)
		return
	}
	var _, remaining, changed = player.consumeHeldLocked(1)
	player.mu.Unlock()
	srv.addSummonedEntity(&entity.Instance{
		Type: kind, WorldName: blockWorld.Name, CustomName: nbt.ItemDisplayName(held.NBT),
		X: float32(side.PosX) + 0.5, Y: float32(side.PosY), Z: float32(side.PosZ) + 0.5,
		Yaw: rand.Float32() * 360,
	})
	if changed {
		srv.sendInventorySlot(player, slot, remaining)
		srv.broadcastHeldItem(player)
	}
}
