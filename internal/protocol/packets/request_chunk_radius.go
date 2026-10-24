package packets

import "go-mc-server/internal/protocol"

// RequestChunkRadiusPacket (0x45) is sent by the client to request a view distance.
type RequestChunkRadiusPacket struct {
	ChunkRadius int32
}

func (p *RequestChunkRadiusPacket) ID() byte { return protocol.IDRequestChunkRadius }

func (p *RequestChunkRadiusPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.ChunkRadius)
	return nil
}

func (p *RequestChunkRadiusPacket) Decode(s *protocol.BinaryStream) error {
	v, err := s.ReadVarInt()
	p.ChunkRadius = v
	return err
}
