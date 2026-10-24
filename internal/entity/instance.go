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
}

func (e *Instance) IsDroppedItem() bool {
	return e != nil && e.Type != nil && e.Type.NetworkID == NetworkIDItem && !e.Item.IsAir()
}
