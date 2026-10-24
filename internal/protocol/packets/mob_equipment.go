package packets

import "go-mc-server/internal/protocol"

// MobEquipmentPacket (0x1f) is sent when a mob/player changes held item.
type MobEquipmentPacket struct {
	EntityRuntimeID int64
	Item            protocol.Item
	InventorySlot   byte
	HotbarSlot      byte
	WindowID        byte
}

func (p *MobEquipmentPacket) ID() byte { return protocol.IDMobEquipment }

func (p *MobEquipmentPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteSlot(p.Item)
	s.WriteByte(p.InventorySlot)
	s.WriteByte(p.HotbarSlot)
	s.WriteByte(p.WindowID)
	return nil
}

func (p *MobEquipmentPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.Item, err = s.ReadSlot(); err != nil {
		return err
	}
	if p.InventorySlot, err = s.ReadByte(); err != nil {
		return err
	}
	if p.HotbarSlot, err = s.ReadByte(); err != nil {
		return err
	}
	p.WindowID, err = s.ReadByte()
	return err
}
