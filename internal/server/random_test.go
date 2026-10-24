package server

import "testing"

import "go-mc-server/internal/blockrule"

// randomTestScript 按事先排好的序列吐随机数，用完之后一直返回 0。
type randomTestScript struct {
	values []int
	next   int
}

func (script *randomTestScript) IntN(count int) int {
	var value int = 0
	if script.next < len(script.values) {
		value = script.values[script.next]
	}
	script.next++
	if value < 0 || value >= count {
		return 0
	}
	return value
}

// randomTestChunks 抽区块阶段要用掉 randomTickChunksPerPlayer 次、每次两个数(x、z 偏移)。
// offset 是偏移加上半径之后的值：4 表示偏移 0，8 表示偏移 +4。
func randomTestChunks(offset int) []int {
	var values []int = make([]int, 0, randomTickChunksPerPlayer*2)
	var sample int = 0
	for sample = 0; sample < randomTickChunksPerPlayer; sample++ {
		values = append(values, offset, offset)
	}
	return values
}

// randomTestPicks 每次抽查要用掉三个数(x、y、z)，后面跟着方块自己要的随机数。
func randomTestPicks(pick []int, times int) []int {
	var values []int = nil
	var index int = 0
	for index = 0; index < times; index++ {
		values = append(values, pick...)
	}
	return values
}

// 玩家所在的区块被抽中，抽查的位置正好是小麦，小麦长一个阶段；
// 三次抽查都落在同一格，所以从 3 长到 6。
func TestRandomTickGrowsCropNearPlayer(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	placeTestPut(test, fixture, 8, 4, 8, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockWheat, 3)
	var script []int = append(randomTestChunks(4), randomTestPicks([]int{8, 5, 8, 1}, randomTickPicksPerSubChunk)...)
	fixture.srv.tickRandomWith(&randomTestScript{values: script})
	var id, meta = placeTestBlock(fixture, 8, 5, 8)
	if id != blockrule.BlockWheat || int(meta) != 3+randomTickPicksPerSubChunk {
		test.Fatalf("期望 小麦:%d，实际 %d:%d", 3+randomTickPicksPerSubChunk, id, meta)
	}
}

// 甘蔗数满 15 之后往上接一格；接上去的那格不会被邻居更新当成没有支撑而掉下来。
func TestRandomTickGrowsSugarcaneTaller(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	placeTestPut(test, fixture, 8, 4, 8, blockrule.BlockSand, 0)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockSugarcane, 15)
	var script []int = append(randomTestChunks(4), randomTestPicks([]int{8, 5, 8}, randomTickPicksPerSubChunk)...)
	fixture.srv.tickRandomWith(&randomTestScript{values: script})
	var baseID, baseMeta = placeTestBlock(fixture, 8, 5, 8)
	var topID, topMeta = placeTestBlock(fixture, 8, 6, 8)
	if baseID != blockrule.BlockSugarcane || baseMeta != 2 {
		test.Fatalf("底下那格期望 甘蔗:2(清零后又数了两次)，实际 %d:%d", baseID, baseMeta)
	}
	if topID != blockrule.BlockSugarcane || topMeta != 0 {
		test.Fatalf("上面应接出一格 甘蔗:0，实际 %d:%d", topID, topMeta)
	}
	if updateTestDropped(fixture, 338) != 0 {
		test.Fatalf("接上去的甘蔗不该掉下来，实际掉了 %d", updateTestDropped(fixture, 338))
	}
}

// 没有玩家在线的世界不做随机刻。
func TestRandomTickSkipsWorldWithoutPlayers(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 8, 4, 8, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockWheat, 3)
	var script []int = append(randomTestChunks(4), randomTestPicks([]int{8, 5, 8, 1}, randomTickPicksPerSubChunk)...)
	fixture.srv.tickRandomWith(&randomTestScript{values: script})
	var _, meta = placeTestBlock(fixture, 8, 5, 8)
	if meta != 3 {
		test.Fatalf("没有玩家时小麦不该长，实际 meta=%d", meta)
	}
}

// 抽中的区块还没加载，直接跳过，不能为了随机刻把它生成出来。
func TestRandomTickDoesNotLoadChunks(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	placeTestPut(test, fixture, 8, 4, 8, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockWheat, 3)
	var script []int = append(randomTestChunks(8), randomTestPicks([]int{8, 5, 8, 1}, randomTickPicksPerSubChunk)...)
	fixture.srv.tickRandomWith(&randomTestScript{values: script})
	var _, _, farLoaded = fixture.srv.world.Peek(4*16+8, 5, 4*16+8)
	if farLoaded {
		test.Fatal("抽中的远处区块不该被加载出来")
	}
	var _, meta = placeTestBlock(fixture, 8, 5, 8)
	if meta != 3 {
		test.Fatalf("没抽到玩家脚下的区块，小麦不该长，实际 meta=%d", meta)
	}
}

// 死亡的玩家不算"附近有人"：他所在的区块不做随机刻。
func TestRandomTickSkipsDeadPlayers(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	var _, killErr = fixture.admin().Kill("builder")
	if killErr != nil {
		test.Fatalf("让玩家死亡失败: %v", killErr)
	}
	placeTestPut(test, fixture, 8, 4, 8, blockrule.BlockFarmland, 0)
	placeTestPut(test, fixture, 8, 5, 8, blockrule.BlockWheat, 3)
	var script []int = append(randomTestChunks(4), randomTestPicks([]int{8, 5, 8, 1}, randomTickPicksPerSubChunk)...)
	fixture.srv.tickRandomWith(&randomTestScript{values: script})
	var _, meta = placeTestBlock(fixture, 8, 5, 8)
	if meta != 3 {
		test.Fatalf("玩家已死亡，附近的小麦不该长，实际 meta=%d", meta)
	}
}
