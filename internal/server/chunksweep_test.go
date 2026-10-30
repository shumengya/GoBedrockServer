package server

import "testing"
import "time"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

type wantedCase struct {
	Name   string
	Radius int32
	Inside [2]int32
	Beyond [2]int32
}

// 玩家周围要留的范围 = 视距和随机刻范围里大的那个，再加 2 圈边距；没设视距按默认 4。
// 视距 2 比随机刻范围（4）小，要按 4 算：随机刻抽查的区块不能被卸掉。
func TestWantedChunksRadius(test *testing.T) {
	var cases []wantedCase = []wantedCase{
		{Name: "默认视距 4", Radius: 0, Inside: [2]int32{6, -6}, Beyond: [2]int32{7, 0}},
		{Name: "视距比随机刻范围小", Radius: 2, Inside: [2]int32{6, 6}, Beyond: [2]int32{0, -7}},
		{Name: "视距 10", Radius: 10, Inside: [2]int32{12, 0}, Beyond: [2]int32{13, 0}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item wantedCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = placeTestSetup(sub)
			player.ViewRadius = item.Radius
			var wanted map[string]map[[2]int32]bool = fixture.srv.wantedChunks()
			var set map[[2]int32]bool = wanted["world"]
			if !set[item.Inside] {
				sub.Fatalf("%v 应在要留的范围里", item.Inside)
			}
			if set[item.Beyond] {
				sub.Fatalf("%v 不该在要留的范围里", item.Beyond)
			}
			if !set[[2]int32{0, 0}] {
				sub.Fatal("玩家脚下的区块必须要留")
			}
		})
	}
}

// 不同世界的玩家各管各的世界；玩家记的世界名对不上任何世界时，按落到的默认世界算。
func TestWantedChunksPerWorld(test *testing.T) {
	var fixture, overworld = placeTestSetup(test)
	overworld.X = 160
	var nether *Player = fixture.join("nether", "10.0.0.2", "nether")
	nether.Z = -160
	var lost *Player = fixture.join("lost", "10.0.0.3", "no-such-world")
	lost.X = -320
	var wanted map[string]map[[2]int32]bool = fixture.srv.wantedChunks()
	if !wanted["world"][[2]int32{10, 0}] {
		test.Fatal("主世界玩家站在区块 (10,0)，应要留")
	}
	if wanted["world"][[2]int32{0, -10}] {
		test.Fatal("下界玩家的区块不该算到主世界头上")
	}
	if !wanted["nether"][[2]int32{0, -10}] {
		test.Fatal("下界玩家站在区块 (0,-10)，下界应要留")
	}
	if !wanted["world"][[2]int32{-20, 0}] {
		test.Fatal("世界名对不上的玩家落在默认世界，那里的区块应要留，否则他脚下的区块会被卸掉")
	}
	var _, present = wanted["no-such-world"]
	if present {
		test.Fatal("不该出现不存在的世界名")
	}
}

func sweepTestLoad(fixture *adminFixture, chunkX int32, chunkZ int32) {
	fixture.srv.world.GetBlock(chunkX*16, 1, chunkZ*16)
}

// 卸掉离玩家远的区块，留下近的；玩家附近的区块一个都不动。
func TestSweepChunksRemovesFarKeepsNear(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	sweepTestLoad(fixture, 0, 0)
	sweepTestLoad(fixture, 5, 5)
	sweepTestLoad(fixture, 50, 50)
	sweepTestLoad(fixture, -40, 3)
	var removed int = fixture.srv.sweepChunks()
	if removed != 2 {
		test.Fatalf("期望卸掉 2 个远的，实际 %d", removed)
	}
	var _, _, nearLoaded = fixture.srv.world.Peek(0, 1, 0)
	var _, _, edgeLoaded = fixture.srv.world.Peek(80, 1, 80)
	var _, _, farLoaded = fixture.srv.world.Peek(800, 1, 800)
	if !nearLoaded || !edgeLoaded || farLoaded {
		test.Fatalf("近处和边距内的该留，远处的该卸，实际 近=%v 边距内=%v 远=%v", nearLoaded, edgeLoaded, farLoaded)
	}
}

// 一个人都没有的世界，里面的区块全卸掉。
func TestSweepChunksEmptiesWorldWithoutPlayers(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	var nether *world.World = fixture.srv.worlds["nether"]
	nether.GetBlock(0, 1, 0)
	nether.GetBlock(100, 1, 100)
	var removed int = fixture.srv.sweepChunks()
	if removed != 2 || nether.LoadedChunkCount() != 0 {
		test.Fatalf("没人的世界应清空，实际 移走=%d 剩=%d", removed, nether.LoadedChunkCount())
	}
}

// 远处改过的区块要先存盘再卸，读回来改动还在；没有存储的世界里改过的区块不能卸。
func TestSweepChunksSavesDirtyFarChunk(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	fixture.srv.world.SetStore(storage.NewRegionStore(test.TempDir()))
	placeTestPut(test, fixture, 800, 10, 800, blockrule.BlockStone, 0)
	var removed int = fixture.srv.sweepChunks()
	if removed != 1 {
		test.Fatalf("期望卸掉改过的远处区块，实际 %d", removed)
	}
	var _, _, loaded = fixture.srv.world.Peek(800, 10, 800)
	if loaded {
		test.Fatal("卸掉之后不该还在内存里")
	}
	var id byte = fixture.srv.world.BlockID(800, 10, 800)
	if id != blockrule.BlockStone {
		test.Fatalf("读回来应是存盘时的石头，实际 %d", id)
	}
}

func TestSweepChunksKeepsDirtyFarChunkWithoutStore(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	placeTestPut(test, fixture, 800, 10, 800, blockrule.BlockStone, 0)
	var removed int = fixture.srv.sweepChunks()
	if removed != 0 {
		test.Fatalf("没有存储，改过的区块不能卸，实际卸了 %d 个", removed)
	}
	if fixture.srv.world.BlockID(800, 10, 800) != blockrule.BlockStone {
		test.Fatal("改动应还在")
	}
}

// 上一次还没扫完就不开新的；扫完之后能再开，而且扫完会放开占用。
func TestStartChunkSweepDoesNotOverlap(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	fixture.srv.chunkSweepBusy.Store(true)
	if fixture.srv.startChunkSweep() {
		test.Fatal("上一次还在扫，不该再开一次")
	}
	fixture.srv.chunkSweepBusy.Store(false)
	if !fixture.srv.startChunkSweep() {
		test.Fatal("空闲时应能开始")
	}
	var deadline time.Time = time.Now().Add(2 * time.Second)
	for fixture.srv.chunkSweepBusy.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if fixture.srv.chunkSweepBusy.Load() {
		test.Fatal("扫完应放开占用")
	}
}

// 掉落物判断脚下实不实，只能看内存里已有的区块：区块没加载就当实心停在原地，而且不能因此把区块加载出来，
// 否则远处一个掉落物就让卸载白做。
func TestItemSolidInDoesNotLoadChunks(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	if !fixture.srv.itemSolidIn("world", 800.5, 20, 800.5) {
		test.Fatal("区块没加载，应当实心")
	}
	if fixture.srv.world.LoadedChunkCount() != 0 {
		test.Fatalf("判断实心不该加载区块，实际加载了 %d 个", fixture.srv.world.LoadedChunkCount())
	}
	sweepTestLoad(fixture, 0, 0)
	if fixture.srv.itemSolidIn("world", 8.5, 40, 8.5) {
		test.Fatal("已加载区块里的空气不该实心")
	}
	if !fixture.srv.itemSolidIn("world", 8.5, 2, 8.5) {
		test.Fatal("已加载区块里的泥土应当实心")
	}
}
