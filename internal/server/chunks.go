package server

import (
	"sort"

	"go-mc-server/internal/network"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/world"
)

const (
	chunksPerTick  = 4
	spawnChunkMin  = 16
	defaultViewRad = 4
)

func chunkChebyshev(ax, az, bx, bz int32) int32 {
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dz := az - bz
	if dz < 0 {
		dz = -dz
	}
	if dx > dz {
		return dx
	}
	return dz
}

func orderChunksNearestFirst(chunks [][2]int32, cx, cz int32) {
	sort.SliceStable(chunks, func(i, j int) bool {
		di := chunkChebyshev(chunks[i][0], chunks[i][1], cx, cz)
		dj := chunkChebyshev(chunks[j][0], chunks[j][1], cx, cz)
		if di != dj {
			return di < dj
		}
		if chunks[i][0] != chunks[j][0] {
			return chunks[i][0] < chunks[j][0]
		}
		return chunks[i][1] < chunks[j][1]
	})
}

func (p *Player) enqueueChunks(list [][2]int32) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.SentChunks == nil {
		p.SentChunks = make(map[string]struct{})
	}
	if p.chunkQueued == nil {
		p.chunkQueued = make(map[string]struct{})
	}
	for _, c := range list {
		key := chunkCoordKey(c[0], c[1])
		if _, ok := p.SentChunks[key]; ok {
			continue
		}
		if _, ok := p.chunkQueued[key]; ok {
			continue
		}
		p.chunkQueued[key] = struct{}{}
		p.ChunkQueue = append(p.ChunkQueue, c)
	}
	cx := world.BlockToChunk(p.X)
	cz := world.BlockToChunk(p.Z)
	orderChunksNearestFirst(p.ChunkQueue, cx, cz)
}

func (p *Player) resetChunkView() (rad int32, px, pz float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rad = p.ViewRadius
	if rad <= 0 {
		rad = defaultViewRad
	}
	px, pz = p.X, p.Z
	p.SentChunks = make(map[string]struct{})
	p.chunkQueued = make(map[string]struct{})
	p.ChunkQueue = nil
	p.viewEpoch++
	p.chunkInflight = 0
	p.SpawnChunksSent = 0
	p.NeedSpawnStatus = true
	return
}

func (srv *Server) enqueuePlayerView(p *Player) {
	if p == nil {
		return
	}
	rad, px, pz := p.resetChunkView()
	list := world.ChunksAroundChunk(world.BlockToChunk(px), world.BlockToChunk(pz), rad)
	p.enqueueChunks(list)
	srv.setPlayerImmobile(p, true)
}

func (srv *Server) enqueueAroundMove(p *Player) {
	if p == nil {
		return
	}
	p.mu.RLock()
	rad := p.ViewRadius
	px, pz := p.X, p.Z
	p.mu.RUnlock()
	if rad <= 0 {
		rad = defaultViewRad
	}
	p.enqueueChunks(world.ChunksAroundChunk(world.BlockToChunk(px), world.BlockToChunk(pz), rad))
}

func (srv *Server) setPlayerImmobile(p *Player, frozen bool) {
	if p == nil || p.Session == nil {
		return
	}
	flags := packets.DefaultPlayerFlags()
	if frozen {
		flags |= packets.EntityFlagImmobile
	}
	p.Session.SendPacket(&packets.SetEntityDataPacket{
		EntityRuntimeID: p.EntityRuntimeID,
		SendFlags:       true,
		Flags:           flags,
	})
}

func (srv *Server) finishChunkSpawn(p *Player) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.Lock()
	p.NeedSpawnStatus = false
	eid := p.EntityRuntimeID
	x, y, z := p.X, p.Y, p.Z
	sess := p.Session
	p.mu.Unlock()
	sess.SendPacket(&packets.MovePlayerPacket{
		EntityRuntimeID: eid,
		X:               x,
		Y:               y,
		Z:               z,
		Mode:            protocol.MovePlayerModeReset,
		OnGround:        true,
	})
	sess.SendPacket(&packets.PlayStatusPacket{Status: protocol.PlayStatusPlayerSpawn})
	sess.SetState(network.StatePlaying)
	srv.setPlayerImmobile(p, false)
	srv.sendWorldEntities(sess, srv.playerWorldName(p))
	srv.showPlayerInWorld(p)
}

func (srv *Server) tickChunkSend() {
	// 先把 worker 做好的区块包交出去，再补新的单，这样一个 tick 里腾出来的名额马上就能再用。
	srv.drainChunkResults()
	srv.mu.RLock()
	list := make([]*Player, 0, len(srv.players))
	for _, p := range srv.players {
		list = append(list, p)
	}
	srv.mu.RUnlock()
	for _, p := range list {
		srv.submitQueuedChunks(p)
	}
}
