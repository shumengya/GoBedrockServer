package packets

import "go-mc-server/internal/protocol"

// ChangeDimensionPacket (0x3d) tells the client to switch vanilla dimension.
//
// Source: pocketmine/network/mcpe/protocol/ChangeDimensionPacket.php (protocol 113).
type ChangeDimensionPacket struct {
	Dimension int32
	X, Y, Z   float32
	Respawn   bool
}

func (p *ChangeDimensionPacket) ID() byte { return protocol.IDChangeDimension }

func (p *ChangeDimensionPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.Dimension)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteBool(p.Respawn)
	return nil
}

func (p *ChangeDimensionPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Dimension, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	p.Respawn, err = s.ReadBool()
	return err
}
