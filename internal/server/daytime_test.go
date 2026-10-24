package server

import "testing"

import "go-mc-server/internal/world"

// 平时每 200 tick 同步一次；冻结的世界每 20 tick 一次，把客户端拉回冻结的时刻。
func TestTimeSyncDue(test *testing.T) {
	if timeSyncDue(1, false) || timeSyncDue(199, false) {
		test.Fatal("运行中的世界，200 tick 之前不该同步")
	}
	if !timeSyncDue(200, false) {
		test.Fatal("到 200 tick 应同步")
	}
	if timeSyncDue(19, true) || !timeSyncDue(20, true) || !timeSyncDue(40, true) {
		test.Fatal("冻结的世界每 20 tick 同步一次")
	}
	if timeSyncDue(20, false) {
		test.Fatal("没冻结的世界不需要每秒同步")
	}
}

// 每个世界各走各的时钟，冻结的那个不动。
func TestTickDayNightAdvancesPerWorld(test *testing.T) {
	var overworld *world.World = world.New("world")
	var nether *world.World = world.New("nether")
	nether.SetTimeStopped(true)
	var srv *Server = &Server{
		world:   overworld,
		worlds:  map[string]*world.World{"world": overworld, "nether": nether},
		players: nil,
	}
	var index int = 0
	for index = 0; index < 50; index++ {
		srv.tickDayNight()
	}
	if overworld.TimeTicks() != 50 {
		test.Fatalf("主世界应走 50 tick，实际 %d", overworld.TimeTicks())
	}
	if nether.TimeTicks() != 0 {
		test.Fatalf("冻结的地狱不该走，实际 %d", nether.TimeTicks())
	}
}

// 同步计数器到 200 归零，之后继续循环，不会越攒越大。
func TestTickDayNightResetsSyncTicker(test *testing.T) {
	var overworld *world.World = world.New("world")
	var srv *Server = &Server{world: overworld, worlds: map[string]*world.World{"world": overworld}}
	var index int = 0
	for index = 0; index < timeSyncTicks; index++ {
		srv.tickDayNight()
	}
	if srv.timeTicker != 0 {
		test.Fatalf("走满一个周期计数器应归零，实际 %d", srv.timeTicker)
	}
	srv.tickDayNight()
	if srv.timeTicker != 1 {
		test.Fatalf("归零后应重新计数，实际 %d", srv.timeTicker)
	}
}
