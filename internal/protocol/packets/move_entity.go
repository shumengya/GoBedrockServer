package packets

import "go-mc-server/internal/protocol"

// MoveEntityPacket (0x12) updates a non-player entity's position.
//
// Source: pocketmine/network/mcpe/protocol/MoveEntityPacket.php
type MoveEntityPacket struct {
	EntityRuntimeID int64
	X, Y, Z         float32
	Pitch           float32
	HeadYaw         float32
	Yaw             float32
	OnGround        bool
	Teleported      bool
}

func (p *MoveEntityPacket) ID() byte { return protocol.IDMoveEntity }

func (p *MoveEntityPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteByteRotation(p.Pitch)
	s.WriteByteRotation(p.HeadYaw)
	s.WriteByteRotation(p.Yaw)
	s.WriteBool(p.OnGround)
	s.WriteBool(p.Teleported)
	return nil
}

func (p *MoveEntityPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.Pitch, err = s.ReadByteRotation(); err != nil {
		return err
	}
	if p.HeadYaw, err = s.ReadByteRotation(); err != nil {
		return err
	}
	if p.Yaw, err = s.ReadByteRotation(); err != nil {
		return err
	}
	if p.OnGround, err = s.ReadBool(); err != nil {
		return err
	}
	p.Teleported, err = s.ReadBool()
	return err
}
