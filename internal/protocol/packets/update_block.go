package packets

import "go-mc-server/internal/protocol"

// UpdateBlockPacket (0x16) tells the client a block changed.
type UpdateBlockPacket struct {
	X, Y, Z   int32
	BlockID   uint32
	BlockData uint32
	Flags     uint32
}

func (p *UpdateBlockPacket) ID() byte { return protocol.IDUpdateBlock }

func (p *UpdateBlockPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteUnsignedVarInt(p.BlockID)
	s.WriteUnsignedVarInt((p.Flags << 4) | (p.BlockData & 0x0f))
	return nil
}

func (p *UpdateBlockPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.X, p.Y, p.Z, err = s.ReadBlockCoords(); err != nil {
		return err
	}
	if p.BlockID, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	mask, err := s.ReadUnsignedVarInt()
	if err != nil {
		return err
	}
	p.BlockData = mask & 0x0f
	p.Flags = mask >> 4
	return nil
}
