package packets

import "go-mc-server/internal/protocol"

// AdventureSettingsPacket (0x37) configures player capability flags.
type AdventureSettingsPacket struct {
	// Flags bitmask (see bit constants below).
	Flags          uint32
	UserPermission uint32
	// Convenience fields – populated on decode, used to build Flags on encode.
	WorldImmutable bool
	NoPvP          bool
	NoPvM          bool
	NoMvP          bool
	AutoJump       bool
	AllowFlight    bool
	NoClip         bool
	WorldBuilder   bool
	IsFlying       bool
	Muted          bool
}

const (
	flagWorldImmutable = 1 << 0
	flagNoPvP          = 1 << 1
	flagNoPvM          = 1 << 2
	flagNoMvP          = 1 << 3
	flagAutoJump       = 1 << 5
	flagAllowFlight    = 1 << 6
	flagNoClip         = 1 << 7
	flagWorldBuilder   = 1 << 8
	flagIsFlying       = 1 << 9
	flagMuted          = 1 << 10
)

func (p *AdventureSettingsPacket) ID() byte { return protocol.IDAdventureSettings }

func (p *AdventureSettingsPacket) buildFlags() uint32 {
	var f uint32
	if p.WorldImmutable {
		f |= flagWorldImmutable
	}
	if p.NoPvP {
		f |= flagNoPvP
	}
	if p.NoPvM {
		f |= flagNoPvM
	}
	if p.NoMvP {
		f |= flagNoMvP
	}
	if p.AutoJump {
		f |= flagAutoJump
	}
	if p.AllowFlight {
		f |= flagAllowFlight
	}
	if p.NoClip {
		f |= flagNoClip
	}
	if p.WorldBuilder {
		f |= flagWorldBuilder
	}
	if p.IsFlying {
		f |= flagIsFlying
	}
	if p.Muted {
		f |= flagMuted
	}
	return f | p.Flags
}

func (p *AdventureSettingsPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteUnsignedVarInt(p.buildFlags())
	s.WriteUnsignedVarInt(p.UserPermission)
	return nil
}

func (p *AdventureSettingsPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Flags, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.UserPermission, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	p.WorldImmutable = p.Flags&flagWorldImmutable != 0
	p.NoPvP = p.Flags&flagNoPvP != 0
	p.NoPvM = p.Flags&flagNoPvM != 0
	p.NoMvP = p.Flags&flagNoMvP != 0
	p.AutoJump = p.Flags&flagAutoJump != 0
	p.AllowFlight = p.Flags&flagAllowFlight != 0
	p.NoClip = p.Flags&flagNoClip != 0
	p.WorldBuilder = p.Flags&flagWorldBuilder != 0
	p.IsFlying = p.Flags&flagIsFlying != 0
	p.Muted = p.Flags&flagMuted != 0
	return nil
}
