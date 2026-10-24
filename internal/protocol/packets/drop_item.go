package packets

import (
	"fmt"

	"go-mc-server/internal/protocol"
)

// DropItemPacket (0x2e) is sent by the client when dropping an item (Q / inventory).
//
// Source: pocketmine/network/mcpe/protocol/DropItemPacket.php
type DropItemPacket struct {
	Type byte
	Item protocol.Item
}

func (p *DropItemPacket) ID() byte { return protocol.IDDropItem }

func (p *DropItemPacket) Encode(s *protocol.BinaryStream) error {
	return fmt.Errorf("DropItemPacket.Encode: server never sends this")
}

func (p *DropItemPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Type, err = s.ReadByte(); err != nil {
		return err
	}
	p.Item, err = s.ReadSlot()
	return err
}
