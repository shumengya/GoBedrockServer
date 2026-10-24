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
		var changes []blockrule.Change = blockrule.OnNeighborChange(view, pos)
		var changeIndex int = 0
		for changeIndex = 0; changeIndex < len(changes); changeIndex++ {
			if srv.applyChangeLocked(worldName, blockWorld, changes[changeIndex]) {
				queue = enqueueAround(queue, queued, changes[changeIndex].Pos)
			}
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
	if edit.ID == blockrule.BlockAir {
		var tile, hasTile = blockWorld.RemoveTile(posX, posY, posZ)
		if hasTile && tile.ID == world.TileChest {
			srv.dropChestContents(worldName, tile)
		}
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
	return true
}
