package storage_test

import "bytes"
import "compress/gzip"
import "compress/zlib"
import "encoding/binary"
import "os"
import "path/filepath"
import "strings"
import "testing"

import "go-mc-server/internal/storage"

func pmTestField(name string, kind byte, data []byte) []byte {
	var head = binary.BigEndian.AppendUint16([]byte{kind}, uint16(len(name)))
	return append(append(head, []byte(name)...), data...)
}

func pmTestList(name string, kind byte, size int, data []byte) []byte {
	return pmTestField(name, 9, append(binary.BigEndian.AppendUint32([]byte{kind}, uint32(size)), data...))
}

func pmTestArray(name string, data []byte) []byte {
	return pmTestField(name, 7, append(binary.BigEndian.AppendUint32(nil, uint32(len(data))), data...))
}

func pmTestRoot(name string, fields []byte) []byte {
	return pmTestField("", 10, append(pmTestField(name, 10, append(fields, 0)), 0))
}

func pmTestLevel(test *testing.T, dir string) {
	var fields = pmTestField("RandomSeed", 4, []byte{255, 255, 255, 255, 255, 255, 255, 249})
	fields = append(fields, pmTestField("generatorName", 8, []byte{0, 6, 'n', 'o', 'r', 'm', 'a', 'l'})...)
	fields = append(fields, pmTestField("SpawnX", 3, []byte{0, 0, 0, 1})...)
	fields = append(fields, pmTestField("SpawnY", 3, []byte{0, 0, 0, 65})...)
	fields = append(fields, pmTestField("SpawnZ", 3, []byte{0, 0, 0, 3})...)
	fields = append(fields, pmTestField("Time", 4, []byte{0, 0, 0, 0, 0, 0, 1, 2})...)
	var buf = bytes.Buffer{}
	var writer = gzip.NewWriter(&buf)
	var _, err = writer.Write(pmTestRoot("Data", fields))
	if err != nil {
		test.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		test.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "level.dat"), buf.Bytes(), 0644); err != nil {
		test.Fatal(err)
	}
}

func pmTestRegion(test *testing.T, dir, format string, extra []byte) {
	var fields = pmTestField("xPos", 3, []byte{255, 255, 255, 255})
	fields = append(fields, pmTestField("zPos", 3, []byte{0, 0, 0, 32})...)
	// 区块 (-1,32)，局部 (2,37,3) 是羊毛:11，邻格的半字节不同以暴露奇偶翻转。
	if format == "mcr" {
		var ids = make([]byte, 32768)
		var values = make([]byte, 16384)
		ids[4517], values[2258] = 35, 0xb2
		fields = append(fields, pmTestArray("Blocks", ids)...)
		fields = append(fields, pmTestArray("Data", values)...)
	} else {
		var ids = make([]byte, 4096)
		var values = make([]byte, 2048)
		if format == "mca" {
			ids[1330], values[665] = 35, 0x2b
			ids[1331], ids[1332], ids[1333], ids[1334] = 54, 54, 61, 154
			copy(ids[1335:], []byte{63, 26, 144, 140, 199, 130})
		} else {
			ids[565], values[282] = 35, 0xb2
		}
		var section = pmTestField("Y", 1, []byte{2})
		section = append(section, pmTestArray("Blocks", ids)...)
		section = append(section, pmTestArray("Data", values)...)
		fields = append(fields, pmTestList("Sections", 10, 1, append(section, 0))...)
	}
	fields = append(fields, extra...)
	var biomes = make([]byte, 256)
	biomes[50] = 23
	fields = append(fields, pmTestArray("Biomes", biomes)...)
	var buf = bytes.Buffer{}
	var writer = zlib.NewWriter(&buf)
	var _, err = writer.Write(pmTestRoot("Level", fields))
	if err != nil {
		test.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		test.Fatal(err)
	}
	var region = make([]byte, 12288)
	binary.BigEndian.PutUint32(region[31*4:], 0x201)
	binary.BigEndian.PutUint32(region[8192:], uint32(1+buf.Len()))
	region[8196] = 2
	copy(region[8197:], buf.Bytes())
	if err = os.MkdirAll(filepath.Join(dir, "region"), 0755); err != nil {
		test.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "region", "r.-1.1."+format), region, 0644); err != nil {
		test.Fatal(err)
	}
}

// 三种区域格式应还原同一个方块、颜色和群系，并能用正常 GMR 存储器逐列回读。
func TestPMImportTerrain(test *testing.T) {
	var formats = [...]string{"mca", "mcr", "mcapm"}
	var index = 0
	for index = 0; index < len(formats); index++ {
		var source = test.TempDir()
		var target = filepath.Join(test.TempDir(), "Mine")
		pmTestLevel(test, source)
		pmTestRegion(test, source, formats[index], nil)
		var count, err = storage.ImportPMWorld(source, target)
		if err != nil || count != 1 {
			test.Fatalf("期望导入 %s 一列，实际 %d / %v", formats[index], count, err)
		}
		var meta, metaErr = storage.LoadWorldMeta(filepath.Join(target, "world.json"), storage.WorldDefaults{})
		if metaErr != nil || meta.Name != "Mine" || meta.Seed != -7 || meta.Time != 258 || meta.Generator != "normal" || meta.Spawn != [3]int32{1, 65, 3} {
			test.Fatalf("期望保留种子、时间和出生点，实际 %+v / %v", meta, metaErr)
		}
		var column, loadErr = storage.NewRegionStore(target).LoadColumn(-1, 32)
		if loadErr != nil || column == nil {
			test.Fatalf("期望读回负坐标列，实际 %v / %v", column, loadErr)
		}
		var id, data = column.GetBlock(2, 37, 3)
		var disk = column.EncodeDisk()
		if id != 35 || data != 11 || disk[len(disk)-256+50] != 23 {
			test.Fatalf("期望 %s 羊毛 35:11、群系 23，实际 %d:%d / %d", formats[index], id, data, disk[len(disk)-256+50])
		}
	}
}

// 不支持的实体数据不能静默扔掉；已有世界也不能被覆盖。
func TestPMImportRefusesLoss(test *testing.T) {
	var source = test.TempDir()
	var target = filepath.Join(test.TempDir(), "Mine")
	pmTestLevel(test, source)
	pmTestRegion(test, source, "mca", pmTestList("TileEntities", 10, 1, []byte{0}))
	var _, err = storage.ImportPMWorld(source, target)
	if err == nil || !strings.Contains(err.Error(), "TileEntities") {
		test.Fatalf("期望报告不支持的方块实体，实际 %v", err)
	}
	var _, statErr = os.Stat(target)
	if !os.IsNotExist(statErr) {
		test.Fatalf("期望不留下目标世界，实际 %v", statErr)
	}
	if err = os.Mkdir(target, 0755); err != nil {
		test.Fatal(err)
	}
	_, err = storage.ImportPMWorld(source, target)
	if err == nil || !strings.Contains(err.Error(), "已存在") {
		test.Fatalf("期望拒绝覆盖已有目录，实际 %v", err)
	}
}

// 用只读参考树的真实 level.dat 验证类型、字节序和生成器映射；纯 Go 仓库不带这份本机资料。
func TestPMImportBundledWorlds(test *testing.T) {
	var ref = filepath.Join("..", "..", "pocketmine服务器参考", "worlds")
	var _, statErr = os.Stat(ref)
	if os.IsNotExist(statErr) {
		test.Skip("未附带参考地图")
	}
	var samples = [...][2]string{{"world", "normal"}, {"nether", "nether_terrain"}, {"ender", "end_terrain"}}
	var index = 0
	for index = 0; index < len(samples); index++ {
		var sample = samples[index]
		var dest = filepath.Join(test.TempDir(), "Imported")
		var count, err = storage.ImportPMWorld(filepath.Join(ref, sample[0]), dest)
		if err != nil || count != 0 {
			test.Fatalf("期望 %s 样本只有元数据，实际 %d 列 / %v", sample[0], count, err)
		}
		var meta, readErr = storage.LoadWorldMeta(filepath.Join(dest, "world.json"), storage.WorldDefaults{})
		if readErr != nil || meta.Name != "Imported" || meta.Generator != sample[1] || meta.Spawn != [3]int32{256, 70, 256} || meta.Seed != 1777430934 || meta.Time != 0 {
			test.Fatalf("期望还原 %s 样本，实际 %+v / %v", sample[0], meta, readErr)
		}
	}
}
