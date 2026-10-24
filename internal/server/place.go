package server

import "fmt"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// placeHeldBlock 处理玩家用手持方块点击世界：算落点和朝向，校验，写入，广播，扣一个。
// 整个过程在 blockMu 里完成。两个玩家同时往同一格放方块时，后到的会看到先到的已经放好，
// 而不是两个都通过"落点是空气"的检查，一个方块被放两次、背包扣两次。
func (srv *Server) placeHeldBlock(player *Player, pkt *packets.UseItemPacket, held protocol.Item) {
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld *world.World = srv.playerWorld(player)
	var worldName string = srv.playerWorldName(player)
	var yaw, _ = player.Look()
	var target blockrule.Pos = blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	var changes, placed = blockrule.Place(
		loadingView{target: blockWorld},
		blockrule.PlaceRequest{
			ItemID:   held.ID,
			ItemMeta: held.Meta,
			Target:   target,
			Face:     pkt.Face,
			ClickX:   pkt.FX,
			ClickY:   pkt.FY,
			ClickZ:   pkt.FZ,
			Yaw:      yaw,
		},
	)
	if !placed {
		logger.Debug(
			"server",
			"%s 放置忽略: 面 %d 对 (%d,%d,%d) 放不下 id=%d",
			player.Username,
			pkt.Face,
			pkt.X,
			pkt.Y,
			pkt.Z,
			held.ID,
		)
		// 种子、门这类客户端会先自己摆上去，服务端不认就要把真实状态发回去。
		srv.resyncPlacement(player, blockWorld, target, pkt.Face, nil)
		return
	}
	var reason, notice = srv.placementRejection(player, held, changes)
	if reason != "" {
		logger.Debug("server", "%s 放置被拒: %s", player.Username, reason)
		srv.notifyBlocked(player, notice)
		srv.resyncPlacement(player, blockWorld, target, pkt.Face, changes)
		return
	}
	var index int = 0
	for index = 0; index < len(changes); index++ {
		var edit blockrule.Change = changes[index]
		var cell blockPos = changeCell(edit)
		if !blockWorld.SetBlock(cell.PosX, cell.PosY, cell.PosZ, edit.ID, edit.Meta) {
			logger.Debug("server", "%s 放置失败 (%d,%d,%d)", player.Username, cell.PosX, cell.PosY, cell.PosZ)
			srv.resyncPlacement(player, blockWorld, target, pkt.Face, changes)
			return
		}
		logger.Info(
			"server",
			"%s 放置方块 id=%d 于 (%d,%d,%d)",
			player.Username,
			edit.ID,
			cell.PosX,
			cell.PosY,
			cell.PosZ,
		)
		srv.broadcastBlock(worldName, cell.PosX, cell.PosY, cell.PosZ, uint32(edit.ID), uint32(edit.Meta))
		srv.onBlockPlaced(player, cell.PosX, cell.PosY, cell.PosZ, edit.ID)
	}
	var origins []blockrule.Pos = make([]blockrule.Pos, 0, len(changes))
	for index = 0; index < len(changes); index++ {
		origins = append(origins, changes[index].Pos)
	}
	srv.settleBlocksLocked(worldName, blockWorld, origins)
	var slot, remaining, changed = player.ConsumeHeld(1)
	if changed {
		srv.sendInventorySlot(player, slot, remaining)
		// 最后一个方块放完手变空，别人那边也要同步
		if remaining.IsAir() {
			srv.broadcastHeldItem(player)
		}
	}
}

// placementRejection 逐项检查这次放置能不能做。
// 返回拒绝原因（写日志用），空串表示放行；第二个返回值是给玩家看的提示，可以为空。
func (srv *Server) placementRejection(
	player *Player,
	held protocol.Item,
	changes []blockrule.Change,
) (string, string) {
	var posX, posY, posZ = player.Position()
	var reach float32 = srv.placeDistance(player)
	var index int = 0
	for index = 0; index < len(changes); index++ {
		var cell blockPos = changeCell(changes[index])
		if !blockInReach(posX, posY, posZ, cell, reach) {
			return fmt.Sprintf("(%d,%d,%d) 距离过远", cell.PosX, cell.PosY, cell.PosZ), "§c目标方块太远，够不着"
		}
		if srv.spawnProtected(player, cell.PosX, cell.PosZ) {
			return fmt.Sprintf(
				"(%d,%d,%d) 在出生点保护范围内",
				cell.PosX,
				cell.PosY,
				cell.PosZ,
			), "§c这里在出生点保护范围内，不能放置方块"
		}
	}
	// 背包里必须真有这个方块，创造模式跳过。
	if !player.HasItem(held, 1) {
		return fmt.Sprintf("背包里没有 %d:%d", held.ID, held.Meta),
			fmt.Sprintf("§c背包里没有 %d:%d，放不了", held.ID, held.Meta)
	}
	// 实心方块不能放进玩家自己的脚底、头、头上那一格；火把、花这类不实心的可以。
	for index = 0; index < len(changes); index++ {
		var edit blockrule.Change = changes[index]
		var cell blockPos = changeCell(edit)
		if blockrule.IsSolid(edit.ID) && IsBodyColumn(posX, posY, posZ, cell.PosX, cell.PosY, cell.PosZ) {
			return fmt.Sprintf(
				"(%d,%d,%d) 是脚底/头/下一格 pos=(%.2f,%.2f,%.2f)",
				cell.PosX,
				cell.PosY,
				cell.PosZ,
				posX,
				posY,
				posZ,
			), ""
		}
	}
	return "", ""
}

func changeCell(edit blockrule.Change) blockPos {
	return blockPos{PosX: edit.Pos.PosX, PosY: edit.Pos.PosY, PosZ: edit.Pos.PosZ}
}

// resyncBlock 把服务端这一格真实的方块发给玩家，纠正客户端自己先摆上去的方块。
func (srv *Server) resyncBlock(player *Player, blockWorld *world.World, pos blockrule.Pos) {
	if player == nil || player.Session == nil || pos.PosY < 0 || pos.PosY > 255 {
		return
	}
	var blockID, blockMeta = blockWorld.GetBlock(pos.PosX, pos.PosY, pos.PosZ)
	player.Session.SendPacket(&packets.UpdateBlockPacket{
		X:         pos.PosX,
		Y:         pos.PosY,
		Z:         pos.PosZ,
		BlockID:   uint32(blockID),
		BlockData: uint32(blockMeta),
		Flags:     protocol.UpdateBlockFlagAllPriority,
	})
}

// resyncPlacement 放置被拒后，把客户端可能自己先摆过的格子都推回真实状态：
// 被点击的方块、它旁边的落点，以及规则算出来要改的格子。
// 只发给放置的人，其他玩家根本没收到过这次放置。
func (srv *Server) resyncPlacement(
	player *Player,
	blockWorld *world.World,
	target blockrule.Pos,
	face int32,
	changes []blockrule.Change,
) {
	srv.resyncBlock(player, blockWorld, target)
	var side, sideOK = target.Side(face)
	if sideOK {
		srv.resyncBlock(player, blockWorld, side)
	}
	var index int = 0
	for index = 0; index < len(changes); index++ {
		srv.resyncBlock(player, blockWorld, changes[index].Pos)
	}
}
