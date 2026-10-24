package protocol

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
)

// EncodeBatch encodes a slice of raw packet buffers (each already has its ID byte)
// into a single BatchPacket payload (ID=0xFE followed by zlib-compressed data).
//
// Wire format of the zlib payload:
//   for each packet:
//     UnsignedVarInt(len(packetBuf))
//     packetBuf bytes
func EncodeBatch(packets [][]byte) ([]byte, error) {
	var inner bytes.Buffer
	s := NewWriter()
	for _, pkt := range packets {
		s.WriteUnsignedVarInt(uint32(len(pkt)))
		s.WriteBytes(pkt)
	}

	w, err := zlib.NewWriterLevel(&inner, zlib.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write(s.Bytes()); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}

	out := make([]byte, 1+inner.Len())
	out[0] = IDBatch
	copy(out[1:], inner.Bytes())
	return out, nil
}

// DecodeBatch decompresses a BatchPacket payload (the data after the 0xFE ID byte)
// and returns each inner packet buffer (including its ID byte).
func DecodeBatch(data []byte) ([][]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	s := NewStream(raw)
	var packets [][]byte
	count := 0
	for !s.EOF() {
		if count > 500 {
			return nil, errors.New("too many packets in a single batch")
		}
		length, err := s.ReadUnsignedVarInt()
		if err != nil {
			return nil, err
		}
		pkt, err := s.ReadBytes(int(length))
		if err != nil {
			return nil, err
		}
		// Make a copy to avoid referencing the underlying slice.
		cp := make([]byte, len(pkt))
		copy(cp, pkt)
		packets = append(packets, cp)
		count++
	}
	return packets, nil
}
