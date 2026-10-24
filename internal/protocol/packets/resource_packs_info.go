package packets

import "go-mc-server/internal/protocol"

// ResourcePackEntry represents a single resource / behavior pack entry.
type ResourcePackEntry struct {
	PackID      string
	PackVersion string
	PackSize    uint64
}

// ResourcePacksInfoPacket (0x06) tells the client which packs the server has.
// For a vanilla server with no packs, send empty lists.
type ResourcePacksInfoPacket struct {
	MustAccept         bool
	BehaviorPackEntries []ResourcePackEntry
	ResourcePackEntries []ResourcePackEntry
}

func (p *ResourcePacksInfoPacket) ID() byte { return protocol.IDResourcePacksInfo }

func (p *ResourcePacksInfoPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBool(p.MustAccept)
	s.WriteUint16LE(uint16(len(p.BehaviorPackEntries)))
	for _, e := range p.BehaviorPackEntries {
		s.WriteString(e.PackID)
		s.WriteString(e.PackVersion)
		s.WriteUint64LE(e.PackSize)
	}
	s.WriteUint16LE(uint16(len(p.ResourcePackEntries)))
	for _, e := range p.ResourcePackEntries {
		s.WriteString(e.PackID)
		s.WriteString(e.PackVersion)
		s.WriteUint64LE(e.PackSize)
	}
	return nil
}

func (p *ResourcePacksInfoPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}
