package packets

import "go-mc-server/internal/protocol"

// InteractPacket (0x21) is sent by the client when looking at / clicking an entity.
// ACTION_MOUSEOVER (4) is spammed while the crosshair moves; we decode and ignore it.
type InteractPacket struct {
	Action byte
	Target int64
}

const (
	InteractRightClick   byte = 1
	InteractLeftClick    byte = 2
	InteractLeaveVehicle byte = 3
	InteractMouseover    byte = 4
)

func (p *InteractPacket) ID() byte { return protocol.IDInteract }

func (p *InteractPacket) Encode(s *protocol.BinaryStream) error {
	if err := s.WriteByte(p.Action); err != nil {
		return err
	}
	s.WriteEntityID(p.Target)
	return nil
}

func (p *InteractPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Action, err = s.ReadByte(); err != nil {
		return err
	}
	p.Target, err = s.ReadEntityID()
	return err
}
