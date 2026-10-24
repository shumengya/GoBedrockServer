package packets

import "go-mc-server/internal/protocol"

// StartGamePacket (0x0b) provides the client with all world/player initialisation data.
type StartGamePacket struct {
	EntityUniqueID          int64
	EntityRuntimeID         int64
	PlayerGamemode          int32
	X, Y, Z                 float32
	Pitch, Yaw              float32
	Seed                    int32
	Dimension               int32
	Generator               int32 // 0=old, 1=infinite, 2=flat
	WorldGamemode           int32
	Difficulty              int32
	SpawnX, SpawnY, SpawnZ  int32
	HasAchievementsDisabled bool
	DayCycleStopTime        int32 // -1 = not stopped
	EduMode                 bool
	RainLevel               float32
	LightningLevel          float32
	CommandsEnabled         bool
	IsTexturePacksRequired  bool
	LevelID                 string
	WorldName               string
	PremiumWorldTemplateID  string
}

func (p *StartGamePacket) ID() byte { return protocol.IDStartGame }

func (p *StartGamePacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityUniqueID)
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteVarInt(p.PlayerGamemode)
	s.WriteVector3f(p.X, p.Y, p.Z)
	s.WriteLFloat(p.Pitch)
	s.WriteLFloat(p.Yaw)
	s.WriteVarInt(p.Seed)
	s.WriteVarInt(p.Dimension)
	s.WriteVarInt(p.Generator)
	s.WriteVarInt(p.WorldGamemode)
	s.WriteVarInt(p.Difficulty)
	s.WriteBlockCoords(p.SpawnX, p.SpawnY, p.SpawnZ)
	s.WriteBool(p.HasAchievementsDisabled)
	s.WriteVarInt(p.DayCycleStopTime)
	s.WriteBool(p.EduMode)
	s.WriteLFloat(p.RainLevel)
	s.WriteLFloat(p.LightningLevel)
	s.WriteBool(p.CommandsEnabled)
	s.WriteBool(p.IsTexturePacksRequired)
	s.WriteUnsignedVarInt(0) // gamerules count
	s.WriteString(p.LevelID)
	s.WriteString(p.WorldName)
	s.WriteString(p.PremiumWorldTemplateID)
	return nil
}

func (p *StartGamePacket) Decode(s *protocol.BinaryStream) error {
	return nil // server never receives this
}
