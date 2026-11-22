package packets

import "go-mc-server/internal/protocol"

const (
	MobEffectAdd    byte = 1
	MobEffectModify byte = 2
	MobEffectRemove byte = 3
)

type MobEffectPacket struct {
	EntityRuntimeID int64
	Event           byte
	EffectID        int32
	Amplifier       int32
	Particles       bool
	Duration        int32
}

func (pkt *MobEffectPacket) ID() byte { return protocol.IDMobEffect }

func (pkt *MobEffectPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteEntityID(pkt.EntityRuntimeID)
	stream.WriteByte(pkt.Event)
	stream.WriteVarInt(pkt.EffectID)
	stream.WriteVarInt(pkt.Amplifier)
	stream.WriteBool(pkt.Particles)
	stream.WriteVarInt(pkt.Duration)
	return nil
}

func (pkt *MobEffectPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	if pkt.EntityRuntimeID, err = stream.ReadEntityID(); err != nil {
		return err
	}
	if pkt.Event, err = stream.ReadByte(); err != nil {
		return err
	}
	if pkt.EffectID, err = stream.ReadVarInt(); err != nil {
		return err
	}
	if pkt.Amplifier, err = stream.ReadVarInt(); err != nil {
		return err
	}
	if pkt.Particles, err = stream.ReadBool(); err != nil {
		return err
	}
	pkt.Duration, err = stream.ReadVarInt()
	return err
}
