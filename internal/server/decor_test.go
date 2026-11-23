package server

import "bytes"
import "maps"
import "slices"
import "testing"
import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// 空框只取一件并保留物品标签；转动不扣第二件。受保护时不能拍出，重复拍打也只能掉一次。
func TestItemFrameLifecycle(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 64, 0, blockrule.BlockStone, 0)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, placeTestHold(player, 389, 0, 1))
	var id, meta = placeTestBlock(fixture, 4, 64, 0)
	if id != 199 || meta != 0 {
		test.Fatalf("期望东面展示框 199:0，实际 %d:%d", id, meta)
	}
	var held = placeTestHold(player, 35, 2, 2)
	held.NBT = []byte{10, 0, 0, 8, 1, 0, 'a', 1, 0, 'b', 0}
	player.HeldItem, player.Inventory[0] = held, held
	var index int = 0
	for index = 0; index < 10; index++ {
		placeTestUse(fixture, player, 4, 64, 0, blockrule.FaceEast, 0.5, protocol.Air())
	}
	var tile, _ = fixture.srv.world.TileAt(4, 64, 0)
	if tile.ID != world.TileItemFrame || tile.Rotation != 1 || tile.Items[0].Count != 1 || !bytes.Equal(tile.Items[0].NBT, held.NBT) || placeTestCount(player) != 1 {
		test.Fatalf("期望框内与背包各一件、旋转角 1，实际 %+v / %d", tile, placeTestCount(player))
	}
	var punch = &packets.ItemFrameDropItemPacket{X: 4, Y: 64, Z: 0}
	fixture.srv.cfg.SpawnProtection = 10
	fixture.srv.handleItemFrameDrop(player.Session, punch)
	if updateTestDropped(fixture, 35) != 0 {
		test.Fatal("期望保护区内不掉落，实际已掉落")
	}
	fixture.srv.cfg.SpawnProtection = 0
	fixture.srv.handleItemFrameDrop(player.Session, punch)
	fixture.srv.handleItemFrameDrop(player.Session, punch)
	tile, _ = fixture.srv.world.TileAt(4, 64, 0)
	if !tile.Items[0].IsAir() || tile.Rotation != 0 || multiTestDropped(fixture, 35, 2) != 1 {
		test.Fatalf("期望空框和一件掉落，实际 %+v / %d", tile, updateTestDropped(fixture, 35))
	}
	var droppedIDs = slices.Sorted(maps.Keys(fixture.srv.entities))
	if len(droppedIDs) != 1 || !bytes.Equal(fixture.srv.entities[droppedIDs[0]].Item.NBT, held.NBT) {
		test.Fatal("期望唯一的掉落物保留原标签，实际不符")
	}
	updateTestCreative(player)
	updateTestBreak(fixture, player, 3, 64, 0)
	if multiTestDropped(fixture, 389, 0) != 1 || updateTestDropped(fixture, 35) != 1 {
		test.Fatal("期望失去支撑只再掉一个框，实际掉落不符")
	}
}

// 从物品放置的漏斗应朝向被点击的侧面，并允许把真实物品移到第五格。
func TestHopperStorageWindow(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 2, 64, 0, blockrule.BlockStone, 0)
	placeTestUse(fixture, player, 2, 64, 0, blockrule.FaceEast, 0.5, placeTestHold(player, 410, 0, 1))
	var blockID, meta = placeTestBlock(fixture, 3, 64, 0)
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceEast, 0.5, protocol.Air())
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if blockID != world.BlockHopper || meta != 4 || !found || tile.ID != world.TileHopper || len(tile.Items) != 5 || player.OpenWindowType != 8 {
		test.Fatalf("期望出口朝西、五格的 type 8 漏斗，实际 %d:%d / %+v / %d", blockID, meta, tile, player.OpenWindowType)
	}
	containerTestWrite(fixture, player, protocol.WindowContainer, 4, protocol.Item{ID: 264, Count: 3})
	var stack = containerTestSlot(fixture, 4)
	if stack.ID != 264 || stack.Count != 3 {
		test.Fatalf("期望第五格有三颗钻石，实际 %+v", stack)
	}
}

// 花盆拒绝高草 meta 1，接受树苗；挖掉支撑时盆和里面那一株分别掉落。
func TestFlowerPotLifecycle(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, placeTestHold(player, 390, 0, 1))
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 1, placeTestHold(player, 31, 1, 1))
	if placeTestCount(player) != 1 {
		test.Fatal("期望不能入盆的物品留在手里，实际已扣除")
	}
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 1, placeTestHold(player, 6, 5, 2))
	placeTestUse(fixture, player, 3, 64, 0, blockrule.FaceUp, 1, protocol.Air())
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != 140 || meta != 1 || placeTestCount(player) != 1 {
		test.Fatalf("期望盆内一株、手中一株，实际 %d:%d / %d", id, meta, placeTestCount(player))
	}
	updateTestCreative(player)
	updateTestBreak(fixture, player, 3, 63, 0)
	if multiTestDropped(fixture, 390, 0) != 1 || multiTestDropped(fixture, 6, 5) != 1 {
		test.Fatal("期望各掉一个花盆和深色橡树苗，实际不符")
	}
}

// 头颅方块 meta 是支撑方向，物品 meta 是种类；直接挖取和失去支撑都应还原种类。
func TestSkullDropsType(test *testing.T) {
	var direct int = 0
	for direct = 0; direct <= 1; direct++ {
		var fixture, player = placeTestSetup(test)
		player.Yaw = -90
		placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
		placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, placeTestHold(player, 397, 4, 1))
		var tile, _ = fixture.srv.world.TileAt(3, 64, 0)
		if tile.ID != world.TileSkull || tile.SkullType != 4 || tile.Rotation != 12 {
			test.Fatalf("期望苦力怕头颅、旋转 12，实际 %+v", tile)
		}
		if direct == 1 {
			multiTestMine(player, 3, 64, 0, 144)
			updateTestBreak(fixture, player, 3, 64, 0)
		} else {
			updateTestCreative(player)
			updateTestBreak(fixture, player, 3, 63, 0)
		}
		if multiTestDropped(fixture, 397, 4) != 1 {
			test.Fatalf("期望掉一颗苦力怕头颅，实际 %d", multiTestDropped(fixture, 397, 4))
		}
	}
}
