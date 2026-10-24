package server

import "errors"
import "math"
import "testing"

import "go-mc-server/internal/command"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"

// /tp 的坐标按脚算，内部存的是眼睛高度，进出都要换算。
func TestPlayerPositionReturnsFeet(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	var posX, posY, posZ, online = fixture.admin().PlayerPosition("ste")
	if !online || !nearly(posX, 0) || !nearly(posY, 64) || !nearly(posZ, 0) {
		test.Fatalf("应返回脚下坐标 (0,64,0): %v %v %v online=%v", posX, posY, posZ, online)
	}
	var _, _, _, ghost = fixture.admin().PlayerPosition("Ghost")
	if ghost {
		test.Fatal("不在线应返回 false")
	}
}

func TestTeleportToPositionSameWorld(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	steve.RecordMovement(0, 64, 0, steve.lastMoveAt)
	var name, err = fixture.admin().TeleportToPosition("steve", 100, 70, -50)
	if err != nil || name != "Steve" {
		test.Fatalf("传送: name=%q err=%v", name, err)
	}
	var posX, posY, posZ = steve.Position()
	if !nearly(posX, 100) || !nearly(posY, 70+playerEyeHeight) || !nearly(posZ, -50) {
		test.Fatalf("位置应是 (100, 70+眼高, -50): %v %v %v", posX, posY, posZ)
	}
	if steve.movementSnapshot().Known {
		test.Fatal("传送后必须清掉移动基准，否则下一次上报会被当成瞬移拉回来")
	}
	if steve.WorldName != "world" {
		test.Fatalf("同世界传送不该换世界: %q", steve.WorldName)
	}
}

// 命令层查过一遍，桥接层是信任边界，还要再查：NaN / Inf / 越界都不能写进玩家位置。
func TestTeleportToPositionRejectsBadInput(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	var bad [][3]float32 = [][3]float32{
		{float32(math.NaN()), 64, 0},
		{0, float32(math.Inf(1)), 0},
		{99999999, 64, 0},
		{0, 9999, 0},
	}
	var index int = 0
	for index = 0; index < len(bad); index++ {
		var _, err = fixture.admin().TeleportToPosition("Steve", bad[index][0], bad[index][1], bad[index][2])
		if err == nil {
			test.Errorf("非法坐标 %v 应被拒绝", bad[index])
		}
	}
	var posX, posY, posZ = steve.Position()
	if posX != 0 || !nearly(posY, 64+playerEyeHeight) || posZ != 0 {
		test.Fatalf("被拒绝的传送不该改位置: %v %v %v", posX, posY, posZ)
	}
}

// 跨世界传送：先走换世界流程，再落到目标玩家的精确坐标，而不是目标世界的出生点。
func TestTeleportToPlayerCrossWorld(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	var alex *Player = fixture.join("Alex", "1.1.1.2", "nether")
	alex.X, alex.Y, alex.Z = 30, 80+playerEyeHeight, -20
	steve.RecordMovement(0, 64, 0, steve.lastMoveAt)
	var moved, target, err = fixture.admin().TeleportToPlayer("steve", "alex")
	if err != nil || moved != "Steve" || target != "Alex" {
		test.Fatalf("传送: %q -> %q err=%v", moved, target, err)
	}
	if steve.WorldName != "nether" || steve.Dimension != 1 {
		test.Fatalf("应进入地狱: world=%q dim=%d", steve.WorldName, steve.Dimension)
	}
	var posX, posY, posZ = steve.Position()
	if !nearly(posX, 30) || !nearly(posY, 80+playerEyeHeight) || !nearly(posZ, -20) {
		test.Fatalf("应落在 Alex 的位置而不是地狱出生点: %v %v %v", posX, posY, posZ)
	}
	if steve.movementSnapshot().Known {
		test.Fatal("跨世界传送后必须清掉移动基准")
	}
}

func TestTeleportRequiresPlayingTarget(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	var loader *Player = fixture.join("Loader", "1.1.1.2", "world")
	loader.Session.SetState(network.StateSpawning)
	var _, _, err = fixture.admin().TeleportToPlayer("Steve", "Loader")
	if !errors.Is(err, command.ErrPlayerNotFound) {
		test.Fatalf("目标还在加载，应报未找到: %v", err)
	}
	_, _, err = fixture.admin().TeleportToPlayer("Loader", "Steve")
	if !errors.Is(err, command.ErrPlayerNotFound) {
		test.Fatalf("被传送者还在加载，也应报未找到: %v", err)
	}
	if posX, _, _ := steve.Position(); posX != 0 {
		test.Fatal("失败的传送不该动任何人")
	}
}

// 生存模式被杀：死亡并掉出整个背包（新玩家快捷栏有 9 组物品）；再杀一次报已死亡。
func TestKillSurvivalDropsInventory(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	var name, err = fixture.admin().Kill("stev")
	if err != nil || name != "Steve" {
		test.Fatalf("击杀: name=%q err=%v", name, err)
	}
	if !steve.IsDead() || steve.Health() != 0 {
		test.Fatal("应已死亡且血量为 0")
	}
	if len(fixture.srv.entities) != 9 {
		test.Fatalf("应掉出 9 组物品，实际 %d", len(fixture.srv.entities))
	}
	var _, again = fixture.admin().Kill("Steve")
	if again == nil {
		test.Fatal("已经死亡的玩家再杀应报错")
	}
}

// /kill 在创造模式也要生效（普通伤害对创造模式无效），但创造模式死亡不掉物品。
func TestKillCreativeStillDiesWithoutDrops(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var steve *Player = fixture.join("Steve", "1.1.1.1", "world")
	steve.GameMode = int32(protocol.GameModeCreative)
	var _, err = fixture.admin().Kill("Steve")
	if err != nil || !steve.IsDead() {
		test.Fatalf("创造模式应能被 /kill: err=%v dead=%v", err, steve.IsDead())
	}
	if len(fixture.srv.entities) != 0 {
		test.Fatalf("创造模式死亡不该掉物品，实际 %d", len(fixture.srv.entities))
	}
}

func TestKillRequiresPlaying(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var loader *Player = fixture.join("Loader", "1.1.1.1", "world")
	loader.Session.SetState(network.StateSpawning)
	var _, err = fixture.admin().Kill("Loader")
	if !errors.Is(err, command.ErrPlayerNotFound) || loader.IsDead() {
		test.Fatalf("还在加载的玩家不能被击杀: err=%v dead=%v", err, loader.IsDead())
	}
}
