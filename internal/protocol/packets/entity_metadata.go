package packets

import "go-mc-server/internal/protocol"

// Metadata keys / types from Entity.php (MCPE 1.1).
const (
	EntityDataFlags      = 0
	EntityDataVariant    = 2
	EntityDataNametag    = 4
	EntityDataAir        = 7
	EntityDataLeadHolder = 38
	EntityDataScale      = 39
	EntityDataMaxAir     = 43

	EntityDataTypeByte   = 0
	EntityDataTypeShort  = 1
	EntityDataTypeInt    = 2
	EntityDataTypeFloat  = 3
	EntityDataTypeString = 4
	EntityDataTypeSlot   = 5
	EntityDataTypePos    = 6
	EntityDataTypeLong   = 7
	EntityDataTypeVec3   = 8

	DefaultAirTicks int16 = 400
)

// MetadataEntry is one entity metadata property as encoded by putEntityMetadata.
type MetadataEntry struct {
	Key    uint32
	Type   uint32
	Byte   byte
	Short  int16
	Int    int32
	Float  float32
	String string
	Long   int64
}

// DefaultEntityMetadata matches Entity.php $dataProperties at spawn.
func DefaultEntityMetadata(name string) []MetadataEntry {
	return []MetadataEntry{
		{Key: EntityDataFlags, Type: EntityDataTypeLong, Long: 0},
		{Key: EntityDataAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
		{Key: EntityDataMaxAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
		{Key: EntityDataNametag, Type: EntityDataTypeString, String: name},
		{Key: EntityDataLeadHolder, Type: EntityDataTypeLong, Long: -1},
		{Key: EntityDataScale, Type: EntityDataTypeFloat, Float: 1},
	}
}

// WriteEntityMetadata writes UnsignedVarInt count + entries (PocketMine putEntityMetadata).
func WriteEntityMetadata(s *protocol.BinaryStream, entries []MetadataEntry) {
	s.WriteUnsignedVarInt(uint32(len(entries)))
	for _, e := range entries {
		s.WriteUnsignedVarInt(e.Key)
		s.WriteUnsignedVarInt(e.Type)
		switch e.Type {
		case EntityDataTypeByte:
			_ = s.WriteByte(e.Byte)
		case EntityDataTypeShort:
			s.WriteInt16LE(e.Short)
		case EntityDataTypeInt:
			s.WriteVarInt(e.Int)
		case EntityDataTypeFloat:
			s.WriteLFloat(e.Float)
		case EntityDataTypeString:
			s.WriteString(e.String)
		case EntityDataTypeLong:
			s.WriteVarLong(e.Long)
		case EntityDataTypeVec3:
			s.WriteVector3f(e.Float, 0, 0)
		default:
			// Slot/Pos unused in this round.
		}
	}
}
