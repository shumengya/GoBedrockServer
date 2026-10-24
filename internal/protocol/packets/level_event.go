package packets

import "go-mc-server/internal/protocol"

// LevelEventPacket (0x1a) plays a world event: particles, weather, block crack.
//
// Source: LevelEventPacket.php encode:
//
//	putVarInt(evid); putVector3f(x,y,z); putVarInt(data)
type LevelEventPacket struct {
	EventID int32
	X, Y, Z float32
	Data    int32
}

// Event IDs from LevelEventPacket.php (protocol 113).
const (
	LevelEventParticleDestroy    int32 = 2001
	LevelEventParticlePunchBlock int32 = 2014
	LevelEventBlockStartBreak    int32 = 3600
	LevelEventBlockStopBreak     int32 = 3601
)

func (p *LevelEventPacket) ID() byte { return protocol.IDLevelEvent }

func (p *LevelEventPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteVarInt(p.EventID)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteVarInt(p.Data)
	return nil
}

func (p *LevelEventPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EventID, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	p.Data, err = s.ReadVarInt()
	return err
}
