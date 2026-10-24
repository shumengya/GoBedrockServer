package packets

import "go-mc-server/internal/protocol"

// SetPlayerGameTypePacket (0x3e) sets the client's game mode.
//
// Source: pocketmine/network/mcpe/protocol/SetPlayerGameTypePacket.php
type SetPlayerGameTypePacket struct {
	GameMode int32
}

func (p *SetPlayerGameTypePacket) ID() byte { return protocol.IDSetPlayerGameType }

func (p *SetPlayerGameTypePacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.GameMode)
	return nil
}

func (p *SetPlayerGameTypePacket) Decode(s *protocol.BinaryStream) error {
	v, err := s.ReadVarInt()
	p.GameMode = v
	return err
}
