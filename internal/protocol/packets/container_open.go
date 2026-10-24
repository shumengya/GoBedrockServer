package packets

import "go-mc-server/internal/protocol"

// ContainerOpenPacket (0x30) tells the client to open a window.
type ContainerOpenPacket struct {
	WindowID byte
	Type     byte
	X, Y, Z  int32
	EntityID int64
}

func (p *ContainerOpenPacket) ID() byte { return protocol.IDContainerOpen }

func (p *ContainerOpenPacket) Encode(s *protocol.BinaryStream) error {
	if err := s.WriteByte(p.WindowID); err != nil {
		return err
	}
	if err := s.WriteByte(p.Type); err != nil {
		return err
	}
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteEntityID(p.EntityID)
	return nil
}

func (p *ContainerOpenPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.WindowID, err = s.ReadByte(); err != nil {
		return err
	}
	if p.Type, err = s.ReadByte(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadBlockCoords(); err != nil {
		return err
	}
	p.EntityID, err = s.ReadEntityID()
	return err
}
