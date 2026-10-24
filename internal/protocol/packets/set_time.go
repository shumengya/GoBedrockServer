package packets

import "go-mc-server/internal/protocol"

// SetTimePacket (0x0a) sets the in-game time.
type SetTimePacket struct {
	Time int32
}

func (p *SetTimePacket) ID() byte { return protocol.IDSetTime }

func (p *SetTimePacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.Time)
	return nil
}

func (p *SetTimePacket) Decode(s *protocol.BinaryStream) error {
	v, err := s.ReadVarInt()
	p.Time = v
	return err
}
