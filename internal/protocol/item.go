package protocol

// Item is an inventory slot as encoded by PocketMine BinaryStream getSlot/putSlot.
type Item struct {
	ID    int32
	Meta  int32
	Count byte
	NBT   []byte
}

func Air() Item { return Item{} }

func (it Item) IsAir() bool { return it.ID <= 0 || it.Count == 0 }

func (s *BinaryStream) ReadSlot() (Item, error) {
	id, err := s.ReadVarInt()
	if err != nil {
		return Item{}, err
	}
	if id <= 0 {
		return Air(), nil
	}
	aux, err := s.ReadVarInt()
	if err != nil {
		return Item{}, err
	}
	meta := aux >> 8
	if meta == 0x7fff {
		meta = -1
	}
	count := byte(aux & 0xff)
	nbtLen, err := s.ReadUint16LE()
	if err != nil {
		return Item{}, err
	}
	var nbt []byte
	if nbtLen > 0 {
		nbt, err = s.ReadBytes(int(nbtLen))
		if err != nil {
			return Item{}, err
		}
	}
	canPlace, err := s.ReadVarInt()
	if err != nil {
		return Item{}, err
	}
	for i := int32(0); i < canPlace; i++ {
		if _, err = s.ReadString(); err != nil {
			return Item{}, err
		}
	}
	canDestroy, err := s.ReadVarInt()
	if err != nil {
		return Item{}, err
	}
	for i := int32(0); i < canDestroy; i++ {
		if _, err = s.ReadString(); err != nil {
			return Item{}, err
		}
	}
	return Item{ID: id, Meta: meta, Count: count, NBT: nbt}, nil
}

func (s *BinaryStream) WriteSlot(it Item) {
	if it.ID <= 0 {
		s.WriteVarInt(0)
		return
	}
	s.WriteVarInt(it.ID)
	aux := ((it.Meta & 0x7fff) << 8) | int32(it.Count)
	s.WriteVarInt(aux)
	if len(it.NBT) > 0 {
		s.WriteUint16LE(uint16(len(it.NBT)))
		s.WriteBytes(it.NBT)
	} else {
		s.WriteUint16LE(0)
	}
	s.WriteVarInt(0)
	s.WriteVarInt(0)
}
