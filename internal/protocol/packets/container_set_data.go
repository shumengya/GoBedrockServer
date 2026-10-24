package packets

import "fmt"

import "go-mc-server/internal/protocol"

// ContainerSetDataPacket (0x33) 更新窗口里的一个数值属性。熔炉用它画烤制箭头和火焰：
// 属性 0 是烤制进度，属性 1 是火焰剩余。
type ContainerSetDataPacket struct {
	WindowID byte
	Property int32
	Value    int32
}

func (p *ContainerSetDataPacket) ID() byte { return protocol.IDContainerSetData }

func (p *ContainerSetDataPacket) Encode(s *protocol.BinaryStream) error {
	var err error = s.WriteByte(p.WindowID)
	if err != nil {
		return fmt.Errorf("写窗口号: %w", err)
	}
	s.WriteVarInt(p.Property)
	s.WriteVarInt(p.Value)
	return nil
}

func (p *ContainerSetDataPacket) Decode(s *protocol.BinaryStream) error {
	var err error = nil
	p.WindowID, err = s.ReadByte()
	if err != nil {
		return fmt.Errorf("读窗口号: %w", err)
	}
	p.Property, err = s.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读属性号: %w", err)
	}
	p.Value, err = s.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读属性值: %w", err)
	}
	return nil
}
