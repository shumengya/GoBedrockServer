package storage_test

import "bytes"
import "encoding/binary"
import "os"
import "path/filepath"
import "strings"
import "testing"
import "go-mc-server/internal/storage"

func pmTestText(name, value string) []byte {
	return pmTestField(name, 8, append(binary.BigEndian.AppendUint16(nil, uint16(len(value))), value...))
}

func pmTestInt(name string, value int32) []byte {
	return pmTestField(name, 3, binary.BigEndian.AppendUint32(nil, uint32(value)))
}

func pmTestContainer(id string, pos int32, fields ...[]byte) []byte {
	var out = pmTestText("id", id)
	out = append(out, pmTestInt("x", pos)...)
	out = append(out, pmTestInt("y", 37)...)
	out = append(out, pmTestInt("z", 515)...)
	var index = 0
	for index = 0; index < len(fields); index++ {
		out = append(out, fields[index]...)
	}
	return append(out, 0)
}

// 经本服启动时的加载路径读回两半箱子、末格物品的嵌套 NBT、炉火进度和漏斗冷却。
func TestPMImportContainers(test *testing.T) {
	var source, data = test.TempDir(), test.TempDir()
	var dest = storage.WorldDir(data, "Mine")
	var dirErr = os.MkdirAll(filepath.Dir(dest), 0755)
	if dirErr != nil {
		test.Fatal(dirErr)
	}
	pmTestLevel(test, source)
	var inner = pmTestField("lvl", 2, []byte{1, 2})
	var extra = pmTestList("ench", 10, 1, append(inner, 0))
	extra = append(extra, pmTestList("empty", 3, 0, nil)...)
	var stack = pmTestText("id", "minecraft:wool")
	stack = append(stack, pmTestField("Slot", 1, []byte{26})...)
	stack = append(stack, pmTestField("Count", 1, []byte{5})...)
	stack = append(stack, pmTestField("Damage", 2, []byte{0, 11})...)
	stack = append(stack, pmTestField("tag", 10, append(extra, 0))...)
	var tiles = pmTestContainer("Chest", -13, pmTestList("Items", 10, 1, append(stack, 0)), pmTestInt("pairx", -12), pmTestInt("pairz", 515))
	tiles = append(tiles, pmTestContainer("Chest", -12, pmTestInt("pairx", -13), pmTestInt("pairz", 515))...)
	tiles = append(tiles, pmTestContainer("Furnace", -11, pmTestField("BurnTime", 2, []byte{1, 33}), pmTestField("CookTime", 2, []byte{0, 29}), pmTestField("MaxTime", 2, []byte{6, 64}))...)
	tiles = append(tiles, pmTestContainer("Hopper", -10, pmTestInt("TransferCooldown", 7))...)
	pmTestRegion(test, source, "mca", pmTestList("TileEntities", 10, 4, tiles))
	var _, err = storage.ImportPMWorld(source, dest)
	if err != nil {
		test.Fatalf("期望导入含容器的世界，实际 %v", err)
	}
	var loaded, openErr = storage.OpenWorld(data, storage.WorldDefaults{Name: "Mine"})
	if openErr != nil {
		test.Fatalf("期望重开新世界，实际 %v", openErr)
	}
	var chest, found = loaded.TileAt(-13, 37, 515)
	var other, second = loaded.TileAt(-12, 37, 515)
	if !found || !second || !chest.Paired || !other.Paired || chest.PairX != -12 || other.PairX != -13 || chest.PairZ != 515 || len(chest.Items) != 27 {
		test.Fatalf("期望双箱按坐标配对，实际 %+v / %+v", chest, other)
	}
	var held = chest.Items[26]
	var want = []byte{10, 0, 0, 9, 4, 0, 'e', 'n', 'c', 'h', 10, 1, 0, 0, 0, 2, 3, 0, 'l', 'v', 'l', 2, 1, 0, 9, 5, 0, 'e', 'm', 'p', 't', 'y', 3, 0, 0, 0, 0, 0}
	if held.ID != 35 || held.Meta != 11 || held.Count != 5 || !bytes.Equal(held.NBT, want) || !chest.Items[0].IsAir() {
		test.Fatalf("期望末格有五块带小端 NBT 的蓝羊毛，实际 %+v", held)
	}
	var furnace, fired = loaded.TileAt(-11, 37, 515)
	var hopper, cooled = loaded.TileAt(-10, 37, 515)
	if !fired || !cooled || furnace.BurnTime != 289 || furnace.CookTime != 29 || furnace.MaxTime != 1600 || hopper.TransferCooldown != 7 {
		test.Fatalf("期望保留燃烧、烹饪及漏斗计时，实际 %+v / %+v", furnace, hopper)
	}
}

// 告示牌的空行、床的白色、头颅角度和展示物的数字 ID 经过存档后不被默认值替换。
func TestPMImportDecorations(test *testing.T) {
	var source, data = test.TempDir(), test.TempDir()
	var dest = storage.WorldDir(data, "Mine")
	var dirErr = os.MkdirAll(filepath.Dir(dest), 0755)
	if dirErr != nil {
		test.Fatal(dirErr)
	}
	pmTestLevel(test, source)
	var tiles = pmTestContainer("Sign", -9, pmTestText("Text1", "北门"), pmTestText("Text4", "出口"))
	tiles = append(tiles, pmTestContainer("Bed", -8, pmTestField("color", 1, []byte{0}))...)
	tiles = append(tiles, pmTestContainer("Skull", -7, pmTestField("Rot", 1, []byte{11}), pmTestField("SkullType", 1, []byte{5}))...)
	tiles = append(tiles, pmTestContainer("FlowerPot", -6, pmTestField("item", 2, []byte{0, 6}), pmTestInt("mData", 2))...)
	var stack = pmTestField("id", 2, []byte{1, 8})
	stack = append(stack, pmTestField("Count", 1, []byte{1})...)
	tiles = append(tiles, pmTestContainer("ItemFrame", -5, pmTestField("Item", 10, append(stack, 0)), pmTestField("ItemRotation", 1, []byte{3}), pmTestField("ItemDropChance", 5, []byte{63, 128, 0, 0}))...)
	tiles = append(tiles, pmTestContainer("EnderChest", -4)...)
	pmTestRegion(test, source, "mca", pmTestList("TileEntities", 10, 6, tiles))
	var _, err = storage.ImportPMWorld(source, dest)
	if err != nil {
		test.Fatalf("期望导入装饰与末影箱，实际 %v", err)
	}
	var loaded, openErr = storage.OpenWorld(data, storage.WorldDefaults{Name: "Mine"})
	if openErr != nil {
		test.Fatal(openErr)
	}
	var sign, _ = loaded.TileAt(-9, 37, 515)
	var bed, found = loaded.TileAt(-8, 37, 515)
	var skull, _ = loaded.TileAt(-7, 37, 515)
	var pot, _ = loaded.TileAt(-6, 37, 515)
	var frame, _ = loaded.TileAt(-5, 37, 515)
	var ender, _ = loaded.TileAt(-4, 37, 515)
	if sign.Lines != [4]string{"北门", "", "", "出口"} || !found || bed.Color != 0 || skull.SkullType != 5 || skull.Rotation != 11 || ender.ID != "EnderChest" {
		test.Fatalf("期望保留文字、床色、头颅和末影箱，实际 %+v / %+v / %+v / %+v", sign, bed, skull, ender)
	}
	if len(pot.Items) != 1 || pot.Items[0].ID != 6 || pot.Items[0].Meta != 2 || len(frame.Items) != 1 || frame.Items[0].ID != 264 || frame.Rotation != 3 {
		test.Fatalf("期望盆中白桦树苗、框中钻石且旋转 3，实际 %+v / %+v", pot, frame)
	}
}

// 已知容器带无法还原的锁或自定义名称时，也必须报出字段和位置，不能发布残缺世界。
func TestPMImportUnknownContainerState(test *testing.T) {
	var source, dest = test.TempDir(), filepath.Join(test.TempDir(), "Mine")
	pmTestLevel(test, source)
	var tiles = pmTestContainer("Chest", -13, pmTestText("Lock", "key"))
	pmTestRegion(test, source, "mca", pmTestList("TileEntities", 10, 1, tiles))
	var _, err = storage.ImportPMWorld(source, dest)
	if err == nil || !strings.Contains(err.Error(), "Lock") || !strings.Contains(err.Error(), "[-13 37 515]") {
		test.Fatalf("期望定位不支持的锁，实际 %v", err)
	}
	var _, statErr = os.Stat(dest)
	if !os.IsNotExist(statErr) {
		test.Fatalf("期望不发布新图，实际 %v", statErr)
	}
}
