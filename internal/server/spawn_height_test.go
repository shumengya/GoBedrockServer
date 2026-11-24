package server

import (
	"testing"

	"go-mc-server/config"
	"go-mc-server/internal/network"
)

// 首次登录、跨世界和死亡重生都按眼高发位置；旧海洋岛的出生方块本身就是地板。
func TestSpawnFeetOnLanding(test *testing.T) {
	var cfg = config.Default()
	cfg.DataDir, cfg.ServerDir = test.TempDir(), test.TempDir()
	var srv = New(cfg)
	var session = &network.Session{ClientUUID: "00000000-0000-0000-0000-000000000001", Username: "Steve"}
	srv.spawnPlayer(session)
	var player = srv.players[session.ClientUUID]
	var _, eye, _ = player.Position()
	if !nearly(eye-playerEyeHeight, 5) {
		test.Fatalf("期望首次出生脚底 Y=5，实际 %f", eye-playerEyeHeight)
	}
	if !srv.transferPlayer(player, "ocean") {
		test.Fatal("期望进入旧海洋岛，实际传送失败")
	}
	_, eye, _ = player.Position()
	if !nearly(eye-playerEyeHeight, 65) {
		test.Fatalf("期望站在旧岛 Y=64 方块上方，实际脚底 %f", eye-playerEyeHeight)
	}
	if !srv.transferPlayer(player, "nether") {
		test.Fatal("期望进入新下界，实际传送失败")
	}
	var landing = float32(srv.worlds["nether"].SpawnY)
	_, eye, _ = player.Position()
	if !nearly(eye-playerEyeHeight, landing) {
		test.Fatalf("期望换维度脚底 %f，实际 %f", landing, eye-playerEyeHeight)
	}
	player.combat.Dead = true
	player.UpdatePosition(0, 100, 0, 0, 0, false)
	srv.respawnPlayer(player)
	_, eye, _ = player.Position()
	if player.IsDead() || !nearly(eye-playerEyeHeight, landing) {
		test.Fatalf("期望复活落在 %f，实际 %f，死亡状态 %t", landing, eye-playerEyeHeight, player.IsDead())
	}
}
