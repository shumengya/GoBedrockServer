package server

import "maps"
import "math"
import "slices"
import "sync/atomic"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// entityMetadata 生成出生包里的实体元数据。下落的方块要多带一项 variant：
// 方块 ID 放低 8 位，方块 meta 放在它上面，客户端据此画出是哪种方块在掉。
func entityMetadata(instance *entity.Instance) []packets.MetadataEntry {
	var metadata []packets.MetadataEntry = packets.DefaultEntityMetadata()
	if instance.IsFallingBlock() {
		metadata = append(metadata, packets.MetadataEntry{
			Key:  packets.EntityDataVariant,
			Type: packets.EntityDataTypeInt,
			Int:  int32(instance.BlockID) | int32(instance.BlockData)<<8,
		})
	}
	return metadata
}

// spawnFallingBlock 在 pos 这一格生成一个下落中的方块实体，blockID、blockData 是原来的方块。
// 调用方持有 blockMu，这里再拿 mu，顺序和 blockMu 的约定一致。
func (srv *Server) spawnFallingBlock(worldName string, pos blockrule.Pos, blockID byte, blockData byte) {
	var kind, found = entity.ByID(entity.NetworkIDFallingSand)
	if !found {
		return
	}
	var runtimeID int64 = atomic.AddInt64(&srv.entityIDCounter, 1)
	var instance *entity.Instance = &entity.Instance{
		RuntimeID: runtimeID,
		Type:      kind,
		X:         float32(pos.PosX) + 0.5,
		Y:         float32(pos.PosY),
		Z:         float32(pos.PosZ) + 0.5,
		WorldName: worldName,
		BlockID:   blockID,
		BlockData: blockData,
	}
	srv.mu.Lock()
	srv.entities[runtimeID] = instance
	srv.mu.Unlock()
	srv.broadcastInWorld(worldName, addEntityPacket(instance), "")
}

// tickFallingBlocks 每个 tick 让所有下落中的方块走一步。
func (srv *Server) tickFallingBlocks() {
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	srv.mu.RLock()
	var ids []int64 = slices.Sorted(maps.Keys(srv.entities))
	srv.mu.RUnlock()
	var index int = 0
	for index = 0; index < len(ids); index++ {
		srv.mu.RLock()
		var instance *entity.Instance = srv.entities[ids[index]]
		srv.mu.RUnlock()
		if !instance.IsFallingBlock() {
			continue
		}
		srv.stepFallingBlockLocked(instance)
	}
}

func (srv *Server) stepFallingBlockLocked(instance *entity.Instance) {
	var blockWorld *world.World = srv.worldByName(instance.WorldName)
	var cellX int32 = int32(math.Floor(float64(instance.X)))
	var cellZ int32 = int32(math.Floor(float64(instance.Z)))
	// 位置字段别的协程在发出生包时也会读，改的时候要拿 mu。
	srv.mu.Lock()
	var landed bool = entity.StepFallingBlock(instance, func(cellY int32) bool {
		return fallingSolid(blockWorld, cellX, cellY, cellZ)
	})
	var posY float32 = instance.Y
	srv.mu.Unlock()
	if landed {
		srv.landFallingBlockLocked(instance, blockWorld)
		return
	}
	if posY < entity.FallingVoidY {
		srv.removeFallingEntity(instance)
		return
	}
	srv.broadcastInWorld(instance.WorldName, &packets.MoveEntityPacket{
		EntityRuntimeID: instance.RuntimeID,
		X:               instance.X,
		Y:               posY + entity.FallingBaseOffset,
		Z:               instance.Z,
		OnGround:        false,
	}, "")
}

// fallingSolid 判断下落列里的这一格是不是实心。区块没加载的当成实心，让方块停住，
// 而不是穿过一片还没读进内存的地面；世界底部以下读到的是空气，方块会一路掉进虚空。
func fallingSolid(blockWorld *world.World, cellX int32, cellY int32, cellZ int32) bool {
	var id, _, loaded = blockWorld.Peek(cellX, cellY, cellZ)
	if !loaded {
		return true
	}
	return blockrule.IsSolid(id)
}

func (srv *Server) removeFallingEntity(instance *entity.Instance) {
	srv.mu.Lock()
	delete(srv.entities, instance.RuntimeID)
	srv.mu.Unlock()
	srv.broadcastInWorld(instance.WorldName, &packets.RemoveEntityPacket{EntityUniqueID: instance.RuntimeID}, "")
}

// landFallingBlockLocked 落地：落点能顶替（空气、水、高草这类）就把方块放回去，
// 落点被占着（比如一根火把）就掉成物品，对齐参考端 FallingSand。
func (srv *Server) landFallingBlockLocked(instance *entity.Instance, blockWorld *world.World) {
	var cell blockrule.Pos = blockrule.Pos{
		PosX: int32(math.Floor(float64(instance.X))),
		PosY: int32(math.Floor(float64(instance.Y))),
		PosZ: int32(math.Floor(float64(instance.Z))),
	}
	srv.removeFallingEntity(instance)
	var existingID, existingMeta = blockWorld.GetBlock(cell.PosX, cell.PosY, cell.PosZ)
	if !blockrule.IsReplaceable(existingID, existingMeta) {
		srv.spawnDroppedItem(
			protocol.Item{ID: int32(instance.BlockID), Meta: int32(instance.BlockData), Count: 1},
			float32(cell.PosX)+0.5,
			float32(cell.PosY)+0.5,
			float32(cell.PosZ)+0.5,
			0,
			0.1,
			0,
			10,
			instance.WorldName,
		)
		return
	}
	srv.applyChangeLocked(
		instance.WorldName,
		blockWorld,
		blockrule.Change{Pos: cell, ID: instance.BlockID, Meta: instance.BlockData},
	)
	srv.settleBlocksLocked(instance.WorldName, blockWorld, []blockrule.Pos{cell})
}
