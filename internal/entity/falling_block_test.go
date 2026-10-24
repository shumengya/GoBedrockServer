package entity_test

import "testing"

import "go-mc-server/internal/entity"

// solidUpTo 返回一块"第 topCell 格及以下全是实心"的地面。
func solidUpTo(topCell int32) func(int32) bool {
	return func(cellY int32) bool {
		return cellY <= topCell
	}
}

// 从高处落到地面：落在地面顶面的整数高度上，速度清零，标记着地。
func TestStepFallingBlockLandsOnFloor(test *testing.T) {
	var block *entity.Instance = &entity.Instance{Y: 64}
	var landed bool = false
	var steps int = 0
	for steps = 0; steps < 200 && !landed; steps++ {
		landed = entity.StepFallingBlock(block, solidUpTo(59))
	}
	if !landed {
		test.Fatalf("200 步内应该落地，实际停在 Y=%v", block.Y)
	}
	if block.Y != 60 || block.SpeedY != 0 || !block.OnGround {
		test.Fatalf("期望 Y=60 速度 0 着地，实际 Y=%v 速度=%v 着地=%v", block.Y, block.SpeedY, block.OnGround)
	}
}

// 一步掉得比一格还多时，不能穿过薄薄的一层地面：只有第 50 格是实心（顶面在 51），
// 从 52.5 以接近终端速度往下，一步落到 50.56，会越过 52 这个没有地面的面，再到 51 才碰到地面，
// 必须停在 51；只看起点那一层就会漏掉它，直接掉到 50.56 穿过去。
func TestStepFallingBlockDoesNotTunnelThinFloor(test *testing.T) {
	var block *entity.Instance = &entity.Instance{Y: 52.5, SpeedY: -1.9}
	var thinFloor func(int32) bool = func(cellY int32) bool {
		return cellY == 50
	}
	var landed bool = entity.StepFallingBlock(block, thinFloor)
	if !landed || block.Y != 51 {
		test.Fatalf("期望落在 Y=51，实际 落地=%v Y=%v", landed, block.Y)
	}
}

// 下面什么都没有就一直往下掉，每一步都比上一步更低。
func TestStepFallingBlockKeepsFallingThroughHole(test *testing.T) {
	var block *entity.Instance = &entity.Instance{Y: 64}
	var nothing func(int32) bool = func(cellY int32) bool {
		return false
	}
	var step int = 0
	for step = 0; step < 10; step++ {
		var previous float32 = block.Y
		var landed bool = entity.StepFallingBlock(block, nothing)
		if landed {
			test.Fatalf("第 %d 步不该落地", step)
		}
		if block.Y >= previous {
			test.Fatalf("第 %d 步应该更低，实际 %v -> %v", step, previous, block.Y)
		}
		if block.OnGround {
			test.Fatalf("第 %d 步不该着地", step)
		}
	}
}

// 本来就压在地面上的方块，第一步就判落地，不会陷进去。
func TestStepFallingBlockRestingLandsImmediately(test *testing.T) {
	var block *entity.Instance = &entity.Instance{Y: 60}
	var landed bool = entity.StepFallingBlock(block, solidUpTo(59))
	if !landed || block.Y != 60 {
		test.Fatalf("期望立刻落地在 Y=60，实际 落地=%v Y=%v", landed, block.Y)
	}
}

type fallingBlockCase struct {
	Name     string
	Instance *entity.Instance
	Expect   bool
}

// 只有网络 ID 是下落方块、并且带了方块 ID 的才算下落中的方块。
func TestIsFallingBlock(test *testing.T) {
	var missing *entity.Instance = nil
	var cases []fallingBlockCase = []fallingBlockCase{
		{Name: "空指针", Instance: missing, Expect: false},
		{Name: "没有类型", Instance: &entity.Instance{BlockID: 12}, Expect: false},
		{
			Name:     "下落方块带沙子",
			Instance: &entity.Instance{Type: &entity.Type{NetworkID: entity.NetworkIDFallingSand}, BlockID: 12},
			Expect:   true,
		},
		{
			Name:     "下落方块没带方块(比如 /summon 出来的空壳)",
			Instance: &entity.Instance{Type: &entity.Type{NetworkID: entity.NetworkIDFallingSand}},
			Expect:   false,
		},
		{
			Name:     "别的实体不管有没有方块 ID",
			Instance: &entity.Instance{Type: &entity.Type{NetworkID: entity.NetworkIDItem}, BlockID: 12},
			Expect:   false,
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item fallingBlockCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			if item.Instance.IsFallingBlock() != item.Expect {
				sub.Fatalf("期望 %v，实际 %v", item.Expect, item.Instance.IsFallingBlock())
			}
		})
	}
}
