package packets

import "go-mc-server/internal/protocol"

// FullChunkDataPacket (0x3a) sends a 16×16 column of chunk data to the client.
// The Data field must be pre-serialised sub-chunk data (see world/chunk.go).
type FullChunkDataPacket struct {
	ChunkX int32
	ChunkZ int32
	Data   []byte
}

func (p *FullChunkDataPacket) ID() byte { return protocol.IDFullChunkData }

func (p *FullChunkDataPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.ChunkX)
	s.WriteVarInt(p.ChunkZ)
	s.WriteString(string(p.Data)) // length-prefixed blob
	return nil
}

func (p *FullChunkDataPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}
