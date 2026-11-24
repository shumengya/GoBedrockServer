package server

import "maps"
import "slices"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/item"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/smelting"
import "go-mc-server/internal/world"

func (srv *Server) tickHoppers() {
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var names = slices.Sorted(maps.Keys(srv.worlds))
	var idx int = 0
	for idx = 0; idx < len(names); idx++ {
		var name = names[idx]
		var level = srv.worlds[name]
		var positions = level.TilePositions(world.TileHopper)
		if len(positions) == 0 {
			continue
		}
		var nearby = srv.hopperDropsByChunk(name)
		var index int = 0
		for index = 0; index < len(positions); index++ {
			var coords = positions[index]
			var pos = blockrule.Pos{PosX: coords[0], PosY: coords[1], PosZ: coords[2]}
			var blockID, face, loaded = level.Peek(pos.PosX, pos.PosY, pos.PosZ)
			if !loaded || blockID != world.BlockHopper {
				continue
			}
			srv.collectHopperDrops(name, level, pos, nearby[[2]int32{coords[0] >> 4, coords[2] >> 4}])
			var hopper, _ = level.TileAt(pos.PosX, pos.PosY, pos.PosZ)
			if hopper.TransferCooldown > 0 {
				hopper.TransferCooldown--
				level.SetTile(hopper)
				continue
			}
			var above = hopperContainers(level, pos.Add(0, 1, 0))
			if len(above) > 0 && srv.hopperMoveOne(name, level, above, []world.Tile{hopper}, true, 0) {
				hopper, _ = level.TileAt(pos.PosX, pos.PosY, pos.PosZ)
			}
			var below, _, _ = level.Peek(pos.PosX, pos.PosY-1, pos.PosZ)
			if below == world.BlockHopper {
				continue
			}
			var target, valid = pos.Side(int32(face))
			if !valid || face == 1 {
				continue
			}
			var output = hopperContainers(level, target)
			if len(output) > 0 {
				srv.hopperMoveOne(name, level, []world.Tile{hopper}, output, false, face)
			}
		}
	}
}

// 只从已加载的普通容器搬运。末影箱属于玩家；花盆和展示框的单格不是容器。
func hopperContainers(level *world.World, pos blockrule.Pos) []world.Tile {
	var blockID, _, loaded = level.Peek(pos.PosX, pos.PosY, pos.PosZ)
	if !loaded || !isContainerBlock(blockID) || blockID == world.BlockEnderChest {
		return nil
	}
	var tile, found = level.TileAt(pos.PosX, pos.PosY, pos.PosZ)
	if !found || tile.ID != containerTileFor(blockID) {
		return nil
	}
	var halves = chestHalves(level, tile)
	if len(halves) == 2 {
		var partnerID, _, present = level.Peek(tile.PairX, tile.Y, tile.PairZ)
		if !present || partnerID != world.BlockChest {
			return []world.Tile{tile}
		}
	}
	return halves
}

// 优先合并完全相同的物品，再用空格。调用方可传入熔炉的单个可写槽。
func hopperSpace(slots []protocol.Item, stack protocol.Item) int {
	var empty = -1
	var limit = item.StackLimit(stack)
	var index int = 0
	for index = 0; index < len(slots); index++ {
		if slots[index].IsAir() {
			if empty < 0 {
				empty = index
			}
		} else if sameStack(slots[index], stack) && slots[index].Count < limit {
			return index
		}
	}
	return empty
}

// 返回是否搬走一件；读写发生在 blockMu 内，窗口移槽和存档无法看见半次搬运。
func (srv *Server) hopperMoveOne(name string, level *world.World, source, target []world.Tile, pulling bool, face byte) bool {
	var from = windowItems(source)
	var into = windowItems(target)
	var start, end = 0, len(from)
	if source[0].ID == world.TileFurnace {
		start = smelting.SlotOutput
		var fuel = from[smelting.SlotFuel]
		if from[start].IsAir() && fuel.ID == 325 && fuel.Meta == 0 {
			start, end = smelting.SlotFuel, smelting.SlotFuel+1
		}
	}
	var index int = start
	for index = start; index < end; index++ {
		var stack = from[index]
		if stack.IsAir() {
			continue
		}
		var dest = -1
		if target[0].ID == world.TileFurnace {
			var slot = smelting.SlotInput
			if face != 0 {
				if smelting.FuelTicks(stack) == 0 {
					continue
				}
				slot = smelting.SlotFuel
			}
			if hopperSpace(into[slot:slot+1], stack) >= 0 {
				dest = slot
			}
		} else {
			dest = hopperSpace(into, stack)
		}
		if dest < 0 {
			if pulling {
				return false
			}
			continue
		}
		var src, srcSlot, _ = windowSlotTarget(source, index)
		var dst, dstSlot, _ = windowSlotTarget(target, dest)
		src.Items[srcSlot].Count--
		if src.Items[srcSlot].Count == 0 {
			src.Items[srcSlot] = protocol.Air()
		}
		stack.Count = dst.Items[dstSlot].Count + 1
		dst.Items[dstSlot] = stack
		if src.ID == world.TileHopper {
			src.TransferCooldown = 8
		}
		if dst.ID == world.TileHopper {
			dst.TransferCooldown = 8
		}
		level.SetTile(src)
		level.SetTile(dst)
		srv.syncHopperSlot(name, level, src, srcSlot)
		srv.syncHopperSlot(name, level, dst, dstSlot)
		return true
	}
	return false
}

func (srv *Server) syncHopperSlot(name string, level *world.World, tile world.Tile, slot int) {
	var halves = chestHalves(level, tile)
	var anchor = halves[0]
	var windowSlot = slot
	if anchor.X != tile.X || anchor.Z != tile.Z {
		windowSlot += len(anchor.Items)
	}
	var viewers = srv.containerViewers(name, anchor.X, anchor.Y, anchor.Z)
	var index int = 0
	for index = 0; index < len(viewers); index++ {
		srv.resendContainerSlot(viewers[index], windowSlot, tile.Items[slot])
	}
}

// 按掉落物中心所在区块分组一次，与参考端的 getChunkEntities 范围一致。
func (srv *Server) hopperDropsByChunk(name string) map[[2]int32][]int64 {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	var nearby = make(map[[2]int32][]int64)
	var ids = slices.Sorted(maps.Keys(srv.entities))
	var index int = 0
	for index = 0; index < len(ids); index++ {
		var drop = srv.entities[ids[index]]
		if !drop.IsDroppedItem() || !sameWorldName(drop.WorldName, name, srv.world.Name) {
			continue
		}
		var chunk = [2]int32{world.BlockToChunk(drop.X), world.BlockToChunk(drop.Z)}
		nearby[chunk] = append(nearby[chunk], ids[index])
	}
	return nearby
}

func (srv *Server) collectHopperDrops(name string, level *world.World, pos blockrule.Pos, ids []int64) {
	if len(ids) == 0 {
		return
	}
	var tile, _ = level.TileAt(pos.PosX, pos.PosY, pos.PosZ)
	var before = slices.Clone(tile.Items)
	var removed []int64 = nil
	var left, bottom, front = float32(pos.PosX), float32(pos.PosY), float32(pos.PosZ)
	srv.mu.Lock()
	var index int = 0
	for index = 0; index < len(ids); index++ {
		var drop = srv.entities[ids[index]]
		if drop == nil || drop.X+entity.ItemWidth/2 <= left || drop.X-entity.ItemWidth/2 >= left+1 ||
			drop.Z+entity.ItemWidth/2 <= front || drop.Z-entity.ItemWidth/2 >= front+1 ||
			drop.Y+entity.ItemHeight <= bottom || drop.Y >= bottom+2 {
			continue
		}
		var slots = slices.Clone(tile.Items)
		var stack = drop.Item
		var limit = item.StackLimit(stack)
		for stack.Count > 0 {
			var dest = hopperSpace(slots, stack)
			if dest < 0 {
				break
			}
			var count = min(stack.Count, limit-slots[dest].Count)
			var piece = stack
			piece.Count = slots[dest].Count + count
			slots[dest] = piece
			stack.Count -= count
		}
		if stack.Count != 0 {
			continue
		}
		tile.Items = slots
		delete(srv.entities, ids[index])
		removed = append(removed, ids[index])
	}
	if len(removed) > 0 {
		level.SetTile(tile)
	}
	srv.mu.Unlock()
	for index = 0; index < len(removed); index++ {
		srv.broadcastInWorld(name, &packets.RemoveEntityPacket{EntityUniqueID: removed[index]}, "")
	}
	var slot int = 0
	for slot = 0; slot < len(before); slot++ {
		if !smelting.SameItem(before[slot], tile.Items[slot]) {
			srv.syncHopperSlot(name, level, tile, slot)
		}
	}
}
