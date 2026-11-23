package nbt

import "go-mc-server/internal/protocol"

// 背包 Slot 存固定宽度的小端 NBT。放进框里后，Item.tag 随方块实体使用网络 NBT。
// 原始字节留在物品上；无效标签只省略客户端显示，不破坏整列数据或掉落实物。
func frameItemTag(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var source = protocol.NewStream(raw)
	var kind, err = source.ReadByte()
	if err != nil || kind != TagCompound {
		return nil
	}
	var _, valid = itemTagString(source)
	if !valid {
		return nil
	}
	var target = protocol.NewWriter()
	_ = target.WriteByte(TagCompound)
	target.WriteString("tag")
	if !frameTagValue(source, target, TagCompound, 1) || source.Remaining() != 0 {
		return nil
	}
	return target.Bytes()
}

func itemTagString(source *protocol.BinaryStream) ([]byte, bool) {
	var length, err = source.ReadUint16LE()
	if err != nil {
		return nil, false
	}
	var value, readErr = source.ReadBytes(int(length))
	return value, readErr == nil
}

func frameTagValue(source, target *protocol.BinaryStream, kind byte, depth int) bool {
	if depth > maxReadDepth {
		return false
	}
	var width int = 0
	switch kind {
	case TagByte:
		width = 1
	case TagShort:
		width = 2
	case TagFloat:
		width = 4
	case TagDouble:
		width = 8
	case TagInt:
		var value, err = source.ReadInt32LE()
		if err != nil {
			return false
		}
		target.WriteVarInt(value)
		return true
	case TagLong:
		var value, err = source.ReadInt64LE()
		if err != nil {
			return false
		}
		target.WriteVarLong(value)
		return true
	case TagString:
		var value, valid = itemTagString(source)
		if !valid {
			return false
		}
		target.WriteString(string(value))
		return true
	case TagByteArray, TagIntArray:
		var length, err = source.ReadInt32LE()
		var stride int = 1
		if kind == TagIntArray {
			stride = 4
		}
		if err != nil || length < 0 || int(length) > source.Remaining()/stride {
			return false
		}
		target.WriteVarInt(length)
		width = int(length) * stride
	case TagCompound:
		for {
			var child, err = source.ReadByte()
			if err != nil {
				return false
			}
			_ = target.WriteByte(child)
			if child == TagEnd {
				return true
			}
			var name, valid = itemTagString(source)
			if !valid {
				return false
			}
			target.WriteString(string(name))
			if !frameTagValue(source, target, child, depth+1) {
				return false
			}
		}
	case TagList:
		var child, err = source.ReadByte()
		if err != nil {
			return false
		}
		var size, readErr = source.ReadInt32LE()
		if readErr != nil || size < 0 || int(size) > source.Remaining() {
			return false
		}
		_ = target.WriteByte(child)
		target.WriteVarInt(size)
		var index int32 = 0
		for index = 0; index < size; index++ {
			if !frameTagValue(source, target, child, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
	var payload, err = source.ReadBytes(width)
	if err != nil {
		return false
	}
	target.WriteBytes(payload)
	return true
}
