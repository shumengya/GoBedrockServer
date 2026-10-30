package server

import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// sentLog 记录服务端发给某个玩家的所有包（带包 ID 的原始字节）。
type sentLog struct {
	raws [][]byte
}

// captureSent 从现在起记下服务端发给这个玩家的包。
func captureSent(player *Player) *sentLog {
	var log *sentLog = &sentLog{}
	player.Session.SentHook = func(raw []byte) {
		log.raws = append(log.raws, append([]byte(nil), raw...))
	}
	return log
}

// count 某种包发过几个。
func (log *sentLog) count(packetID byte) int {
	var total int = 0
	var index int = 0
	for index = 0; index < len(log.raws); index++ {
		if len(log.raws[index]) > 0 && log.raws[index][0] == packetID {
			total++
		}
	}
	return total
}

// blockEntities 发过的所有 BlockEntityData，按发送顺序。
func (log *sentLog) blockEntities(test *testing.T) []packets.BlockEntityDataPacket {
	var found []packets.BlockEntityDataPacket = nil
	var index int = 0
	for index = 0; index < len(log.raws); index++ {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDBlockEntityData {
			continue
		}
		var pkt packets.BlockEntityDataPacket = packets.BlockEntityDataPacket{}
		var err error = pkt.Decode(protocol.NewStream(log.raws[index][1:]))
		if err != nil {
			test.Fatalf("解码 BlockEntityData 失败: %v", err)
		}
		found = append(found, pkt)
	}
	return found
}

// lastBlockEntityAt 在 (posX,posY,posZ) 最后一次发的 BlockEntityData 的 NBT，没发过返回 false。
func (log *sentLog) lastBlockEntityAt(test *testing.T, posX int32, posY int32, posZ int32) ([]byte, bool) {
	var all []packets.BlockEntityDataPacket = log.blockEntities(test)
	var index int = 0
	for index = len(all) - 1; index >= 0; index-- {
		if all[index].X == posX && all[index].Y == posY && all[index].Z == posZ {
			return all[index].NBT, true
		}
	}
	return nil, false
}

// lastContainerContent 最后一次发给这个玩家的、指定窗口号的整排格子，没发过返回 false。
func (log *sentLog) lastContainerContent(test *testing.T, windowID uint32) ([]protocol.Item, bool) {
	var index int = 0
	for index = len(log.raws) - 1; index >= 0; index-- {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDContainerSetContent {
			continue
		}
		var pkt packets.ContainerSetContentPacket = packets.ContainerSetContentPacket{}
		var err error = pkt.Decode(protocol.NewStream(log.raws[index][1:]))
		if err != nil {
			test.Fatalf("解码 ContainerSetContent 失败: %v", err)
		}
		if pkt.WindowID == windowID {
			return pkt.Slots, true
		}
	}
	return nil, false
}

// lastArmorPacket 最后一次发给这个玩家的、关于某个实体的 MobArmorEquipment，没发过返回 false。
func (log *sentLog) lastArmorPacket(test *testing.T, entityID int64) (packets.MobArmorEquipmentPacket, bool) {
	var index int = 0
	for index = len(log.raws) - 1; index >= 0; index-- {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDMobArmorEquipment {
			continue
		}
		var pkt packets.MobArmorEquipmentPacket = packets.MobArmorEquipmentPacket{}
		var err error = pkt.Decode(protocol.NewStream(log.raws[index][1:]))
		if err != nil {
			test.Fatalf("解码 MobArmorEquipment 失败: %v", err)
		}
		if pkt.EntityRuntimeID == entityID {
			return pkt, true
		}
	}
	return packets.MobArmorEquipmentPacket{}, false
}

// lastContainerSlot 最后一次发给这个玩家的、指定窗口的 ContainerSetSlot，没发过返回 false。
func (log *sentLog) lastContainerSlot(test *testing.T, windowID byte) (packets.ContainerSetSlotPacket, bool) {
	var index int = 0
	for index = len(log.raws) - 1; index >= 0; index-- {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDContainerSetSlot {
			continue
		}
		var pkt packets.ContainerSetSlotPacket = packets.ContainerSetSlotPacket{}
		var err error = pkt.Decode(protocol.NewStream(log.raws[index][1:]))
		if err != nil {
			test.Fatalf("解码 ContainerSetSlot 失败: %v", err)
		}
		if pkt.WindowID == windowID {
			return pkt, true
		}
	}
	return packets.ContainerSetSlotPacket{}, false
}

// countContent 指定窗口的 ContainerSetContent 发过几次。
func (log *sentLog) countContent(test *testing.T, windowID uint32) int {
	var total int = 0
	var index int = 0
	for index = 0; index < len(log.raws); index++ {
		if len(log.raws[index]) == 0 || log.raws[index][0] != protocol.IDContainerSetContent {
			continue
		}
		var pkt packets.ContainerSetContentPacket = packets.ContainerSetContentPacket{}
		var err error = pkt.Decode(protocol.NewStream(log.raws[index][1:]))
		if err != nil {
			test.Fatalf("解码 ContainerSetContent 失败: %v", err)
		}
		if pkt.WindowID == windowID {
			total++
		}
	}
	return total
}

// firstIndexOf 某种包第一次出现在第几个，没有返回 -1。用来断言先后顺序。
func (log *sentLog) firstIndexOf(packetID byte) int {
	var index int = 0
	for index = 0; index < len(log.raws); index++ {
		if len(log.raws[index]) > 0 && log.raws[index][0] == packetID {
			return index
		}
	}
	return -1
}
