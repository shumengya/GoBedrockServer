package server

import "go-mc-server/internal/protocol"

// materialNeed 是一种材料（id + meta）需要的总数。
type materialNeed struct {
	ID    int32
	Meta  int32
	Count int
}

// tallyMaterials 把要扣的物品按 id:meta 合并，同种材料分散在几格也只算一笔。
func tallyMaterials(consumed []protocol.Item) []materialNeed {
	var needs []materialNeed = nil
	var index int = 0
	for index = 0; index < len(consumed); index++ {
		var item protocol.Item = consumed[index]
		var merged bool = false
		var slot int = 0
		for slot = 0; slot < len(needs) && !merged; slot++ {
			if needs[slot].ID == item.ID && needs[slot].Meta == item.Meta {
				needs[slot].Count += int(item.Count)
				merged = true
			}
		}
		if !merged {
			needs = append(needs, materialNeed{ID: item.ID, Meta: item.Meta, Count: int(item.Count)})
		}
	}
	return needs
}

// removeMatchingLocked 从背包扣 count 个 id:meta（先扣手持槽），不够就一个都不扣。调用方须持有 p.mu。
func (p *Player) removeMatchingLocked(id int32, meta int32, count int) bool {
	if countMatching(p.Inventory, id, meta) < count {
		return false
	}
	var held int = p.heldInventorySlotLocked()
	var need int = takeMatching(p, held, id, meta, count)
	var index int = 0
	for index = 0; index < len(p.Inventory) && need > 0; index++ {
		if index == held {
			continue
		}
		need = takeMatching(p, index, id, meta, need)
	}
	return need == 0
}

// craft 在同一把锁里完成「验料、扣料、加产物」，中间不会被别的改动插进来。
// 材料先全部验过再动手，任何一种不够就整笔作废。返回背包放不下的产物。
// 创造模式也照常扣料：创造玩家开工作台合成，用的是自己手里实打实的东西。
func (p *Player) craft(consumed []protocol.Item, outputs []protocol.Item) ([]protocol.Item, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.combat.Dead || len(consumed) == 0 {
		return nil, false
	}
	var needs []materialNeed = tallyMaterials(consumed)
	var index int = 0
	for index = 0; index < len(needs); index++ {
		if countMatching(p.Inventory, needs[index].ID, needs[index].Meta) < needs[index].Count {
			return nil, false
		}
	}
	for index = 0; index < len(needs); index++ {
		p.removeMatchingLocked(needs[index].ID, needs[index].Meta, needs[index].Count)
	}
	var leftovers []protocol.Item = nil
	for index = 0; index < len(outputs); index++ {
		var leftover, _ = p.addItemLocked(outputs[index])
		if !leftover.IsAir() {
			leftovers = append(leftovers, leftover)
		}
	}
	return leftovers, true
}
