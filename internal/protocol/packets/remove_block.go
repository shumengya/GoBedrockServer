package packets

import "go-mc-server/internal/protocol"

// RemoveBlockPacket (0x15) is sent by the client when breaking a block.
type RemoveBlockPacket struct {
	X, Y, Z int32
}

func (p *RemoveBlockPacket) ID() byte { return protocol.IDRemoveBlock }

func (p *RemoveBlockPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	return nil
}

func (p *RemoveBlockPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	p.X, p.Y, p.Z, err = s.ReadBlockCoords()
	return err
}
