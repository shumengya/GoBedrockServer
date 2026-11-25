package storage_test

import "bytes"
import "encoding/binary"
import "math"
import "os"
import "path/filepath"
import "strings"
import "testing"

import "go-mc-server/internal/storage"

func pmTestDrop() []byte {
	var fields = pmTestText("id", "Item")
	var pos, motion = []byte{}, []byte{}
	var coords = [...]float64{-13.25, 65.5, 515.75}
	var speed = [...]float64{0.125, -0.25, 0.5}
	var index = 0
	for index = 0; index < 3; index++ {
		pos = binary.BigEndian.AppendUint64(pos, math.Float64bits(coords[index]))
		motion = binary.BigEndian.AppendUint64(motion, math.Float64bits(speed[index]))
	}
	fields = append(fields, pmTestList("Pos", 6, 3, pos)...)
	fields = append(fields, pmTestList("Motion", 6, 3, motion)...)
	var angles = binary.BigEndian.AppendUint32(nil, math.Float32bits(77.5))
	angles = binary.BigEndian.AppendUint32(angles, math.Float32bits(-2))
	fields = append(fields, pmTestList("Rotation", 5, 2, angles)...)
	var item = pmTestField("id", 2, []byte{1, 8})
	item = append(item, pmTestField("Count", 1, []byte{3})...)
	item = append(item, pmTestField("Damage", 2, []byte{0, 4})...)
	item = append(item, pmTestField("tag", 10, append(pmTestField("v", 2, []byte{1, 2}), 0))...)
	fields = append(fields, pmTestField("Item", 10, append(item, 0))...)
	fields = append(fields, pmTestField("PickupDelay", 2, []byte{127, 255})...)
	fields = append(fields, pmTestField("Age", 2, []byte{1, 44})...)
	fields = append(fields, pmTestField("OnGround", 1, []byte{1})...)
	fields = append(fields, pmTestField("Health", 2, []byte{0, 5})...)
	fields = append(fields, pmTestField("Fire", 2, []byte{0, 0})...)
	fields = append(fields, pmTestField("Air", 2, []byte{1, 44})...)
	fields = append(fields, pmTestField("Invulnerable", 1, []byte{0})...)
	return append(fields, pmTestField("FallDistance", 5, []byte{0, 0, 0, 0})...)
}

// 同一列内两份相同的物品不能合并，位置、速度、旋转及物品 tag 必须能从原生实体存档读回。
func TestPMImportDrops(test *testing.T) {
	var data, source = test.TempDir(), test.TempDir()
	var dest = storage.WorldDir(data, "Mine")
	var err = os.Mkdir(filepath.Dir(dest), 0755)
	if err != nil {
		test.Fatal(err)
	}
	pmTestLevel(test, source)
	var drop = append(pmTestDrop(), 0)
	pmTestRegion(test, source, "mca", pmTestList("Entities", 10, 2, append(drop, drop...)))
	var _, importErr = storage.ImportPMWorld(source, dest)
	if importErr != nil {
		test.Fatalf("期望导入掉落物，实际 %v", importErr)
	}
	var list, readErr = storage.LoadEntities(data, "Mine")
	if readErr != nil || len(list) != 2 {
		test.Fatalf("期望两份独立的掉落物，实际 %d / %v", len(list), readErr)
	}
	var first = list[0]
	if first.NetworkID != 64 || first.ID != 264 || first.Count != 3 || first.Meta != 4 || !bytes.Equal(first.NBT, []byte{10, 0, 0, 2, 1, 0, 'v', 2, 1, 0}) || first.Age != 300 || first.PickupDelay != 32767 || !first.OnGround {
		test.Fatalf("期望还原物品、落地标志和计时，实际 %+v", first)
	}
	if first.X != -13.25 || first.Y != 65.5 || first.Z != 515.75 || first.SpeedX != 0.125 || first.SpeedY != -0.25 || first.SpeedZ != 0.5 || first.Yaw != 77.5 || first.Pitch != -2 {
		test.Fatalf("期望还原位置、速度和朝向，实际 %+v", first)
	}
}

// 同一列中即使已经转换了合法物品，下一条实体的未知状态也要阻止发布整张图。
func TestPMImportDropRefusesLoss(test *testing.T) {
	var cases = []struct {
		entry  []byte
		reason string
	}{
		{append(pmTestDrop(), pmTestText("Owner", "Alex")...), "Owner"},
		{pmTestText("id", "Cow"), "Cow"},
	}
	var index = 0
	for index = 0; index < len(cases); index++ {
		var source, dest = test.TempDir(), filepath.Join(test.TempDir(), "Mine")
		pmTestLevel(test, source)
		var list = append(append(pmTestDrop(), 0), append(cases[index].entry, 0)...)
		pmTestRegion(test, source, "mcr", pmTestList("Entities", 10, 2, list))
		var _, err = storage.ImportPMWorld(source, dest)
		if err == nil || !strings.Contains(err.Error(), "区块 (-1,32) Entities[1]") || !strings.Contains(err.Error(), cases[index].reason) {
			test.Fatalf("期望按记录定位 %s，实际 %v", cases[index].reason, err)
		}
		var _, statErr = os.Stat(dest)
		if !os.IsNotExist(statErr) {
			test.Fatalf("期望目标不存在，实际 %v", statErr)
		}
	}
}
