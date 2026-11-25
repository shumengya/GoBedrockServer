package packets

import "fmt"

import "go-mc-server/internal/protocol"

// AddPlayerPacket (0x0c) spawns another player's model for a client.
//
// Source: pocketmine/network/mcpe/protocol/AddPlayerPacket.php (protocol 113).
// 客户端按 UUID 去 PlayerList 里找皮肤，所以必须先发 PlayerList ADD 再发这个包。
type AddPlayerPacket struct {
	UUID            [16]byte
	Username        string
	EntityUniqueID  int64
	EntityRuntimeID int64
	PosX            float32 // 脚底坐标
	PosY            float32
	PosZ            float32
	SpeedX          float32
	SpeedY          float32
	SpeedZ          float32
	Pitch           float32
	HeadYaw         float32
	Yaw             float32
	Item            protocol.Item
	Metadata        []MetadataEntry
}

func (packet *AddPlayerPacket) ID() byte { return protocol.IDAddPlayer }

func (packet *AddPlayerPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteUUID(packet.UUID)
	stream.WriteString(packet.Username)
	stream.WriteEntityID(packet.EntityUniqueID)
	stream.WriteEntityID(packet.EntityRuntimeID)
	stream.WriteVector3f(packet.PosX, packet.PosY, packet.PosZ)
	stream.WriteVector3f(packet.SpeedX, packet.SpeedY, packet.SpeedZ)
	stream.WriteLFloat(packet.Pitch)
	stream.WriteLFloat(packet.HeadYaw)
	stream.WriteLFloat(packet.Yaw)
	stream.WriteSlot(packet.Item)
	WriteEntityMetadata(stream, packet.Metadata)
	return nil
}

func (packet *AddPlayerPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	packet.UUID, err = stream.ReadUUID()
	if err != nil {
		return fmt.Errorf("读取 UUID: %w", err)
	}
	packet.Username, err = stream.ReadString()
	if err != nil {
		return fmt.Errorf("读取用户名: %w", err)
	}
	packet.EntityUniqueID, err = stream.ReadEntityID()
	if err != nil {
		return fmt.Errorf("读取 UniqueID: %w", err)
	}
	packet.EntityRuntimeID, err = stream.ReadEntityID()
	if err != nil {
		return fmt.Errorf("读取 RuntimeID: %w", err)
	}
	packet.PosX, packet.PosY, packet.PosZ, err = stream.ReadVector3f()
	if err != nil {
		return fmt.Errorf("读取坐标: %w", err)
	}
	packet.SpeedX, packet.SpeedY, packet.SpeedZ, err = stream.ReadVector3f()
	if err != nil {
		return fmt.Errorf("读取速度: %w", err)
	}
	packet.Pitch, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取 Pitch: %w", err)
	}
	packet.HeadYaw, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取 HeadYaw: %w", err)
	}
	packet.Yaw, err = stream.ReadLFloat()
	if err != nil {
		return fmt.Errorf("读取 Yaw: %w", err)
	}
	packet.Item, err = stream.ReadSlot()
	if err != nil {
		return fmt.Errorf("读取手持物品: %w", err)
	}
	var count uint32 = 0
	count, err = stream.ReadUnsignedVarInt()
	if err != nil {
		return fmt.Errorf("读取元数据数量: %w", err)
	}
	// 服务端只发不收，测试里只校验条目数。
	packet.Metadata = make([]MetadataEntry, count)
	return nil
}

// PlayerMetadata is Human::$dataProperties as sent in AddPlayer:
// 名字常显、可攀爬、在呼吸（否则别人头上会冒气泡）。
func PlayerMetadata(name string, flags uint64) []MetadataEntry {
	return []MetadataEntry{
		{Key: EntityDataFlags, Type: EntityDataTypeLong, Long: int64(flags)},
		{Key: EntityDataAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
		{Key: EntityDataMaxAir, Type: EntityDataTypeShort, Short: DefaultAirTicks},
		{Key: EntityDataNametag, Type: EntityDataTypeString, String: name},
		{Key: EntityDataLeadHolder, Type: EntityDataTypeLong, Long: -1},
		{Key: EntityDataScale, Type: EntityDataTypeFloat, Float: 1},
	}
}
