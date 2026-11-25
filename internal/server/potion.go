package server

import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

const itemPotion int32 = 373
const itemGlassBottle int32 = 374

const (
	effectSpeed          = 1
	effectSlowness       = 2
	effectStrength       = 5
	effectHealing        = 6
	effectHarming        = 7
	effectJump           = 8
	effectWaterBreathing = 13
	effectInvisible      = 14
	effectNightVision    = 16
	effectWeakness       = 18
)

// 下标是药水物品的 meta，不是 MobEffect ID；1–4 可饮用但没有效果，0 是参考端不让喝的水瓶。
var potionKinds = [...]struct{ id, ticks, amplifier int32 }{
	5: {effectNightVision, 3600, 0}, 6: {effectNightVision, 9600, 0},
	7: {effectInvisible, 3600, 0}, 8: {effectInvisible, 9600, 0},
	9: {effectJump, 3600, 0}, 10: {effectJump, 9600, 0}, 11: {effectJump, 1800, 1},
	12: {effectFireResistance, 3600, 0}, 13: {effectFireResistance, 9600, 0},
	14: {effectSpeed, 3600, 0}, 15: {effectSpeed, 9600, 0}, 16: {effectSpeed, 1800, 1},
	17: {effectSlowness, 1800, 0}, 18: {effectSlowness, 4800, 0},
	19: {effectWaterBreathing, 3600, 0}, 20: {effectWaterBreathing, 9600, 0},
	21: {effectHealing, 1, 0}, 22: {effectHealing, 1, 1}, 23: {effectHarming, 1, 0}, 24: {effectHarming, 1, 1},
	25: {effectPoison, 900, 0}, 26: {effectPoison, 2400, 0}, 27: {effectPoison, 440, 1},
	28: {effectRegen, 900, 0}, 29: {effectRegen, 2400, 0}, 30: {effectRegen, 440, 1},
	31: {effectStrength, 3600, 0}, 32: {effectStrength, 9600, 0}, 33: {effectStrength, 1800, 1},
	34: {effectWeakness, 1800, 0}, 35: {effectWeakness, 4800, 0},
}

func (srv *Server) applyPotion(player *Player, meta int32) {
	var spec = potionKinds[meta]
	if spec.id != 0 && player.addEffect(storage.EffectData{ID: spec.id, Duration: spec.ticks, Amplifier: spec.amplifier}) {
		srv.syncPotionChange(player, spec.id)
	}
}

func (player *Player) effectLevel(id int32) int32 {
	player.mu.RLock()
	defer player.mu.RUnlock()
	var effect = player.combat.Effects[id]
	if effect.Duration <= 0 {
		return 0
	}
	return effect.Amplifier + 1
}

func (player *Player) potionSpeed() float32 {
	return 0.1 * (1 + 0.2*float32(player.effectLevel(effectSpeed))) * max(0, 1-0.15*float32(player.effectLevel(effectSlowness)))
}

func (srv *Server) syncPotionChange(player *Player, id int32) {
	switch id {
	case effectHealthBoost:
		if player.Session != nil {
			var vitals = player.vitals()
			player.Session.SendPacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.HealthAttribute(vitals)}})
		}
	case effectSpeed, effectSlowness:
		if player.Session != nil {
			player.Session.SendPacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.MovementAttribute(player.potionSpeed())}})
		}
	case effectInvisible:
		var flags = packets.PlayerFlags(player.effectLevel(effectInvisible) > 0)
		srv.broadcastInWorld(srv.playerWorldName(player), &packets.SetEntityDataPacket{EntityRuntimeID: player.EntityRuntimeID, SendFlags: true, Flags: flags}, "")
	}
}
