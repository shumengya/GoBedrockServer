package packets

import "fmt"

import "go-mc-server/internal/crafting"
import "go-mc-server/internal/protocol"

// 单个物品清单的长度上限，对齐参考端 CraftingEventPacket 的 128。
// 超了直接报错而不是截断：截断会让后面的字段错位。
const craftingEventMaxItems uint32 = 128

// CraftingDataPacket (0x35) 把全部配方发给客户端，客户端本地合成界面据此列出可做的东西。
//
// Source: pocketmine/network/mcpe/protocol/CraftingDataPacket.php
type CraftingDataPacket struct {
	Recipes      []crafting.Recipe
	CleanRecipes bool
}

func (pkt *CraftingDataPacket) ID() byte { return protocol.IDCraftingData }

func (pkt *CraftingDataPacket) Encode(stream *protocol.BinaryStream) error {
	stream.WriteUnsignedVarInt(uint32(len(pkt.Recipes)))
	var index int = 0
	for index = 0; index < len(pkt.Recipes); index++ {
		var err error = writeRecipe(stream, pkt.Recipes[index])
		if err != nil {
			return fmt.Errorf("写入第 %d 条配方: %w", index, err)
		}
	}
	stream.WriteBool(pkt.CleanRecipes)
	return nil
}

// Decode 服务端不会收到这个包。
func (pkt *CraftingDataPacket) Decode(stream *protocol.BinaryStream) error {
	return nil
}

func writeRecipe(stream *protocol.BinaryStream, recipe crafting.Recipe) error {
	stream.WriteVarInt(int32(recipe.Kind))
	switch recipe.Kind {
	case crafting.KindShapeless:
		stream.WriteUnsignedVarInt(uint32(len(recipe.Input)))
		writeSlots(stream, recipe.Input)
		writeFirstOutput(stream, recipe)
		stream.WriteUUID(recipe.ID)
	case crafting.KindShaped:
		stream.WriteVarInt(recipe.Width)
		stream.WriteVarInt(recipe.Height)
		writeSlots(stream, recipe.Input)
		writeFirstOutput(stream, recipe)
		stream.WriteUUID(recipe.ID)
	case crafting.KindFurnace:
		stream.WriteVarInt(recipe.Input[0].ID)
		stream.WriteSlot(recipe.Output[0])
	case crafting.KindFurnaceData:
		stream.WriteVarInt(recipe.Input[0].ID)
		stream.WriteVarInt(recipe.Input[0].Meta)
		stream.WriteSlot(recipe.Output[0])
	default:
		return fmt.Errorf("不支持的配方种类 %d", recipe.Kind)
	}
	return nil
}

func writeSlots(stream *protocol.BinaryStream, items []protocol.Item) {
	var index int = 0
	for index = 0; index < len(items); index++ {
		stream.WriteSlot(items[index])
	}
}

// writeFirstOutput 只发第一个产物，和参考端一致（它对多产物配方也只发第一个）。
// 蛋糕的 3 个空桶由服务端合成时补发。
func writeFirstOutput(stream *protocol.BinaryStream, recipe crafting.Recipe) {
	stream.WriteUnsignedVarInt(1)
	stream.WriteSlot(recipe.Output[0])
}

// CraftingEventPacket (0x36) 客户端点了合成产物时发来：配方 id、摆在格子里的材料、声称的产物。
//
// Source: pocketmine/network/mcpe/protocol/CraftingEventPacket.php
type CraftingEventPacket struct {
	WindowID byte
	Type     int32
	RecipeID [16]byte
	Input    []protocol.Item
	Output   []protocol.Item
}

func (pkt *CraftingEventPacket) ID() byte { return protocol.IDCraftingEvent }

func (pkt *CraftingEventPacket) Encode(stream *protocol.BinaryStream) error {
	var err error = stream.WriteByte(pkt.WindowID)
	if err != nil {
		return fmt.Errorf("写入窗口 id: %w", err)
	}
	stream.WriteVarInt(pkt.Type)
	stream.WriteUUID(pkt.RecipeID)
	stream.WriteUnsignedVarInt(uint32(len(pkt.Input)))
	writeSlots(stream, pkt.Input)
	stream.WriteUnsignedVarInt(uint32(len(pkt.Output)))
	writeSlots(stream, pkt.Output)
	return nil
}

func (pkt *CraftingEventPacket) Decode(stream *protocol.BinaryStream) error {
	var err error = nil
	pkt.WindowID, err = stream.ReadByte()
	if err != nil {
		return fmt.Errorf("读取窗口 id: %w", err)
	}
	pkt.Type, err = stream.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读取合成类型: %w", err)
	}
	pkt.RecipeID, err = stream.ReadUUID()
	if err != nil {
		return fmt.Errorf("读取配方 id: %w", err)
	}
	pkt.Input, err = readSlots(stream)
	if err != nil {
		return fmt.Errorf("读取材料: %w", err)
	}
	pkt.Output, err = readSlots(stream)
	if err != nil {
		return fmt.Errorf("读取产物: %w", err)
	}
	return nil
}

func readSlots(stream *protocol.BinaryStream) ([]protocol.Item, error) {
	var count uint32 = 0
	var err error = nil
	count, err = stream.ReadUnsignedVarInt()
	if err != nil {
		return nil, fmt.Errorf("读取数量: %w", err)
	}
	if count > craftingEventMaxItems {
		return nil, fmt.Errorf("物品数量 %d 超过上限 %d", count, craftingEventMaxItems)
	}
	var items []protocol.Item = make([]protocol.Item, 0, count)
	var index uint32 = 0
	for index = 0; index < count; index++ {
		var item protocol.Item = protocol.Item{}
		item, err = stream.ReadSlot()
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 格: %w", index, err)
		}
		items = append(items, item)
	}
	return items, nil
}
