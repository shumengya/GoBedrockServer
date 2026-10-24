package packets

import "go-mc-server/internal/protocol"

// PlayerActionPacket (0x24) is sent by the client for player actions.
type PlayerActionPacket struct {
	EntityRuntimeID int64
	Action          int32
	X, Y, Z         int32
	Face            int32
}

// Action IDs for MCPE 1.1.x (PlayerActionPacket.php).
const (
	ActionStartBreak         = 0
	ActionAbortBreak         = 1
	ActionStopBreak          = 2
	ActionReleaseItem        = 5
	ActionStopSleeping       = 6
	ActionSpawnSameDimension = 7
	ActionJump               = 8
	ActionStartSprint        = 9
	ActionStopSprint         = 10
	ActionStartSneak         = 11
	ActionStopSneak          = 12
	ActionSpawnOverworld     = 13
	ActionSpawnNether        = 14
	ActionStartGlide         = 15
	ActionStopGlide          = 16
	ActionBuildDenied        = 17
	ActionContinueBreak      = 18
)

func ActionName(action int32) string {
	switch action {
	case ActionStartBreak:
		return "START_BREAK"
	case ActionAbortBreak:
		return "ABORT_BREAK"
	case ActionStopBreak:
		return "STOP_BREAK"
	case ActionReleaseItem:
		return "RELEASE_ITEM"
	case ActionStopSleeping:
		return "STOP_SLEEPING"
	case ActionSpawnSameDimension:
		return "SPAWN_SAME_DIMENSION"
	case ActionJump:
		return "JUMP"
	case ActionStartSprint:
		return "START_SPRINT"
	case ActionStopSprint:
		return "STOP_SPRINT"
	case ActionStartSneak:
		return "START_SNEAK"
	case ActionStopSneak:
		return "STOP_SNEAK"
	case ActionSpawnOverworld:
		return "SPAWN_OVERWORLD"
	case ActionSpawnNether:
		return "SPAWN_NETHER"
	case ActionStartGlide:
		return "START_GLIDE"
	case ActionStopGlide:
		return "STOP_GLIDE"
	case ActionBuildDenied:
		return "BUILD_DENIED"
	case ActionContinueBreak:
		return "CONTINUE_BREAK"
	default:
		return "UNKNOWN"
	}
}

func (p *PlayerActionPacket) ID() byte { return protocol.IDPlayerAction }

func (p *PlayerActionPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteVarInt(p.Action)
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteVarInt(p.Face)
	return nil
}

func (p *PlayerActionPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.EntityRuntimeID, err = s.ReadEntityID(); err != nil {
		return err
	}
	if p.Action, err = s.ReadVarInt(); err != nil {
		return err
	}
	if p.X, p.Y, p.Z, err = s.ReadBlockCoords(); err != nil {
		return err
	}
	p.Face, err = s.ReadVarInt()
	return err
}
