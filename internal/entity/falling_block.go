package entity

import "math"

// NetworkIDFallingSand 是下落方块实体的网络 ID，沙子、沙砾落下时都用它。
const NetworkIDFallingSand int32 = 66

const FallingGravity float32 = 0.04
const FallingDrag float32 = 0.02

// FallingBaseOffset 参考端同步下落方块的位置时，Y 要加上这个偏移（出生包不加）。
const FallingBaseOffset float32 = 0.49

// FallingVoidY 掉到这个高度以下就当作掉进虚空，直接消失。
const FallingVoidY float32 = -16

// IsFallingBlock 是不是正在下落的方块。BlockID 为 0 的（比如 /summon 出来的空壳）不算。
func (e *Instance) IsFallingBlock() bool {
	return e != nil && e.Type != nil && e.Type.NetworkID == NetworkIDFallingSand && e.BlockID != 0
}

// StepFallingBlock 让下落方块走一个 tick，返回这一步有没有落地。
// 重力、移动、阻力的顺序照参考端。方块只在自己所在的方块列里直下，solidCell 判断这一列里
// 第 cellY 格是不是实心。每 tick 最多掉两格左右，所以要把这一步经过的每个落脚面都查一遍，
// 不能只看终点，否则会穿过薄薄的一层地面。
func StepFallingBlock(e *Instance, solidCell func(cellY int32) bool) bool {
	const epsilon float32 = 0.0001
	e.SpeedY -= FallingGravity
	var nextY float32 = e.Y + e.SpeedY
	// 落脚面是整数高度 surface，它是第 surface-1 格的顶面。
	var startSurface int32 = int32(math.Floor(float64(e.Y + epsilon)))
	var surface int32 = 0
	for surface = startSurface; float32(surface) >= nextY-epsilon; surface-- {
		if solidCell(surface - 1) {
			e.Y = float32(surface)
			e.SpeedY = 0
			e.OnGround = true
			return true
		}
	}
	e.Y = nextY
	e.OnGround = false
	e.SpeedY *= 1 - FallingDrag
	return false
}
