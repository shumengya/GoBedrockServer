package nbt

import "go-mc-server/internal/protocol"

// ItemDisplayName 读取背包物品固定宽度小端 NBT 的 display.Name；忽略其他路径上的同名字段。
func ItemDisplayName(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var source = protocol.NewStream(raw)
	var root, err = source.ReadByte()
	if err != nil || root != TagCompound {
		return ""
	}
	var _, valid = itemTagString(source)
	if !valid || !findItemField(source, 1, TagCompound, "display") || !findItemField(source, 2, TagString, "Name") {
		return ""
	}
	var name, readable = itemTagString(source)
	if !readable {
		return ""
	}
	return string(name)
}

// 未命中的字段按类型完整跳过，包括嵌套列表；长度和深度沿用物品框的 NBT 规则。
func findItemField(source *protocol.BinaryStream, depth int, kind byte, name string) bool {
	var skipped = protocol.NewWriter()
	for {
		var child, err = source.ReadByte()
		if err != nil || child == TagEnd {
			return false
		}
		var field, valid = itemTagString(source)
		if !valid {
			return false
		}
		if child == kind && string(field) == name {
			return true
		}
		if !frameTagValue(source, skipped, child, depth+1) {
			return false
		}
	}
}
