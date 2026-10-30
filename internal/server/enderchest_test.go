package server

import "bytes"
import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/storage"
import "go-mc-server/internal/world"

// enderTestPlace 玩家站在原点、面朝 yaw，在 (posX,64,posZ) 放一个末影箱，脚下垫一块石头当支撑。
func enderTestPlace(test *testing.T, fixture *adminFixture, player *Player, posX int32, posZ int32, yaw float32) {
	player.UpdatePosition(0, 64+playerEyeHeight, 0, yaw, 0, true)
	placeTestPut(test, fixture, posX, 63, posZ, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 130, 0, 64)
	placeTestUse(fixture, player, posX, 63, posZ, blockrule.FaceUp, 1, held)
}

func enderTestFilled(items []protocol.Item) int {
	var total int = 0
	var index int = 0
	for index = 0; index < len(items); index++ {
		if !items[index].IsAir() {
			total++
		}
	}
	return total
}

// 放下末影箱：朝向照玩家偏航，建出没有物品栏的方块实体，并把它发给客户端（客户端靠它画出箱子）。
func TestPlaceEnderChestCreatesTile(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var log *sentLog = captureSent(player)
	enderTestPlace(test, fixture, player, 3, 0, chestYawWest)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != blockrule.BlockEnderChest || meta != 5 {
		test.Fatalf("落点期望 末影箱:5，实际 %d:%d", id, meta)
	}
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found || tile.ID != world.TileEnderChest || len(tile.Items) != 0 {
		test.Fatalf("应建出没有物品栏的末影箱方块实体，实际 找到=%v %+v", found, tile)
	}
	var sent, wasSent = log.lastBlockEntityAt(test, 3, 64, 0)
	if !wasSent || !bytes.Equal(sent, nbt.SpawnEnderChest(3, 64, 0)) {
		test.Fatalf("应把末影箱的方块实体发给客户端，实际 发过=%v %x", wasSent, sent)
	}
}

// 同一个末影箱，每个玩家打开看到的是他自己的东西，不是方块里的，也不是别人的。
func TestEnderChestShowsEachPlayersOwnItems(test *testing.T) {
	var fixture, alice = placeTestSetup(test)
	var bob *Player = fixture.join("bob", "10.0.0.2", "world")
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockEnderChest, 5)
	alice.setEnderChestSlot(2, protocol.Item{ID: 264, Count: 5})
	bob.setEnderChestSlot(2, protocol.Item{ID: 265, Count: 1})
	var aliceLog *sentLog = captureSent(alice)
	var bobLog *sentLog = captureSent(bob)
	fixture.srv.openContainer(alice, 3, 64, 0)
	fixture.srv.openContainer(bob, 3, 64, 0)
	var aliceSlots, aliceSent = aliceLog.lastContainerContent(test, protocol.WindowContainer)
	var bobSlots, bobSent = bobLog.lastContainerContent(test, protocol.WindowContainer)
	if !aliceSent || !bobSent {
		test.Fatalf("两个人都应收到窗口内容，实际 alice=%v bob=%v", aliceSent, bobSent)
	}
	if len(aliceSlots) != 27 || len(bobSlots) != 27 {
		test.Fatalf("末影箱窗口应是 27 格，实际 alice=%d bob=%d", len(aliceSlots), len(bobSlots))
	}
	if aliceSlots[2].ID != 264 || aliceSlots[2].Count != 5 {
		test.Fatalf("alice 第 2 格期望 264x5，实际 %+v", aliceSlots[2])
	}
	if bobSlots[2].ID != 265 || bobSlots[2].Count != 1 {
		test.Fatalf("bob 第 2 格期望 265x1，实际 %+v", bobSlots[2])
	}
	if fixture.srv.world.BlockID(3, 64, 0) != blockrule.BlockEnderChest {
		test.Fatal("夹具：方块应还是末影箱")
	}
	if alice.OpenWindowType != protocol.InventoryTypeContainer {
		test.Fatalf("末影箱用普通容器窗口类型，实际 %d", alice.OpenWindowType)
	}
}

// 往末影箱窗口里写，写进的是玩家自己的末影箱：方块实体不存东西，别的玩家不受影响，越界的不写。
func TestEnderChestSlotWriteGoesToPlayerNotTile(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockEnderChest)
	var other *Player = fixture.join("other", "10.0.0.2", "world")
	containerTestWriteAt(fixture, player, 4, protocol.Item{ID: 264, Count: 6})
	containerTestWriteAt(fixture, player, 27, protocol.Item{ID: 1, Count: 1})
	containerTestWriteAt(fixture, player, -1, protocol.Item{ID: 2, Count: 1})
	var mine []protocol.Item = player.enderChestItems()
	if mine[4].ID != 264 || mine[4].Count != 6 {
		test.Fatalf("第 4 格期望 264x6，实际 %+v", mine[4])
	}
	if enderTestFilled(mine) != 1 {
		test.Fatalf("越界的写入不该落进任何一格，期望只有 1 格有东西，实际 %d 格", enderTestFilled(mine))
	}
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found || len(tile.Items) != 0 {
		test.Fatalf("方块实体不该存东西，实际 找到=%v 格数=%d", found, len(tile.Items))
	}
	if enderTestFilled(other.enderChestItems()) != 0 {
		test.Fatal("别的玩家的末影箱不该被动")
	}
}

// 窗口没开、窗口号不对、人走远、方块换了、方块实体没了，对末影箱的改动一律不收。
func TestEnderChestSlotWriteRejected(test *testing.T) {
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
			var fixture, player = containerTestOpened(sub, blockrule.BlockEnderChest)
			item.Spoil(fixture, player)
			containerTestWrite(fixture, player, item.Window, item.Slot, protocol.Item{ID: 1, Count: 9})
			if enderTestFilled(player.enderChestItems()) != 0 {
				sub.Fatalf("改动不该写进去，实际 %+v", player.enderChestItems())
			}
		})
	}
}

// 末影箱的东西跟着玩家走，不跟着方块：在一个末影箱里放的东西，打开另一个末影箱也在。
func TestEnderChestContentsFollowPlayerAcrossBlocks(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockEnderChest, 5)
	placeTestPut(test, fixture, -3, 64, 0, blockrule.BlockEnderChest, 5)
	fixture.srv.openContainer(player, 3, 64, 0)
	containerTestWriteAt(fixture, player, 7, protocol.Item{ID: 388, Count: 2})
	var log *sentLog = captureSent(player)
	fixture.srv.openContainer(player, -3, 64, 0)
	var slots, sent = log.lastContainerContent(test, protocol.WindowContainer)
	if !sent || len(slots) != 27 {
		test.Fatalf("打开第二个末影箱应收到 27 格内容，实际 发过=%v 格数=%d", sent, len(slots))
	}
	if slots[7].ID != 388 || slots[7].Count != 2 {
		test.Fatalf("第二个末影箱里应看到同样的东西，第 7 格期望 388x2，实际 %+v", slots[7])
	}
	if player.OpenTX != -3 {
		test.Fatalf("窗口位置应是第二个末影箱 x=-3，实际 %d", player.OpenTX)
	}
}

// 老世界里已经有末影箱方块、却没有方块实体：打开时当场建出来，和箱子一样。
func TestOpenEnderChestWithoutTileCreatesOne(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockEnderChest, 5)
	fixture.srv.openContainer(player, 3, 64, 0)
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found || tile.ID != world.TileEnderChest {
		test.Fatalf("应当场建出末影箱方块实体，实际 找到=%v 种类=%q", found, tile.ID)
	}
	if player.OpenWindow != protocol.WindowContainer || player.OpenTX != 3 {
		test.Fatalf("应打开窗口，实际 窗口=%d X=%d", player.OpenWindow, player.OpenTX)
	}
}

// 拆末影箱：窗口关掉，方块实体删掉，但玩家末影箱里的东西一件不丢、也不掉在地上。
func TestBreakEnderChestKeepsPlayerItems(test *testing.T) {
	var fixture, player = containerTestOpened(test, blockrule.BlockEnderChest)
	updateTestCreative(player)
	player.setEnderChestSlot(0, protocol.Item{ID: 264, Count: 9})
	updateTestBreak(fixture, player, 3, 64, 0)
	if updateTestDropped(fixture, 264) != 0 {
		test.Fatalf("玩家末影箱里的东西不该掉出来，实际掉了 %d", updateTestDropped(fixture, 264))
	}
	if player.enderChestItems()[0].Count != 9 {
		test.Fatalf("玩家末影箱里的东西应还在，实际 %+v", player.enderChestItems()[0])
	}
	if player.OpenWindow != 0 || player.OpenWindowType != 0 {
		test.Fatalf("窗口应被关掉，实际 %d/%d", player.OpenWindow, player.OpenWindowType)
	}
	var _, found = fixture.srv.world.TileAt(3, 64, 0)
	if found {
		test.Fatal("方块实体应随方块一起删掉")
	}
}

type enderPairCase struct {
	Name       string
	EnderFirst bool
}

// 末影箱和普通箱子不配对：不管谁先放，哪怕朝向相同、并排。
func TestEnderChestNeverPairsWithChest(test *testing.T) {
	var cases []enderPairCase = []enderPairCase{
		{Name: "先放末影箱再放箱子", EnderFirst: true},
		{Name: "先放箱子再放末影箱", EnderFirst: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item enderPairCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = placeTestSetup(sub)
			var firstID string = world.TileChest
			var secondID string = world.TileEnderChest
			if item.EnderFirst {
				enderTestPlace(sub, fixture, player, 3, 0, chestYawWest)
				chestTestPlace(sub, fixture, player, 3, 1, chestYawWest)
				firstID = world.TileEnderChest
				secondID = world.TileChest
			} else {
				chestTestPlace(sub, fixture, player, 3, 0, chestYawWest)
				enderTestPlace(sub, fixture, player, 3, 1, chestYawWest)
			}
			var first world.Tile = chestTestTile(fixture, 3, 0)
			var second world.Tile = chestTestTile(fixture, 3, 1)
			if first.ID != firstID || second.ID != secondID {
				sub.Fatalf("方块实体种类期望 %q/%q，实际 %q/%q", firstID, secondID, first.ID, second.ID)
			}
			if first.Paired || second.Paired {
				sub.Fatalf("末影箱和箱子不该配对，实际 %+v / %+v", first, second)
			}
		})
	}
}

// 末影箱随玩家存档：存出去的快照带着它，读回来的新玩家看到同样的东西；没用过的玩家快照里没有这个字段。
func TestPlayerSnapshotCarriesEnderChest(test *testing.T) {
	var fixture, source = placeTestSetup(test)
	var target *Player = fixture.join("target", "10.0.0.2", "world")
	var empty storage.PlayerData = source.Snapshot("world")
	if empty.EnderChest != nil {
		test.Fatalf("没用过末影箱的快照不该带这个字段，实际 %d 格", len(empty.EnderChest))
	}
	source.setEnderChestSlot(3, protocol.Item{ID: 264, Meta: 0, Count: 4})
	source.setEnderChestSlot(26, protocol.Item{ID: 35, Meta: 14, Count: 64})
	target.ApplySave(source.Snapshot("world"))
	var restored []protocol.Item = target.enderChestItems()
	if len(restored) != 27 {
		test.Fatalf("读回来应是 27 格，实际 %d", len(restored))
	}
	if restored[3].ID != 264 || restored[3].Count != 4 {
		test.Fatalf("第 3 格期望 264x4，实际 %+v", restored[3])
	}
	if restored[26].ID != 35 || restored[26].Meta != 14 || restored[26].Count != 64 {
		test.Fatalf("第 26 格期望 35:14x64，实际 %+v", restored[26])
	}
}
