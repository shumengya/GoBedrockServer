package packets

import "go-mc-server/internal/protocol"

// ChunkRadiusUpdatedPacket (0x46) confirms the server-side chunk radius.
type ChunkRadiusUpdatedPacket struct {
	ChunkRadius int32
}

func (p *ChunkRadiusUpdatedPacket) ID() byte { return protocol.IDChunkRadiusUpdated }

func (p *ChunkRadiusUpdatedPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.ChunkRadius)
	return nil
}

func (p *ChunkRadiusUpdatedPacket) Decode(s *protocol.BinaryStream) error {
	v, err := s.ReadVarInt()
	p.ChunkRadius = v
	return err
}
