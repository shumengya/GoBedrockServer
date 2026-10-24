package packets

import "go-mc-server/internal/protocol"

// ResourcePackStackPacket (0x07) tells the client the ordered stack of packs to apply.
type ResourcePackStackPacket struct {
	MustAccept          bool
	BehaviorPackStack   []ResourcePackEntry
	ResourcePackStack   []ResourcePackEntry
}

func (p *ResourcePackStackPacket) ID() byte { return protocol.IDResourcePackStack }

func (p *ResourcePackStackPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBool(p.MustAccept)
	s.WriteUnsignedVarInt(uint32(len(p.BehaviorPackStack)))
	for _, e := range p.BehaviorPackStack {
		s.WriteString(e.PackID)
		s.WriteString(e.PackVersion)
	}
	s.WriteUnsignedVarInt(uint32(len(p.ResourcePackStack)))
	for _, e := range p.ResourcePackStack {
		s.WriteString(e.PackID)
		s.WriteString(e.PackVersion)
	}
	return nil
}

func (p *ResourcePackStackPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}
