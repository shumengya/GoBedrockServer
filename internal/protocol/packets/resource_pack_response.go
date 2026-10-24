package packets

import "go-mc-server/internal/protocol"

// ResourcePackClientResponsePacket (0x08) is sent by the client in response to
// ResourcePacksInfoPacket and ResourcePackStackPacket.
type ResourcePackClientResponsePacket struct {
	Status  byte
	PackIDs []string
}

func (p *ResourcePackClientResponsePacket) ID() byte {
	return protocol.IDResourcePackClientResponse
}

func (p *ResourcePackClientResponsePacket) Encode(s *protocol.BinaryStream) error {
	s.WriteByte(p.Status)
	s.WriteUint16LE(uint16(len(p.PackIDs)))
	for _, id := range p.PackIDs {
		s.WriteString(id)
	}
	return nil
}

func (p *ResourcePackClientResponsePacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Status, err = s.ReadByte(); err != nil {
		return err
	}
	count, err := s.ReadUint16LE()
	if err != nil {
		return err
	}
	p.PackIDs = make([]string, 0, count)
	for i := 0; i < int(count); i++ {
		id, err := s.ReadString()
		if err != nil {
			return err
		}
		p.PackIDs = append(p.PackIDs, id)
	}
	return nil
}
