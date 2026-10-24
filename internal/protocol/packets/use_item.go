package packets

import "go-mc-server/internal/protocol"

// UseItemPacket (0x23) is sent by the client when using/placing an item.
type UseItemPacket struct {
	X, Y, Z         int32
	BlockID         uint32
	Face            int32
	FX, FY, FZ      float32
	PosX, PosY, PosZ float32
	Slot            int32
	Item            protocol.Item
}

func (p *UseItemPacket) ID() byte { return protocol.IDUseItem }

func (p *UseItemPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteUnsignedVarInt(p.BlockID)
	s.WriteVarInt(p.Face)
	s.WriteVector3f(p.FX, p.FY, p.FZ)
	s.WriteVector3f(p.PosX, p.PosY, p.PosZ)
	s.WriteVarInt(p.Slot)
	s.WriteSlot(p.Item)
	return nil
}

func (p *UseItemPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.X, p.Y, p.Z, err = s.ReadBlockCoords(); err != nil {
		return err
	}
	if p.BlockID, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.Face, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.FX, p.FY, p.FZ, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.PosX, p.PosY, p.PosZ, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.Slot, err = s.ReadVarInt(); err != nil {
		return err
	}
	p.Item, err = s.ReadSlot()
	return err
}
