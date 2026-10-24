package raknet

import (
	"encoding/binary"
	"fmt"
	"go-mc-server/internal/protocol"
)

// Reliability types.
const (
	Unreliable         = 0
	UnreliableSequenced = 1
	Reliable            = 2
	ReliableOrdered     = 3
	ReliableSequenced   = 4
)

// EncapsulatedPacket is a single payload wrapped inside a RakNet datagram.
type EncapsulatedPacket struct {
	Reliability   byte
	HasSplit      bool
	MessageIndex  uint32 // present for reliable packets
	SequenceIndex uint32 // present for sequenced/ordered
	OrderIndex    uint32 // present for ordered
	OrderChannel  byte

	// Split packet fields.
	SplitCount uint32
	SplitID    uint16
	SplitIndex uint32

	Payload []byte
}

// Decode reads one EncapsulatedPacket from the given buffer at the given offset.
// Returns the number of bytes consumed.
func decodeEncapsulated(buf []byte, offset int) (*EncapsulatedPacket, int, error) {
	start := offset
	if offset >= len(buf) {
		return nil, 0, fmt.Errorf("encapsulated: empty buffer")
	}

	flags := buf[offset]
	offset++

	ep := &EncapsulatedPacket{}
	ep.Reliability = (flags >> 5) & 0x07
	ep.HasSplit = (flags & 0x10) != 0

	// Length is in bits (big-endian uint16).
	if offset+2 > len(buf) {
		return nil, 0, fmt.Errorf("encapsulated: no length bytes")
	}
	bitLength := binary.BigEndian.Uint16(buf[offset : offset+2])
	offset += 2
	payloadLen := int(bitLength+7) / 8

	if isReliable(ep.Reliability) {
		if offset+3 > len(buf) {
			return nil, 0, fmt.Errorf("encapsulated: no message index")
		}
		ep.MessageIndex = protocol.ReadTriadLE(buf[offset : offset+3])
		offset += 3
	}

	if isSequenced(ep.Reliability) {
		if offset+3 > len(buf) {
			return nil, 0, fmt.Errorf("encapsulated: no sequence index")
		}
		ep.SequenceIndex = protocol.ReadTriadLE(buf[offset : offset+3])
		offset += 3
	}

	if isOrdered(ep.Reliability) {
		if offset+4 > len(buf) {
			return nil, 0, fmt.Errorf("encapsulated: no order index/channel")
		}
		ep.OrderIndex = protocol.ReadTriadLE(buf[offset : offset+3])
		offset += 3
		ep.OrderChannel = buf[offset]
		offset++
	}

	if ep.HasSplit {
		if offset+10 > len(buf) {
			return nil, 0, fmt.Errorf("encapsulated: no split fields")
		}
		ep.SplitCount = binary.BigEndian.Uint32(buf[offset : offset+4])
		offset += 4
		ep.SplitID = binary.BigEndian.Uint16(buf[offset : offset+2])
		offset += 2
		ep.SplitIndex = binary.BigEndian.Uint32(buf[offset : offset+4])
		offset += 4
	}

	if offset+payloadLen > len(buf) {
		return nil, 0, fmt.Errorf("encapsulated: payload truncated (need %d, have %d)", payloadLen, len(buf)-offset)
	}
	ep.Payload = make([]byte, payloadLen)
	copy(ep.Payload, buf[offset:offset+payloadLen])
	offset += payloadLen

	return ep, offset - start, nil
}

// encode serialises the EncapsulatedPacket into a byte slice.
func encodeEncapsulated(ep *EncapsulatedPacket) []byte {
	var b []byte

	flags := ep.Reliability << 5
	if ep.HasSplit {
		flags |= 0x10
	}
	b = append(b, flags)

	// Length in bits.
	bitLen := uint16(len(ep.Payload) * 8)
	b = append(b, byte(bitLen>>8), byte(bitLen))

	if isReliable(ep.Reliability) {
		b = append(b, protocol.WriteTriadLE(ep.MessageIndex)...)
	}
	if isSequenced(ep.Reliability) {
		b = append(b, protocol.WriteTriadLE(ep.SequenceIndex)...)
	}
	if isOrdered(ep.Reliability) {
		b = append(b, protocol.WriteTriadLE(ep.OrderIndex)...)
		b = append(b, ep.OrderChannel)
	}
	if ep.HasSplit {
		sc := make([]byte, 4)
		binary.BigEndian.PutUint32(sc, ep.SplitCount)
		b = append(b, sc...)
		si := make([]byte, 2)
		binary.BigEndian.PutUint16(si, ep.SplitID)
		b = append(b, si...)
		sx := make([]byte, 4)
		binary.BigEndian.PutUint32(sx, ep.SplitIndex)
		b = append(b, sx...)
	}

	b = append(b, ep.Payload...)
	return b
}

func isReliable(r byte) bool {
	return r == Reliable || r == ReliableOrdered || r == ReliableSequenced
}

func isSequenced(r byte) bool {
	return r == UnreliableSequenced || r == ReliableSequenced
}

func isOrdered(r byte) bool {
	return r == ReliableOrdered
}

// headerSize returns the header byte-size of an EncapsulatedPacket (excluding payload).
func headerSize(reliability byte, hasSplit bool) int {
	size := 3 // flags + bitLength
	if isReliable(reliability) {
		size += 3
	}
	if isSequenced(reliability) {
		size += 3
	}
	if isOrdered(reliability) {
		size += 4
	}
	if hasSplit {
		size += 10
	}
	return size
}
