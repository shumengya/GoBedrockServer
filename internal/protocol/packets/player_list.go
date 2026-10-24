package packets

import "go-mc-server/internal/protocol"

// PlayerListEntry is one record in a PlayerListPacket.
//
// Source: PocketMine PlayerListPacket.php (ADD entry order):
//   putUUID, putEntityId, putString(name), putString(skinId), putString(skin)
type PlayerListEntry struct {
	UUID     [16]byte
	EntityID int64
	Name     string
	SkinID   string
	SkinData []byte
}

// PlayerListPacket (0x3f) adds or removes players from the tab list.
type PlayerListPacket struct {
	Action  byte // PlayerListAdd (0) or PlayerListRemove (1)
	Entries []PlayerListEntry
}

func (p *PlayerListPacket) ID() byte { return protocol.IDPlayerList }

func (p *PlayerListPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteByte(p.Action)
	s.WriteUnsignedVarInt(uint32(len(p.Entries)))
	for _, e := range p.Entries {
		s.WriteUUID(e.UUID)
		if p.Action == protocol.PlayerListAdd {
			s.WriteEntityID(e.EntityID)
			s.WriteString(e.Name)
			s.WriteString(e.SkinID)
			// Skin data is sent as a length-prefixed string (UnsignedVarInt + bytes).
			s.WriteString(string(e.SkinData))
		}
	}
	return nil
}

func (p *PlayerListPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}
