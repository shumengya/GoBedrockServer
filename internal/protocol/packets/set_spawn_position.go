package packets

import "go-mc-server/internal/protocol"

// SetSpawnPositionPacket (0x2b) sets the world or player spawn point.
//
// SpawnType: 0 = player spawn, 1 = world spawn.
// SpawnForced: true when the spawn was set by a bed or /setworldspawn.
type SetSpawnPositionPacket struct {
	SpawnType   int32
	X, Y, Z     int32
	SpawnForced bool
}

func (p *SetSpawnPositionPacket) ID() byte { return protocol.IDSetSpawnPosition }

func (p *SetSpawnPositionPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.SpawnType)
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteBool(p.SpawnForced)
	return nil
}

func (p *SetSpawnPositionPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}
