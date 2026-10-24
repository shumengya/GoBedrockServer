package world_test

import "slices"
import "testing"

import "go-mc-server/internal/world"

// LoadedSubChunks 只列已经在内存里的区块中有数据的子区块，区块没加载返回空，也不会把它加载出来。
func TestLoadedSubChunks(test *testing.T) {
	var target *world.World = world.New("sub")
	if target.LoadedSubChunks(0, 0) != nil {
		test.Fatal("没加载的区块应返回 nil")
	}
	var _, _, loaded = target.Peek(1, 5, 1)
	if loaded {
		test.Fatal("LoadedSubChunks 不该把区块加载出来")
	}
	target.SetBlock(1, 5, 1, world.BlockStone, 0)
	var first []int = target.LoadedSubChunks(0, 0)
	if !slices.Equal(first, []int{0}) {
		test.Fatalf("平坦世界只有最底下一个子区块有数据，实际 %v", first)
	}
	target.SetBlock(1, 40, 1, world.BlockStone, 0)
	var second []int = target.LoadedSubChunks(0, 0)
	if !slices.Equal(second, []int{0, 2}) {
		test.Fatalf("y=40 在第 2 个子区块，实际 %v", second)
	}
	target.SetBlock(-1, 70, -1, world.BlockStone, 0)
	var negative []int = target.LoadedSubChunks(-1, -1)
	if !slices.Equal(negative, []int{0, 4}) {
		test.Fatalf("负坐标区块期望 [0 4]，实际 %v", negative)
	}
}
