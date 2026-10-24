package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/smelting"
import "go-mc-server/internal/world"

// furnaceTestSetup 在 (3,64,0) 放一个熔炉方块，并给它的方块实体塞上输入和燃料。
func furnaceTestSetup(test *testing.T, input protocol.Item, fuel protocol.Item) *adminFixture {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockFurnace, 2)
	var tile world.Tile = world.NewFurnaceTile(3, 64, 0)
	tile.Items[smelting.SlotInput] = input
	tile.Items[smelting.SlotFuel] = fuel
	fixture.srv.world.SetTile(tile)
	return fixture
}

func furnaceTestTicks(fixture *adminFixture, count int) {
	var tick int = 0
	for tick = 0; tick < count; tick++ {
		fixture.srv.tickFurnaces()
	}
}

func furnaceTestTile(fixture *adminFixture) world.Tile {
	var tile, _ = fixture.srv.world.TileAt(3, 64, 0)
	return tile
}

func furnaceTestBlock(fixture *adminFixture) (byte, byte) {
	return placeTestBlock(fixture, 3, 64, 0)
}

// 点燃的那个 tick：方块变成烧着的熔炉(62)，朝向(meta)不变；方块实体里燃料扣一个，烧 1599 烤 1。
func TestFurnaceLightsAndStartsCooking(test *testing.T) {
	var fixture *adminFixture = furnaceTestSetup(test, protocol.Item{ID: 15, Count: 4}, protocol.Item{ID: 263, Count: 2})
	furnaceTestTicks(fixture, 1)
	var id, meta = furnaceTestBlock(fixture)
	if id != blockrule.BlockBurningFurnace || meta != 2 {
		test.Fatalf("期望 烧着的熔炉:2，实际 %d:%d", id, meta)
	}
	var tile world.Tile = furnaceTestTile(fixture)
	if tile.BurnTime != 1599 || tile.CookTime != 1 || tile.MaxTime != 1600 || tile.Items[smelting.SlotFuel].Count != 1 {
		test.Fatalf("期望 烧1599/烤1/最大1600/燃料1，实际 %d/%d/%d/%d", tile.BurnTime, tile.CookTime, tile.MaxTime, tile.Items[smelting.SlotFuel].Count)
	}
}

// 烤 200 个 tick 出一个铁锭，输入扣一个。
func TestFurnaceProducesOutput(test *testing.T) {
	var fixture *adminFixture = furnaceTestSetup(test, protocol.Item{ID: 15, Count: 4}, protocol.Item{ID: 263, Count: 2})
	furnaceTestTicks(fixture, 200)
	var tile world.Tile = furnaceTestTile(fixture)
	var output protocol.Item = tile.Items[smelting.SlotOutput]
	if output.ID != 265 || output.Count != 1 || tile.Items[smelting.SlotInput].Count != 3 || tile.CookTime != 0 {
		test.Fatalf("期望 1 个铁锭、输入剩 3、烤制归零，实际 产物=%+v 输入=%d 烤=%d", output, tile.Items[smelting.SlotInput].Count, tile.CookTime)
	}
}

// 燃料不够烤完一个：木棍只烧 100 个 tick，烧完后熄火，输入没动。第 100 个 tick 方块还亮着，下一个 tick 才熄。
func TestFurnaceGoesOutWhenFuelRunsOut(test *testing.T) {
	var fixture *adminFixture = furnaceTestSetup(test, protocol.Item{ID: 15, Count: 1}, protocol.Item{ID: 280, Count: 1})
	furnaceTestTicks(fixture, 100)
	var litID, _ = furnaceTestBlock(fixture)
	if litID != blockrule.BlockBurningFurnace {
		test.Fatalf("第 100 个 tick 还应亮着，实际 %d", litID)
	}
	furnaceTestTicks(fixture, 1)
	var darkID, _ = furnaceTestBlock(fixture)
	var tile world.Tile = furnaceTestTile(fixture)
	if darkID != blockrule.BlockFurnace || tile.BurnTime != 0 || tile.CookTime != 0 {
		test.Fatalf("第 101 个 tick 应熄了，实际 方块=%d 烧=%d 烤=%d", darkID, tile.BurnTime, tile.CookTime)
	}
	if tile.Items[smelting.SlotInput].Count != 1 || !tile.Items[smelting.SlotOutput].IsAir() {
		test.Fatalf("没烤完，输入应原样、产物为空，实际 输入=%d 产物=%+v", tile.Items[smelting.SlotInput].Count, tile.Items[smelting.SlotOutput])
	}
}

// 两根木棍接力烧：中间换燃料那一个 tick 方块也不能闪成熄火，一直亮到烤完。
func TestFurnaceStaysLitAcrossRefuel(test *testing.T) {
	var fixture *adminFixture = furnaceTestSetup(test, protocol.Item{ID: 15, Count: 1}, protocol.Item{ID: 280, Count: 2})
	var tick int = 0
	for tick = 1; tick <= 200; tick++ {
		furnaceTestTicks(fixture, 1)
		var id, _ = furnaceTestBlock(fixture)
		if id != blockrule.BlockBurningFurnace {
			test.Fatalf("第 %d 个 tick 方块应一直亮着，实际 %d", tick, id)
		}
	}
	var tile world.Tile = furnaceTestTile(fixture)
	if tile.Items[smelting.SlotOutput].Count != 1 {
		test.Fatalf("200 个 tick 后应出 1 个，实际 %+v", tile.Items[smelting.SlotOutput])
	}
	furnaceTestTicks(fixture, 1)
	var darkID, _ = furnaceTestBlock(fixture)
	if darkID != blockrule.BlockFurnace {
		test.Fatalf("烤完又没燃料，第 201 个 tick 应熄，实际 %d", darkID)
	}
}

// 方块还显示烧着、方块实体里却什么都没有(比如服务器是在烧的时候崩的)：下一个 tick 熄掉。
func TestStaleBurningFurnaceGoesDark(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockBurningFurnace, 2)
	fixture.srv.world.SetTile(world.NewFurnaceTile(3, 64, 0))
	furnaceTestTicks(fixture, 1)
	var id, meta = furnaceTestBlock(fixture)
	if id != blockrule.BlockFurnace || meta != 2 {
		test.Fatalf("期望熄成 熔炉:2，实际 %d:%d", id, meta)
	}
}

// 方块实体还在、方块已经不是熔炉了：不碰它。
func TestFurnaceTickSkipsNonFurnaceBlock(test *testing.T) {
	var fixture *adminFixture = furnaceTestSetup(test, protocol.Item{ID: 15, Count: 1}, protocol.Item{ID: 263, Count: 1})
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockStone, 0)
	furnaceTestTicks(fixture, 5)
	var tile world.Tile = furnaceTestTile(fixture)
	if tile.BurnTime != 0 || tile.Items[smelting.SlotFuel].Count != 1 {
		test.Fatalf("方块不是熔炉，不该烧，实际 烧=%d 燃料=%d", tile.BurnTime, tile.Items[smelting.SlotFuel].Count)
	}
}

// 区块没加载的熔炉不烧，也不能为了烧它把区块加载出来。
func TestFurnaceTickSkipsUnloadedChunk(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var tile world.Tile = world.NewFurnaceTile(500, 64, 500)
	tile.Items[smelting.SlotInput] = protocol.Item{ID: 15, Count: 1}
	tile.Items[smelting.SlotFuel] = protocol.Item{ID: 263, Count: 1}
	fixture.srv.world.SetTile(tile)
	furnaceTestTicks(fixture, 5)
	var after, _ = fixture.srv.world.TileAt(500, 64, 500)
	if after.BurnTime != 0 || after.Items[smelting.SlotFuel].Count != 1 {
		test.Fatalf("没加载的熔炉不该烧，实际 烧=%d 燃料=%d", after.BurnTime, after.Items[smelting.SlotFuel].Count)
	}
	var _, _, loaded = fixture.srv.world.Peek(500, 64, 500)
	if loaded {
		test.Fatal("不该把区块加载出来")
	}
}

type furnaceWriteCase struct {
	Name     string
	Slot     int32
	Proposed protocol.Item
	Expect   protocol.Item
}

// 玩家开着熔炉窗口，产物格只能取不能放：想多放、换成别的东西、往空格里放，都被拒绝，真实内容不变；
// 取走一部分、整叠取走可以；输入格和燃料格先照旧让客户端随便放（防刷在背包事务校验那一步做）。
func TestFurnaceOutputSlotIsTakeOnly(test *testing.T) {
	var current protocol.Item = protocol.Item{ID: 265, Count: 5}
	var cases []furnaceWriteCase = []furnaceWriteCase{
		{Name: "凭空放多", Slot: 2, Proposed: protocol.Item{ID: 265, Count: 64}, Expect: current},
		{Name: "换成别的东西", Slot: 2, Proposed: protocol.Item{ID: 264, Count: 5}, Expect: current},
		{Name: "取走一部分", Slot: 2, Proposed: protocol.Item{ID: 265, Count: 2}, Expect: protocol.Item{ID: 265, Count: 2}},
		{Name: "整叠取走", Slot: 2, Proposed: protocol.Air(), Expect: protocol.Air()},
		{Name: "输入格可以放", Slot: 0, Proposed: protocol.Item{ID: 15, Count: 8}, Expect: protocol.Item{ID: 15, Count: 8}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item furnaceWriteCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = placeTestSetup(sub)
			placeTestPut(sub, fixture, 3, 64, 0, blockrule.BlockFurnace, 0)
			var tile world.Tile = world.NewFurnaceTile(3, 64, 0)
			tile.Items[smelting.SlotOutput] = current
			fixture.srv.world.SetTile(tile)
			fixture.srv.openContainer(player, 3, 64, 0)
			containerTestWrite(fixture, player, protocol.WindowContainer, item.Slot, item.Proposed)
			var actual protocol.Item = containerTestSlot(fixture, int(item.Slot))
			if !smelting.SameItem(actual, item.Expect) {
				sub.Fatalf("期望 %+v，实际 %+v", item.Expect, actual)
			}
		})
	}
}

// 空的产物格不能凭空放东西进去。
func TestFurnaceEmptyOutputRejectsPlacement(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockFurnace, 0)
	fixture.srv.openContainer(player, 3, 64, 0)
	containerTestWrite(fixture, player, protocol.WindowContainer, 2, protocol.Item{ID: 264, Count: 3})
	if !containerTestSlot(fixture, 2).IsAir() {
		test.Fatalf("空的产物格不该能放东西，实际 %+v", containerTestSlot(fixture, 2))
	}
}

// 发给客户端的进度：烤制进度原样，火焰是剩余占比乘 200。
func TestFurnaceProgressPackets(test *testing.T) {
	var state smelting.State = smelting.State{BurnTime: 800, MaxTime: 1600, CookTime: 73}
	var progress []packets.Packet = furnaceProgressPackets(state)
	if len(progress) != 2 {
		test.Fatalf("应有 2 个包，实际 %d", len(progress))
	}
	var cook, cookOK = progress[0].(*packets.ContainerSetDataPacket)
	var fire, fireOK = progress[1].(*packets.ContainerSetDataPacket)
	if !cookOK || !fireOK {
		test.Fatalf("两个都应是 ContainerSetDataPacket，实际 %T、%T", progress[0], progress[1])
	}
	if cook.Property != 0 || cook.Value != 73 || fire.Property != 1 || fire.Value != 100 {
		test.Fatalf("期望 属性0=73、属性1=100，实际 %d=%d、%d=%d", cook.Property, cook.Value, fire.Property, fire.Value)
	}
	if cook.WindowID != protocol.WindowContainer || fire.WindowID != protocol.WindowContainer {
		test.Fatalf("窗口号应是 %d，实际 %d、%d", protocol.WindowContainer, cook.WindowID, fire.WindowID)
	}
}

// 前后状态里哪些格子变了。
func TestChangedFurnaceSlots(test *testing.T) {
	var before smelting.State = smelting.State{
		Input:  protocol.Item{ID: 15, Count: 4},
		Fuel:   protocol.Item{ID: 263, Count: 2},
		Output: protocol.Air(),
	}
	var after smelting.State = before
	after.Fuel = protocol.Item{ID: 263, Count: 1}
	after.Output = protocol.Item{ID: 265, Count: 1}
	after.BurnTime = 1599
	var changed []int = changedFurnaceSlots(before, after)
	if len(changed) != 2 || changed[0] != 1 || changed[1] != 2 {
		test.Fatalf("期望变了 [1 2]，实际 %v", changed)
	}
	if len(changedFurnaceSlots(before, before)) != 0 {
		test.Fatal("没变化应返回空")
	}
}

// 玩家开着熔炉，同步给他的路径跑起来不出错(测试里会话没有真实连接，包会被丢掉)，窗口也不会被误关。
func TestFurnaceTickWithViewerKeepsWindowOpen(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockFurnace, 0)
	var tile world.Tile = world.NewFurnaceTile(3, 64, 0)
	tile.Items[smelting.SlotInput] = protocol.Item{ID: 15, Count: 1}
	tile.Items[smelting.SlotFuel] = protocol.Item{ID: 263, Count: 1}
	fixture.srv.world.SetTile(tile)
	fixture.srv.openContainer(player, 3, 64, 0)
	furnaceTestTicks(fixture, 3)
	if player.OpenWindow != protocol.WindowContainer || player.OpenWindowType != protocol.InventoryTypeFurnace {
		test.Fatalf("窗口应仍开着，实际 %d/%d", player.OpenWindow, player.OpenWindowType)
	}
	if furnaceTestTile(fixture).CookTime != 3 {
		test.Fatalf("烤了 3 个 tick，实际 %d", furnaceTestTile(fixture).CookTime)
	}
}
