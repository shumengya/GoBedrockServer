package packets

import "go-mc-server/internal/protocol"

// ContainerSetSlotPacket (0x32) is sent by the client when a inventory slot changes
// (place, open inventory, drag). HotbarSlot in client dumps is often 0 even when Slot is 8;
// only Slot+Item are authoritative for the inventory array.
type ContainerSetSlotPacket struct {
	WindowID   byte
	Slot       int32
	HotbarSlot int32
	Item       protocol.Item
	SelectSlot byte
}

func (p *ContainerSetSlotPacket) ID() byte { return protocol.IDContainerSetSlot }

func (p *ContainerSetSlotPacket) Encode(s *protocol.BinaryStream) error {
	if err := s.WriteByte(p.WindowID); err != nil {
		return err
	}
	s.WriteVarInt(p.Slot)
	s.WriteVarInt(p.HotbarSlot)
	s.WriteSlot(p.Item)
	return s.WriteByte(p.SelectSlot)
}

func (p *ContainerSetSlotPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.WindowID, err = s.ReadByte(); err != nil {
		return err
	}
	if p.Slot, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.HotbarSlot, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.Item, err = s.ReadSlot(); err != nil {
		return err
	}
	p.SelectSlot, err = s.ReadByte()
	return err
}
