package packets

import "go-mc-server/internal/protocol"

// TextPacket (0x09) carries chat messages and system notifications.
type TextPacket struct {
	Type       byte
	Source     string // used for TYPE_CHAT, TYPE_WHISPER, TYPE_POPUP
	Message    string
	Parameters []string // used for TYPE_TRANSLATION
}

func (p *TextPacket) ID() byte { return protocol.IDText }

func (p *TextPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteByte(p.Type)
	switch p.Type {
	case protocol.TextTypeChat, protocol.TextTypeWhisper, protocol.TextTypePopup:
		s.WriteString(p.Source)
		s.WriteString(p.Message)
	case protocol.TextTypeRaw, protocol.TextTypeTip, protocol.TextTypeSystem:
		s.WriteString(p.Message)
	case protocol.TextTypeTranslation:
		s.WriteString(p.Message)
		s.WriteUnsignedVarInt(uint32(len(p.Parameters)))
		for _, param := range p.Parameters {
			s.WriteString(param)
		}
	}
	return nil
}

func (p *TextPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Type, err = s.ReadByte(); err != nil {
		return err
	}
	switch p.Type {
	case protocol.TextTypeChat, protocol.TextTypeWhisper, protocol.TextTypePopup:
		if p.Source, err = s.ReadString(); err != nil {
			return err
		}
		p.Message, err = s.ReadString()
	case protocol.TextTypeRaw, protocol.TextTypeTip, protocol.TextTypeSystem:
		p.Message, err = s.ReadString()
	case protocol.TextTypeTranslation:
		if p.Message, err = s.ReadString(); err != nil {
			return err
		}
		count, e := s.ReadUnsignedVarInt()
		if e != nil {
			return e
		}
		p.Parameters = make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			param, e2 := s.ReadString()
			if e2 != nil {
				return e2
			}
			p.Parameters = append(p.Parameters, param)
		}
	}
	return err
}
