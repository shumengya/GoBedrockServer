package server

import "math"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

func (srv *Server) teleportAfterChorusFruit(player *Player, source dropRandom) {
	var target *world.World = srv.playerWorld(player)
	var posX, eyeY, posZ = player.Position()
	var feetY float32 = eyeY - playerEyeHeight
	var baseX int32 = int32(math.Floor(float64(posX)))
	var baseY int32 = min(256, int32(math.Floor(float64(feetY))))
	var baseZ int32 = int32(math.Floor(float64(posZ)))
	var attempt int = 0
	for attempt = 0; attempt < 16; attempt++ {
		var cellX int32 = baseX + int32(randRange(source, -8, 8))
		var cellY int32 = min(255, baseY+int32(randRange(source, -8, 8)))
		var cellZ int32 = baseZ + int32(randRange(source, -8, 8))
		for cellY >= 0 && !blockrule.IsSolid(target.BlockID(cellX, cellY, cellZ)) {
			cellY--
		}
		// 候选点向下找支撑；脚和头两格都得留空，不能落进水或岩浆。
		if cellY < 0 || cellY > 253 {
			continue
		}
		var clear = func(height int32) bool {
			var id byte = target.BlockID(cellX, height, cellZ)
			return !blockrule.IsSolid(id) && !isWaterBlock(id) && !isLavaBlock(id)
		}
		if !clear(cellY+1) || !clear(cellY+2) {
			continue
		}
		var sound = func(soundX, soundY, soundZ float32) {
			srv.broadcastInWorld(target.Name, &packets.LevelEventPacket{EventID: packets.LevelEventSoundEndermanTeleport, X: soundX, Y: soundY, Z: soundZ}, "")
		}
		sound(posX, feetY, posZ)
		posX, feetY, posZ = float32(cellX)+0.5, float32(cellY+1), float32(cellZ)+0.5
		srv.teleportPlayer(player, target.Name, posX, feetY+playerEyeHeight, posZ)
		sound(posX, feetY, posZ)
		return
	}
}
