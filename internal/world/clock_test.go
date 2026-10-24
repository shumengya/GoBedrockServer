package world

import "testing"

// 时间每 tick 走一格，冻住后原地不动。
func TestClockAdvanceAndStop(test *testing.T) {
	var target *World = New("clock")
	var index int = 0
	for index = 0; index < 100; index++ {
		target.AdvanceTime()
	}
	if target.TimeTicks() != 100 {
		test.Fatalf("走了 100 tick，应为 100，实际 %d", target.TimeTicks())
	}
	target.SetTimeStopped(true)
	target.AdvanceTime()
	if target.TimeTicks() != 100 {
		test.Fatal("冻住后时间不该再走")
	}
	target.SetTimeStopped(false)
	target.AdvanceTime()
	if target.TimeTicks() != 101 {
		test.Fatal("解冻后应继续走")
	}
}

// DayTime 只看当天第几 tick，跨天要回绕。
func TestClockDayTime(test *testing.T) {
	var target *World = New("clock")
	target.SetTimeTicks(TimeFull*3 + TimeNoon)
	if target.DayTime() != TimeNoon {
		test.Fatalf("第 4 天正午应为 %d，实际 %d", TimeNoon, target.DayTime())
	}
}

// /time add 允许减，减过头要折回合法区间而不是变成负数。
func TestClockAddNegativeWraps(test *testing.T) {
	var target *World = New("clock")
	target.SetTimeTicks(100)
	target.AddTimeTicks(-300)
	if target.TimeTicks() < 0 {
		test.Fatalf("时间不该是负数: %d", target.TimeTicks())
	}
	if target.DayTime() != TimeFull-200 {
		test.Fatalf("100-300 应折成当天 %d，实际 %d", TimeFull-200, target.DayTime())
	}
}

// 回绕点是整天的倍数，所以回绕前后"当天几点"不变，也不会溢出 int32。
func TestClockWrapKeepsDayTime(test *testing.T) {
	var target *World = New("clock")
	target.SetTimeTicks(TimeNoon)
	var wrap int64 = int64(TimeFull) * tickWrapDays
	if normalizeTicks(wrap+int64(TimeNoon)) != TimeNoon {
		test.Fatal("回绕后当天时间必须保持不变")
	}
	if normalizeTicks(wrap-1) != int32(wrap-1) {
		test.Fatal("回绕点前一格应原样保留")
	}
	target.SetTimeTicks(int32(wrap - 1))
	target.AdvanceTime()
	if target.TimeTicks() != 0 {
		test.Fatalf("走到回绕点应归零，实际 %d", target.TimeTicks())
	}
}
