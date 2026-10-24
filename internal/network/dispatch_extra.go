package network

import "go-mc-server/internal/protocol/packets"

// dispatchExtra 处理后加的几种包，返回 true 表示已经处理。
// 用二值类型断言而不是再往 dispatch 那个 switch 里加 case：
// 项目规范禁止类型 switch，旧的那个只是存量，不在这里扩大。
func (s *Session) dispatchExtra(pkt packets.Packet) bool {
	var eventPkt, isEvent = pkt.(*packets.EntityEventPacket)
	if isEvent {
		if s.OnEntityEvent != nil {
			s.OnEntityEvent(s, eventPkt)
		}
		return true
	}
	var craftPkt, isCraft = pkt.(*packets.CraftingEventPacket)
	if isCraft {
		if s.OnCraftingEvent != nil {
			s.OnCraftingEvent(s, craftPkt)
		}
		return true
	}
	return false
}
