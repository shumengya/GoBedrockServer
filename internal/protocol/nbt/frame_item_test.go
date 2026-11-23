package nbt

import "bytes"
import "testing"
import "go-mc-server/internal/protocol"

// 命名物品的字符串长度和负整数都要转成网络格式；截断的标签不能使后面的方块实体错位。
func TestFrameItemTag(test *testing.T) {
	var raw = []byte{10, 0, 0, 10, 7, 0, 'd', 'i', 's', 'p', 'l', 'a', 'y', 8, 4, 0, 'N', 'a', 'm', 'e', 2, 0, 'H', 'i', 0, 3, 1, 0, 'v', 255, 255, 255, 255, 0}
	var want = []byte{10, 3, 't', 'a', 'g', 10, 7, 'd', 'i', 's', 'p', 'l', 'a', 'y', 8, 4, 'N', 'a', 'm', 'e', 2, 'H', 'i', 0, 3, 1, 'v', 1, 0}
	var item = protocol.Item{ID: 267, Count: 1, Meta: 5, NBT: raw}
	var frame = SpawnItemFrame(-1, 64, 2, 7, item)
	if !bytes.Contains(frame, want) {
		test.Fatalf("期望嵌入网络 NBT %x，实际 %x", want, frame)
	}
	item.NBT = raw[:len(raw)-1]
	var malformed = SpawnItemFrame(-1, 64, 2, 7, item)
	item.NBT = nil
	if !bytes.Equal(malformed, SpawnItemFrame(-1, 64, 2, 7, item)) {
		test.Fatal("期望截断的标签整体省略，实际有残片")
	}
}
