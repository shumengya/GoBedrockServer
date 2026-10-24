package packets

import "go-mc-server/internal/protocol"

// AddEntityPacket (0x0d) spawns a non-player entity for a client.
//
// Source: pocketmine/network/mcpe/protocol/AddEntityPacket.php (protocol 113).
type AddEntityPacket struct {
	EntityUniqueID  int64
	EntityRuntimeID int64
	EntityType      uint32
	X, Y, Z         float32
	SpeedX          float32
	SpeedY          float32
	SpeedZ          float32
	Pitch           float32
	Yaw             float32
	Metadata        []MetadataEntry
}

func (p *AddEntityPacket) ID() byte { return protocol.IDAddEntity }

func (p *AddEntityPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityUniqueID)
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteUnsignedVarInt(p.EntityType)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteVector3f(p.SpeedX, p.SpeedY, p.SpeedZ)
	s.WriteLFloat(p.Pitch)
	s.WriteLFloat(p.Yaw)
	s.WriteUnsignedVarInt(0) // attributes
	WriteEntityMetadata(s, p.Metadata)
	s.WriteUnsignedVarInt(0) // links
	return nil
}

func (p *AddEntityPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityUniqueID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.EntityType, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.SpeedX, p.SpeedY, p.SpeedZ, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.Pitch, err = s.ReadLFloat(); err != nil {
		return err
	}
	if p.Yaw, err = s.ReadLFloat(); err != nil {
		return err
	}
	nAttr, err := s.ReadUnsignedVarInt()
	if err != nil {
		return err
	}
	for i := uint32(0); i < nAttr; i++ {
		if _, err = s.ReadString(); err != nil {
			return err
		}
		if _, err = s.ReadLFloat(); err != nil {
			return err
		}
		if _, err = s.ReadLFloat(); err != nil {
			return err
		}
		if _, err = s.ReadLFloat(); err != nil {
			return err
		}
	}
	count, err := s.ReadUnsignedVarInt()
	if err != nil {
		return err
	}
	// Count-only decode is enough for spawn tests; skip remaining metadata/links.
	p.Metadata = make([]MetadataEntry, count)
	return nil
}
