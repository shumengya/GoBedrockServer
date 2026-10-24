package protocol

import (
	"encoding/binary"
	"errors"
	"math"
)

// BinaryStream provides read/write methods matching the MCPE binary protocol.
// Strings are length-prefixed with an UnsignedVarInt; multi-byte integers are
// little-endian unless noted; floats are little-endian (LFloat).
type BinaryStream struct {
	buf    []byte
	offset int
}

// NewStream creates a BinaryStream wrapping existing data for reading.
func NewStream(data []byte) *BinaryStream {
	return &BinaryStream{buf: data}
}

// NewWriter creates an empty BinaryStream for writing.
func NewWriter() *BinaryStream {
	return &BinaryStream{}
}

// Bytes returns the underlying buffer (for writing use).
func (s *BinaryStream) Bytes() []byte {
	return s.buf
}

// Remaining returns how many bytes are left to read.
func (s *BinaryStream) Remaining() int {
	return len(s.buf) - s.offset
}

// EOF returns true when there is nothing left to read.
func (s *BinaryStream) EOF() bool {
	return s.offset >= len(s.buf)
}

// reset clears the buffer and resets the offset.
func (s *BinaryStream) reset() {
	s.buf = s.buf[:0]
	s.offset = 0
}

// ---- Raw helpers ----

func (s *BinaryStream) read(n int) ([]byte, error) {
	if s.offset+n > len(s.buf) {
		return nil, errors.New("not enough bytes to read")
	}
	b := s.buf[s.offset : s.offset+n]
	s.offset += n
	return b, nil
}

func (s *BinaryStream) write(b []byte) {
	s.buf = append(s.buf, b...)
}

// ---- Bool ----

func (s *BinaryStream) ReadBool() (bool, error) {
	b, err := s.read(1)
	if err != nil {
		return false, err
	}
	return b[0] != 0, nil
}

func (s *BinaryStream) WriteBool(v bool) {
	if v {
		s.buf = append(s.buf, 0x01)
	} else {
		s.buf = append(s.buf, 0x00)
	}
}

// ---- Byte ----

func (s *BinaryStream) ReadByte() (byte, error) {
	b, err := s.read(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (s *BinaryStream) WriteByte(v byte) error {
	s.buf = append(s.buf, v)
	return nil
}

// ---- Big-endian integers (used by RakNet layer) ----

func (s *BinaryStream) ReadUint16BE() (uint16, error) {
	b, err := s.read(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (s *BinaryStream) WriteUint16BE(v uint16) {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadUint32BE() (uint32, error) {
	b, err := s.read(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

func (s *BinaryStream) WriteUint32BE(v uint32) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadInt32BE() (int32, error) {
	v, err := s.ReadUint32BE()
	return int32(v), err
}

func (s *BinaryStream) WriteInt32BE(v int32) {
	s.WriteUint32BE(uint32(v))
}

func (s *BinaryStream) ReadUint64BE() (uint64, error) {
	b, err := s.read(8)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

func (s *BinaryStream) WriteUint64BE(v uint64) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadInt64BE() (int64, error) {
	v, err := s.ReadUint64BE()
	return int64(v), err
}

func (s *BinaryStream) WriteInt64BE(v int64) {
	s.WriteUint64BE(uint64(v))
}

// ---- Little-endian integers (MCPE game packets) ----

func (s *BinaryStream) ReadUint16LE() (uint16, error) {
	b, err := s.read(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (s *BinaryStream) WriteUint16LE(v uint16) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadInt16LE() (int16, error) {
	v, err := s.ReadUint16LE()
	return int16(v), err
}

func (s *BinaryStream) WriteInt16LE(v int16) {
	s.WriteUint16LE(uint16(v))
}

func (s *BinaryStream) ReadUint32LE() (uint32, error) {
	b, err := s.read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (s *BinaryStream) WriteUint32LE(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadInt32LE() (int32, error) {
	v, err := s.ReadUint32LE()
	return int32(v), err
}

func (s *BinaryStream) WriteInt32LE(v int32) {
	s.WriteUint32LE(uint32(v))
}

func (s *BinaryStream) ReadUint64LE() (uint64, error) {
	b, err := s.read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (s *BinaryStream) WriteUint64LE(v uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	s.write(b)
}

func (s *BinaryStream) ReadInt64LE() (int64, error) {
	v, err := s.ReadUint64LE()
	return int64(v), err
}

func (s *BinaryStream) WriteInt64LE(v int64) {
	s.WriteUint64LE(uint64(v))
}

// ---- Little-endian float (LFloat) ----

func (s *BinaryStream) ReadLFloat() (float32, error) {
	b, err := s.read(4)
	if err != nil {
		return 0, err
	}
	bits := binary.LittleEndian.Uint32(b)
	return math.Float32frombits(bits), nil
}

func (s *BinaryStream) WriteLFloat(v float32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
	s.write(b)
}

// ---- VarInt (zigzag-encoded signed int32) ----

func (s *BinaryStream) ReadVarInt() (int32, error) {
	u, err := s.ReadUnsignedVarInt()
	if err != nil {
		return 0, err
	}
	// zigzag decode
	return int32((u >> 1) ^ -(u & 1)), nil
}

func (s *BinaryStream) WriteVarInt(v int32) {
	// zigzag encode
	u := uint32((v << 1) ^ (v >> 31))
	s.WriteUnsignedVarInt(u)
}

// ---- UnsignedVarInt (varint uint32) ----

func (s *BinaryStream) ReadUnsignedVarInt() (uint32, error) {
	var result uint32
	var shift uint
	for {
		if s.offset >= len(s.buf) {
			return 0, errors.New("varint: unexpected EOF")
		}
		b := s.buf[s.offset]
		s.offset++
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 35 {
			return 0, errors.New("varint: overflow")
		}
	}
	return result, nil
}

func (s *BinaryStream) WriteUnsignedVarInt(v uint32) {
	for v >= 0x80 {
		s.buf = append(s.buf, byte(v)|0x80)
		v >>= 7
	}
	s.buf = append(s.buf, byte(v))
}

// ---- VarLong (zigzag-encoded signed int64) ----

func (s *BinaryStream) ReadVarLong() (int64, error) {
	u, err := s.ReadUnsignedVarLong()
	if err != nil {
		return 0, err
	}
	return int64((u >> 1) ^ -(u & 1)), nil
}

func (s *BinaryStream) WriteVarLong(v int64) {
	u := uint64((v << 1) ^ (v >> 63))
	s.WriteUnsignedVarLong(u)
}

// ---- UnsignedVarLong ----

func (s *BinaryStream) ReadUnsignedVarLong() (uint64, error) {
	var result uint64
	var shift uint
	for {
		if s.offset >= len(s.buf) {
			return 0, errors.New("varlong: unexpected EOF")
		}
		b := s.buf[s.offset]
		s.offset++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 70 {
			return 0, errors.New("varlong: overflow")
		}
	}
	return result, nil
}

func (s *BinaryStream) WriteUnsignedVarLong(v uint64) {
	for v >= 0x80 {
		s.buf = append(s.buf, byte(v)|0x80)
		v >>= 7
	}
	s.buf = append(s.buf, byte(v))
}

// ---- String (UnsignedVarInt length prefix + UTF-8 bytes) ----

func (s *BinaryStream) ReadString() (string, error) {
	length, err := s.ReadUnsignedVarInt()
	if err != nil {
		return "", err
	}
	b, err := s.read(int(length))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *BinaryStream) WriteString(v string) {
	s.WriteUnsignedVarInt(uint32(len(v)))
	s.write([]byte(v))
}

// ---- EntityId (VarInt — 32-bit zigzag) ----
//
// Source: BinaryStream.php → putEntityId() calls putVarInt(), NOT putVarLong().
// Using VarLong would corrupt every packet that carries an entity ID.
func (s *BinaryStream) ReadEntityID() (int64, error) {
	v, err := s.ReadVarInt()
	return int64(v), err
}

func (s *BinaryStream) WriteEntityID(v int64) {
	s.WriteVarInt(int32(v))
}

// ---- UUID (4 little-endian int32, shuffled order: part1, part0, part3, part2) ----
//
// Source: BinaryStream.php → putUUID():
//
//	$this->putLInt($uuid->getPart(1));
//	$this->putLInt($uuid->getPart(0));
//	$this->putLInt($uuid->getPart(3));
//	$this->putLInt($uuid->getPart(2));
//
// where parts[0..3] are the four 4-byte chunks of the UUID read in
// BIG-endian order from the canonical 16-byte hex form.
//
// Input: 16 raw bytes from a UUID, ordered as in the canonical
// "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" string.
func (s *BinaryStream) WriteUUID(b [16]byte) {
	// part0 = bytes 0..3, part1 = bytes 4..7, part2 = bytes 8..11, part3 = bytes 12..15
	// putLInt of partN = write those four bytes reversed (BE → LE).
	// Order written: part1, part0, part3, part2.
	s.write([]byte{b[7], b[6], b[5], b[4]})     // part1 LE
	s.write([]byte{b[3], b[2], b[1], b[0]})     // part0 LE
	s.write([]byte{b[15], b[14], b[13], b[12]}) // part3 LE
	s.write([]byte{b[11], b[10], b[9], b[8]})   // part2 LE
}

func (s *BinaryStream) ReadUUID() ([16]byte, error) {
	var out [16]byte
	buf, err := s.read(16)
	if err != nil {
		return out, err
	}
	// Reverse-of-write: reconstruct the canonical 16 bytes.
	out[7], out[6], out[5], out[4] = buf[0], buf[1], buf[2], buf[3]
	out[3], out[2], out[1], out[0] = buf[4], buf[5], buf[6], buf[7]
	out[15], out[14], out[13], out[12] = buf[8], buf[9], buf[10], buf[11]
	out[11], out[10], out[9], out[8] = buf[12], buf[13], buf[14], buf[15]
	return out, nil
}

// ---- BlockCoords (VarInt x, UnsignedVarInt y, VarInt z) ----

func (s *BinaryStream) ReadBlockCoords() (x, y, z int32, err error) {
	if x, err = s.ReadVarInt(); err != nil {
		return
	}
	yu, e := s.ReadUnsignedVarInt()
	if e != nil {
		err = e
		return
	}
	y = int32(yu)
	z, err = s.ReadVarInt()
	return
}

func (s *BinaryStream) WriteBlockCoords(x, y, z int32) {
	s.WriteVarInt(x)
	s.WriteUnsignedVarInt(uint32(y))
	s.WriteVarInt(z)
}

// ---- Vector3f (LFloat x, LFloat y, LFloat z) ----

func (s *BinaryStream) ReadVector3f() (x, y, z float32, err error) {
	if x, err = s.ReadLFloat(); err != nil {
		return
	}
	if y, err = s.ReadLFloat(); err != nil {
		return
	}
	z, err = s.ReadLFloat()
	return
}

func (s *BinaryStream) WriteVector3f(x, y, z float32) {
	s.WriteLFloat(x)
	s.WriteLFloat(y)
	s.WriteLFloat(z)
}

// ---- Byte rotation (degrees packed into a byte, 360/256) ----
//
// Source: DataPacket.php getByteRotation / putByteRotation.

func (s *BinaryStream) ReadByteRotation() (float32, error) {
	b, err := s.ReadByte()
	if err != nil {
		return 0, err
	}
	return float32(b) * (360.0 / 256.0), nil
}

func (s *BinaryStream) WriteByteRotation(deg float32) {
	v := int(deg / (360.0 / 256.0))
	_ = s.WriteByte(byte(v))
}

// ---- Raw slice ----

func (s *BinaryStream) ReadBytes(n int) ([]byte, error) {
	return s.read(n)
}

func (s *BinaryStream) WriteBytes(b []byte) {
	s.write(b)
}

// ReadRemaining returns all bytes from the current offset to the end.
func (s *BinaryStream) ReadRemaining() []byte {
	b := s.buf[s.offset:]
	s.offset = len(s.buf)
	return b
}

// 3-byte little-endian triad (used by RakNet sequence numbers).
func ReadTriadLE(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func WriteTriadLE(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16)}
}
