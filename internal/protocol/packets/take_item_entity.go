package packets

import "go-mc-server/internal/protocol"

// TakeItemEntityPacket (0x11) plays the pickup animation.
//
// Source: pocketmine/network/mcpe/protocol/TakeItemEntityPacket.php
// encode: target (item eid) then eid (collector).
type TakeItemEntityPacket struct {
	TargetEntityID int64
	CollectorEID   int64
}

func (p *TakeItemEntityPacket) ID() byte { return protocol.IDTakeItemEntity }

func (p *TakeItemEntityPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.TargetEntityID)
	s.WriteEntityID(p.CollectorEID)
	return nil
}

func (p *TakeItemEntityPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.TargetEntityID, err = s.ReadEntityID(); err != nil {
		return err
	}
	p.CollectorEID, err = s.ReadEntityID()
	return err
}
