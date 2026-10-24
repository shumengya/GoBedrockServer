package packets

import "go-mc-server/internal/protocol"

// MovePlayerPacket (0x13) is sent by both client and server to synchronise position.
type MovePlayerPacket struct {
	EntityRuntimeID int64
	X, Y, Z         float32
	Pitch           float32
	Yaw             float32
	BodyYaw         float32
	Mode            byte
	OnGround        bool
	RidingEID       int64
	TeleportCause   int32 // only when Mode == teleport
	TeleportItem    int32 // only when Mode == teleport
}

func (p *MovePlayerPacket) ID() byte { return protocol.IDMovePlayer }

func (p *MovePlayerPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteLFloat(p.Pitch)
	s.WriteLFloat(p.Yaw)
	s.WriteLFloat(p.BodyYaw)
	s.WriteByte(p.Mode)
	s.WriteBool(p.OnGround)
	s.WriteEntityID(p.RidingEID)
	if p.Mode == protocol.MovePlayerModeTeleport {
		s.WriteInt32LE(p.TeleportCause)
		s.WriteInt32LE(p.TeleportItem)
	}
	return nil
}

func (p *MovePlayerPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.Pitch, err = s.ReadLFloat(); err != nil {
		return err
	}
	if p.Yaw, err = s.ReadLFloat(); err != nil {
		return err
	}
	if p.BodyYaw, err = s.ReadLFloat(); err != nil {
		return err
	}
	if p.Mode, err = s.ReadByte(); err != nil {
		return err
	}
	if p.OnGround, err = s.ReadBool(); err != nil {
		return err
	}
	if p.RidingEID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.Mode == protocol.MovePlayerModeTeleport {
		if p.TeleportCause, err = s.ReadInt32LE(); err != nil {
			return err
		}
		p.TeleportItem, err = s.ReadInt32LE()
	}
	return err
}
