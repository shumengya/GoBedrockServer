package server

import (
	"bytes"
	"slices"

	"go-mc-server/internal/crafting"
	"go-mc-server/internal/item"
	"go-mc-server/internal/protocol"
)

// 桌面客户端的光标和合成格没有独立槽位包，参考端用 36 格悬浮背包记录已取出但未放回的物品。
type floatingInventory []protocol.Item

func sameStack(left, right protocol.Item) bool {
	return left.ID == right.ID && left.Meta == right.Meta && bytes.Equal(left.NBT, right.NBT)
}

func (slots *floatingInventory) put(stack protocol.Item) bool {
	if stack.IsAir() {
		return true
	}
	var limit = item.StackLimit(stack)
	for index := range *slots {
		var dest = &(*slots)[index]
		if !sameStack(*dest, stack) || dest.Count >= limit {
			continue
		}
		var moved = min(limit-dest.Count, stack.Count)
		dest.Count += moved
		stack.Count -= moved
		if stack.Count == 0 {
			return true
		}
	}
	for stack.Count > 0 && len(*slots) < protocol.PlayerInventorySize {
		var part = stack
		part.Count = min(limit, stack.Count)
		*slots = append(*slots, part)
		stack.Count -= part.Count
	}
	return stack.Count == 0
}

// 在候选副本上扣除，失败时调用方放弃整份副本；普通移槽必须连同 NBT 完全匹配。
func (slots *floatingInventory) take(stack protocol.Item, ingredient bool) bool {
	if stack.IsAir() {
		return true
	}
	for index := 0; index < len(*slots); {
		var source = &(*slots)[index]
		var matches = sameStack(*source, stack)
		if ingredient {
			matches = source.ID == stack.ID && (source.Meta == stack.Meta || stack.Meta == crafting.MetaAny) &&
				(len(stack.NBT) == 0 || bytes.Equal(stack.NBT, source.NBT))
		}
		if !matches {
			index++
			continue
		}
		var moved = min(source.Count, stack.Count)
		stack.Count -= moved
		source.Count -= moved
		if source.Count == 0 {
			*slots = slices.Delete(*slots, index, index+1)
		} else {
			index++
		}
		if stack.Count == 0 {
			return true
		}
	}
	return false
}

func (player *Player) exchangeSlotLocked(old, next protocol.Item) bool {
	if player.combat.Dead || player.GameMode == protocol.GameModeSpectator {
		return false
	}
	if sameStack(old, next) {
		var common = min(old.Count, next.Count)
		old.Count -= common
		next.Count -= common
	}
	var candidate = slices.Clone(player.floating)
	if !candidate.take(next, false) && player.GameMode != protocol.GameModeCreative {
		return false
	}
	if !candidate.put(old) {
		return false
	}
	player.floating = candidate
	return true
}

func (player *Player) returnFloatingLocked() []protocol.Item {
	var overflow []protocol.Item
	for _, stack := range player.floating {
		if leftover, _ := player.addItemLocked(stack); !leftover.IsAir() {
			overflow = append(overflow, leftover)
		}
	}
	player.floating = nil
	return overflow
}

func (srv *Server) returnFloating(player *Player) {
	player.mu.Lock()
	if len(player.floating) == 0 {
		player.mu.Unlock()
		return
	}
	var excess = player.returnFloatingLocked()
	var posX, posY, posZ = player.X, player.Y, player.Z
	player.mu.Unlock()
	if player.isVisible() {
		srv.sendPlayerInventory(player)
		srv.broadcastHeldItem(player)
	}
	for _, stack := range excess {
		srv.spawnDroppedItem(stack, posX, posY, posZ, 0, 0.1, 0, 10, srv.playerWorldName(player))
	}
}

func (player *Player) payForDrop(stack protocol.Item) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.combat.Dead || player.GameMode == protocol.GameModeSpectator {
		return false
	}
	if player.GameMode == protocol.GameModeCreative {
		return true
	}
	var candidate = slices.Clone(player.floating)
	if candidate.take(stack, false) {
		player.floating = candidate
		return true
	}
	var total int
	for _, entry := range player.Inventory {
		if sameStack(entry, stack) {
			total += int(entry.Count)
		}
	}
	if total < int(stack.Count) {
		return false
	}
	var held = player.heldInventorySlotLocked()
	for offset := -1; offset < len(player.Inventory) && stack.Count > 0; offset++ {
		var slot = offset
		if offset == -1 {
			slot = held
		} else if slot == held {
			continue
		}
		if slot < 0 || slot >= len(player.Inventory) {
			continue
		}
		var entry = &player.Inventory[slot]
		if !sameStack(*entry, stack) {
			continue
		}
		var moved = min(entry.Count, stack.Count)
		entry.Count -= moved
		stack.Count -= moved
		if entry.Count == 0 {
			*entry = protocol.Air()
		}
		if slot == held {
			player.HeldItem = *entry
		}
	}
	return true
}

func (player *Player) craftFloating(book *crafting.Book, offered crafting.Recipe, output []protocol.Item, bench bool) bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.combat.Dead || player.GameMode == protocol.GameModeSpectator {
		return false
	}
	var tryRecipe = func(recipe crafting.Recipe) bool {
		if !recipe.Craftable() || (recipe.NeedsBench() && !bench) || !recipe.OutputMatches(output) {
			return false
		}
		var candidate = slices.Clone(player.floating)
		// 先扣指定 meta 的材料，再用剩余材料满足通配项。
		for pass := 0; pass < 2; pass++ {
			for _, need := range recipe.Input {
				if (need.Meta == crafting.MetaAny) != (pass == 1) {
					continue
				}
				if !candidate.take(need, true) {
					return false
				}
			}
		}
		for _, product := range recipe.Output {
			if !candidate.put(product) {
				return false
			}
		}
		player.floating = candidate
		return true
	}
	if tryRecipe(offered) {
		return true
	}
	for _, recipe := range book.Recipes() {
		if recipe.ID != offered.ID && tryRecipe(recipe) {
			return true
		}
	}
	return false
}
