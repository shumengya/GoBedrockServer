package server

import "math"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// useItemOnBlock 处理拿着锄头、骨粉这类不是方块的物品点击方块。
// 返回 true 表示这次点击被物品消耗了，不再走放置流程。潜行时不触发，和方块的右键交互一样。
func (srv *Server) useItemOnBlock(player *Player, pkt *packets.UseItemPacket, held protocol.Item) bool {
	if player.isSneaking() {
		return false
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld = srv.playerWorld(player)
	var worldName string = srv.playerWorldName(player)
	var result, handled = blockrule.UseItem(
		peekView{target: blockWorld},
		blockrule.ItemUseRequest{
			ItemID:   held.ID,
			ItemMeta: held.Meta,
			Target:   blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z},
			Face:     pkt.Face,
		},
		defaultDropRandom{},
	)
	if !handled {
		return false
	}
	if player.gameMode() == int32(protocol.GameModeSpectator) {
		return true
	}
	if srv.spawnProtected(player, pkt.X, pkt.Z) {
		srv.notifyBlocked(player, "§c这里在出生点保护范围内，不能操作方块")
		return true
	}
	srv.applyUpdateLocked(worldName, blockWorld, result.Update)
	if result.Consume {
		var slot, remaining, changed = player.ConsumeHeld(1)
		if changed {
			srv.sendInventorySlot(player, slot, remaining)
			if remaining.IsAir() {
				srv.broadcastHeldItem(player)
			}
		}
	}
	if result.Wear > 0 {
		srv.useHeldTool(player, result.Wear)
	}
	return true
}

// trampleFarmland 玩家落地时，脚下是耕地就有概率被踩成泥土。落差由客户端上报。
func (srv *Server) trampleFarmland(player *Player, fallDistance float32, random dropRandom) {
	if player.gameMode() == int32(protocol.GameModeSpectator) {
		return
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var posX, posY, posZ = player.Position()
	// 玩家记的是眼睛的位置，减去眼高再往下一点点，就是脚底踩着的那一格。
	var spot blockrule.Pos = blockrule.Pos{
		PosX: int32(math.Floor(float64(posX))),
		PosY: int32(math.Floor(float64(posY - playerEyeHeight - 0.01))),
		PosZ: int32(math.Floor(float64(posZ))),
	}
	var blockWorld = srv.playerWorld(player)
	var update, trampled = blockrule.Trample(peekView{target: blockWorld}, spot, fallDistance, random)
	if !trampled {
		return
	}
	srv.applyUpdateLocked(srv.playerWorldName(player), blockWorld, update)
}
