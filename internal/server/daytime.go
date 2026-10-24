package server

import "maps"
import "slices"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// 时间同步周期。1.1 的 SetTime 包没有「时间已冻结」标志，客户端收到后会自己接着走，
// 所以平时每 10 秒校一次（对齐参考端 200 tick），冻结时每秒校一次，把客户端拉回冻结的时刻。
const timeSyncTicks int = 200
const timeSyncStoppedTicks int = 20

// timeSyncDue 判断这一 tick 是否该给某个世界补发时间。
// 平时每 timeSyncTicks 发一次；冻结的世界每 timeSyncStoppedTicks 发一次。
func timeSyncDue(ticker int, stopped bool) bool {
	if ticker >= timeSyncTicks {
		return true
	}
	return stopped && ticker%timeSyncStoppedTicks == 0
}

// tickDayNight 每 tick 推进所有世界的时钟，并按周期把时间同步给客户端。
// 每个世界各走各的：空岛、地狱、末地和主世界互不影响。
func (srv *Server) tickDayNight() {
	srv.timeTicker++
	var current int = srv.timeTicker
	if current >= timeSyncTicks {
		srv.timeTicker = 0
	}
	var list []*world.World = slices.Collect(maps.Values(srv.worlds))
	var index int = 0
	for index = 0; index < len(list); index++ {
		list[index].AdvanceTime()
		if timeSyncDue(current, list[index].TimeStopped()) {
			srv.broadcastWorldTime(list[index])
		}
	}
}

// broadcastWorldTime 把世界当前时间发给该世界里的所有玩家。
func (srv *Server) broadcastWorldTime(target *world.World) {
	srv.broadcastInWorld(target.Name, &packets.SetTimePacket{Time: target.TimeTicks()}, "")
}

// sendWorldTimeTo 进服、切世界时单独给一个玩家发时间。
func (srv *Server) sendWorldTimeTo(sess *network.Session, target *world.World) {
	if sess == nil || target == nil {
		return
	}
	sess.SendPacket(&packets.SetTimePacket{Time: target.TimeTicks()})
}
