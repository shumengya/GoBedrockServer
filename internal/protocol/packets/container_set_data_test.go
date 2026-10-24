package packets_test

import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

type containerDataCase struct {
	Name     string
	WindowID byte
	Property int32
	Value    int32
}

// 窗口号一个字节，属性号和值都是变长整数；负数走 zigzag，要能原样还回来。
func TestContainerSetDataRoundTrip(test *testing.T) {
	var cases []containerDataCase = []containerDataCase{
		{Name: "烤制进度", WindowID: 2, Property: 0, Value: 137},
		{Name: "火焰剩余", WindowID: 2, Property: 1, Value: 200},
		{Name: "值为零", WindowID: 2, Property: 1, Value: 0},
		{Name: "大一点的值", WindowID: 2, Property: 0, Value: 20000},
		{Name: "负数", WindowID: 3, Property: 5, Value: -1},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item containerDataCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var original *packets.ContainerSetDataPacket = &packets.ContainerSetDataPacket{
				WindowID: item.WindowID,
				Property: item.Property,
				Value:    item.Value,
			}
			var raw, encodeErr = packets.EncodePacket(original)
			if encodeErr != nil {
				sub.Fatalf("编码失败: %v", encodeErr)
			}
			if raw[0] != protocol.IDContainerSetData {
				sub.Fatalf("包 ID 期望 0x%02x，实际 0x%02x", protocol.IDContainerSetData, raw[0])
			}
			var decoded *packets.ContainerSetDataPacket = &packets.ContainerSetDataPacket{}
			var decodeErr error = decoded.Decode(protocol.NewStream(raw[1:]))
			if decodeErr != nil {
				sub.Fatalf("解码失败: %v", decodeErr)
			}
			if *decoded != *original {
				sub.Fatalf("期望 %+v，实际 %+v", *original, *decoded)
			}
		})
	}
}

// 登记在包注册表里，按 ID 能查到。
func TestContainerSetDataRegistered(test *testing.T) {
	var looked packets.Packet = packets.Lookup(protocol.IDContainerSetData)
	var _, isData = looked.(*packets.ContainerSetDataPacket)
	if !isData {
		test.Fatalf("按 ID 查到的应是 ContainerSetDataPacket，实际 %T", looked)
	}
}
