package packets_test

import "bytes"
import "testing"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"

// 拍框请求不携带物品；固定字节区分带符号的 X/Z 和无符号的 Y。
func TestItemFrameDropWire(test *testing.T) {
	var packet, registered = packets.Lookup(0x47).(*packets.ItemFrameDropItemPacket)
	if !registered {
		test.Fatal("期望注册展示框取物包，实际未找到")
	}
	var wire = []byte{0x01, 0x40, 0x04}
	var err = packet.Decode(protocol.NewStream(wire))
	if err != nil || packet.X != -1 || packet.Y != 64 || packet.Z != 2 {
		test.Fatalf("期望 (-1,64,2)，实际 %+v / %v", packet, err)
	}
	var writer = protocol.NewWriter()
	err = packet.Encode(writer)
	if err != nil || !bytes.Equal(wire, writer.Bytes()) || packet.ID() != 0x47 {
		test.Fatalf("期望 47 01 40 04，实际 %02x %x / %v", packet.ID(), writer.Bytes(), err)
	}
	if packet.Decode(protocol.NewStream(wire[:2])) == nil {
		test.Fatal("期望缺失 Z 的包被拒绝，实际通过")
	}
}
