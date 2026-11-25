package packets

import "testing"

import "go-mc-server/internal/crafting"
import "go-mc-server/internal/protocol"

// decodedRecipe 是测试里独立写的解码结果，不复用生产代码的写入逻辑，才能真正验证线格式。
type decodedRecipe struct {
	kind   int32
	width  int32
	height int32
	input  []protocol.Item
	output []protocol.Item
	uuid   [16]byte
}

func readItems(test *testing.T, stream *protocol.BinaryStream, count int) []protocol.Item {
	var items []protocol.Item = make([]protocol.Item, 0, count)
	var index int = 0
	for index = 0; index < count; index++ {
		var item, err = stream.ReadSlot()
		if err != nil {
			test.Fatalf("读取物品 %d: %v", index, err)
		}
		items = append(items, item)
	}
	return items
}

func readCount(test *testing.T, stream *protocol.BinaryStream) int {
	var count, err = stream.ReadUnsignedVarInt()
	if err != nil {
		test.Fatalf("读取数量: %v", err)
	}
	return int(count)
}

func decodeCraftingData(test *testing.T, raw []byte) ([]decodedRecipe, bool) {
	var stream *protocol.BinaryStream = protocol.NewStream(raw[1:])
	var total int = readCount(test, stream)
	var out []decodedRecipe = make([]decodedRecipe, 0, total)
	var index int = 0
	for index = 0; index < total; index++ {
		var recipe decodedRecipe = decodedRecipe{}
		var err error = nil
		recipe.kind, err = stream.ReadVarInt()
		if err != nil {
			test.Fatalf("读取种类: %v", err)
		}
		switch recipe.kind {
		case 0:
			recipe.input = readItems(test, stream, readCount(test, stream))
			recipe.output = readItems(test, stream, readCount(test, stream))
			recipe.uuid, err = stream.ReadUUID()
		case 1:
			recipe.width, err = stream.ReadVarInt()
			if err != nil {
				test.Fatalf("读取宽度: %v", err)
			}
			recipe.height, err = stream.ReadVarInt()
			if err != nil {
				test.Fatalf("读取高度: %v", err)
			}
			recipe.input = readItems(test, stream, int(recipe.width*recipe.height))
			recipe.output = readItems(test, stream, readCount(test, stream))
			recipe.uuid, err = stream.ReadUUID()
		case 2:
			var inputID int32 = 0
			inputID, err = stream.ReadVarInt()
			recipe.input = []protocol.Item{{ID: inputID, Meta: crafting.MetaAny}}
			recipe.output = readItems(test, stream, 1)
		case 3:
			var inputID int32 = 0
			var inputMeta int32 = 0
			inputID, err = stream.ReadVarInt()
			if err != nil {
				test.Fatalf("读取熔炼输入 id: %v", err)
			}
			inputMeta, err = stream.ReadVarInt()
			recipe.input = []protocol.Item{{ID: inputID, Meta: inputMeta}}
			recipe.output = readItems(test, stream, 1)
		default:
			test.Fatalf("第 %d 条出现未知种类 %d", index, recipe.kind)
		}
		if err != nil {
			test.Fatalf("第 %d 条配方解码失败: %v", index, err)
		}
		out = append(out, recipe)
	}
	var clean, err = stream.ReadBool()
	if err != nil || !clean {
		test.Fatalf("末尾 cleanRecipes 应为 true: %v %v", clean, err)
	}
	if !stream.EOF() {
		test.Fatalf("包没读完，剩 %d 字节", stream.Remaining())
	}
	return out, clean
}

// 整个内置配方书编码后能被原样读回：种类、尺寸、材料、产物、uuid 全部一致，字节流刚好读完。
func TestCraftingDataRoundTrip(test *testing.T) {
	var book, err = crafting.Default()
	if err != nil {
		test.Fatalf("加载配方: %v", err)
	}
	var raw []byte = nil
	raw, err = EncodePacket(&CraftingDataPacket{Recipes: book.Recipes(), CleanRecipes: true})
	if err != nil {
		test.Fatalf("编码: %v", err)
	}
	var decoded, _ = decodeCraftingData(test, raw)
	var source []crafting.Recipe = book.Recipes()
	if len(decoded) != len(source) {
		test.Fatalf("条数 %d != %d", len(decoded), len(source))
	}
	var index int = 0
	for index = 0; index < len(source); index++ {
		var want crafting.Recipe = source[index]
		var have decodedRecipe = decoded[index]
		if have.kind != int32(want.Kind) {
			test.Fatalf("第 %d 条种类 %d != %d", index, have.kind, want.Kind)
		}
		if len(have.input) != len(want.Input) || have.output[0].ID != want.Output[0].ID || have.output[0].Meta != want.Output[0].Meta {
			test.Fatalf("第 %d 条材料或产物不一致", index)
		}
		if want.Craftable() && have.uuid != want.ID {
			test.Fatalf("第 %d 条 uuid 不一致", index)
		}
		if want.Kind == crafting.KindShaped && (have.width != want.Width || have.height != want.Height) {
			test.Fatalf("第 %d 条尺寸不一致", index)
		}
	}
}

// 不限 meta 的材料（-1）经过线格式后要还原成 -1，否则客户端把「任意木板」当成 meta 0x7fff。
func TestCraftingDataWildcardMeta(test *testing.T) {
	var recipe crafting.Recipe = crafting.Recipe{
		Kind:   crafting.KindShapeless,
		Input:  []protocol.Item{{ID: 5, Meta: crafting.MetaAny, Count: 1}},
		Output: []protocol.Item{{ID: 280, Count: 4}},
	}
	var raw, err = EncodePacket(&CraftingDataPacket{Recipes: []crafting.Recipe{recipe}})
	if err != nil {
		test.Fatalf("编码: %v", err)
	}
	var stream *protocol.BinaryStream = protocol.NewStream(raw[1:])
	readCount(test, stream)
	var _, kindErr = stream.ReadVarInt()
	if kindErr != nil {
		test.Fatalf("读取种类: %v", kindErr)
	}
	var items []protocol.Item = readItems(test, stream, readCount(test, stream))
	if items[0].Meta != crafting.MetaAny {
		test.Fatalf("通配 meta 往返后变成 %d", items[0].Meta)
	}
}

// 不认识的配方种类要报错，不能悄悄写出半截包。
func TestCraftingDataRejectsUnknownKind(test *testing.T) {
	var _, err = EncodePacket(&CraftingDataPacket{Recipes: []crafting.Recipe{{Kind: crafting.Kind(99)}}})
	if err == nil {
		test.Fatal("未知种类必须报错")
	}
}

func TestCraftingEventRoundTrip(test *testing.T) {
	var original *CraftingEventPacket = &CraftingEventPacket{
		WindowID: 1,
		Type:     1,
		RecipeID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Input:    []protocol.Item{{ID: 5, Meta: 0, Count: 1}, protocol.Air(), {ID: 280, Count: 2}},
		Output:   []protocol.Item{{ID: 270, Count: 1}},
	}
	var raw, err = EncodePacket(original)
	if err != nil {
		test.Fatalf("编码: %v", err)
	}
	var decoded *CraftingEventPacket = &CraftingEventPacket{}
	err = decoded.Decode(protocol.NewStream(raw[1:]))
	if err != nil {
		test.Fatalf("解码: %v", err)
	}
	if decoded.WindowID != 1 || decoded.Type != 1 || decoded.RecipeID != original.RecipeID {
		test.Fatalf("头部字段不一致: %+v", decoded)
	}
	if len(decoded.Input) != 3 || decoded.Input[0].ID != 5 || !decoded.Input[1].IsAir() || decoded.Input[2].Count != 2 {
		test.Fatalf("材料不一致: %+v", decoded.Input)
	}
	if len(decoded.Output) != 1 || decoded.Output[0].ID != 270 {
		test.Fatalf("产物不一致: %+v", decoded.Output)
	}
}

// 恶意客户端报一个巨大的物品数，必须当场拒绝，不能照着去分配内存。
func TestCraftingEventRejectsHugeCount(test *testing.T) {
	var stream *protocol.BinaryStream = protocol.NewWriter()
	var writeErr error = stream.WriteByte(0)
	if writeErr != nil {
		test.Fatalf("写入窗口 id: %v", writeErr)
	}
	stream.WriteVarInt(0)
	stream.WriteUUID([16]byte{})
	stream.WriteUnsignedVarInt(1 << 30)
	var decoded *CraftingEventPacket = &CraftingEventPacket{}
	var err error = decoded.Decode(protocol.NewStream(stream.Bytes()))
	if err == nil {
		test.Fatal("超大物品数必须报错")
	}
}

// 属性包要带上真实血量和饥饿值，同步饥饿时不能把血量顶回满。
func TestPlayerAttributesCarryVitals(test *testing.T) {
	var attrs []Attribute = PlayerAttributes(PlayerVitals{Health: 7, MaxHealth: 20, Hunger: 13, Saturation: 2.5, Exhaustion: 1.25})
	if len(attrs) != 11 {
		test.Fatalf("属性数应为 11，实际 %d", len(attrs))
	}
	if attrs[4].Name != "minecraft:health" || attrs[4].Value != 7 {
		test.Fatalf("血量槽: %+v", attrs[4])
	}
	if attrs[7].Name != "minecraft:player.hunger" || attrs[7].Value != 13 {
		test.Fatalf("饥饿槽: %+v", attrs[7])
	}
	if attrs[1].Name != "minecraft:player.saturation" || attrs[1].Value != 2.5 {
		test.Fatalf("饱和度槽: %+v", attrs[1])
	}
	if attrs[2].Name != "minecraft:player.exhaustion" || attrs[2].Value != 1.25 {
		test.Fatalf("疲劳度槽: %+v", attrs[2])
	}
}

func TestEntityEventRoundTrip(test *testing.T) {
	var raw, err = EncodePacket(&EntityEventPacket{EntityRuntimeID: 7, Event: EntityEventUseItem, Data: 3})
	if err != nil {
		test.Fatalf("编码: %v", err)
	}
	var decoded *EntityEventPacket = &EntityEventPacket{}
	err = decoded.Decode(protocol.NewStream(raw[1:]))
	if err != nil || decoded.EntityRuntimeID != 7 || decoded.Event != 9 || decoded.Data != 3 {
		test.Fatalf("往返不一致: %+v err=%v", decoded, err)
	}
}
