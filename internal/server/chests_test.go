package server

import "bytes"
import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/world"

// 偏航角对应的箱子 meta：90 朝西 → 5（并排在南北），270 朝东 → 4（并排在南北），0 朝南 → 2（并排在东西）。
const chestYawWest float32 = 90
const chestYawEast float32 = 270

// chestTestPlace 玩家站在原点、面朝 yaw，在 (posX,64,posZ) 放一个箱子，脚下垫一块石头当支撑。
func chestTestPlace(test *testing.T, fixture *adminFixture, player *Player, posX int32, posZ int32, yaw float32) {
	player.UpdatePosition(0, 64+playerEyeHeight, 0, yaw, 0, true)
	placeTestPut(test, fixture, posX, 63, posZ, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 54, 0, 64)
	placeTestUse(fixture, player, posX, 63, posZ, blockrule.FaceUp, 1, held)
}

// chestTestPair 直接在 (posX,64,firstZ) 和 (posX,64,secondZ) 摆两个配好对的箱子，不走放置流程。
func chestTestPair(test *testing.T, fixture *adminFixture, posX int32, firstZ int32, secondZ int32) {
	placeTestPut(test, fixture, posX, 64, firstZ, blockrule.BlockChest, 5)
	placeTestPut(test, fixture, posX, 64, secondZ, blockrule.BlockChest, 5)
	fixture.srv.world.SetTile(world.NewChestTile(posX, 64, firstZ))
	fixture.srv.world.SetTile(world.NewChestTile(posX, 64, secondZ))
	if !fixture.srv.world.PairChests(posX, 64, firstZ, posX, secondZ) {
		test.Fatal("夹具没配成对")
	}
}

func chestTestTile(fixture *adminFixture, posX int32, posZ int32) world.Tile {
	var tile, _ = fixture.srv.world.TileAt(posX, 64, posZ)
	return tile
}

func chestTestPaired(fixture *adminFixture, firstZ int32, secondZ int32) bool {
	var first world.Tile = chestTestTile(fixture, 3, firstZ)
	var second world.Tile = chestTestTile(fixture, 3, secondZ)
	return first.Paired && first.PairX == 3 && first.PairZ == secondZ &&
		second.Paired && second.PairX == 3 && second.PairZ == firstZ
}

// 两个朝向相同的箱子并排放，自动配成大箱子；先放的那个也要记上对方，两半都要重新发给客户端，
// 客户端靠 pairx、pairz 才把它们画成一个。
func TestPlaceChestsSideBySidePair(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var log *sentLog = captureSent(player)
	chestTestPlace(test, fixture, player, 3, 0, chestYawWest)
	chestTestPlace(test, fixture, player, 3, 1, chestYawWest)
	if !chestTestPaired(fixture, 0, 1) {
		test.Fatalf("两个箱子应互相配对，实际 %+v / %+v", chestTestTile(fixture, 3, 0), chestTestTile(fixture, 3, 1))
	}
	var firstNBT, firstSent = log.lastBlockEntityAt(test, 3, 64, 0)
	if !firstSent || !bytes.Equal(firstNBT, nbt.SpawnPairedChest(3, 64, 0, 3, 1)) {
		test.Fatalf("先放的那个箱子最后一次发给客户端的应是配对样子，实际 发过=%v %x", firstSent, firstNBT)
	}
	var secondNBT, secondSent = log.lastBlockEntityAt(test, 3, 64, 1)
	if !secondSent || !bytes.Equal(secondNBT, nbt.SpawnPairedChest(3, 64, 1, 3, 0)) {
		test.Fatalf("后放的那个箱子最后一次发给客户端的应是配对样子，实际 发过=%v %x", secondSent, secondNBT)
	}
}

type chestNoPairCase struct {
	Name      string
	FirstX    int32
	FirstZ    int32
	FirstYaw  float32
	SecondX   int32
	SecondZ   int32
	SecondYaw float32
}

// 这些摆法不该配对：朝向不同；正对着或背对着（不是并排）；隔了一格。
func TestPlaceChestsThatMustNotPair(test *testing.T) {
	var cases []chestNoPairCase = []chestNoPairCase{
		{Name: "朝向不同", FirstX: 3, FirstZ: 0, FirstYaw: chestYawWest, SecondX: 3, SecondZ: 1, SecondYaw: chestYawEast},
		{Name: "正对着不是并排", FirstX: 3, FirstZ: 0, FirstYaw: chestYawWest, SecondX: 4, SecondZ: 0, SecondYaw: chestYawWest},
		{Name: "隔了一格", FirstX: 3, FirstZ: 0, FirstYaw: chestYawWest, SecondX: 3, SecondZ: 2, SecondYaw: chestYawWest},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item chestNoPairCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = placeTestSetup(sub)
			chestTestPlace(sub, fixture, player, item.FirstX, item.FirstZ, item.FirstYaw)
			chestTestPlace(sub, fixture, player, item.SecondX, item.SecondZ, item.SecondYaw)
			var first world.Tile = chestTestTile(fixture, item.FirstX, item.FirstZ)
			var second world.Tile = chestTestTile(fixture, item.SecondX, item.SecondZ)
			if first.ID != world.TileChest || second.ID != world.TileChest {
				sub.Fatalf("两个箱子都应放成功，实际 %q / %q", first.ID, second.ID)
			}
			if first.Paired || second.Paired {
				sub.Fatalf("不该配对，实际 %+v / %+v", first, second)
			}
		})
	}
}

// 有人正开着一个单箱，旁边又放下一个同朝向的箱子配成了大箱子：他那个 27 格的窗口必须关掉，
// 不然他接着往"第 5 格"写，写进的已经不是他看到的那一格。
func TestPlaceChestClosesNeighborWindowWhenPairing(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	var viewer *Player = fixture.join("viewer", "10.0.0.2", "world")
	chestTestPlace(test, fixture, player, 3, 0, chestYawWest)
	fixture.srv.openContainer(viewer, 3, 64, 0)
	if viewer.OpenWindow != protocol.WindowContainer {
		test.Fatal("夹具：旁观者应已打开 z=0 的箱子")
	}
	var viewerLog *sentLog = captureSent(viewer)
	chestTestPlace(test, fixture, player, 3, 1, chestYawWest)
	if !chestTestPaired(fixture, 0, 1) {
		test.Fatal("夹具：两个箱子应已配对")
	}
	if viewer.OpenWindow != 0 || viewer.OpenWindowType != 0 {
		test.Fatalf("配对后旁观者的窗口应被关掉，实际 %d/%d", viewer.OpenWindow, viewer.OpenWindowType)
	}
	if viewerLog.count(protocol.IDContainerClose) != 1 {
		test.Fatalf("旁观者应恰好收到 1 个关窗口包，实际 %d", viewerLog.count(protocol.IDContainerClose))
	}
}

// 第一个候选位置上的箱子已经配过对，要接着看第二个候选，而不是就此放弃。
// 摆法：z=-2 和 z=-1 已配对，z=1 是单个；新箱子放 z=0，北边的 z=-1 配不上，该和南边的 z=1 配。
func TestPlaceChestSkipsAlreadyPairedNeighbor(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	chestTestPlace(test, fixture, player, 3, 1, chestYawWest)
	chestTestPlace(test, fixture, player, 3, -2, chestYawWest)
	chestTestPlace(test, fixture, player, 3, -1, chestYawWest)
	if !chestTestPaired(fixture, -2, -1) {
		test.Fatal("夹具：z=-2 和 z=-1 应已配对")
	}
	chestTestPlace(test, fixture, player, 3, 0, chestYawWest)
	if !chestTestPaired(fixture, 0, 1) {
		test.Fatalf("新箱子应和 z=1 的单箱配对，实际 %+v / %+v", chestTestTile(fixture, 3, 0), chestTestTile(fixture, 3, 1))
	}
	if !chestTestPaired(fixture, -2, -1) {
		test.Fatal("原来那对不该被拆")
	}
}

// 大箱子的两半在窗口里的顺序只看位置，点哪一半打开都一样：z 大的排前面，窗口位置记在它身上。
func TestOpenDoubleChestFromEitherHalf(test *testing.T) {
	var fromZ []int32 = []int32{0, 1}
	var index int = 0
	for index = 0; index < len(fromZ); index++ {
		var clickedZ int32 = fromZ[index]
		var fixture, player = placeTestSetup(test)
		chestTestPair(test, fixture, 3, 0, 1)
		fixture.srv.world.SetTileSlot(3, 64, 0, 5, protocol.Item{ID: 280, Count: 1})
		fixture.srv.world.SetTileSlot(3, 64, 1, 3, protocol.Item{ID: 263, Count: 2})
		fixture.srv.openContainer(player, 3, 64, clickedZ)
		if player.OpenTX != 3 || player.OpenTY != 64 || player.OpenTZ != 1 {
			test.Fatalf("点 z=%d：窗口位置应记在前面那半 (3,64,1)，实际 (%d,%d,%d)",
				clickedZ, player.OpenTX, player.OpenTY, player.OpenTZ)
		}
		if player.OpenWindowType != protocol.InventoryTypeContainer {
			test.Fatalf("点 z=%d：大箱子也是容器窗口类型，实际 %d", clickedZ, player.OpenWindowType)
		}
		var halves []world.Tile = chestHalves(fixture.srv.world, chestTestTile(fixture, 3, clickedZ))
		var items []protocol.Item = windowItems(halves)
		if len(items) != 54 {
			test.Fatalf("点 z=%d：窗口应有 54 格，实际 %d", clickedZ, len(items))
		}
		if items[3].ID != 263 || items[27+5].ID != 280 {
			test.Fatalf("点 z=%d：前 27 格应是 z=1 的箱子、后 27 格是 z=0 的，实际 第3格=%d 第32格=%d",
				clickedZ, items[3].ID, items[32].ID)
		}
	}
}

type windowSlotCase struct {
	Name   string
	Slot   int
	Expect bool
	TileX  int32
	Local  int
}

// 窗口格子编号落到具体哪个方块实体的哪一格：前 27 格在第一个里，后 27 格在第二个里，越界不落。
func TestWindowSlotTarget(test *testing.T) {
	var halves []world.Tile = []world.Tile{
		world.NewChestTile(10, 64, 0),
		world.NewChestTile(20, 64, 0),
	}
	var cases []windowSlotCase = []windowSlotCase{
		{Name: "第 0 格", Slot: 0, Expect: true, TileX: 10, Local: 0},
		{Name: "第 26 格", Slot: 26, Expect: true, TileX: 10, Local: 26},
		{Name: "第 27 格", Slot: 27, Expect: true, TileX: 20, Local: 0},
		{Name: "第 53 格", Slot: 53, Expect: true, TileX: 20, Local: 26},
		{Name: "第 54 格越界", Slot: 54, Expect: false},
		{Name: "负数越界", Slot: -1, Expect: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item windowSlotCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var target, local, inRange = windowSlotTarget(halves, item.Slot)
			if inRange != item.Expect {
				sub.Fatalf("在范围内期望 %v，实际 %v", item.Expect, inRange)
			}
			if inRange && (target.X != item.TileX || local != item.Local) {
				sub.Fatalf("期望 方块实体 x=%d 第 %d 格，实际 x=%d 第 %d 格", item.TileX, item.Local, target.X, local)
			}
		})
	}
}

// 熔炉和单个箱子的窗口只有它自己一个方块实体。
func TestChestHalvesOfSingleContainers(test *testing.T) {
	var fixture, _ = placeTestSetup(test)
	fixture.srv.world.SetTile(world.NewChestTile(3, 64, 0))
	fixture.srv.world.SetTile(world.NewFurnaceTile(3, 64, 5))
	var chest, _ = fixture.srv.world.TileAt(3, 64, 0)
	var furnace, _ = fixture.srv.world.TileAt(3, 64, 5)
	if len(chestHalves(fixture.srv.world, chest)) != 1 {
		test.Fatalf("单个箱子应只有 1 个，实际 %d", len(chestHalves(fixture.srv.world, chest)))
	}
	if len(chestHalves(fixture.srv.world, furnace)) != 1 {
		test.Fatalf("熔炉应只有 1 个，实际 %d", len(chestHalves(fixture.srv.world, furnace)))
	}
}

// 往大箱子窗口里写：前 27 格写进 z 大的那个，后 27 格写进另一个；越界的不写。
func TestDoubleChestSlotWriteGoesToRightHalf(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	chestTestPair(test, fixture, 3, 0, 1)
	fixture.srv.openContainer(player, 3, 64, 0)
	containerTestWriteAt(fixture, player, 5, protocol.Item{ID: 1, Count: 9})
	containerTestWriteAt(fixture, player, 30, protocol.Item{ID: 2, Count: 8})
	containerTestWriteAt(fixture, player, 54, protocol.Item{ID: 3, Count: 7})
	containerTestWriteAt(fixture, player, -1, protocol.Item{ID: 4, Count: 6})
	var front world.Tile = chestTestTile(fixture, 3, 1)
	var back world.Tile = chestTestTile(fixture, 3, 0)
	if front.Items[5].ID != 1 || front.Items[5].Count != 9 {
		test.Fatalf("第 5 格应写进 z=1 的箱子第 5 格，实际 %+v", front.Items[5])
	}
	if back.Items[3].ID != 2 || back.Items[3].Count != 8 {
		test.Fatalf("第 30 格应写进 z=0 的箱子第 3 格，实际 %+v", back.Items[3])
	}
	var total int = 0
	var slot int = 0
	for slot = 0; slot < 27; slot++ {
		total += int(front.Items[slot].Count) + int(back.Items[slot].Count)
	}
	if total != 9+8 {
		test.Fatalf("越界的写入不该落进任何一格，两个箱子里物品总数期望 17，实际 %d", total)
	}
}

func containerTestWriteAt(fixture *adminFixture, player *Player, slot int32, item protocol.Item) {
	containerTestWrite(fixture, player, protocol.WindowContainer, slot, item)
}

// 大箱子只要有一半够得着就能操作；两半都够不着才拒绝。玩家眼睛在 (0,65.62,0)，交互距离 8：
// z=7 的那半距离约 7.6 够得着，z=8 的约 8.6 够不着。
func TestDoubleChestReachEitherHalf(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	chestTestPair(test, fixture, 0, 7, 8)
	fixture.srv.openContainer(player, 0, 64, 7)
	containerTestWriteAt(fixture, player, 0, protocol.Item{ID: 1, Count: 4})
	var far, _ = fixture.srv.world.TileAt(0, 64, 8)
	if far.Items[0].ID != 1 {
		test.Fatalf("近的那半够得着，写入应通过（窗口位置在远的那半上），实际 %+v", far.Items[0])
	}
	player.UpdatePosition(0, 64+playerEyeHeight, -20, 0, 0, true)
	containerTestWriteAt(fixture, player, 1, protocol.Item{ID: 2, Count: 4})
	far, _ = fixture.srv.world.TileAt(0, 64, 8)
	if far.Items[1].ID != 0 {
		test.Fatalf("两半都够不着时不该写入，实际 %+v", far.Items[1])
	}
}

// 拆掉大箱子的任意一半：只掉拆掉这半的东西，两个人的窗口（不管开的是哪一半）都关掉，剩下的那半变回单箱、东西还在。
func TestBreakDoubleChestHalf(test *testing.T) {
	var breakZs []int32 = []int32{0, 1}
	var index int = 0
	for index = 0; index < len(breakZs); index++ {
		var breakZ int32 = breakZs[index]
		var keepZ int32 = 1 - breakZ
		var fixture, player = placeTestSetup(test)
		updateTestCreative(player)
		var other *Player = fixture.join("other", "10.0.0.2", "world")
		chestTestPair(test, fixture, 3, 0, 1)
		fixture.srv.world.SetTileSlot(3, 64, 0, 0, protocol.Item{ID: 280, Count: 5})
		fixture.srv.world.SetTileSlot(3, 64, 1, 0, protocol.Item{ID: 263, Count: 8})
		fixture.srv.openContainer(player, 3, 64, 0)
		fixture.srv.openContainer(other, 3, 64, 1)
		var otherLog *sentLog = captureSent(other)
		updateTestBreak(fixture, player, 3, 64, breakZ)
		var wantStick int = 0
		var wantCoal int = 0
		if breakZ == 0 {
			wantStick = 5
		} else {
			wantCoal = 8
		}
		if updateTestDropped(fixture, 280) != wantStick || updateTestDropped(fixture, 263) != wantCoal {
			test.Fatalf("拆 z=%d：只该掉出这半的东西，期望 木棍=%d 煤=%d，实际 木棍=%d 煤=%d",
				breakZ, wantStick, wantCoal, updateTestDropped(fixture, 280), updateTestDropped(fixture, 263))
		}
		if player.OpenWindow != 0 || other.OpenWindow != 0 {
			test.Fatalf("拆 z=%d：两个人的窗口都该关掉，实际 %d / %d", breakZ, player.OpenWindow, other.OpenWindow)
		}
		var kept world.Tile = chestTestTile(fixture, 3, keepZ)
		if kept.ID != world.TileChest || kept.Paired {
			test.Fatalf("拆 z=%d：剩下的应是不再配对的单箱，实际 %+v", breakZ, kept)
		}
		var keptSlot protocol.Item = kept.Items[0]
		if keptSlot.IsAir() {
			test.Fatalf("拆 z=%d：剩下那半里的东西不该丢", breakZ)
		}
		var redrawn, redrawnSent = otherLog.lastBlockEntityAt(test, 3, 64, keepZ)
		if !redrawnSent || !bytes.Equal(redrawn, nbt.SpawnChest(3, 64, keepZ)) {
			test.Fatalf("拆 z=%d：剩下那半应重新发给客户端，画成单箱，实际 发过=%v %x", breakZ, redrawnSent, redrawn)
		}
		if otherLog.count(protocol.IDContainerClose) != 1 {
			test.Fatalf("拆 z=%d：另一个玩家应恰好收到 1 个关窗口包，实际 %d", breakZ, otherLog.count(protocol.IDContainerClose))
		}
	}
}

type stalePairCase struct {
	Name  string
	Setup func(test *testing.T, fixture *adminFixture)
}

// 箱子记着配对、另一半却不认账（不存在，或存在但没记配对），打开时把这条配对解掉，当单箱开：27 格，位置是它自己。
func TestOpenRepairsStalePair(test *testing.T) {
	var cases []stalePairCase = []stalePairCase{
		{
			Name: "另一半不存在",
			Setup: func(test *testing.T, fixture *adminFixture) {
				placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockChest, 5)
			},
		},
		{
			Name: "另一半是没记配对的单箱",
			Setup: func(test *testing.T, fixture *adminFixture) {
				placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockChest, 5)
				placeTestPut(test, fixture, 3, 64, 1, blockrule.BlockChest, 5)
				fixture.srv.world.SetTile(world.NewChestTile(3, 64, 1))
			},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item stalePairCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = placeTestSetup(sub)
			item.Setup(sub, fixture)
			fixture.srv.world.SetTile(world.Tile{
				ID: world.TileChest, X: 3, Y: 64, Z: 0,
				Paired: true, PairX: 3, PairZ: 1,
				Items: make([]protocol.Item, 27),
			})
			var log *sentLog = captureSent(player)
			fixture.srv.openContainer(player, 3, 64, 0)
			var tile world.Tile = chestTestTile(fixture, 3, 0)
			if tile.Paired {
				sub.Fatalf("残缺的配对应被解掉，实际 %+v", tile)
			}
			var repaired, repairedSent = log.lastBlockEntityAt(sub, 3, 64, 0)
			if !repairedSent || !bytes.Equal(repaired, nbt.SpawnChest(3, 64, 0)) {
				sub.Fatalf("解掉配对后应把单箱的样子重新发出去，实际 发过=%v %x", repairedSent, repaired)
			}
			if player.OpenTZ != 0 {
				sub.Fatalf("应当单箱开，窗口位置在它自己 z=0，实际 z=%d", player.OpenTZ)
			}
			var halves []world.Tile = chestHalves(fixture.srv.world, tile)
			if len(windowItems(halves)) != 27 {
				sub.Fatalf("应是 27 格，实际 %d", len(windowItems(halves)))
			}
			var neighbor world.Tile = chestTestTile(fixture, 3, 1)
			if neighbor.Paired {
				sub.Fatalf("邻居不该被带着配对，实际 %+v", neighbor)
			}
		})
	}
}
