package server

import "maps"
import "slices"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/crafting"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/smelting"
import "go-mc-server/internal/world"

// ContainerSetData 里熔炉用的两个属性：烤制进度（箭头）和火焰剩余。
const furnacePropertyCook int32 = 0
const furnacePropertyFire int32 = 1

func furnaceStateOf(tile world.Tile) smelting.State {
	return smelting.State{
		Input:    tile.Items[smelting.SlotInput],
		Fuel:     tile.Items[smelting.SlotFuel],
		Output:   tile.Items[smelting.SlotOutput],
		BurnTime: tile.BurnTime,
		CookTime: tile.CookTime,
		MaxTime:  tile.MaxTime,
	}
}

func storeFurnaceState(tile *world.Tile, state smelting.State) {
	tile.Items[smelting.SlotInput] = state.Input
	tile.Items[smelting.SlotFuel] = state.Fuel
	tile.Items[smelting.SlotOutput] = state.Output
	tile.BurnTime = state.BurnTime
	tile.CookTime = state.CookTime
	tile.MaxTime = state.MaxTime
}

func furnaceItems(state smelting.State) [3]protocol.Item {
	return [3]protocol.Item{state.Input, state.Fuel, state.Output}
}

// changedFurnaceSlots 两个状态之间物品有变化的格子编号。
func changedFurnaceSlots(before smelting.State, after smelting.State) []int {
	var beforeItems [3]protocol.Item = furnaceItems(before)
	var afterItems [3]protocol.Item = furnaceItems(after)
	var changed []int = nil
	var slot int = 0
	for slot = 0; slot < len(beforeItems); slot++ {
		if !smelting.SameItem(beforeItems[slot], afterItems[slot]) {
			changed = append(changed, slot)
		}
	}
	return changed
}

// furnaceProgressPackets 把烤制进度和火焰剩余发给打开着熔炉的客户端。
func furnaceProgressPackets(state smelting.State) []packets.Packet {
	return []packets.Packet{
		&packets.ContainerSetDataPacket{
			WindowID: protocol.WindowContainer,
			Property: furnacePropertyCook,
			Value:    state.CookTime,
		},
		&packets.ContainerSetDataPacket{
			WindowID: protocol.WindowContainer,
			Property: furnacePropertyFire,
			Value:    state.FireTicks(),
		},
	}
}

// tickFurnaces 每个 tick 让所有已加载世界里的熔炉各烧一步。
func (srv *Server) tickFurnaces() {
	// 配方书在启动时已经加载校验过（New 里加载失败会直接退出），这里不会失败。
	var book, loadErr = crafting.Default()
	if loadErr != nil {
		return
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var names []string = slices.Sorted(maps.Keys(srv.worlds))
	var nameIndex int = 0
	for nameIndex = 0; nameIndex < len(names); nameIndex++ {
		var blockWorld *world.World = srv.worlds[names[nameIndex]]
		var positions [][3]int32 = blockWorld.TilePositions(world.TileFurnace)
		var index int = 0
		for index = 0; index < len(positions); index++ {
			srv.tickFurnaceAt(names[nameIndex], blockWorld, book, positions[index])
		}
	}
}

// tickFurnaceAt 推进一个熔炉。区块没加载、方块已经不是熔炉的，方块实体原样不动。
func (srv *Server) tickFurnaceAt(
	worldName string,
	blockWorld *world.World,
	recipes smelting.Recipes,
	position [3]int32,
) {
	var blockID, _, loaded = blockWorld.Peek(position[0], position[1], position[2])
	if !loaded || (blockID != world.BlockFurnace && blockID != world.BlockBurningFurnace) {
		return
	}
	var before smelting.State = smelting.State{}
	var after smelting.State = smelting.State{}
	blockWorld.UpdateTile(position[0], position[1], position[2], func(tile *world.Tile) bool {
		before = furnaceStateOf(*tile)
		after = before
		// 什么都不会发生的熔炉不用算。方块亮不亮不靠这里，下面按状态重新对一遍，
		// 所以方块还亮着、方块实体却是空的这种残留，照样会被对成熄的。
		if before.Idle() {
			return false
		}
		after = smelting.Step(before, recipes)
		if after.Equal(before) {
			return false
		}
		storeFurnaceState(tile, after)
		return true
	})
	// 燃烧时间减到 0 的那一个 tick 方块还亮着，下一个 tick 没续上燃料才熄，续上了就不会闪一下。
	srv.syncFurnaceBlock(worldName, blockWorld, position, blockID, before.Lit() || after.Lit())
	srv.syncFurnaceViewers(worldName, position, before, after)
}

// syncFurnaceBlock 让方块和烧不烧对上：烧着用 62，没烧用 61，朝向（meta）保留。
func (srv *Server) syncFurnaceBlock(
	worldName string,
	blockWorld *world.World,
	position [3]int32,
	blockID byte,
	lit bool,
) {
	var wanted byte = world.BlockFurnace
	if lit {
		wanted = world.BlockBurningFurnace
	}
	if blockID == wanted {
		return
	}
	var _, meta, _ = blockWorld.Peek(position[0], position[1], position[2])
	srv.applyChangeLocked(worldName, blockWorld, blockrule.Change{
		Pos:  blockrule.Pos{PosX: position[0], PosY: position[1], PosZ: position[2]},
		ID:   wanted,
		Meta: meta,
	})
}

// syncFurnaceViewers 把变了的格子和烤制进度发给正开着这个熔炉的玩家。
func (srv *Server) syncFurnaceViewers(worldName string, position [3]int32, before smelting.State, after smelting.State) {
	if before.Equal(after) {
		return
	}
	var viewers []*Player = srv.containerViewers(worldName, position[0], position[1], position[2])
	if len(viewers) == 0 {
		return
	}
	var changed []int = changedFurnaceSlots(before, after)
	var items [3]protocol.Item = furnaceItems(after)
	var progress []packets.Packet = furnaceProgressPackets(after)
	var viewerIndex int = 0
	for viewerIndex = 0; viewerIndex < len(viewers); viewerIndex++ {
		var session *network.Session = viewers[viewerIndex].Session
		if session == nil {
			continue
		}
		var index int = 0
		for index = 0; index < len(changed); index++ {
			session.SendPacket(&packets.ContainerSetSlotPacket{
				WindowID: protocol.WindowContainer,
				Slot:     int32(changed[index]),
				Item:     items[changed[index]],
			})
		}
		for index = 0; index < len(progress); index++ {
			session.SendPacket(progress[index])
		}
	}
}

// resendContainerSlot 客户端的改动被拒绝了，把服务端真实的这一格发回去，它本地多画出来的东西就会消掉。
func (srv *Server) resendContainerSlot(player *Player, slot int, item protocol.Item) {
	if player.Session == nil {
		return
	}
	player.Session.SendPacket(&packets.ContainerSetSlotPacket{
		WindowID: protocol.WindowContainer,
		Slot:     int32(slot),
		Item:     item,
	})
}
