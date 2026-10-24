package packets

import "go-mc-server/internal/protocol"

// ContainerClosePacket (0x31) is sent when the client closes a window.
type ContainerClosePacket struct {
	WindowID byte
}

func (p *ContainerClosePacket) ID() byte { return protocol.IDContainerClose }

func (p *ContainerClosePacket) Encode(s *protocol.BinaryStream) error {
	return s.WriteByte(p.WindowID)
}

func (p *ContainerClosePacket) Decode(s *protocol.BinaryStream) error {
	var err error
	p.WindowID, err = s.ReadByte()
	return err
}
