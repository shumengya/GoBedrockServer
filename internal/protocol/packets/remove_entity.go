package packets

import "go-mc-server/internal/protocol"

// RemoveEntityPacket (0x0e) despawns an entity.
//
// Source: pocketmine/network/mcpe/protocol/RemoveEntityPacket.php
type RemoveEntityPacket struct {
	EntityUniqueID int64
}

func (p *RemoveEntityPacket) ID() byte { return protocol.IDRemoveEntity }

func (p *RemoveEntityPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityUniqueID)
	return nil
}

func (p *RemoveEntityPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	p.EntityUniqueID, err = s.ReadEntityID()
	return err
}
