package server

import "testing"
import "time"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// 出生点保护：半径内拦，半径外放行，半径为 0 表示关闭。
func TestInsideSpawnProtection(test *testing.T) {
	if !insideSpawnProtection(0, 0, 16, 3, 4) {
		test.Fatal("(3,4) 距原点 5 格，应在 16 格保护内")
	}
	if !insideSpawnProtection(0, 0, 16, 16, 0) {
		test.Fatal("正好 16 格应算在保护内（参考端是 <=）")
	}
	if insideSpawnProtection(0, 0, 16, 20, 0) {
		test.Fatal("20 格超出 16 格保护")
	}
	if insideSpawnProtection(0, 0, 0, 0, 0) {
		test.Fatal("半径 0 应关闭保护")
	}
	if insideSpawnProtection(0, 0, -1, 0, 0) {
		test.Fatal("负数半径应关闭保护")
	}
}

// 摔落换算：3 格以内不疼，超出部分向下取整。
func TestFallDamageFor(test *testing.T) {
	type fallCase struct {
		distance float32
		want     float32
	}
	var cases []fallCase = []fallCase{
		{distance: 0, want: 0},
		{distance: 3, want: 0},
		{distance: 3.9, want: 0},
		{distance: 4, want: 1},
		{distance: 4.9, want: 1},
		{distance: 10, want: 7},
		{distance: 23, want: 20},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var current fallCase = cases[index]
		var got float32 = fallDamageFor(current.distance)
		if got != current.want {
			test.Errorf("落差 %.1f 格: got %v want %v", current.distance, got, current.want)
		}
	}
}

// 客户端报的物品和服务端手里的对不上就得拒；客户端空手报空物品是正常的。
func TestHeldMismatchReason(test *testing.T) {
	var stone protocol.Item = protocol.Item{ID: 1, Count: 64}
	var dirt protocol.Item = protocol.Item{ID: 3, Count: 64}
	if heldMismatchReason(stone, stone) != "" {
		test.Fatal("同一个方块应放行")
	}
	if heldMismatchReason(stone, protocol.Air()) != "" {
		test.Fatal("客户端省略物品字段时应放行")
	}
	if heldMismatchReason(stone, dirt) == "" {
		test.Fatal("物品 id 不一致应拒")
	}
}

// 环境伤害：岩浆、水、空气的判定要分清楚。
func TestBlockHazardClassify(test *testing.T) {
	if !isLavaBlock(world.BlockStillLava) || !isLavaBlock(world.BlockFlowingLava) {
		test.Fatal("静止和流动岩浆都算岩浆")
	}
	if isLavaBlock(world.BlockStillWater) {
		test.Fatal("水不是岩浆")
	}
	if !isWaterBlock(world.BlockStillWater) || !isWaterBlock(world.BlockFlowingWater) {
		test.Fatal("静止和流动水都算水")
	}
	if isSuffocating(world.BlockAir) || isSuffocating(world.BlockStillWater) || isSuffocating(world.BlockStillLava) {
		test.Fatal("空气和液体都憋不死人")
	}
	if !isSuffocating(world.BlockStone) {
		test.Fatal("石头能憋死人")
	}
}

// 环境伤害的节拍器：按周期触发，互不干扰。
func TestDueForTick(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	var fires int = 0
	var index int = 0
	for index = 0; index < 30; index++ {
		if player.dueForTick(lifeTickFire, 10) {
			fires++
		}
	}
	if fires != 3 {
		test.Fatalf("30 tick 里每 10 tick 触发一次，应触发 3 次，实际 %d", fires)
	}
	// 另一路节拍器不该被上面跑掉的计数影响。
	if !player.dueForTick(lifeTickLava, 5) {
		test.Fatal("岩浆节拍器是独立的，第一次应立刻触发")
	}
}

// 死亡消息的翻译键要覆盖到所有伤害来源，不能有漏。
func TestDeathTranslationKeys(test *testing.T) {
	var causes []DamageCause = []DamageCause{
		DamageCauseFall,
		DamageCauseVoid,
		DamageCauseSuffocation,
		DamageCauseDrowning,
		DamageCauseLava,
		DamageCauseFire,
		DamageCauseCactus,
		DamageCausePlayer,
	}
	var index int = 0
	for index = 0; index < len(causes); index++ {
		var key string = deathTranslationKey(causes[index])
		if key == "" || key == "death.attack.generic" {
			test.Errorf("%s 没有专用死亡消息", damageLabel(causes[index]))
		}
	}
}

// 死亡掉落要把背包搬空，并把手持物一起清掉。
func TestTakeAllItemsClearsInventory(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Item{ID: itemIronSword, Count: 1})
	player.Inventory[5] = protocol.Item{ID: 1, Count: 64}
	var dumped []protocol.Item = player.takeAllItems()
	if len(dumped) != 2 {
		test.Fatalf("应掉出 2 组物品，实际 %d", len(dumped))
	}
	var index int = 0
	for index = 0; index < len(player.Inventory); index++ {
		if !player.Inventory[index].IsAir() {
			test.Fatalf("槽位 %d 没清空: %+v", index, player.Inventory[index])
		}
	}
	if !player.HeldItem.IsAir() {
		test.Fatal("手持物也要一起清掉")
	}
}

// 移动校验：瞬移要拦，正常走路和首次上报要放行。
func TestMoveRejectReason(test *testing.T) {
	var srv *Server = &Server{world: world.New("world")}
	var player *Player = newPlayer(&network.Session{}, 1)
	player.WorldName = srv.world.Name
	var start time.Time = time.Unix(5000, 0)
	if srv.moveRejectReason(player, 0, 64, 0, start) != "" {
		test.Fatal("第一次上报没有基准，应放行")
	}
	player.RecordMovement(0, 64, 0, start)
	// 走过 1 秒（20 tick），挪了 5 格，正常速度。
	var walkReason string = srv.moveRejectReason(player, 5, 64, 0, start.Add(time.Second))
	if walkReason != "" {
		test.Fatalf("正常走路被拒: %s", walkReason)
	}
	player.RecordMovement(0, 64, 0, start)
	// 同样 1 秒里挪了 200 格，是瞬移。
	if srv.moveRejectReason(player, 200, 64, 0, start.Add(time.Second)) == "" {
		test.Fatal("瞬移应被拒")
	}
	player.RecordMovement(0, 64, 0, start)
	// 垂直瞬移到天上。
	if srv.moveRejectReason(player, 0, 200, 0, start.Add(time.Second)) == "" {
		test.Fatal("飞行应被拒")
	}
}


// 拒绝提示要节流：同一条内容在间隔内只发一次，换了内容立刻放行。
func TestNoticeThrottle(test *testing.T) {
	var player *Player = newTestPlayer(protocol.Air())
	var start time.Time = time.Unix(7000, 0)
	if !player.claimNotice("太远了", start, 2*time.Second) {
		test.Fatal("第一次应该能发")
	}
	if player.claimNotice("太远了", start.Add(500*time.Millisecond), 2*time.Second) {
		test.Fatal("间隔内同一条提示不该重复发")
	}
	if !player.claimNotice("太远了", start.Add(3*time.Second), 2*time.Second) {
		test.Fatal("过了间隔应该再发一次")
	}
	if !player.claimNotice("被保护了", start.Add(3*time.Second), 2*time.Second) {
		test.Fatal("换了内容应该立刻放行，否则玩家看不到新原因")
	}
}

// 内部拒绝原因翻给玩家看：防作弊判定不能提示（等于教作弊怎么调），其余要有话说。
func TestBreakRejectMessage(test *testing.T) {
	if breakRejectMessage("挖掘时间不足") != "" {
		test.Fatal("挖掘太快不该提示，那是在教作弊者调参数")
	}
	if breakRejectMessage("未开始挖掘") != "" {
		test.Fatal("没开始挖掘是正常客户端不会触发的情况，不该提示")
	}
	var causes []string = []string{"旁观模式", "不可破坏", "距离过远", "出生点保护"}
	var index int = 0
	for index = 0; index < len(causes); index++ {
		if breakRejectMessage(causes[index]) == "" {
			test.Errorf("%s 应该给玩家一句提示", causes[index])
		}
	}
	if breakRejectMessage("不认识的原因") != "" {
		test.Fatal("未知原因不提示，免得漏出内部信息")
	}
}

// 玩家列表快照要是副本，遍历时不受并发改动影响。
func TestPlayerListSnapshot(test *testing.T) {
	var srv *Server = &Server{players: make(map[string]*Player)}
	var one *Player = newPlayer(&network.Session{}, 1)
	var two *Player = newPlayer(&network.Session{}, 2)
	srv.players["a"] = one
	srv.players["b"] = two
	var list []*Player = srv.playerList()
	if len(list) != 2 {
		test.Fatalf("应返回 2 个玩家，实际 %d", len(list))
	}
	srv.players["c"] = newPlayer(&network.Session{}, 3)
	if len(list) != 2 {
		test.Fatal("快照不该跟着 map 变")
	}
}
