package packets

import "go-mc-server/internal/protocol"

// MobArmorEquipmentPacket (0x20) 告诉客户端某个实体身上穿着什么：头盔、胸甲、护腿、靴子。
// 玩家自己的盔甲不靠它，走 ContainerSetContent 的盔甲窗口（protocol.ContainerArmor）；
// 其他玩家看见的穿戴靠这个包。
type MobArmorEquipmentPacket struct {
	EntityRuntimeID int64
	Slots           [protocol.ArmorSize]protocol.Item
}

func (p *MobArmorEquipmentPacket) ID() byte { return protocol.IDMobArmorEquipment }

func (p *MobArmorEquipmentPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	var index int = 0
	for index = 0; index < len(p.Slots); index++ {
		s.WriteSlot(p.Slots[index])
	}
	return nil
}

func (p *MobArmorEquipmentPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	p.EntityRuntimeID, err = s.ReadEntityID()
	if err != nil {
		return err
	}
	var index int = 0
	for index = 0; index < len(p.Slots); index++ {
		p.Slots[index], err = s.ReadSlot()
		if err != nil {
			return err
		}
	}
	return nil
}
