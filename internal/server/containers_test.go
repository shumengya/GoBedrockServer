package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// containerTestOpened 在 (3,64,0) 放一个方块并打开它，返回夹具和玩家。玩家站在原点，离它 3 格多，够得着。
func containerTestOpened(test *testing.T, blockID byte) (*adminFixture, *Player) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockID, 0)
	fixture.srv.openContainer(player, 3, 64, 0)
	return fixture, player
}

func containerTestSlot(fixture *adminFixture, slot int) protocol.Item {
	var tile, _ = fixture.srv.world.TileAt(3, 64, 0)
	return tile.Items[slot]
}

func containerTestWrite(fixture *adminFixture, player *Player, windowID byte, slot int32, item protocol.Item) {
	// 这些用例专测窗口、位置和槽位规则；先给光标实际的物品，避免被来源校验提前拒绝。
	player.floating.put(item)
	fixture.srv.handleContainerWindowSlot(player, &packets.ContainerSetSlotPacket{WindowID: windowID, Slot: slot, Item: item})
}

// 打开箱子：方块实体当场建好，27 格；玩家记下开的是哪个窗口、哪个位置。
func TestOpenChestCreatesTileAndWindow(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found || tile.ID != world.TileChest || len(tile.Items) != 27 {
		test.Fatalf("应建好 27 格的箱子，实际 找到=%v 种类=%q 格数=%d", found, tile.ID, len(tile.Items))
	}
	if player.OpenWindow != protocol.WindowContainer || player.OpenWindowType != protocol.InventoryTypeContainer {
		test.Fatalf("窗口应是容器/箱子类型，实际 %d/%d", player.OpenWindow, player.OpenWindowType)
	}
	if player.OpenTX != 3 || player.OpenTY != 64 || player.OpenTZ != 0 {
		test.Fatalf("应记下 (3,64,0)，实际 (%d,%d,%d)", player.OpenTX, player.OpenTY, player.OpenTZ)
	}
}

// 熔炉用熔炉窗口类型，只有 3 格；烧着的熔炉也一样。
func TestOpenFurnaceUsesFurnaceWindow(test *testing.T) {
	var ids []byte = []byte{blockrule.BlockFurnace, blockrule.BlockBurningFurnace}
	var index int = 0
	for index = 0; index < len(ids); index++ {
		var fixture, player = containerTestOpened(test, ids[index])
		var tile, found = fixture.srv.world.TileAt(3, 64, 0)
		if !found || tile.ID != world.TileFurnace || len(tile.Items) != 3 {
			test.Fatalf("方块 %d 应建好 3 格的熔炉，实际 找到=%v 种类=%q 格数=%d", ids[index], found, tile.ID, len(tile.Items))
		}
		if player.OpenWindowType != protocol.InventoryTypeFurnace {
			test.Fatalf("方块 %d 应是熔炉窗口类型，实际 %d", ids[index], player.OpenWindowType)
		}
	}
}

// 已经有方块实体的容器，打开时用已有的，里面的东西不能被清掉。
func TestOpenContainerKeepsExistingContents(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockChest, 0)
	var chest world.Tile = world.NewChestTile(3, 64, 0)
	chest.Items[4] = protocol.Item{ID: 264, Count: 7}
	fixture.srv.world.SetTile(chest)
	fixture.srv.openContainer(player, 3, 64, 0)
	if containerTestSlot(fixture, 4).ID != 264 || containerTestSlot(fixture, 4).Count != 7 {
		test.Fatalf("已有内容应保留，实际 %+v", containerTestSlot(fixture, 4))
	}
}

// 窗口开着、人够得着，对容器的改动写进方块实体。
func TestContainerSlotWriteAccepted(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	containerTestWrite(fixture, player, protocol.WindowContainer, 2, protocol.Item{ID: 1, Count: 9})
	if containerTestSlot(fixture, 2).ID != 1 || containerTestSlot(fixture, 2).Count != 9 {
		test.Fatalf("应写进第 2 格，实际 %+v", containerTestSlot(fixture, 2))
	}
}

type containerRejectCase struct {
	Name   string
	Spoil  func(fixture *adminFixture, player *Player)
	Window byte
	Slot   int32
}

// 这些情况下对容器的改动一律不收：窗口没开、窗口号不对、槽位越界、人走远了、方块被换掉了、方块实体没了。
func TestContainerSlotWriteRejected(test *testing.T) {
	var cases []containerRejectCase = []containerRejectCase{
		{
			Name:   "窗口已经关了",
			Spoil:  func(fixture *adminFixture, player *Player) { player.OpenWindow = 0 },
			Window: protocol.WindowContainer,
			Slot:   2,
		},
		{
			Name:   "窗口号不是容器窗口",
			Spoil:  func(fixture *adminFixture, player *Player) {},
			Window: protocol.WindowWorkbench,
			Slot:   2,
		},
		{
			Name:   "槽位越界",
			Spoil:  func(fixture *adminFixture, player *Player) {},
			Window: protocol.WindowContainer,
			Slot:   27,
		},
		{
			Name:   "槽位是负数",
			Spoil:  func(fixture *adminFixture, player *Player) {},
			Window: protocol.WindowContainer,
			Slot:   -1,
		},
		{
			Name: "人走远了",
			Spoil: func(fixture *adminFixture, player *Player) {
				player.UpdatePosition(100, 64+playerEyeHeight, 100, 0, 0, true)
			},
			Window: protocol.WindowContainer,
			Slot:   2,
		},
		{
			Name: "方块被换成了石头",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.world.SetBlock(3, 64, 0, blockrule.BlockStone, 0)
			},
			Window: protocol.WindowContainer,
			Slot:   2,
		},
		{
			Name: "方块实体没了",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.world.RemoveTile(3, 64, 0)
			},
			Window: protocol.WindowContainer,
			Slot:   2,
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item containerRejectCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = containerTestOpened(sub, blockrule.BlockChest)
			item.Spoil(fixture, player)
			containerTestWrite(fixture, player, item.Window, item.Slot, protocol.Item{ID: 1, Count: 9})
			var tile, found = fixture.srv.world.TileAt(3, 64, 0)
			if !found {
				return
			}
			var slot int = 0
			for slot = 0; slot < len(tile.Items); slot++ {
				if !tile.Items[slot].IsAir() {
					sub.Fatalf("改动不该写进去，第 %d 格出现了 %+v", slot, tile.Items[slot])
				}
			}
		})
	}
}

// 拆掉箱子：里面的东西掉出来，正开着它的玩家窗口被关掉。创造模式拆箱子本身不掉东西，只看里面的。
func TestBreakChestDropsContentsAndClosesWindow(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	updateTestCreative(player)
	fixture.srv.world.SetTileSlot(3, 64, 0, 0, protocol.Item{ID: 280, Count: 5})
	fixture.srv.world.SetTileSlot(3, 64, 0, 26, protocol.Item{ID: 280, Count: 2})
	updateTestBreak(fixture, player, 3, 64, 0)
	if updateTestDropped(fixture, 280) != 7 {
		test.Fatalf("应掉出 7 根木棍，实际 %d", updateTestDropped(fixture, 280))
	}
	if player.OpenWindow != 0 || player.OpenWindowType != 0 {
		test.Fatalf("窗口应被关掉，实际 %d/%d", player.OpenWindow, player.OpenWindowType)
	}
	var _, found = fixture.srv.world.TileAt(3, 64, 0)
	if found {
		test.Fatal("方块实体应随方块一起删掉")
	}
}

// 熔炉里的东西也一样掉出来，不止箱子。
func TestBreakFurnaceDropsContents(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockFurnace)
	updateTestCreative(player)
	fixture.srv.world.SetTileSlot(3, 64, 0, 1, protocol.Item{ID: 263, Count: 8})
	updateTestBreak(fixture, player, 3, 64, 0)
	if updateTestDropped(fixture, 263) != 8 {
		test.Fatalf("应掉出 8 个煤，实际 %d", updateTestDropped(fixture, 263))
	}
}

// 只关别人开的那个窗口：另一个玩家开着别的箱子，拆这个箱子不影响他。
func TestBreakChestLeavesOtherViewersAlone(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockChest)
	updateTestCreative(player)
	var other *Player = fixture.join("other", "10.0.0.2", "world")
	placeTestPut(test, fixture, -3, 64, 0, blockrule.BlockChest, 0)
	fixture.srv.openContainer(other, -3, 64, 0)
	updateTestBreak(fixture, player, 3, 64, 0)
	if other.OpenWindow != protocol.WindowContainer || other.OpenTX != -3 {
		test.Fatalf("别人开着的另一个箱子不该被关，实际 窗口=%d X=%d", other.OpenWindow, other.OpenTX)
	}
}

// 整条链路：手里拿着方块点箱子，是打开箱子，不是往箱子旁边放方块；潜行也一样。
func TestClickChestOpensItEvenWithBlockInHand(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockChest, 0)
	var held protocol.Item = placeTestHold(player, 1, 0, 64)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, held)
	if player.OpenWindow != protocol.WindowContainer || player.OpenTX != 3 {
		test.Fatalf("应打开箱子，实际 窗口=%d X=%d", player.OpenWindow, player.OpenTX)
	}
	var sideID, _ = placeTestBlock(fixture, 4, 64, 0)
	if sideID != blockrule.BlockAir || placeTestCount(player) != 64 {
		test.Fatalf("不该往旁边放方块，实际 旁边=%d 剩余=%d", sideID, placeTestCount(player))
	}
}
