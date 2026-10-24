package world

import "sync"

// 一天的 tick 数与各时段的起点，数值对齐参考端 Level::TIME_*。
const TimeDay int32 = 1000
const TimeNoon int32 = 6000
const TimeSunset int32 = 12000
const TimeNight int32 = 13000
const TimeMidnight int32 = 18000
const TimeSunrise int32 = 23000
const TimeFull int32 = 24000

// tickWrapDays 是累计 tick 回绕的天数。回绕点是整天的倍数，所以不影响当天几点；
// 80000 天 × 24000 = 1.92e9，刚好留在 int32 里，SetTime 包不会溢出。
const tickWrapDays int64 = 80000

// clock 是世界时钟。推进在 tick 协程，/time 在命令协程，所以必须加锁。
type clock struct {
	mu      sync.Mutex
	ticks   int32
	stopped bool
}

// normalizeTicks 把任意累计值收回合法区间，负数也折成正的。
func normalizeTicks(total int64) int32 {
	var wrap int64 = int64(TimeFull) * tickWrapDays
	var folded int64 = total % wrap
	if folded < 0 {
		folded += wrap
	}
	return int32(folded)
}

// TimeTicks 返回累计 tick。
func (w *World) TimeTicks() int32 {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	return w.clock.ticks
}

// DayTime 返回当天的第几 tick（0–23999）。
func (w *World) DayTime() int32 {
	return w.TimeTicks() % TimeFull
}

// SetTimeTicks 直接设定累计 tick，存档读取和 /time set 用。
func (w *World) SetTimeTicks(ticks int32) {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	w.clock.ticks = normalizeTicks(int64(ticks))
}

// AddTimeTicks 在当前时间上加（或减）delta。
func (w *World) AddTimeTicks(delta int32) {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	w.clock.ticks = normalizeTicks(int64(w.clock.ticks) + int64(delta))
}

// TimeStopped 报告时间是否被 /time stop 冻住。
func (w *World) TimeStopped() bool {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	return w.clock.stopped
}

func (w *World) SetTimeStopped(stopped bool) {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	w.clock.stopped = stopped
}

// AdvanceTime 走一个 tick；冻住时什么都不做。
func (w *World) AdvanceTime() {
	w.clock.mu.Lock()
	defer w.clock.mu.Unlock()
	if w.clock.stopped {
		return
	}
	w.clock.ticks = normalizeTicks(int64(w.clock.ticks) + 1)
}
