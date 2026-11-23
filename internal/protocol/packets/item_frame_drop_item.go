package packets

import "go-mc-server/internal/protocol"

// ItemFrameDropItemPacket（0x47）只指定展示框坐标；掉哪件物品由服务端决定。
type ItemFrameDropItemPacket struct {
	X, Y, Z int32
}

func (pkt *ItemFrameDropItemPacket) ID() byte { return protocol.IDItemFrameDropItem }

func (pkt *ItemFrameDropItemPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteBlockCoords(pkt.X, pkt.Y, pkt.Z)
	return nil
}

func (pkt *ItemFrameDropItemPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	pkt.X, pkt.Y, pkt.Z, err = stream.ReadBlockCoords()
	return err
}
