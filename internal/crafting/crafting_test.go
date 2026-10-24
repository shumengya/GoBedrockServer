package crafting

import "testing"

import "go-mc-server/internal/protocol"

func loadBook(test *testing.T) *Book {
	var book, err = Default()
	if err != nil {
		test.Fatalf("加载内置配方失败: %v", err)
	}
	return book
}

// findByOutput 找产物是 id:meta 的第一条指定种类的配方。
func findByOutput(book *Book, kind Kind, outputID int32, outputMeta int32) (Recipe, bool) {
	var list []Recipe = book.Recipes()
	var index int = 0
	for index = 0; index < len(list); index++ {
		var recipe Recipe = list[index]
		if recipe.Kind != kind || len(recipe.Output) == 0 {
			continue
		}
		if recipe.Output[0].ID == outputID && recipe.Output[0].Meta == outputMeta {
			return recipe, true
		}
	}
	return Recipe{}, false
}

func mustFind(test *testing.T, book *Book, kind Kind, outputID int32, outputMeta int32) Recipe {
	var recipe, found = findByOutput(book, kind, outputID, outputMeta)
	if !found {
		test.Fatalf("配方书里找不到产物 %d:%d", outputID, outputMeta)
	}
	return recipe
}

func cell(id int32, meta int32) protocol.Item {
	return protocol.Item{ID: id, Meta: meta, Count: 1}
}

// matchesGrid 只关心能不能匹配，不关心扣哪些料。
func matchesGrid(recipe Recipe, input []protocol.Item) bool {
	var _, matched = recipe.Match(input)
	return matched
}

func grid3() []protocol.Item {
	return make([]protocol.Item, 9)
}

// 配方数量对得上参考端数据：0/1 型进合成表，2/3 型进熔炼表，4/5 型跳过。
func TestLoadCounts(test *testing.T) {
	var book *Book = loadBook(test)
	var shapeless int = 0
	var shaped int = 0
	var furnace int = 0
	var list []Recipe = book.Recipes()
	var index int = 0
	for index = 0; index < len(list); index++ {
		switch list[index].Kind {
		case KindShapeless:
			shapeless++
		case KindShaped:
			shaped++
		case KindFurnace, KindFurnaceData:
			furnace++
		}
	}
	if shapeless != 73 || shaped != 342 || furnace != 67 {
		test.Fatalf("无序 %d 有序 %d 熔炼 %d，期望 73/342/67", shapeless, shaped, furnace)
	}
	if book.Skipped() != 20 {
		test.Fatalf("应跳过 20 条（多配方 4 + 潜影盒 16），实际 %d", book.Skipped())
	}
}

// 所有合成配方都能用自己的 id 查回来。
func TestFindByID(test *testing.T) {
	var book *Book = loadBook(test)
	var list []Recipe = book.Recipes()
	var index int = 0
	for index = 0; index < len(list); index++ {
		if !list[index].Craftable() {
			continue
		}
		var found, ok = book.Find(list[index].ID)
		if !ok || found.Output[0].ID != list[index].Output[0].ID {
			test.Fatalf("第 %d 条配方按 id 查不回来", index)
		}
	}
	var _, missing = book.Find([16]byte{1, 2, 3})
	if missing {
		test.Fatal("不存在的 id 不该查到")
	}
}

// 原木 → 木板是 1×1 有序配方，每种原木对应一种木板（17:1 云杉 → 5:1），一块出 4 块。
func TestShapedPlanksFollowLogType(test *testing.T) {
	var book *Book = loadBook(test)
	var oak Recipe = mustFind(test, book, KindShaped, 5, 0)
	var spruce Recipe = mustFind(test, book, KindShaped, 5, 1)
	var input []protocol.Item = grid3()
	input[4] = cell(17, 0)
	var consumed, matched = oak.Match(input)
	if !matched || len(consumed) != 1 || consumed[0].ID != 17 {
		test.Fatalf("橡木合成木板失败: matched=%v consumed=%+v", matched, consumed)
	}
	if oak.Output[0].Count != 4 {
		test.Fatalf("一块原木应出 4 块木板，实际 %d", oak.Output[0].Count)
	}
	if matchesGrid(spruce, input) {
		test.Fatal("橡木不能合成云杉木板")
	}
}

// 蘑菇煲是真正的无序配方：红蘑菇、棕蘑菇（不限 meta）加一个碗，怎么摆都行，摆多摆少都不行。
func TestShapelessAnyOrder(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShapeless, 282, 0)
	var forward []protocol.Item = grid3()
	forward[0], forward[1], forward[2] = cell(39, 0), cell(40, 0), cell(281, 0)
	var consumed, matched = recipe.Match(forward)
	if !matched || len(consumed) != 3 {
		test.Fatalf("正序摆放应能合成: matched=%v consumed=%+v", matched, consumed)
	}
	var scrambled []protocol.Item = grid3()
	scrambled[8], scrambled[3], scrambled[4] = cell(281, 0), cell(40, 0), cell(39, 0)
	if !matchesGrid(recipe, scrambled) {
		test.Fatal("乱序、散在不同格子也应能合成")
	}
	var missing []protocol.Item = grid3()
	missing[0], missing[1] = cell(39, 0), cell(40, 0)
	if matchesGrid(recipe, missing) {
		test.Fatal("少一个碗必须拒绝")
	}
	var doubled []protocol.Item = grid3()
	doubled[0], doubled[1], doubled[2], doubled[3] = cell(39, 0), cell(39, 0), cell(40, 0), cell(281, 0)
	if matchesGrid(recipe, doubled) {
		test.Fatal("多放一个红蘑菇必须拒绝")
	}
}

// 有序配方摆在合成格哪个位置都认：2 块木板竖着放出木棍，放在最右下角也行。
func TestShapedStickAnywhere(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 280, 0)
	var corner []protocol.Item = grid3()
	corner[5] = cell(5, 0)
	corner[8] = cell(5, 0)
	var consumed, matched = recipe.Match(corner)
	if !matched || len(consumed) != 2 {
		test.Fatalf("右下角竖放木板应能合成木棍: matched=%v consumed=%+v", matched, consumed)
	}
	var topLeft []protocol.Item = grid3()
	topLeft[0] = cell(5, 0)
	topLeft[3] = cell(5, 0)
	if !matchesGrid(recipe, topLeft) {
		test.Fatal("左上角竖放木板也应能合成")
	}
}

// 不限 meta 的材料会按玩家实际摆的物品扣，扣的是那种木板而不是笼统的 5:0。
func TestWildcardConsumesActualMeta(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 280, 0)
	var input []protocol.Item = grid3()
	input[1] = cell(5, 2)
	input[4] = cell(5, 2)
	var consumed, matched = recipe.Match(input)
	if !matched {
		test.Fatal("桦木板应能当木棍材料")
	}
	if consumed[0].Meta != 2 || consumed[1].Meta != 2 {
		test.Fatalf("应扣桦木板 5:2，实际 %+v", consumed)
	}
}

// 斧头配方是不对称的，镜像摆放也要认。
func TestShapedMirror(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 271, 0)
	var normal []protocol.Item = grid3()
	normal[0], normal[1] = cell(5, 0), cell(5, 0)
	normal[3], normal[4] = cell(5, 0), cell(280, 0)
	normal[7] = cell(280, 0)
	if !matchesGrid(recipe, normal) {
		test.Fatal("标准摆法应能合成木斧")
	}
	var mirrored []protocol.Item = grid3()
	mirrored[1], mirrored[2] = cell(5, 0), cell(5, 0)
	mirrored[4], mirrored[5] = cell(280, 0), cell(5, 0)
	mirrored[7] = cell(280, 0)
	if !matchesGrid(recipe, mirrored) {
		test.Fatal("镜像摆法也应能合成木斧")
	}
}

// 多放一格、少放一格、放错材料都必须拒绝。
func TestMatchRejectsWrongLayouts(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 280, 0)
	var extra []protocol.Item = grid3()
	extra[0], extra[3], extra[6] = cell(5, 0), cell(5, 0), cell(5, 0)
	if matchesGrid(recipe, extra) {
		test.Fatal("三块木板竖排不是木棍配方，必须拒绝")
	}
	var single []protocol.Item = grid3()
	single[4] = cell(5, 0)
	if matchesGrid(recipe, single) {
		test.Fatal("只放一块木板必须拒绝")
	}
	var wrong []protocol.Item = grid3()
	wrong[0], wrong[3] = cell(5, 0), cell(1, 0)
	if matchesGrid(recipe, wrong) {
		test.Fatal("木板加石头不是木棍配方")
	}
	var stray []protocol.Item = grid3()
	stray[0], stray[3], stray[8] = cell(5, 0), cell(5, 0), cell(3, 0)
	if matchesGrid(recipe, stray) {
		test.Fatal("格子里混进多余物品必须拒绝")
	}
}

// 背包 2×2：输入只有 4 格也能合成；长度不是 4 或 9 一律不认。
func TestSmallGridAndBadLength(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 58, 0)
	if recipe.NeedsBench() {
		test.Fatal("工作台本身的配方是 2×2，不该要求工作台")
	}
	var small []protocol.Item = []protocol.Item{cell(5, 0), cell(5, 0), cell(5, 0), cell(5, 0)}
	if !matchesGrid(recipe, small) {
		test.Fatal("2×2 摆 4 块木板应能合成工作台")
	}
	if matchesGrid(recipe, small[:3]) {
		test.Fatal("长度 3 的输入不合法")
	}
	var oversize []protocol.Item = make([]protocol.Item, 10)
	if matchesGrid(recipe, oversize) {
		test.Fatal("长度 10 的输入不合法")
	}
}

// 3×3 的配方（镐子）必须标成要工作台。
func TestNeedsBench(test *testing.T) {
	var book *Book = loadBook(test)
	var pickaxe Recipe = mustFind(test, book, KindShaped, 270, 0)
	if !pickaxe.NeedsBench() {
		test.Fatal("木镐是 3×3 配方，必须要工作台")
	}
}

// 产物校验只比 id 和 meta，不比数量。
func TestOutputMatches(test *testing.T) {
	var book *Book = loadBook(test)
	var recipe Recipe = mustFind(test, book, KindShaped, 5, 0)
	if !recipe.OutputMatches([]protocol.Item{{ID: 5, Meta: 0, Count: 1}}) {
		test.Fatal("id/meta 一致应通过，数量不参与比较")
	}
	if recipe.OutputMatches([]protocol.Item{{ID: 5, Meta: 1, Count: 4}}) {
		test.Fatal("meta 不同应拒绝")
	}
	if recipe.OutputMatches(nil) {
		test.Fatal("没有产物应拒绝")
	}
}

// 熔炼配方不能当合成配方用。
func TestFurnaceRecipesNotCraftable(test *testing.T) {
	var book *Book = loadBook(test)
	var smelt Recipe = mustFind(test, book, KindFurnace, 1, 0)
	if smelt.Craftable() {
		test.Fatal("熔炼配方不可合成")
	}
	if matchesGrid(smelt, grid3()) {
		test.Fatal("熔炼配方不该匹配合成格")
	}
}

func TestParseUUID(test *testing.T) {
	var id, err = ParseUUID("03a2ec83-9100-3990-8c3c-ea27b9f23e57")
	if err != nil || id[0] != 0x03 || id[15] != 0x57 {
		test.Fatalf("id=%x err=%v", id, err)
	}
	_, err = ParseUUID("not-a-uuid")
	if err == nil {
		test.Fatal("非法 uuid 必须报错")
	}
}
