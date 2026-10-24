package world_test

import "testing"

import "go-mc-server/internal/world"

// Peek 只读内存里已有的区块，读没加载的区块不能把它生成出来，否则一次放方块就会连带生成一大片。
func TestPeekDoesNotLoadChunk(test *testing.T) {
	var target *world.World = world.New("peek")
	var _, _, loaded = target.Peek(100, 70, 100)
	if loaded {
		test.Fatal("没加载过的区块应返回 loaded=false")
	}
	var _, _, again = target.Peek(100, 70, 100)
	if again {
		test.Fatal("Peek 之后区块仍应未加载，说明 Peek 把它生成出来了")
	}
}

// 区块被写入后 Peek 能读到方块和 meta，负坐标要落到正确的区块和区块内位置。
func TestPeekReadsLoadedChunk(test *testing.T) {
	var target *world.World = world.New("peek")
	if !target.SetBlock(100, 70, 100, world.BlockStone, 3) {
		test.Fatal("写入方块失败")
	}
	var id, data, loaded = target.Peek(100, 70, 100)
	if !loaded || id != world.BlockStone || data != 3 {
		test.Fatalf("期望 (石头,3,已加载)，实际 (%d,%d,%v)", id, data, loaded)
	}
	if !target.SetBlock(-1, 70, -1, world.BlockDirt, 5) {
		test.Fatal("写入负坐标方块失败")
	}
	var negID, negData, negLoaded = target.Peek(-1, 70, -1)
	if !negLoaded || negID != world.BlockDirt || negData != 5 {
		test.Fatalf("负坐标期望 (泥土,5,已加载)，实际 (%d,%d,%v)", negID, negData, negLoaded)
	}
}

// 区块加载后，没放过方块的位置读到的是空气，且仍算已加载。
func TestPeekEmptyCellInLoadedChunk(test *testing.T) {
	var target *world.World = world.New("peek")
	target.SetBlock(0, 70, 0, world.BlockStone, 0)
	var id, data, loaded = target.Peek(1, 200, 1)
	if !loaded || id != world.BlockAir || data != 0 {
		test.Fatalf("期望 (空气,0,已加载)，实际 (%d,%d,%v)", id, data, loaded)
	}
}

// 超出 0~255 的位置按已加载的空气处理，即便那里的区块根本没加载。
func TestPeekOutsideHeightIsLoadedAir(test *testing.T) {
	var target *world.World = world.New("peek")
	var below, belowData, belowLoaded = target.Peek(500, -1, 500)
	if !belowLoaded || below != world.BlockAir || belowData != 0 {
		test.Fatalf("y=-1 期望 (空气,0,已加载)，实际 (%d,%d,%v)", below, belowData, belowLoaded)
	}
	var above, aboveData, aboveLoaded = target.Peek(500, 256, 500)
	if !aboveLoaded || above != world.BlockAir || aboveData != 0 {
		test.Fatalf("y=256 期望 (空气,0,已加载)，实际 (%d,%d,%v)", above, aboveData, aboveLoaded)
	}
}
