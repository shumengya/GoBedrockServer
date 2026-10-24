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

	IDChest   = "Chest"
	IDBed     = "Bed"
	IDFurnace = "Furnace"
	IDSign    = "Sign"
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
