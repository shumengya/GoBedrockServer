package raknet

import (
	"encoding/binary"
	"fmt"
	"go-mc-server/internal/protocol"
)

// decodeDatagram decodes a RakNet datagram (0x80–0x8f) and returns all
// encapsulated packets inside it along with the sequence number.
func decodeDatagram(data []byte) (seqNum uint32, packets []*EncapsulatedPacket, err error) {
	if len(data) < 4 {
		return 0, nil, fmt.Errorf("datagram: too short (%d bytes)", len(data))
	}
	// data[0] is the datagram type flag; sequence number is a 3-byte LE triad.
	seqNum = protocol.ReadTriadLE(data[1:4])
	offset := 4
	for offset < len(data) {
		ep, n, e := decodeEncapsulated(data, offset)
		if e != nil {
			return seqNum, packets, e
		}
		packets = append(packets, ep)
		offset += n
	}
	return seqNum, packets, nil
}

// encodeDatagram builds a datagram from a sequence number and a list of already-encoded
// EncapsulatedPacket byte slices.
func encodeDatagram(seqNum uint32, encapsulated [][]byte) []byte {
	total := 4
	for _, e := range encapsulated {
		total += len(e)
	}
	b := make([]byte, 0, total)
	b = append(b, 0x84) // datagram flag
	b = append(b, protocol.WriteTriadLE(seqNum)...)
	for _, e := range encapsulated {
		b = append(b, e...)
	}
	return b
}

// decodeACK decodes an ACK (0xc0) or NACK (0xa0) packet and returns the
// sequence numbers it covers.
func decodeACK(data []byte) ([]uint32, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("ack: too short")
	}
	count := int(binary.BigEndian.Uint16(data[1:3]))
	offset := 3
	var seqNums []uint32
	for i := 0; i < count; i++ {
		if offset >= len(data) {
			break
		}
		rangeType := data[offset]
		offset++
		if rangeType == 1 {
			// Single sequence number.
			if offset+3 > len(data) {
				break
			}
			seqNums = append(seqNums, protocol.ReadTriadLE(data[offset:offset+3]))
			offset += 3
		} else {
			// Range [min, max].
			if offset+6 > len(data) {
				break
			}
			min := protocol.ReadTriadLE(data[offset : offset+3])
			max := protocol.ReadTriadLE(data[offset+3 : offset+6])
			offset += 6
			for s := min; s <= max; s++ {
				seqNums = append(seqNums, s)
			}
		}
	}
	return seqNums, nil
}

// encodeACK builds an ACK or NACK packet for the given sequence numbers.
func encodeACK(id byte, seqNums []uint32) []byte {
	if len(seqNums) == 0 {
		return nil
	}
	// Build ranges.
	type ackRange struct{ min, max uint32 }
	var ranges []ackRange
	cur := ackRange{seqNums[0], seqNums[0]}
	for _, n := range seqNums[1:] {
		if n == cur.max+1 {
			cur.max = n
		} else {
			ranges = append(ranges, cur)
			cur = ackRange{n, n}
		}
	}
	ranges = append(ranges, cur)

	b := make([]byte, 0, 3+len(ranges)*7)
	b = append(b, id)
	cnt := make([]byte, 2)
	binary.BigEndian.PutUint16(cnt, uint16(len(ranges)))
	b = append(b, cnt...)
	for _, r := range ranges {
		if r.min == r.max {
			b = append(b, 1) // single
			b = append(b, protocol.WriteTriadLE(r.min)...)
		} else {
			b = append(b, 0) // range
			b = append(b, protocol.WriteTriadLE(r.min)...)
			b = append(b, protocol.WriteTriadLE(r.max)...)
		}
	}
	return b
}
