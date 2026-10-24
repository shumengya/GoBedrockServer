package storage

import "os"
import "testing"

// 世界时间和「是否冻结」都要存盘，重启后昼夜接着走（或接着冻住）。
func TestWorldClockRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var loaded, err = OpenWorld(dataDir, WorldDefaults{Name: "clock", Seed: 1, SpawnY: 5})
	if err != nil {
		test.Fatal(err)
	}
	loaded.SetTimeTicks(13500)
	loaded.SetTimeStopped(true)
	err = SaveWorldFrom(dataDir, loaded)
	if err != nil {
		test.Fatal(err)
	}
	var reloaded, reloadErr = OpenWorld(dataDir, WorldDefaults{Name: "clock", SpawnY: 5})
	if reloadErr != nil {
		test.Fatal(reloadErr)
	}
	if reloaded.TimeTicks() != 13500 || !reloaded.TimeStopped() {
		test.Fatalf("重开后时间=%d 冻结=%v，期望 13500/true", reloaded.TimeTicks(), reloaded.TimeStopped())
	}
}

// 升级前的 world.json 没有 timeStopped 字段，读出来必须是「没冻结」，时间照旧。
func TestLegacyWorldMetaWithoutTimeStopped(test *testing.T) {
	var dataDir string = test.TempDir()
	var err error = EnsureLayout(dataDir, "old")
	if err != nil {
		test.Fatal(err)
	}
	var legacy string = "{\n  \"name\": \"old\",\n  \"seed\": 7,\n  \"spawn\": [0, 5, 0],\n  \"time\": 4321,\n  \"generator\": \"flat\"\n}\n"
	err = os.WriteFile(WorldMetaPath(dataDir, "old"), []byte(legacy), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	var loaded, openErr = OpenWorld(dataDir, WorldDefaults{Name: "old", SpawnY: 5})
	if openErr != nil {
		test.Fatal(openErr)
	}
	if loaded.TimeTicks() != 4321 || loaded.TimeStopped() {
		test.Fatalf("老存档时间=%d 冻结=%v，期望 4321/false", loaded.TimeTicks(), loaded.TimeStopped())
	}
}

// 饥饿状态随玩家存档往返，防止靠退服重进回满饥饿。
func TestPlayerFoodRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var input PlayerData = PlayerData{
		UUID: "bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee",
		Name: "Alex",
		Food: &FoodData{Level: 7, Saturation: 1.5, Exhaustion: 2.25, Timer: 33},
	}
	var err error = SavePlayer(dataDir, input)
	if err != nil {
		test.Fatal(err)
	}
	var output, found, loadErr = LoadPlayer(dataDir, input.UUID)
	if loadErr != nil || !found {
		test.Fatalf("读取失败 found=%v err=%v", found, loadErr)
	}
	if output.Food == nil || output.Food.Level != 7 || output.Food.Saturation != 1.5 || output.Food.Exhaustion != 2.25 || output.Food.Timer != 33 {
		test.Fatalf("饥饿数据往返不一致: %+v", output.Food)
	}
}

// 没有 food 字段的老玩家存档读出来是 nil，调用方据此保持满饥饿；
// 如果读成零值结构体，老玩家一进服就会被当成饿到 0。
func TestLegacyPlayerWithoutFood(test *testing.T) {
	var dataDir string = test.TempDir()
	var err error = os.MkdirAll(PlayersDir(dataDir), 0o755)
	if err != nil {
		test.Fatal(err)
	}
	var uuid string = "cccccccc-bbbb-cccc-dddd-eeeeeeeeeeee"
	var legacy string = "{\"uuid\":\"" + uuid + "\",\"name\":\"Old\",\"world\":\"world\",\"gamemode\":0,\"inventory\":[],\"heldSlot\":0}"
	err = os.WriteFile(PlayerPath(dataDir, uuid), []byte(legacy), 0o644)
	if err != nil {
		test.Fatal(err)
	}
	var output, found, loadErr = LoadPlayer(dataDir, uuid)
	if loadErr != nil || !found {
		test.Fatalf("读取失败 found=%v err=%v", found, loadErr)
	}
	if output.Food != nil {
		test.Fatalf("老存档的 Food 必须是 nil，实际 %+v", output.Food)
	}
}
