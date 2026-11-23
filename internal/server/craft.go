package server

import "fmt"

import "go-mc-server/internal/crafting"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// craftingDataBytes 返回编码好的 CraftingData 包。全服共用一份，只编码一次。
func (srv *Server) craftingDataBytes() ([]byte, error) {
	srv.craftOnce.Do(func() {
		var book, loadErr = crafting.Default()
		if loadErr != nil {
			srv.craftErr = fmt.Errorf("加载配方: %w", loadErr)
			return
		}
		var raw, encodeErr = packets.EncodePacket(&packets.CraftingDataPacket{
			Recipes:      book.Recipes(),
			CleanRecipes: true,
		})
		if encodeErr != nil {
			srv.craftErr = fmt.Errorf("编码配方包: %w", encodeErr)
			return
		}
		srv.craftRaw = raw
	})
	return srv.craftRaw, srv.craftErr
}

// sendCraftingData 进服时把配方表发给客户端，没有它客户端的合成界面是空的。
func (srv *Server) sendCraftingData(sess *network.Session) {
	var raw, err = srv.craftingDataBytes()
	if err != nil {
		logger.Error("server", "配方表不可用，%s 无法合成: %v", sess.Username, err)
		return
	}
	sess.SendBatch(raw)
}

// workbenchUsable 报告玩家此刻是否真的在用工作台：开着工作台窗口、方块还在、人在够得着的地方。
// 这些都要服务端自己查，不能信客户端说「我在用工作台」。
func (srv *Server) workbenchUsable(player *Player) bool {
	player.mu.RLock()
	var window byte = player.OpenWindow
	var benchX int32 = player.OpenTX
	var benchY int32 = player.OpenTY
	var benchZ int32 = player.OpenTZ
	player.mu.RUnlock()
	if window != protocol.WindowWorkbench {
		return false
	}
	if blockIDAt(srv.playerWorld(player), benchX, benchY, benchZ) != world.BlockCraftingTable {
		return false
	}
	var posX, posY, posZ = player.Position()
	return blockInReach(posX, posY, posZ, blockPos{PosX: benchX, PosY: benchY, PosZ: benchZ}, maxInteractDistance)
}

// planCraft 校验一次合成请求，通过则给出配方和要扣的料；不通过返回原因。
// 产物以配方为准，客户端声称的产物只拿来识别「双方对配方理解不一致」。
func (srv *Server) planCraft(player *Player, pkt *packets.CraftingEventPacket) (crafting.Recipe, []protocol.Item, string) {
	var book, err = crafting.Default()
	if err != nil {
		return crafting.Recipe{}, nil, "配方书不可用"
	}
	var recipe, found = book.Find(pkt.RecipeID)
	if !found {
		return crafting.Recipe{}, nil, "未知配方"
	}
	if recipe.NeedsBench() && !srv.workbenchUsable(player) {
		return crafting.Recipe{}, nil, "需要工作台"
	}
	var consumed, matched = recipe.Match(pkt.Input)
	if !matched {
		return crafting.Recipe{}, nil, "材料与配方不符"
	}
	if !recipe.OutputMatches(pkt.Output) {
		return crafting.Recipe{}, nil, "产物与配方不符"
	}
	return recipe, consumed, ""
}

// handleCraftingEvent 处理客户端点了合成产物。校验、扣料、发产物都在服务端完成，
// 任何一步不通过都把真实背包推回去，客户端本地多画出来的产物就会消掉。
func (srv *Server) handleCraftingEvent(sess *network.Session, pkt *packets.CraftingEventPacket) {
	var player *Player = srv.playerBySession(sess)
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	if len(pkt.Input) == 0 {
		book, err := crafting.Default()
		if err == nil {
			if recipe, found := book.Find(pkt.RecipeID); found && player.craftFloating(book, recipe, pkt.Output, srv.workbenchUsable(player)) {
				return
			}
		}
		srv.sendPlayerInventory(player)
		return
	}
	var recipe, consumed, reason = srv.planCraft(player, pkt)
	if reason != "" {
		logger.Debug("server", "%s 合成被拒: %s", player.Username, reason)
		srv.sendPlayerInventory(player)
		return
	}
	var leftovers, crafted = player.craft(consumed, recipe.Output)
	if !crafted {
		logger.Debug("server", "%s 合成被拒: 背包里材料不足", player.Username)
		srv.sendPlayerInventory(player)
		return
	}
	srv.sendPlayerInventory(player)
	srv.broadcastHeldItem(player)
	var posX, posY, posZ = player.Position()
	var index int = 0
	for index = 0; index < len(leftovers); index++ {
		srv.spawnDroppedItem(leftovers[index], posX, posY, posZ, 0, 0.1, 0, 10, srv.playerWorldName(player))
	}
	logger.Info("server", "%s 合成 %d:%d×%d", player.Username, recipe.Output[0].ID, recipe.Output[0].Meta, recipe.Output[0].Count)
}
