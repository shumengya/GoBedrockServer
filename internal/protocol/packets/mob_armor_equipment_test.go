package packets_test

import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 盔甲包编码后能原样解出来：实体号、四个槽位的物品、耐久（meta）和空槽都不能变。
func TestMobArmorEquipmentRoundTrip(test *testing.T) {
	var original *packets.MobArmorEquipmentPacket = &packets.MobArmorEquipmentPacket{EntityRuntimeID: 7}
	original.Slots[0] = protocol.Item{ID: 310, Meta: 12, Count: 1}
	original.Slots[2] = protocol.Item{ID: 308, Meta: 0, Count: 1}
	original.Slots[3] = protocol.Item{ID: 313, Meta: 400, Count: 1}
	var raw, encodeErr = packets.EncodePacket(original)
	if encodeErr != nil {
		test.Fatalf("编码失败: %v", encodeErr)
	}
	if raw[0] != protocol.IDMobArmorEquipment {
		test.Fatalf("包 ID 期望 0x%02x，实际 0x%02x", protocol.IDMobArmorEquipment, raw[0])
	}
	var decoded *packets.MobArmorEquipmentPacket = &packets.MobArmorEquipmentPacket{}
	var decodeErr error = decoded.Decode(protocol.NewStream(raw[1:]))
	if decodeErr != nil {
		test.Fatalf("解码失败: %v", decodeErr)
	}
	if decoded.EntityRuntimeID != 7 {
		test.Fatalf("实体号期望 7，实际 %d", decoded.EntityRuntimeID)
	}
	var index int = 0
	for index = 0; index < len(original.Slots); index++ {
		var want protocol.Item = original.Slots[index]
		var got protocol.Item = decoded.Slots[index]
		if got.ID != want.ID || got.Meta != want.Meta || got.Count != want.Count {
			test.Fatalf("第 %d 格期望 %+v，实际 %+v", index, want, got)
		}
	}
}

// 能从注册表按包 ID 取到：客户端万一发来这个包，不会落到「未知包」警告里。
func TestMobArmorEquipmentIsRegistered(test *testing.T) {
	var found packets.Packet = packets.Lookup(protocol.IDMobArmorEquipment)
	var _, isArmor = found.(*packets.MobArmorEquipmentPacket)
	if !isArmor {
		test.Fatalf("注册表里 0x%02x 应是盔甲包，实际 %T", protocol.IDMobArmorEquipment, found)
	}
}
