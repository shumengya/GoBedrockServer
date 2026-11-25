package server

import "os"
import "testing"

import "go-mc-server/config"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"

// 区块加载前，重启恢复的物品既不老化也不能被拾取；加载后继续倒计时，永久延迟保持原值。
func TestDropWaitsForLoadedColumn(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var srv = fixture.srv
	srv.restoreEntities("world", []storage.EntityData{
		{ID: 264, Count: 1, X: -0.25, Y: 65, Age: 1999, PickupDelay: 2, SpeedY: 0.25, OnGround: true},
		{ID: 4, Count: 1, X: -0.25, Y: 65, PickupDelay: entity.InfinitePickupDelay},
		{ID: 1, Count: 1, X: -0.25, Y: 65},
	})
	// 玩家和物品足够近，但处于相邻的两列。
	srv.world.GetBlock(0, 65, 0)
	srv.tickDroppedItems()
	var first = srv.entities[1]
	if first == nil || first.Age != 1999 || first.PickupDelay != 2 || first.Y != 65 || first.SpeedY != 0.25 || !first.OnGround || len(srv.entities) != 3 || srv.world.LoadedChunkCount() != 1 {
		test.Fatalf("期望未加载列的三份物品原样保留，实际 %+v / %d 个实体 / %d 列", first, len(srv.entities), srv.world.LoadedChunkCount())
	}
	player.X = 32
	srv.world.GetBlock(-1, 65, 0)
	srv.tickDroppedItems()
	if first.Age != 2000 || first.PickupDelay != 1 || first.Y <= 65 || srv.entities[2].PickupDelay != 32767 {
		test.Fatalf("期望加载后恢复物理和计时、永久延迟不变，实际 %+v / %d", first, srv.entities[2].PickupDelay)
	}
	srv.tickDroppedItems()
	if srv.entities[1] != nil {
		test.Fatalf("期望过期物品消失，实际 %+v", srv.entities[1])
	}
}

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
