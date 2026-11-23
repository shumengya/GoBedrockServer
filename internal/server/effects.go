package server

import "maps"
import "slices"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/storage"

const (
	effectNausea         = 9
	effectRegen          = 10
	effectResistance     = 11
	effectFireResistance = 12
	effectHunger         = 17
	effectPoison         = 19
	effectAbsorption     = 22
)

func foodEffects(item protocol.Item, random dropRandom) []storage.EffectData {
	switch item.ID {
	case 394:
		// 本地参考树只有物品 ID；触发条件取自 PocketMine-MP 的 PoisonousPotato。
		if random.IntN(101) > 40 {
			return []storage.EffectData{{ID: effectPoison, Duration: 100}}
		}
	case 365, 367:
		var threshold int = 70
		if item.ID == 367 {
			threshold = 20
		}
		if random.IntN(101) >= threshold {
			return []storage.EffectData{{ID: effectHunger, Duration: 600}}
		}
	case 375:
		return []storage.EffectData{{ID: effectPoison, Duration: 80}}
	case itemRawFish, 462:
		if item.ID == 462 || item.Meta == fishMetaPuffer {
			return []storage.EffectData{{ID: effectHunger, Amplifier: 2, Duration: 300}, {ID: effectNausea, Amplifier: 1, Duration: 300}, {ID: effectPoison, Amplifier: 3, Duration: 1200}}
		}
	case 322:
		return []storage.EffectData{{ID: effectRegen, Amplifier: 1, Duration: 100}, {ID: effectAbsorption, Duration: 2400}}
	case 466:
		return []storage.EffectData{{ID: effectRegen, Amplifier: 4, Duration: 600}, {ID: effectResistance, Duration: 6000}, {ID: effectAbsorption, Amplifier: 3, Duration: 2400}, {ID: effectFireResistance, Duration: 6000}}
	}
	return nil
}

func (player *Player) sendEffect(event byte, effect storage.EffectData) {
	if player.Session != nil {
		player.Session.SendPacket(&packets.MobEffectPacket{EntityRuntimeID: player.EntityRuntimeID, Event: event, EffectID: effect.ID, Amplifier: effect.Amplifier, Duration: effect.Duration, Particles: event != packets.MobEffectRemove})
	}
}

func (player *Player) addEffect(effect storage.EffectData) {
	player.mu.Lock()
	var old storage.EffectData = player.combat.Effects[effect.ID]
	if player.combat.Dead || old.Amplifier > effect.Amplifier || (old.Amplifier == effect.Amplifier && old.Duration > effect.Duration) {
		player.mu.Unlock()
		return
	}
	if player.combat.Effects == nil {
		player.combat.Effects = make(map[int32]storage.EffectData)
	}
	player.combat.Effects[effect.ID] = effect
	if effect.ID == effectAbsorption {
		player.combat.Absorption = 4 * float32(effect.Amplifier+1)
	}
	player.mu.Unlock()
	var event byte = packets.MobEffectAdd
	if old.Duration > 0 {
		event = packets.MobEffectModify
	}
	player.sendEffect(event, effect)
	if effect.ID == effectAbsorption {
		player.sendAbsorption()
	}
}

func (player *Player) sendAbsorption() {
	if player.Session != nil {
		player.Session.SendPacket(&packets.UpdateAttributesPacket{EntityRuntimeID: player.EntityRuntimeID, Attributes: []packets.Attribute{packets.AbsorptionAttribute(player.vitals().Absorption)}})
	}
}

func (srv *Server) tickEffects(player *Player) {
	player.mu.Lock()
	if player.combat.Dead || len(player.combat.Effects) == 0 {
		player.mu.Unlock()
		return
	}
	var poison, regen bool = false, false
	var exhaust float32 = 0
	var expired []storage.EffectData = nil
	for _, id := range slices.Sorted(maps.Keys(player.combat.Effects)) {
		var effect storage.EffectData = player.combat.Effects[id]
		switch id {
		case effectPoison:
			poison = effect.Duration%max(int32(1), 25>>uint(effect.Amplifier)) == 0
		case effectRegen:
			regen = effect.Duration%max(int32(1), 40>>uint(effect.Amplifier)) == 0
		case effectHunger:
			exhaust = 0.025 * float32(effect.Amplifier+1)
		}
		effect.Duration--
		if effect.Duration == 0 {
			delete(player.combat.Effects, id)
			expired = append(expired, storage.EffectData{ID: id})
			if id == effectAbsorption {
				player.combat.Absorption = 0
			}
		} else {
			player.combat.Effects[id] = effect
		}
	}
	player.mu.Unlock()
	if regen && player.heal(1) {
		srv.sendHealth(player, player.Health())
	}
	if poison {
		srv.applyDamage(player, 1, DamageCausePoison, nil, true)
	}
	if exhaust > 0 {
		srv.exhaustPlayer(player, exhaust)
	}
	for _, effect := range expired {
		player.sendEffect(packets.MobEffectRemove, effect)
		if effect.ID == effectAbsorption {
			player.sendAbsorption()
		}
	}
}

func (player *Player) syncEffects(event byte) {
	player.mu.RLock()
	var effects = maps.Clone(player.combat.Effects)
	player.mu.RUnlock()
	for _, id := range slices.Sorted(maps.Keys(effects)) {
		var effect = effects[id]
		if event == packets.MobEffectRemove {
			effect = storage.EffectData{ID: id}
		}
		player.sendEffect(event, effect)
	}
}
