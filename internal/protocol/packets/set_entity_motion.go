package packets

import "go-mc-server/internal/protocol"

// SetEntityMotionPacket (0x28) sets an entity's velocity.
//
// Source: pocketmine/network/mcpe/protocol/SetEntityMotionPacket.php
type SetEntityMotionPacket struct {
	EntityRuntimeID int64
	SpeedX          float32
	SpeedY          float32
	SpeedZ          float32
}

func (p *SetEntityMotionPacket) ID() byte { return protocol.IDSetEntityMotion }

func (p *SetEntityMotionPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteVector3f(p.SpeedX, p.SpeedY, p.SpeedZ)
	return nil
}

func (p *SetEntityMotionPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	p.SpeedX, p.SpeedY, p.SpeedZ, err = s.ReadVector3f()
	return err
}
