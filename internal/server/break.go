package server

import "math"

import "go-mc-server/internal/protocol"

// breakTimeTicks 是客户端裂纹动画用的 tick 数（START_BREAK 的 65535/ticks）。
// 能采集时 硬度×1.5，不能采集时 硬度×5（1.1 客户端实测如此；按 PocketMine 的 ×1.5
// 算会让空手挖圆石的裂纹比实际早约 7 秒走完），再除以工具速度。
func breakTimeTicks(blockID byte, itemID, haste, fatigue int32) int32 {
	var info blockInfo = miningInfoFor(blockID)
	if blockUnbreakable(info) {
		return 0
	}
	var tool toolInfo = toolInfoFor(protocol.Item{ID: itemID, Count: 1})
	var speed float64 = 1
	if tool.Kind == toolSword {
		speed = tool.Speed
	}
	if info.PreferredTool != toolNone && tool.Kind == info.PreferredTool {
		speed = tool.Speed
	}
	if info.PreferredTool == toolShears && tool.Kind == toolShears {
		speed = 15
	}
	speed *= (1 + 0.2*float64(haste)) * math.Pow(1.2, float64(haste)) * math.Pow(0.21, float64(fatigue))
	var seconds float64 = info.Hardness * 1.5 / speed
	if !canHarvest(info, tool) {
		seconds = info.Hardness * 5 / speed
	}
	// 减一个极小量：0.8×1.5 这类浮点结果是 24.000000000000004，直接 Ceil 会多算一 tick
	// 命令允许疲劳等级 256；极慢的挖掘时间不能溢出为一刻。
	var ticks int32 = int32(min(float64(math.MaxInt32), math.Ceil(seconds*20-1e-9)))
	return max(ticks, 1)
}
