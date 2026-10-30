package storage

import "os"
import "strings"
import "testing"

import "go-mc-server/internal/protocol"

// 末影箱随玩家存档往返：放在哪一格、是什么、几个，都要原样回来。
func TestPlayerEnderChestRoundtrip(test *testing.T) {
	var dataDir string = test.TempDir()
	var enderItems []protocol.Item = make([]protocol.Item, protocol.ChestSize)
	enderItems[0] = protocol.Item{ID: 264, Count: 3}
	enderItems[26] = protocol.Item{ID: 35, Meta: 14, Count: 64}
	var input PlayerData = PlayerData{
		UUID:       "dddddddd-bbbb-cccc-dddd-eeeeeeeeeeee",
		Name:       "Steve",
		EnderChest: SlotsFromItems(enderItems),
	}
	var saveErr error = SavePlayer(dataDir, input)
	if saveErr != nil {
		test.Fatal(saveErr)
	}
	var output, found, loadErr = LoadPlayer(dataDir, input.UUID)
	if loadErr != nil || !found {
		test.Fatalf("读取失败 found=%v err=%v", found, loadErr)
	}
	var restored []protocol.Item = ItemsFromSlots(output.EnderChest, protocol.ChestSize)
	if restored[0].ID != 264 || restored[0].Count != 3 {
		test.Fatalf("第 0 格期望 264x3，实际 %+v", restored[0])
	}
	if restored[26].ID != 35 || restored[26].Meta != 14 || restored[26].Count != 64 {
		test.Fatalf("第 26 格期望 35:14x64，实际 %+v", restored[26])
	}
}

// 没用过末影箱的玩家不写这个字段，没有 enderChest 的老存档读出来也是空的，不会读成 27 个脏格子。
func TestPlayerWithoutEnderChest(test *testing.T) {
	var dataDir string = test.TempDir()
	var fresh PlayerData = PlayerData{UUID: "eeeeeeee-bbbb-cccc-dddd-eeeeeeeeeeee", Name: "Fresh"}
	var saveErr error = SavePlayer(dataDir, fresh)
	if saveErr != nil {
		test.Fatal(saveErr)
	}
	var raw, readErr = os.ReadFile(PlayerPath(dataDir, fresh.UUID))
	if readErr != nil {
		test.Fatal(readErr)
	}
	if strings.Contains(string(raw), "enderChest") {
		test.Fatalf("没用过末影箱不该写这个字段: %s", string(raw))
	}
	var uuid string = "ffffffff-bbbb-cccc-dddd-eeeeeeeeeeee"
	var legacy string = "{\"uuid\":\"" + uuid + "\",\"name\":\"Old\",\"world\":\"world\",\"gamemode\":0,\"inventory\":[],\"heldSlot\":0}"
	var writeErr error = os.WriteFile(PlayerPath(dataDir, uuid), []byte(legacy), 0o644)
	if writeErr != nil {
		test.Fatal(writeErr)
	}
	var output, found, loadErr = LoadPlayer(dataDir, uuid)
	if loadErr != nil || !found {
		test.Fatalf("读取失败 found=%v err=%v", found, loadErr)
	}
	if len(output.EnderChest) != 0 {
		test.Fatalf("老存档的末影箱应为空，实际 %d 格", len(output.EnderChest))
	}
}
