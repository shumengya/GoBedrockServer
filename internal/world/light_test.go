package world_test

import "encoding/binary"
import "testing"
import "go-mc-server/internal/world"

// 直接读 1.1 包里的半字节，独立于服务端的查询函数。
func wireLight(payload []byte, x, y, z, offset int) byte {
	var index = 1 + (y/16)*10241 + 1 + 4096 + 2048 + offset + 128*x + 8*z + (y%16)/2
	return payload[index] >> uint(4*(y%2)) & 15
}

// 高度图不计玻璃；隔一层水、树叶时亮度按参考端衰减。时间只影响玩法，不改线上的天空光。
func TestSkyLightWireAndClock(test *testing.T) {
	var level = world.New("sky")
	level.Generator = world.GeneratorSky
	level.SetBlock(19, 48, 5, 20, 0)
	level.SetBlock(19, 34, 5, 9, 0)
	level.SetBlock(19, 33, 5, 18, 0)
	level.SetTimeTicks(world.TimeNoon)
	var raw = level.NetworkSerialize(1, 0)
	var height = binary.LittleEndian.Uint16(raw[1+int(raw[0])*10241+2*(5*16+3):])
	if height != 35 || wireLight(raw, 3, 35, 5, 0) != 15 || wireLight(raw, 3, 34, 5, 0) != 12 || wireLight(raw, 3, 33, 5, 0) != 11 || wireLight(raw, 3, 32, 5, 0) != 10 {
		test.Fatalf("期望高度 35、水下 12/11/10，实际 %d / %d / %d / %d", height, wireLight(raw, 3, 34, 5, 0), wireLight(raw, 3, 33, 5, 0), wireLight(raw, 3, 32, 5, 0))
	}
	level.SetBlock(19, 34, 5, 0, 0)
	var noon, _ = level.LightAt(19, 33, 5)
	level.SetTimeTicks(world.TimeMidnight)
	var night, _ = level.LightAt(19, 33, 5)
	if noon != 14 || night != 3 || wireLight(level.NetworkSerialize(1, 0), 3, 33, 5, 0) != 14 {
		test.Fatalf("期望白天 14、午夜 3，实际 %d/%d", noon, night)
	}
}

// 旧列高度把玻璃也算作屋顶；重读后玻璃下方应见天，不能照抄旧高度字段。
func TestSkyLightLegacyHeight(test *testing.T) {
	var store = newMemoryStore()
	var level = world.New("legacy")
	level.SetStore(store)
	level.SetBlock(2, 90, 3, world.BlockGlass, 0)
	var _, err = level.UnloadChunks(keepNone)
	if err != nil {
		test.Fatal(err)
	}
	var disk = store.saved[[2]int32{0, 0}]
	var index = len(disk) - 768 + 2*(3*16+2)
	binary.LittleEndian.PutUint16(disk[index:], 91)
	var wire = level.NetworkSerialize(0, 0)
	var height = binary.LittleEndian.Uint16(wire[1+int(wire[0])*10241+2*(3*16+2):])
	if height != 5 || wireLight(wire, 2, 89, 3, 0) != 15 {
		test.Fatalf("期望高度 5、玻璃下仍有天光，实际 %d / %d", height, wireLight(wire, 2, 89, 3, 0))
	}
}

// 负坐标边界只在邻列加载后接光。卸载光源一侧后不能留下幽灵光，重新读盘会恢复。
func TestBlockLightBoundaryAndDisk(test *testing.T) {
	var level = world.New("nether")
	level.Generator = world.GeneratorNether
	level.SetStore(newMemoryStore())
	level.SetBlock(-1, 31, 8, 50, 0)
	if level.LoadedChunkCount() != 1 {
		test.Fatal("期望火把不唤醒邻区，实际已加载邻区")
	}
	var raw = level.NetworkSerialize(0, 0)
	if raw[0] < 3 || wireLight(raw, 0, 31, 8, 2048) != 13 || wireLight(raw, 0, 32, 8, 2048) != 12 || wireLight(raw, 0, 32, 8, 0) != 0 {
		test.Fatal("期望跨区及空子区块收到方块光、下界天空光为零，实际包不符")
	}
	level.SetBlock(2, 31, 8, 89, 0)
	level.SetBlock(-1, 31, 8, 0, 0)
	var _, other = level.LightAt(-1, 31, 8)
	if other != 12 {
		test.Fatalf("期望拆掉火把后剩萤石亮度 12，实际 %d", other)
	}
	var removed, err = level.UnloadChunks(func(cx, cz int32) bool { return cx < 0 })
	var _, dark = level.LightAt(-1, 31, 8)
	if removed != 1 || err != nil || dark != 0 {
		test.Fatalf("期望卸载光源后灭光，实际 %d / %v / %d", removed, err, dark)
	}
	level.GetBlock(2, 31, 8)
	var _, again = level.LightAt(-1, 31, 8)
	if again != 12 {
		test.Fatalf("期望读档后跨区光恢复 12，实际 %d", again)
	}
}

// 堵住一条通道后，光只能绕路；再换成水，光从直线路径衰减三级。
func TestBlockLightEdits(test *testing.T) {
	var level = world.New("flat")
	level.SetBlock(5, 50, 5, 11, 0)
	level.SetBlock(6, 50, 5, 1, 0)
	var _, detour = level.LightAt(7, 50, 5)
	level.SetBlock(6, 50, 5, 9, 0)
	var _, water = level.LightAt(6, 50, 5)
	level.SetBlock(6, 50, 5, 20, 0)
	var _, glass = level.LightAt(7, 50, 5)
	level.SetBlock(5, 50, 5, 0, 0)
	var _, dark = level.LightAt(7, 50, 5)
	if detour != 11 || water != 12 || glass != 13 || dark != 0 {
		test.Fatalf("期望绕路/水/玻璃/熄光为 11/12/13/0，实际 %d/%d/%d/%d", detour, water, glass, dark)
	}
}
