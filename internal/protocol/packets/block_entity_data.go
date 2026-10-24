package packets

import "go-mc-server/internal/protocol"

// BlockEntityDataPacket (0x38) sends a tile spawn compound to the client.
type BlockEntityDataPacket struct {
	X, Y, Z int32
	NBT     []byte
}

func (p *BlockEntityDataPacket) ID() byte { return protocol.IDBlockEntityData }

func (p *BlockEntityDataPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteBlockCoords(p.X, p.Y, p.Z)
	s.WriteBytes(p.NBT)
	return nil
}

func (p *BlockEntityDataPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.X, p.Y, p.Z, err = s.ReadBlockCoords(); err != nil {
		return err
	}
	p.NBT = append([]byte(nil), s.ReadRemaining()...)
	return nil
}
