package server

import "cmp"
import "maps"
import "slices"
import "strings"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// scheduledBudget 是每个 tick 最多处理多少个到期的定时任务，剩下的顺延到下一个 tick。
const scheduledBudget int = 256

// scheduledKey 是定时任务的身份：哪个世界的哪一格。同一格同时只会有一个任务在排队。
type scheduledKey struct {
	World string
	Pos   blockrule.Pos
}

// scheduleLocked 排一个 delay 个 tick 之后的任务。这一格已经排过队的话保留更早的到期时间，
// 所以反复点击按钮不会把弹回时间越推越晚。调用方必须已经持有 blockMu。
func (srv *Server) scheduleLocked(worldName string, pos blockrule.Pos, delay int) {
	if srv.scheduled == nil {
		srv.scheduled = map[scheduledKey]int64{}
	}
	var key scheduledKey = scheduledKey{World: worldName, Pos: pos}
	var due int64 = srv.blockTick + int64(max(delay, 1))
	var existing, queued = srv.scheduled[key]
	if queued && existing <= due {
		return
	}
	srv.scheduled[key] = due
}

// compareScheduled 先按到期时间排，再按世界和坐标排，让同一个 tick 里的处理顺序是确定的。
func (srv *Server) compareScheduled(left scheduledKey, right scheduledKey) int {
	return cmp.Or(
		cmp.Compare(srv.scheduled[left], srv.scheduled[right]),
		strings.Compare(left.World, right.World),
		cmp.Compare(left.Pos.PosX, right.Pos.PosX),
		cmp.Compare(left.Pos.PosY, right.Pos.PosY),
		cmp.Compare(left.Pos.PosZ, right.Pos.PosZ),
	)
}

// tickScheduled 每个 tick 调用一次：推进计数，处理到期的任务。
func (srv *Server) tickScheduled() {
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	srv.blockTick++
	if len(srv.scheduled) == 0 {
		return
	}
	var keys []scheduledKey = slices.SortedFunc(maps.Keys(srv.scheduled), srv.compareScheduled)
	var processed int = 0
	var index int = 0
	for index = 0; index < len(keys) && processed < scheduledBudget; index++ {
		var key scheduledKey = keys[index]
		// 已经按到期时间排好序，遇到还没到期的，后面的也都没到期。
		if srv.scheduled[key] > srv.blockTick {
			break
		}
		delete(srv.scheduled, key)
		processed++
		var blockWorld *world.World = srv.worldByName(key.World)
		var update blockrule.Update = blockrule.OnScheduled(peekView{target: blockWorld}, key.Pos)
		srv.applyUpdateLocked(key.World, blockWorld, update)
	}
}

// applyUpdateLocked 落地一次规则评估的结果：先写改动，再排定时任务，
// 最后让真正变了的格子周围重新评估。调用方必须已经持有 blockMu。
func (srv *Server) applyUpdateLocked(worldName string, blockWorld *world.World, update blockrule.Update) {
	var origins []blockrule.Pos = make([]blockrule.Pos, 0, len(update.Changes))
	var index int = 0
	for index = 0; index < len(update.Changes); index++ {
		var edit blockrule.Change = update.Changes[index]
		if srv.applyChangeLocked(worldName, blockWorld, edit) && !edit.Quiet {
			origins = append(origins, edit.Pos)
		}
	}
	for index = 0; index < len(update.Schedules); index++ {
		var request blockrule.Schedule = update.Schedules[index]
		srv.scheduleLocked(worldName, request.Pos, request.Delay)
	}
	srv.settleBlocksLocked(worldName, blockWorld, origins)
}

// interactBlock 处理右键点到可交互的方块（门、活板门、栅栏门、拉杆、按钮）。
// 返回 true 表示这次点击被方块消耗了，不再走放置流程。
// 潜行时不触发交互，这样玩家能蹲着往门、拉杆旁边放方块。
func (srv *Server) interactBlock(player *Player, pkt *packets.UseItemPacket) bool {
	if player.isSneaking() {
		return false
	}
	srv.blockMu.Lock()
	defer srv.blockMu.Unlock()
	var blockWorld *world.World = srv.playerWorld(player)
	var worldName string = srv.playerWorldName(player)
	var yaw, _ = player.Look()
	var target blockrule.Pos = blockrule.Pos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	var update, handled = blockrule.Interact(peekView{target: blockWorld}, target, yaw)
	if !handled {
		return false
	}
	if player.gameMode() == int32(protocol.GameModeSpectator) {
		return true
	}
	if srv.spawnProtected(player, pkt.X, pkt.Z) {
		srv.notifyBlocked(player, "§c这里在出生点保护范围内，不能操作方块")
		return true
	}
	srv.applyUpdateLocked(worldName, blockWorld, update)
	return true
}
