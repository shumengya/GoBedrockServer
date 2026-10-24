package packets

import "go-mc-server/internal/protocol"

// LevelSoundEventPacket (0x19) is sent by the client for footstep / hit / place sounds.
// We decode it so it is not logged as "未知包"; gameplay does not need it yet.
type LevelSoundEventPacket struct {
	SoundID     byte
	X, Y, Z     float32
	ExtraData   int32
	Pitch       int32
	UnknownBool bool
	UnknownBool2 bool
}

// Common sound IDs from LevelSoundEventPacket.php (protocol 113).
const (
	SoundItemUseOn byte = 0
	SoundHit       byte = 1
	SoundStep      byte = 2
	SoundJump      byte = 3
	SoundBreak     byte = 4
	SoundPlace     byte = 5
)

func SoundName(id byte) string {
	switch id {
	case SoundItemUseOn:
		return "ITEM_USE_ON"
	case SoundHit:
		return "HIT"
	case SoundStep:
		return "STEP"
	case SoundJump:
		return "JUMP"
	case SoundBreak:
		return "BREAK"
	case SoundPlace:
		return "PLACE"
	default:
		return "OTHER"
	}
}

func (p *LevelSoundEventPacket) ID() byte { return protocol.IDLevelSoundEvent }

func (p *LevelSoundEventPacket) Encode(s *protocol.BinaryStream) error {
	if err := s.WriteByte(p.SoundID); err != nil {
		return err
	}
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteVarInt(p.ExtraData)
	s.WriteVarInt(p.Pitch)
	s.WriteBool(p.UnknownBool)
	s.WriteBool(p.UnknownBool2)
	return nil
}

func (p *LevelSoundEventPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.SoundID, err = s.ReadByte(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadVector3f(); err != nil {
		return err
	}
	if p.ExtraData, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.Pitch, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.UnknownBool, err = s.ReadBool(); err != nil {
		return err
	}
	p.UnknownBool2, err = s.ReadBool()
	return err
}
