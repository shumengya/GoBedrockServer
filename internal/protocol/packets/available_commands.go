package packets

import "go-mc-server/internal/protocol"

// AvailableCommandsPacket (0x4e) tells the client which slash commands exist.
// Protocol 113 encodes two strings: JSON command tree + unknown (usually empty).
type AvailableCommandsPacket struct {
	CommandsJSON string
	Unknown      string
}

func (p *AvailableCommandsPacket) ID() byte { return protocol.IDAvailableCommands }

func (p *AvailableCommandsPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteString(p.CommandsJSON)
	s.WriteString(p.Unknown)
	return nil
}

func (p *AvailableCommandsPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.CommandsJSON, err = s.ReadString(); err != nil {
		return err
	}
	p.Unknown, err = s.ReadString()
	return err
}
