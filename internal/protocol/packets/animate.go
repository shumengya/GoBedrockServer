package packets

import "go-mc-server/internal/protocol"

// AnimatePacket (0x2c) triggers swing/animation on an entity.
type AnimatePacket struct {
	Action          int32
	EntityRuntimeID int64
}

func (p *AnimatePacket) ID() byte { return protocol.IDAnimate }

func (p *AnimatePacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.Action)
	s.WriteEntityID(p.EntityRuntimeID)
	return nil
}

func (p *AnimatePacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Action, err = s.ReadVarInt(); err != nil {
		return err
	}
	p.EntityRuntimeID, err = s.ReadEntityID()
	return err
}
