package packets

import "go-mc-server/internal/protocol"

// EntityFallPacket (0x25) is sent by the client after a fall. Ignored for now.
type EntityFallPacket struct {
	EntityRuntimeID int64
	FallDistance    float32
	Flag            bool
}

func (p *EntityFallPacket) ID() byte { return protocol.IDEntityFall }

func (p *EntityFallPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteLFloat(p.FallDistance)
	s.WriteBool(p.Flag)
	return nil
}

func (p *EntityFallPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.FallDistance, err = s.ReadLFloat(); err != nil {
		return err
	}
	p.Flag, err = s.ReadBool()
	return err
}
