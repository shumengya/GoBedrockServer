package packets

import "go-mc-server/internal/protocol"

// DisconnectPacket (0x05) is sent by the server to kick a player.
type DisconnectPacket struct {
	HideScreen bool
	Message    string
}

func (p *DisconnectPacket) ID() byte { return protocol.IDDisconnect }

func (p *DisconnectPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBool(p.HideScreen)
	if !p.HideScreen {
		s.WriteString(p.Message)
	}
	return nil
}

func (p *DisconnectPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.HideScreen, err = s.ReadBool(); err != nil {
		return err
	}
	if !p.HideScreen {
		p.Message, err = s.ReadString()
	}
	return err
}
