package entity

import "go-mc-server/internal/protocol"

const (
	NetworkIDItem    int32 = 64
	PickupDelayTicks       = 40
	ItemDespawnTicks       = 2000
)

// Instance is a spawned entity in the world.
type Instance struct {
	RuntimeID              int64
	Type                   *Type
	X, Y, Z                float32
	Yaw, Pitch             float32
	SpeedX, SpeedY, SpeedZ float32
	Item                   protocol.Item
	PickupDelay            int
	Age                    int
	OnGround               bool
	ClientFrozen           bool
	WorldName              string

	// BlockID、BlockData 是下落中的方块（沙子、沙砾）自带的方块类型，BlockID 为 0 表示不是下落方块。
	BlockID   byte
	BlockData byte
}

func (e *Instance) IsDroppedItem() bool {
	return e != nil && e.Type != nil && e.Type.NetworkID == NetworkIDItem && !e.Item.IsAir()
}
