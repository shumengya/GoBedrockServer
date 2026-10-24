package server

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// settleBudget 是一次级联最多评估多少格。正常情况下一棵甘蔗塔也就几十格，
// 超过这个数说明规则之间在互相触发，直接停下并记一条警告，不能卡死 tick。
const settleBudget int = 4096

// settleBlocksLocked 在一批方块变了之后，让它们自己和四周的邻居重新评估，
// 受影响的方块变了再往外传，直到没有新的变化。调用方必须已经持有 blockMu。
func (srv *Server) settleBlocksLocked(worldName string, blockWorld *world.World, origins []blockrule.Pos) {
	var view peekView = peekView{target: blockWorld}
	var queue []blockrule.Pos = nil
	var queued map[blockrule.Pos]bool = map[blockrule.Pos]bool{}
	var index int = 0
	for index = 0; index < len(origins); index++ {
		queue = enqueueAround(queue, queued, origins[index])
	}
	var evaluated int = 0
	for evaluated = 0; len(queue) > 0; evaluated++ {
		if evaluated >= settleBudget {
			logger.Warn("server", "方块更新级联超过 %d 次，已中止 world=%s", settleBudget, worldName)
			return
		}
		var pos blockrule.Pos = queue[0]
		queue = queue[1:]
		delete(queued, pos)
		var update blockrule.Update = blockrule.OnNeighborUpdate(view, pos)
		var changeIndex int = 0
		for changeIndex = 0; changeIndex < len(update.Changes); changeIndex++ {
			if srv.applyChangeLocked(worldName, blockWorld, update.Changes[changeIndex]) {
				queue = enqueueAround(queue, queued, update.Changes[changeIndex].Pos)
			}
		}
		for changeIndex = 0; changeIndex < len(update.Schedules); changeIndex++ {
			var request blockrule.Schedule = update.Schedules[changeIndex]
			srv.scheduleLocked(worldName, request.Pos, request.Delay)
		}
	}
}

// enqueueAround 把一格和它的六个邻居排进待评估队列，已经在队列里的不重复排。
func enqueueAround(queue []blockrule.Pos, queued map[blockrule.Pos]bool, center blockrule.Pos) []blockrule.Pos {
	queue = enqueueOne(queue, queued, center)
	var face int32 = 0
	for face = blockrule.FaceDown; face <= blockrule.FaceEast; face++ {
		var side, _ = center.Side(face)
		queue = enqueueOne(queue, queued, side)
	}
	return queue
}

func enqueueOne(queue []blockrule.Pos, queued map[blockrule.Pos]bool, pos blockrule.Pos) []blockrule.Pos {
	if pos.PosY < 0 || pos.PosY > 255 || queued[pos] {
		return queue
	}
	queued[pos] = true
	return append(queue, pos)
}

// applyChangeLocked 把规则算出来的一格改动落到世界上：写方块、清方块实体、
// 按需结算掉落、广播给同世界的玩家。返回 false 表示这格没有实际变化。
func (srv *Server) applyChangeLocked(worldName string, blockWorld *world.World, edit blockrule.Change) bool {
	var posX int32 = edit.Pos.PosX
	var posY int32 = edit.Pos.PosY
	var posZ int32 = edit.Pos.PosZ
	var oldID, oldMeta = blockWorld.GetBlock(posX, posY, posZ)
	if oldID == edit.ID && oldMeta == edit.Meta {
		return false
	}
	if !blockWorld.SetBlock(posX, posY, posZ, edit.ID, edit.Meta) {
		return false
	}
	var bedTile world.Tile = world.Tile{}
	var isBed bool = false
	if edit.ID == blockrule.BlockAir {
		var tile, hasTile = blockWorld.RemoveTile(posX, posY, posZ)
		if hasTile {
			srv.releaseTile(worldName, tile)
		}
		bedTile = tile
		isBed = hasTile && tile.ID == world.TileBed
	}
	if edit.Drop {
		srv.broadcastLevelEvent(
			worldName,
			packets.LevelEventParticleDestroy,
			float32(posX),
			float32(posY),
			float32(posZ),
			int32(oldID)|int32(oldMeta)<<8,
		)
		// 自己掉下来的方块按空手结算掉落，不受任何玩家手持工具影响。
		var drops []protocol.Item = blockDrops(oldID, oldMeta, protocol.Air(), defaultDropRandom{})
		if isBed {
			drops = withBedColor(drops, bedTile.Color)
		}
		var dropIndex int = 0
		for dropIndex = 0; dropIndex < len(drops); dropIndex++ {
			srv.spawnDroppedItem(
				drops[dropIndex],
				float32(posX)+0.5,
				float32(posY)+0.5,
				float32(posZ)+0.5,
				0,
				0.1,
				0,
				10,
				worldName,
			)
		}
	}
	srv.broadcastBlock(worldName, posX, posY, posZ, uint32(edit.ID), uint32(edit.Meta))
	if edit.Fall {
		srv.spawnFallingBlock(worldName, edit.Pos, oldID, oldMeta)
	}
	return true
}

// bedItemID 是床的物品 ID。床的颜色在物品 meta 里，方块 meta 里没有，
// 所以掉落物的颜色要从方块实体里取，不能用 blockDrops 里写死的红色。
const bedItemID int32 = 355

// withBedColor 把掉落里的床换成方块实体记着的颜色。
func withBedColor(drops []protocol.Item, color byte) []protocol.Item {
	var index int = 0
	for index = 0; index < len(drops); index++ {
		if drops[index].ID == bedItemID {
			drops[index].Meta = int32(color)
		}
	}
	return drops
}

// removePartnerLocked 玩家挖掉两格方块（门、床、双层植物）的一半时，另一半跟着一起拆掉。
// 返回另一半按玩家手持物算出的掉落，由调用方决定要不要真的生成（创造模式不生成）。
// 门的下半、床头、双层植物的上半才会有掉落，所以不管先挖哪一半，一共只掉一个。
func (srv *Server) removePartnerLocked(
	worldName string,
	blockWorld *world.World,
	partner blockrule.Pos,
	held protocol.Item,
) []protocol.Item {
	var oldID, oldMeta = blockWorld.GetBlock(partner.PosX, partner.PosY, partner.PosZ)
	var tile, hasTile = blockWorld.TileAt(partner.PosX, partner.PosY, partner.PosZ)
	srv.broadcastLevelEvent(
		worldName,
		packets.LevelEventParticleDestroy,
		float32(partner.PosX),
		float32(partner.PosY),
		float32(partner.PosZ),
		int32(oldID)|int32(oldMeta)<<8,
	)
	srv.applyChangeLocked(worldName, blockWorld, blockrule.Change{Pos: partner, ID: blockrule.BlockAir, Meta: 0})
	var drops []protocol.Item = blockDrops(oldID, oldMeta, held, defaultDropRandom{})
	if hasTile && tile.ID == world.TileBed {
		drops = withBedColor(drops, tile.Color)
	}
	return drops
}
