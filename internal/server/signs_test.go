package server

import "testing"

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/protocol"
import "go-mc-server/internal/protocol/nbt"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

// signTestSetup 在 (3,64,0) 立一块属于 player 的立牌，脚下垫石头。
func signTestSetup(test *testing.T) (*adminFixture, *Player) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	placeTestPut(test, fixture, 3, 64, 0, world.BlockSignPost, 8)
	fixture.srv.world.SetTile(world.NewSignTile(3, 64, 0, player.UUID))
	return fixture, player
}

// signTestEdit 模拟客户端把四行字发上来，返回这一格现在存着的方块实体。
// signTestSend 模拟客户端把四行字发上来。
func signTestSend(fixture *adminFixture, player *Player, lines [4]string) {
	var payload []byte = nbt.WriteNetworkCompound("", func(writer *nbt.Writer) {
		writer.String("id", nbt.IDSign)
		writer.Int("x", 3)
		writer.Int("y", 64)
		writer.Int("z", 0)
		writer.String("Text1", lines[0])
		writer.String("Text2", lines[1])
		writer.String("Text3", lines[2])
		writer.String("Text4", lines[3])
	})
	fixture.srv.handleBlockEntityData(player.Session, &packets.BlockEntityDataPacket{X: 3, Y: 64, Z: 0, NBT: payload})
}

// signTestEdit 写字之后返回这一格现在存着的方块实体。
func signTestEdit(test *testing.T, fixture *adminFixture, player *Player, lines [4]string) world.Tile {
	signTestSend(fixture, player, lines)
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found {
		test.Fatal("方块实体不该消失")
	}
	return tile
}

// 放牌的人写字，四行存进方块实体，写完就不能再改。
func TestSignEditAcceptedOnce(test *testing.T) {
	var fixture, player = signTestSetup(test)
	var tile world.Tile = signTestEdit(test, fixture, player, [4]string{"第一行", "第二行", "", "§c第四行"})
	if tile.Lines != [4]string{"第一行", "第二行", "", "第四行"} {
		test.Fatalf("期望四行写入并去掉格式码，实际 %q", tile.Lines)
	}
	if tile.Editor != "" {
		test.Fatalf("写过之后该清掉编辑权，实际 %q", tile.Editor)
	}
	var again world.Tile = signTestEdit(test, fixture, player, [4]string{"改写", "", "", ""})
	if again.Lines != tile.Lines {
		test.Fatalf("同一块牌不该能改第二次，实际 %q", again.Lines)
	}
}

// 不是放牌的人写字不收，牌子保持原样。
func TestSignEditRejectsOtherPlayer(test *testing.T) {
	var fixture, player = signTestSetup(test)
	var other *Player = fixture.join("other", "10.0.0.2", "world")
	var tile world.Tile = signTestEdit(test, fixture, other, [4]string{"偷写", "", "", ""})
	if tile.Lines != [4]string{} {
		test.Fatalf("别人不该写得进去，实际 %q", tile.Lines)
	}
	if tile.Editor != player.UUID {
		test.Fatalf("编辑权不该被清掉，实际 %q", tile.Editor)
	}
}

type signRejectCase struct {
	Name  string
	Spoil func(fixture *adminFixture, player *Player)
	// Edit 不为 nil 时用它代替默认的四行写字，用来发别的畸形包。
	Edit func(fixture *adminFixture, player *Player)
	// Gone 为真表示这条用例会把方块实体删掉，不再断言它还在。
	Gone bool
}

// 这些情况下写字一律不收，牌子保持原样：NBT 里 id 不对、站得太远、已死、区块没加载、
// 那一格不是告示牌、方块实体没了、方块实体不是告示牌、落在出生点保护里。
func TestSignEditRejected(test *testing.T) {
	var cases []signRejectCase = []signRejectCase{
		{
			Name:  "NBT 里 id 不是 Sign",
			Spoil: func(fixture *adminFixture, player *Player) {},
			Edit: func(fixture *adminFixture, player *Player) {
				var payload []byte = nbt.WriteNetworkCompound("", func(writer *nbt.Writer) {
					writer.String("id", nbt.IDChest)
					writer.String("Text1", "不该进去")
				})
				fixture.srv.handleBlockEntityData(player.Session, &packets.BlockEntityDataPacket{X: 3, Y: 64, Z: 0, NBT: payload})
			},
		},
		{
			Name: "站得太远",
			Spoil: func(fixture *adminFixture, player *Player) {
				player.UpdatePosition(100, 64+playerEyeHeight, 100, 0, 0, true)
			},
		},
		{
			Name: "玩家已经死了",
			Spoil: func(fixture *adminFixture, player *Player) {
				player.forceKill()
			},
		},
		{
			// 方块实体在内存里但区块没加载：不看 loaded 就会顺着方块实体的编辑权把字写进去。
			Name: "区块没加载但方块实体还在",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.worlds["empty"] = world.New("empty")
				player.WorldName = "empty"
				fixture.srv.worlds["empty"].SetTile(world.NewSignTile(3, 64, 0, player.UUID))
			},
		},
		{
			Name: "那一格不是告示牌",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.world.SetBlock(3, 64, 0, blockrule.BlockStone, 0)
			},
		},
		{
			Name: "方块实体没了",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.world.RemoveTile(3, 64, 0)
			},
			Edit: func(fixture *adminFixture, player *Player) {
				signTestSend(fixture, player, [4]string{"不该进去", "", "", ""})
			},
			Gone: true,
		},
		{
			// 编辑权也给了玩家，这样只有 UpdateTile 里的 ID 判断能挡住它。
			Name: "方块实体不是告示牌但编辑权是他",
			Spoil: func(fixture *adminFixture, player *Player) {
				var chest world.Tile = world.NewChestTile(3, 64, 0)
				chest.Editor = player.UUID
				fixture.srv.world.SetTile(chest)
			},
		},
		{
			Name: "出生点保护里",
			Spoil: func(fixture *adminFixture, player *Player) {
				fixture.srv.cfg.SpawnProtection = 16
			},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item signRejectCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var fixture, player = signTestSetup(sub)
			item.Spoil(fixture, player)
			if item.Edit != nil {
				item.Edit(fixture, player)
			} else {
				signTestEdit(sub, fixture, player, [4]string{"不该进去", "", "", ""})
			}
			var tile, found = fixture.srv.world.TileAt(3, 64, 0)
			if !found {
				if !item.Gone {
					sub.Fatal("方块实体不该消失")
				}
				return
			}
			if tile.Lines != [4]string{} {
				sub.Fatalf("不该写得进去，实际 %q", tile.Lines)
			}
			// 换成别的方块实体时编辑权本来就不在，只要求没被写进字。
			if !item.Gone && tile.ID == world.TileSign && tile.Editor != player.UUID {
				sub.Fatalf("编辑权不该被清掉，实际 %q", tile.Editor)
			}
		})
	}
}

// 端到端：手里拿告示牌点地面，牌立起来，方块实体是空的，编辑权是点它的人。
func TestPlaceSignCreatesTile(test *testing.T) {
	var fixture, player = placeTestSetup(test)
	placeTestPut(test, fixture, 3, 63, 0, blockrule.BlockStone, 0)
	var held protocol.Item = placeTestHold(player, 323, 0, 3)
	placeTestUse(fixture, player, 3, 63, 0, blockrule.FaceUp, 1, held)
	var id, meta = placeTestBlock(fixture, 3, 64, 0)
	if id != world.BlockSignPost || meta != 8 {
		test.Fatalf("期望立牌 63:8，实际 %d:%d", id, meta)
	}
	var tile, found = fixture.srv.world.TileAt(3, 64, 0)
	if !found || tile.ID != world.TileSign || tile.Editor != player.UUID {
		test.Fatalf("应建好空告示牌并记下编辑权，实际 找到=%v 种类=%q 编辑权=%q", found, tile.ID, tile.Editor)
	}
	if tile.Lines != [4]string{} {
		test.Fatalf("新牌子应该四行都空，实际 %q", tile.Lines)
	}
}

// 拆掉告示牌，方块实体跟着删掉。
func TestBreakSignRemovesTile(test *testing.T) {
	var fixture, player = signTestSetup(test)
	updateTestCreative(player)
	updateTestBreak(fixture, player, 3, 64, 0)
	var _, found = fixture.srv.world.TileAt(3, 64, 0)
	if found {
		test.Fatal("方块实体应随告示牌一起删掉")
	}
}
