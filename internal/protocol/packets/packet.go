// Package packets contains all MCPE game packet encode/decode implementations.
package packets

import "go-mc-server/internal/protocol"

// Packet is the interface every game packet implements.
type Packet interface {
	// ID returns the packet's numeric identifier.
	ID() byte
	// Encode serialises the packet to a BinaryStream (not including the ID byte).
	Encode(s *protocol.BinaryStream) error
	// Decode deserialises the packet from a BinaryStream (the ID byte has already been consumed).
	Decode(s *protocol.BinaryStream) error
}

// EncodePacket returns the full wire bytes for a packet (ID byte + body).
func EncodePacket(p Packet) ([]byte, error) {
	s := protocol.NewWriter()
	s.WriteByte(p.ID())
	if err := p.Encode(s); err != nil {
		return nil, err
	}
	return s.Bytes(), nil
}
