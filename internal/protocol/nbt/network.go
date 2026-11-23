package nbt

import "go-mc-server/internal/protocol"

// PocketMine Spawnable uses NBT::LITTLE_ENDIAN + write(true): network (varint)
// ints and unsigned-varint string lengths, concatenated after FullChunkData extraData.
const (
	TagEnd      byte = 0
	TagByte     byte = 1
	TagShort    byte = 2
	TagInt      byte = 3
	TagString   byte = 8
	TagCompound byte = 10

	IDChest      = "Chest"
	IDBed        = "Bed"
	IDFurnace    = "Furnace"
	IDSign       = "Sign"
	IDEnderChest = "EnderChest"
	IDFlowerPot  = "FlowerPot"
	IDSkull      = "Skull"
	IDItemFrame  = "ItemFrame"
	IDHopper     = "Hopper"
)

type Writer struct {
	s *protocol.BinaryStream
}

func WriteNetworkCompound(name string, write func(w *Writer)) []byte {
	w := &Writer{s: protocol.NewWriter()}
	_ = w.s.WriteByte(TagCompound)
	w.stringRaw(name)
	write(w)
	_ = w.s.WriteByte(TagEnd)
	return w.s.Bytes()
}

func (w *Writer) String(name, v string) {
	_ = w.s.WriteByte(TagString)
	w.stringRaw(name)
	w.stringRaw(v)
}

func (w *Writer) Int(name string, v int32) {
	_ = w.s.WriteByte(TagInt)
	w.stringRaw(name)
	w.s.WriteVarInt(v)
}

func (w *Writer) Byte(name string, v byte) {
	_ = w.s.WriteByte(TagByte)
	w.stringRaw(name)
	_ = w.s.WriteByte(v)
}

func (w *Writer) stringRaw(v string) {
	w.s.WriteUnsignedVarInt(uint32(len(v)))
	w.s.WriteBytes([]byte(v))
}

// Float 写一个 32 位浮点。网络格式里浮点也是小端原样写，不走变长编码。
func (w *Writer) Float(name string, v float32) {
	_ = w.s.WriteByte(TagFloat)
	w.stringRaw(name)
	w.s.WriteLFloat(v)
}

// Compound 开一个嵌套的复合标签，里面的写法和顶层一样，函数返回时自动收尾。
func (w *Writer) Compound(name string, write func(w *Writer)) {
	_ = w.s.WriteByte(TagCompound)
	w.stringRaw(name)
	write(w)
	_ = w.s.WriteByte(TagEnd)
}

func SpawnChest(x, y, z int32) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDChest)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
	})
}

func SpawnBed(x, y, z int32, color byte) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDBed)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
		w.Byte("color", color)
	})
}

func (w *Writer) Short(name string, v int16) {
	_ = w.s.WriteByte(TagShort)
	w.stringRaw(name)
	w.s.WriteInt16LE(v)
}

// SpawnPairedChest 是已经和相邻箱子配成大箱子的箱子，客户端靠 pairx、pairz 把两个画成一个。
func SpawnPairedChest(x, y, z int32, pairX int32, pairZ int32) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDChest)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
		w.Int("pairx", pairX)
		w.Int("pairz", pairZ)
	})
}

// SpawnEnderChest 末影箱的方块实体只带坐标，里面的东西在玩家身上，不在方块里。
func SpawnEnderChest(x, y, z int32) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDEnderChest)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
	})
}

// SpawnFurnace 带上烧制进度，客户端据此画火焰。
func SpawnFurnace(x, y, z int32, burnTime int16, cookTime int16) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDFurnace)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
		w.Short("BurnTime", burnTime)
		w.Short("CookTime", cookTime)
	})
}

// SpawnSign 告示牌的四行字。
func SpawnSign(x, y, z int32, lines [4]string) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDSign)
		w.String("Text1", lines[0])
		w.String("Text2", lines[1])
		w.String("Text3", lines[2])
		w.String("Text4", lines[3])
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
	})
}

// SpawnFlowerPot 花盆里种的东西。里面空着时 item 是 0，客户端画空花盆。
// item 是短整型、mData 是整型，这两个类型参考端就是这么写的，别改。
func SpawnFlowerPot(x, y, z int32, itemID int16, itemMeta int32) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDFlowerPot)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
		w.Short("item", itemID)
		w.Int("mData", itemMeta)
	})
}

// SpawnSkull 头颅：种类和朝上的那一格的旋转角度。朝侧面放时 Rot 恒为 0，靠方块 meta 定朝向。
func SpawnSkull(x, y, z int32, skullType byte, rot byte) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDSkull)
		w.Byte("SkullType", skullType)
		w.Byte("Rot", rot)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
	})
}

// SpawnItemFrame 物品展示框：里面那张画的旋转角，以及画本身。空框不写 Item。
func SpawnItemFrame(x, y, z int32, rotation byte, item protocol.Item) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDItemFrame)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
		w.Float("ItemDropChance", 1)
		w.Byte("ItemRotation", rotation)
		if item.IsAir() {
			return
		}
		w.Compound("Item", func(itemWriter *Writer) {
			itemWriter.Short("id", int16(item.ID))
			itemWriter.Byte("Count", item.Count)
			itemWriter.Short("Damage", int16(item.Meta))
			itemWriter.s.WriteBytes(frameItemTag(item.NBT))
		})
	})
}

// SpawnHopper 漏斗的方块实体只需要坐标，格子内容客户端不画。
func SpawnHopper(x, y, z int32) []byte {
	return WriteNetworkCompound("", func(w *Writer) {
		w.String("id", IDHopper)
		w.Int("x", x)
		w.Int("y", y)
		w.Int("z", z)
	})
}
