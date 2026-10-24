package nbt

import "go-mc-server/internal/protocol"

// PocketMine Spawnable uses NBT::LITTLE_ENDIAN + write(true): network (varint)
// ints and unsigned-varint string lengths, concatenated after FullChunkData extraData.
const (
	TagEnd      byte = 0
	TagByte     byte = 1
	TagInt      byte = 3
	TagString   byte = 8
	TagCompound byte = 10

	IDChest = "Chest"
	IDBed   = "Bed"
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
