package packets

import "fmt"

import "go-mc-server/internal/protocol"

const EntityEventHurtAnimation byte = 2
const EntityEventDeathAnimation byte = 3

// EntityEventUseItem 是吃完东西（或喝完）时客户端上报的事件，服务端据此扣物品、回饥饿。
const EntityEventUseItem byte = 9
const EntityEventRespawn byte = 17

// EntityEventEatingItem 是吃的过程中客户端周期性发出的，用来让旁观者看到食物碎屑。
const EntityEventEatingItem byte = 57

// SetHealthPacket (0x2a) 把玩家血量同步给自己的客户端。
//
// Source: pocketmine/network/mcpe/protocol/SetHealthPacket.php
type SetHealthPacket struct {
	Health int32
}

func (pkt *SetHealthPacket) ID() byte { return protocol.IDSetHealth }

func (pkt *SetHealthPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteVarInt(pkt.Health)
	return nil
}

func (pkt *SetHealthPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	pkt.Health, err = stream.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读取血量: %w", err)
	}
	return nil
}

// EntityEventPacket (0x1c) 播放受伤、死亡等实体动画。
//
// Source: pocketmine/network/mcpe/protocol/EntityEventPacket.php
type EntityEventPacket struct {
	EntityRuntimeID int64
	Event           byte
	Data            int32
}

func (pkt *EntityEventPacket) ID() byte { return protocol.IDEntityEvent }

func (pkt *EntityEventPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteEntityID(pkt.EntityRuntimeID)
	var err error = stream.WriteByte(pkt.Event)
	if err != nil {
		return fmt.Errorf("写入实体事件: %w", err)
	}
	stream.WriteVarInt(pkt.Data)
	return nil
}

func (pkt *EntityEventPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	pkt.EntityRuntimeID, err = stream.ReadEntityID()
	if err != nil {
		return fmt.Errorf("读取实体 ID: %w", err)
	}
	pkt.Event, err = stream.ReadByte()
	if err != nil {
		return fmt.Errorf("读取实体事件: %w", err)
	}
	pkt.Data, err = stream.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读取事件数据: %w", err)
	}
	return nil
}

// RespawnPacket (0x2d) 告诉死亡的客户端重生点，客户端据此弹出重生界面。
//
// Source: pocketmine/network/mcpe/protocol/RespawnPacket.php
type RespawnPacket struct {
	PositionX float32
	PositionY float32
	PositionZ float32
}

func (pkt *RespawnPacket) ID() byte { return protocol.IDRespawn }

func (pkt *RespawnPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteLFloat(pkt.PositionX)
	stream.WriteLFloat(pkt.PositionY)
	stream.WriteLFloat(pkt.PositionZ)
	return nil
}

func (pkt *RespawnPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	pkt.PositionX, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取重生点 X: %w", err)
	}
	pkt.PositionY, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取重生点 Y: %w", err)
	}
	pkt.PositionZ, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取重生点 Z: %w", err)
	}
	return nil
}
