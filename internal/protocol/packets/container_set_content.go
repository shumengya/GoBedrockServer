package packets

import "go-mc-server/internal/protocol"

// ContainerSetContentPacket (0x34) fills a window (inventory/creative/hotbar).
type ContainerSetContentPacket struct {
	WindowID  uint32
	TargetEID int64
	Slots     []protocol.Item
	Hotbar    []int32
}

func (p *ContainerSetContentPacket) ID() byte { return protocol.IDContainerSetContent }

func (p *ContainerSetContentPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteUnsignedVarInt(p.WindowID)
	s.WriteEntityID(p.TargetEID)
	s.WriteUnsignedVarInt(uint32(len(p.Slots)))
	for _, it := range p.Slots {
		s.WriteSlot(it)
	}
	if p.WindowID == protocol.ContainerInventory && len(p.Hotbar) > 0 {
		s.WriteUnsignedVarInt(uint32(len(p.Hotbar)))
		for _, h := range p.Hotbar {
			s.WriteVarInt(h)
		}
	} else {
		s.WriteUnsignedVarInt(0)
	}
	return nil
}

func (p *ContainerSetContentPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.WindowID, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.TargetEID, err = s.ReadEntityID(); err != nil {
		return err
	}
	count, err := s.ReadUnsignedVarInt()
	if err != nil {
		return err
	}
	p.Slots = make([]protocol.Item, count)
	for i := uint32(0); i < count; i++ {
		if p.Slots[i], err = s.ReadSlot(); err != nil {
			return err
		}
	}
	if p.WindowID == protocol.ContainerInventory {
		n, err := s.ReadUnsignedVarInt()
		if err != nil {
			return err
		}
		p.Hotbar = make([]int32, n)
		for i := uint32(0); i < n; i++ {
			if p.Hotbar[i], err = s.ReadVarInt(); err != nil {
				return err
			}
		}
	}
	return nil
}
