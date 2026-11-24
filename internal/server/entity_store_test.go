package server

import "os"
import "testing"

import "go-mc-server/config"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"

// 写盘后重新构造整台服务器：跨世界的同种生物不串档，名字、姿态及旧格式掉落物均能恢复。
func TestSummonedEntityRestart(test *testing.T) {
	var cfg = config.Default()
	cfg.DataDir, cfg.ServerDir = test.TempDir(), test.TempDir()
	var first = New(cfg)
	var kind, _ = entity.ByID(11)
	first.addSummonedEntity(&entity.Instance{Type: kind, WorldName: first.world.Name, CustomName: "小白", X: 3.5, Y: 65, Z: -4.5, Yaw: 76, Pitch: -12})
	var err = first.spawnEntity(11, 9.5, 55, 4, "nether")
	if err != nil {
		test.Fatal(err)
	}
	first.spawnDroppedItem(protocol.Item{ID: 5, Meta: 2, Count: 4}, 1, 62, 0, 0, 0, 0, 10, first.world.Name)
	var names = [...]string{first.world.Name, "nether"}
	var index = 0
	for index = 0; index < len(names); index++ {
		if err = first.persistWorld(first.worlds[names[index]]); err != nil {
			test.Fatal(err)
		}
	}
	err = os.WriteFile(storage.EntitiesPath(cfg.DataDir, "end"), []byte(`[{"id":35,"meta":3,"count":2,"y":42,"age":17,"pickupDelay":8}]`), 0644)
	if err != nil {
		test.Fatal(err)
	}
	var restart = New(cfg)
	var seen = make(map[string]int)
	var runtimeID int64 = 0
	for runtimeID = 1; runtimeID <= restart.entityIDCounter; runtimeID++ {
		var restored = restart.entities[runtimeID]
		if restored == nil {
			test.Fatalf("期望实体有唯一运行时 ID，实际缺少 %d", runtimeID)
		}
		if restored.IsDroppedItem() {
			if restored.WorldName == "end" && restored.Item.ID == 35 && restored.Item.Meta == 3 && restored.Item.Count == 2 && restored.Age == 17 && restored.PickupDelay == 8 && restored.Y == 42 {
				seen["旧掉落物"]++
			}
			if restored.WorldName == first.world.Name && restored.Item.ID == 5 && restored.Item.Meta == 2 && restored.Item.Count == 4 {
				seen["新掉落物"]++
			}
		} else if restored.Type.NetworkID == 11 {
			if restored.WorldName == first.world.Name && restored.CustomName == "小白" && restored.X == 3.5 && restored.Y == 65 && restored.Z == -4.5 && restored.Yaw == 76 && restored.Pitch == -12 {
				seen["主世界牛"]++
			}
			if restored.WorldName == "nether" && restored.CustomName == "" && restored.X == 9.5 && restored.Y == 55 && restored.Z == 4 {
				seen["下界牛"]++
			}
		}
	}
	if len(restart.entities) != 4 || len(seen) != 4 {
		test.Fatalf("期望完整恢复两头牛和两份掉落物，实际 %v，共 %d 个实体", seen, len(restart.entities))
	}
	err = restart.spawnEntity(32, 0, 65, 0, first.world.Name)
	if err != nil || len(restart.entities) != 5 {
		test.Fatalf("期望新实体不覆盖读档实体，实际 %v / %d", err, len(restart.entities))
	}
}
