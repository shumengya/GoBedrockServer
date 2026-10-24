package packets

import "testing"

import "go-mc-server/internal/protocol"

func TestAddPlayerRoundTrip(test *testing.T) {
	var original *AddPlayerPacket = &AddPlayerPacket{
		UUID:            [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Username:        "Steve",
		EntityUniqueID:  7,
		EntityRuntimeID: 7,
		PosX:            1.5,
		PosY:            4,
		PosZ:            -2.5,
		Pitch:           10,
		HeadYaw:         90,
		Yaw:             90,
		Item:            protocol.Item{ID: 1, Count: 64},
		Metadata:        PlayerMetadata("Steve"),
	}
	var raw, err = EncodePacket(original)
	if err != nil {
		test.Fatal(err)
	}
	if raw[0] != protocol.IDAddPlayer {
		test.Fatalf("id 0x%02x", raw[0])
	}
	var decoded *AddPlayerPacket = &AddPlayerPacket{}
	err = decoded.Decode(protocol.NewStream(raw[1:]))
	if err != nil {
		test.Fatal(err)
	}
	var samePos bool = decoded.PosX == original.PosX && decoded.PosY == original.PosY && decoded.PosZ == original.PosZ
	if decoded.UUID != original.UUID || decoded.Username != "Steve" || decoded.EntityRuntimeID != 7 || !samePos || decoded.Yaw != 90 || decoded.Item.ID != 1 {
		test.Fatalf("got %+v want %+v", decoded, original)
	}
	if len(decoded.Metadata) != len(original.Metadata) {
		test.Fatalf("元数据数量 %d，应为 %d", len(decoded.Metadata), len(original.Metadata))
	}
}
