package packets

import (
	"fmt"

	"go-mc-server/internal/protocol"
)

// AddItemEntityPacket (0x0f) spawns a dropped item.
//
// Source: pocketmine/network/mcpe/protocol/AddItemEntityPacket.php
type AddItemEntityPacket struct {
	EntityUniqueID  int64
	EntityRuntimeID int64
	Item            protocol.Item
	X, Y, Z         float32
	SpeedX          float32
	SpeedY          float32
	SpeedZ          float32
}

func (p *AddItemEntityPacket) ID() byte { return protocol.IDAddItemEntity }

func (p *AddItemEntityPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityUniqueID)
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteSlot(p.Item)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteVector3f(p.SpeedX, p.SpeedY, p.SpeedZ)
	return nil
}

func (p *AddItemEntityPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityUniqueID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.Item, err = s.ReadSlot(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	p.SpeedX, p.SpeedY, p.SpeedZ, err = s.ReadVector3f()
	return err
}

func (p *AddItemEntityPacket) String() string {
	return fmt.Sprintf("AddItemEntity eid=%d item=%d×%d", p.EntityRuntimeID, p.Item.ID, p.Item.Count)
}
