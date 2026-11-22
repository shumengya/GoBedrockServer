package packets_test

import "bytes"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 独立核对参考端三个事件的字节，包括移除事件末尾的零值字段。
func TestMobEffectWire(test *testing.T) {
	var cases = []struct {
		packet packets.MobEffectPacket
		wire   []byte
	}{
		{packets.MobEffectPacket{EntityRuntimeID: 7, Event: packets.MobEffectAdd, EffectID: 19, Amplifier: 3, Particles: true, Duration: 1200}, []byte{0x1d, 14, 1, 38, 6, 1, 0xe0, 0x12}},
		{packets.MobEffectPacket{EntityRuntimeID: 7, Event: packets.MobEffectModify, EffectID: 19, Duration: 80}, []byte{0x1d, 14, 2, 38, 0, 0, 0xa0, 1}},
		{packets.MobEffectPacket{EntityRuntimeID: 7, Event: packets.MobEffectRemove, EffectID: 19}, []byte{0x1d, 14, 3, 38, 0, 0, 0}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var example = cases[index]
		var encoded, err = packets.EncodePacket(&example.packet)
		if err != nil || !bytes.Equal(encoded, example.wire) {
			test.Fatalf("期望 %x，实际 %x (%v)", example.wire, encoded, err)
		}
		var decoded, found = packets.Lookup(example.wire[0]).(*packets.MobEffectPacket)
		if !found {
			test.Fatal("期望 MobEffect 已注册，实际未找到构造器")
		}
		var stream = protocol.NewStream(example.wire[1:])
		err = decoded.Decode(stream)
		if err != nil || !stream.EOF() || *decoded != example.packet {
			test.Fatalf("期望 %+v，实际 %+v (%v)", example.packet, decoded, err)
		}
	}
}
