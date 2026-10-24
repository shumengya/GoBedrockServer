package packets

import (
	"go-mc-server/internal/protocol"
)

// PlayStatusPacket (0x02) tells the client the result of the login attempt
// or signals that the player has spawned.
type PlayStatusPacket struct {
	Status int32
}

func (p *PlayStatusPacket) ID() byte { return protocol.IDPlayStatus }

func (p *PlayStatusPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteInt32BE(p.Status)
	return nil
}

func (p *PlayStatusPacket) Decode(s *protocol.BinaryStream) error {
	v, err := s.ReadInt32BE()
	p.Status = v
	return err
}
