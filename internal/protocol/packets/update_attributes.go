package packets

import (
	"math"

	"go-mc-server/internal/protocol"
)

// Attribute represents a player attribute (health, hunger, xp, etc.).
type Attribute struct {
	Min          float32
	Max          float32
	Value        float32
	DefaultValue float32
	Name         string
}

// UpdateAttributesPacket (0x1e) sends player attribute values to the client.
type UpdateAttributesPacket struct {
	EntityRuntimeID int64
	Attributes      []Attribute
}

func (p *UpdateAttributesPacket) ID() byte { return protocol.IDUpdateAttributes }

func (p *UpdateAttributesPacket) Encode(s *protocol.BinaryStream) error {
	s.WriteEntityID(p.EntityRuntimeID)
	s.WriteUnsignedVarInt(uint32(len(p.Attributes)))
	for _, a := range p.Attributes {
		s.WriteLFloat(a.Min)
		s.WriteLFloat(a.Max)
		s.WriteLFloat(a.Value)
		s.WriteLFloat(a.DefaultValue)
		s.WriteString(a.Name)
	}
	return nil
}

func (p *UpdateAttributesPacket) Decode(s *protocol.BinaryStream) error {
	return nil
}

// DefaultAttributes is the 1.1.x player attribute set sent at spawn.
//
// Source: pocketmine/entity/Attribute.php (IDs 0–10) + Player::sendAttributes(true).
// PHP integer-keyed arrays iterate in ID order, so the client that matches by
// index sees health at slot 4 and movement at slot 5.
//
// Do not send later-protocol names (luck / underwater_movement / fall_damage):
// 1.1 clients mis-parse them and the HUD (hearts, bubbles, speed) explodes.
func DefaultAttributes() []Attribute {
	return PlayerAttributes(DefaultVitals())
}

// PlayerVitals 是客户端 HUD 关心的几项体征。
// 属性包是整包覆盖的，只发饥饿会把血量顶回默认值，所以血量也得一起带上。
type PlayerVitals struct {
	Absorption float32
	Health     float32
	Hunger     float32
	Saturation float32
	Exhaustion float32
}

// DefaultVitals 是满血满饱食的初始状态。
// 饱和度取参考端的 20，而不是原版的 5；它不显示在 HUD 上，只影响饥饿下降的快慢。
func DefaultVitals() PlayerVitals {
	return PlayerVitals{Health: 20, Hunger: 20, Saturation: 20, Exhaustion: 0}
}

// HungerAttributes 只含饥饿相关的三项（饱和度、疲劳度、饥饿值），增量同步用。
// 整包 PlayerAttributes 会把移动速度之类的属性也覆盖回默认值。
func HungerAttributes(vitals PlayerVitals) []Attribute {
	var all []Attribute = PlayerAttributes(vitals)
	var picked []Attribute = nil
	var index int = 0
	for index = 0; index < len(all); index++ {
		var name string = all[index].Name
		if name == "minecraft:player.saturation" || name == "minecraft:player.exhaustion" || name == "minecraft:player.hunger" {
			picked = append(picked, all[index])
		}
	}
	return picked
}

func AbsorptionAttribute(value float32) Attribute {
	return Attribute{Min: 0, Max: math.MaxFloat32, Value: value, DefaultValue: 0, Name: "minecraft:absorption"}
}

func MovementAttribute(value float32) Attribute {
	return Attribute{Min: 0, Max: math.MaxFloat32, Value: value, DefaultValue: 0.1, Name: "minecraft:movement"}
}

// PlayerAttributes 按 1.1 的属性顺序组装整包，顺序不能动：客户端可能按下标匹配。
func PlayerAttributes(vitals PlayerVitals) []Attribute {
	var fltMax float32 = math.MaxFloat32
	return []Attribute{
		AbsorptionAttribute(vitals.Absorption),
		{Min: 0, Max: 20, Value: vitals.Saturation, DefaultValue: 20, Name: "minecraft:player.saturation"},
		{Min: 0, Max: 5, Value: vitals.Exhaustion, DefaultValue: 0, Name: "minecraft:player.exhaustion"},
		{Min: 0, Max: 1, Value: 0, DefaultValue: 0, Name: "minecraft:knockback_resistance"},
		{Min: 0, Max: 20, Value: vitals.Health, DefaultValue: 20, Name: "minecraft:health"},
		MovementAttribute(0.1),
		{Min: 0, Max: 2048, Value: 16, DefaultValue: 16, Name: "minecraft:follow_range"},
		{Min: 0, Max: 20, Value: vitals.Hunger, DefaultValue: 20, Name: "minecraft:player.hunger"},
		{Min: 0, Max: fltMax, Value: 1, DefaultValue: 1, Name: "minecraft:attack_damage"},
		{Min: 0, Max: 24791, Value: 0, DefaultValue: 0, Name: "minecraft:player.level"},
		{Min: 0, Max: 1, Value: 0, DefaultValue: 0, Name: "minecraft:player.experience"},
	}
}
