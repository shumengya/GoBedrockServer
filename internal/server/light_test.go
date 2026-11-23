package server

import "testing"
import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/world"

// 用真实世界亮度驱动随机刻：午夜树苗不长、火把使它成熟；冰雪只受方块光影响。
func TestLightRandomTicks(test *testing.T) {
	var level = world.New("world")
	var view = peekView{target: level}
	var pos = blockrule.Pos{PosX: 4, PosY: 63, PosZ: 4}
	var random = defaultDropRandom{}
	level.SetBlock(4, 63, 4, blockrule.BlockIce, 0)
	level.SetTimeTicks(world.TimeNoon)
	if len(blockrule.OnRandomTick(view, pos, random).Changes) != 0 {
		test.Fatal("期望正午阳光不融冰，实际融化")
	}
	level.SetBlock(5, 63, 4, blockrule.BlockTorch, 0)
	var melt = blockrule.OnRandomTick(view, pos, random)
	if len(melt.Changes) != 1 || melt.Changes[0].ID != blockrule.BlockFlowingWater {
		test.Fatalf("期望火把融冰成水，实际 %+v", melt)
	}
	level.SetBlock(4, 63, 4, blockrule.BlockSnowLayer, 0)
	melt = blockrule.OnRandomTick(view, pos, random)
	if len(melt.Changes) != 1 || melt.Changes[0].ID != 0 {
		test.Fatalf("期望高方块光融雪，实际 %+v", melt)
	}
	level.SetBlock(4, 63, 4, blockrule.BlockGrass, 0)
	level.SetBlock(4, 64, 4, blockrule.BlockStone, 0)
	var dead = blockrule.OnRandomTick(view, pos, random)
	if len(dead.Changes) != 1 || dead.Changes[0].ID != blockrule.BlockDirt {
		test.Fatalf("期望草被石头盖住退化，实际 %+v", dead)
	}
	level.SetBlock(4, 64, 4, 0, 0)
	level.SetBlock(4, 63, 4, blockrule.BlockSapling, 0)
	level.SetTimeTicks(world.TimeMidnight)
	var grow = blockrule.OnRandomTick(view, pos, zeroLightRoll{})
	if len(grow.Changes) != 1 || grow.Changes[0].Meta != 8 {
		test.Fatalf("期望火把旁树苗能成熟，实际 %+v", grow)
	}
	level.SetBlock(5, 63, 4, 0, 0)
	if len(blockrule.OnRandomTick(view, pos, zeroLightRoll{}).Changes) != 0 {
		test.Fatal("期望暗处树苗不长，实际成熟")
	}
}

type zeroLightRoll struct{}

func (zeroLightRoll) IntN(int) int { return 0 }
