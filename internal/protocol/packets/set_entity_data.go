package packets

import "go-mc-server/internal/protocol"

// SetEntityDataPacket (0x27) sends entity metadata to the client.
//
// Metadata: UnsignedVarInt count, then for each entry:
//
//	UnsignedVarInt key, UnsignedVarInt type, value (type-dependent).
//
// Types used here match Entity.php: SHORT=1, LONG=7.
type SetEntityDataPacket struct {
	EntityRuntimeID int64
	// Flags is metadata key 0 (entity flags, type long).
	Flags uint64
	// SendFlags controls whether to include spawn metadata (flags + air).
	SendFlags bool
}

// Entity flag bits for metadata key 0 (type long). Entity.php DATA_FLAG_*.
const (
	EntityFlagCanShowName    uint64 = 1 << 14
	EntityFlagAlwaysShowName uint64 = 1 << 15
	EntityFlagImmobile       uint64 = 1 << 16 // DATA_FLAG_IMMOBILE / NO_AI
	EntityFlagCanClimb       uint64 = 1 << 19
	EntityFlagBreathing      uint64 = 1 << 33
)

// DefaultPlayerFlags is the flag set PocketMine sends for a spawned player
// (nametag visible, can climb, breathing / not drowning).
func DefaultPlayerFlags() uint64 {
	return EntityFlagCanShowName | EntityFlagAlwaysShowName |
		EntityFlagCanClimb | EntityFlagBreathing
}

func PlayerFlags(invisible bool) uint64 {
	if invisible {
		return DefaultPlayerFlags()&^(EntityFlagCanShowName|EntityFlagAlwaysShowName) | (1 << 5)
	}
	return DefaultPlayerFlags()
}

func (p *SetEntityDataPacket) ID() byte { return protocol.IDSetEntityData }

func (p *SetEntityDataPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	if !p.SendFlags {
		s.WriteUnsignedVarInt(0)
		return nil
	}
	// flags + air + max air — without AIR/MAX_AIR and DATA_FLAG_BREATHING
	// the 1.1 HUD draws the oxygen bubble bar on dry land.
	WriteEntityMetadata(s, []MetadataEntry{
		{Key: EntityDataFlags, Type: EntityDataTypeLong, Long: int64(p.Flags)},
		{Key: EntityDataAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
		{Key: EntityDataMaxAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
	})
	return nil
}

func (p *SetEntityDataPacket) Decode(s *protocol.BinaryStream) error {
	return nil // client may send a truncated copy; we ignore it
}
