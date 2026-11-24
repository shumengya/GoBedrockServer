package server

import (
	"go-mc-server/internal/entity"
	"go-mc-server/internal/network"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/world"
)

func startGameGenerator(w *world.World) int32 {
	if w == nil {
		return 2
	}
	switch w.Generator {
	case world.GeneratorNormal, world.GeneratorSky, world.GeneratorOcean, world.GeneratorNether, world.GeneratorEnd, world.GeneratorNetherTerrain, world.GeneratorEndTerrain:
		return 1
	default:
		return 2
	}
}

func (srv *Server) worldByName(name string) *world.World {
	if name != "" {
		if w, ok := srv.worlds[name]; ok {
			return w
		}
	}
	return srv.world
}

func (srv *Server) playerWorld(p *Player) *world.World {
	if p == nil {
		return srv.world
	}
	p.mu.RLock()
	name := p.WorldName
	p.mu.RUnlock()
	return srv.worldByName(name)
}

func (srv *Server) playerWorldName(p *Player) string {
	if p == nil {
		return srv.world.Name
	}
	p.mu.RLock()
	name := p.WorldName
	p.mu.RUnlock()
	if name == "" {
		return srv.world.Name
	}
	return name
}

func sameWorldName(a, b, defaultName string) bool {
	if a == "" {
		a = defaultName
	}
	if b == "" {
		b = defaultName
	}
	return a == b
}

func (srv *Server) broadcastInWorld(worldName string, pkt packets.Packet, skipUUID string) {
	if worldName == "" {
		worldName = srv.world.Name
	}
	raw, err := packets.EncodePacket(pkt)
	if err != nil {
		return
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, player := range srv.players {
		if skipUUID != "" && player.UUID == skipUUID {
			continue
		}
		player.mu.RLock()
		pn := player.WorldName
		player.mu.RUnlock()
		if !sameWorldName(pn, worldName, srv.world.Name) {
			continue
		}
		player.Session.SendBatch(raw)
	}
}

func (srv *Server) despawnWorldEntities(sess *network.Session, worldName string) {
	if sess == nil {
		return
	}
	srv.mu.RLock()
	list := make([]int64, 0, len(srv.entities))
	for _, e := range srv.entities {
		if sameWorldName(e.WorldName, worldName, srv.world.Name) {
			list = append(list, e.RuntimeID)
		}
	}
	srv.mu.RUnlock()
	for _, id := range list {
		sess.SendPacket(&packets.RemoveEntityPacket{EntityUniqueID: id})
	}
}

func (srv *Server) sendWorldEntities(sess *network.Session, worldName string) {
	if sess == nil {
		return
	}
	srv.mu.RLock()
	list := make([]*entity.Instance, 0, len(srv.entities))
	for _, e := range srv.entities {
		if sameWorldName(e.WorldName, worldName, srv.world.Name) {
			list = append(list, e)
		}
	}
	srv.mu.RUnlock()
	for _, e := range list {
		sess.SendPacket(entitySpawnPacket(e))
	}
}

func (srv *Server) resendView(p *Player) {
	srv.enqueuePlayerView(p)
}

func (srv *Server) transferPlayer(p *Player, destName string) bool {
	if p == nil {
		return false
	}
	dest, ok := srv.worlds[destName]
	if !ok || dest == nil {
		return false
	}
	if srv.playerWorldName(p) == dest.Name {
		return false
	}
	// 必须在改 WorldName 之前，才能按旧世界找到要互删的玩家；
	// 新世界的 AddPlayer 等区块加载完由 finishChunkSpawn 发。
	srv.hidePlayerInWorld(p, true)
	p.mu.Lock()
	if p.WorldName == dest.Name {
		p.mu.Unlock()
		return false
	}
	oldName := p.WorldName
	oldDim := srv.worldByName(oldName).ProtocolDimension()
	// 世界名一改，还在 worker 手里的旧世界区块包就不能再发了，趁这把锁就把视野代数加一，
	// 不等到后面重置视野：中间隔着发 ChangeDimension，旧区块包插在后面客户端会收到错世界的区块。
	p.viewEpoch++
	p.WorldName = dest.Name
	p.Dimension = dest.ProtocolDimension()
	p.X = float32(dest.SpawnX) + 0.5
	p.Y = float32(dest.SpawnY) + 1
	p.Z = float32(dest.SpawnZ) + 0.5
	sess := p.Session
	eid := p.EntityRuntimeID
	x, y, z := p.X, p.Y, p.Z
	newDim := p.Dimension
	p.mu.Unlock()
	// 跨世界的位移有几百格，不清掉移动基准会被校验当成瞬移回拉。
	p.ResetMovement()

	srv.despawnWorldEntities(sess, oldName)
	if sess != nil {
		if oldDim != newDim {
			sess.SendPacket(&packets.ChangeDimensionPacket{
				Dimension: newDim,
				X:         x,
				Y:         y,
				Z:         z,
			})
		} else {
			sess.SendPacket(&packets.MovePlayerPacket{
				EntityRuntimeID: eid,
				X:               x,
				Y:               y,
				Z:               z,
				Mode:            protocol.MovePlayerModeTeleport,
				OnGround:        true,
			})
		}
		sess.SendPacket(&packets.SetSpawnPositionPacket{X: dest.SpawnX, Y: dest.SpawnY, Z: dest.SpawnZ})
		srv.sendWorldTimeTo(sess, dest)
	}
	srv.enqueuePlayerView(p)
	return true
}

func (c *commandPlayer) TransferWorld(name string) bool {
	return c.srv.transferPlayer(c.p, name)
}
