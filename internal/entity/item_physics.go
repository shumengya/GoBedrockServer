package entity

import "math"

const (
	ItemGravity float32 = 0.04
	ItemDrag    float32 = 0.02
	ItemWidth   float32 = 0.25
	ItemHeight  float32 = 0.25

	// PickupRadiusXZ is tighter than the old 1.8 sphere. Vertical slack is
	// generous because 1.1 clients may report feet Y or eye Y.
	PickupRadiusXZ float32 = 1.25
	PickupYBelow   float32 = 2
	PickupYAbove   float32 = 2.5

	// Wait until the client has finished its throw arc, then freeze motion
	// without sending MoveEntity (a position snap is what looks like twitching).
	ItemClientFreezeTicks = 20
)

type SolidFunc func(x, y, z float32) bool

// CanPickupItem is true when the player is close enough on XZ and within a
// loose vertical window of the dropped item.
func CanPickupItem(px, py, pz, ix, iy, iz float32) bool {
	dx := ix - px
	dz := iz - pz
	if dx*dx+dz*dz > PickupRadiusXZ*PickupRadiusXZ {
		return false
	}
	return iy+ItemHeight >= py-PickupYBelow && iy <= py+PickupYAbove
}

// StepDroppedItem applies one physics tick. Hitting the ground zeros vertical
// speed (no bounce). PocketMine's move() also zeros motionY on collision;
// the later motionY *= -0.5 is then a no-op.
func StepDroppedItem(e *Instance, solid SolidFunc) {
	if e == nil {
		return
	}
	e.SpeedY -= ItemGravity

	nx := e.X + e.SpeedX
	ny := e.Y + e.SpeedY
	nz := e.Z + e.SpeedZ

	if e.SpeedX != 0 && solid(nx, e.Y, e.Z) {
		e.SpeedX = 0
	} else {
		e.X = nx
	}
	if e.SpeedZ != 0 && solid(e.X, e.Y, nz) {
		e.SpeedZ = 0
	} else {
		e.Z = nz
	}

	switch {
	case e.SpeedY < 0 && solid(e.X, ny, e.Z):
		e.OnGround = true
		e.Y = float32(math.Floor(float64(ny))) + 1
		e.SpeedY = 0
	case e.SpeedY > 0 && solid(e.X, ny, e.Z):
		e.SpeedY = 0
		e.OnGround = false
	default:
		e.Y = ny
		e.OnGround = solid(e.X, e.Y-0.01, e.Z)
		if e.OnGround {
			e.SpeedY = 0
		}
	}

	friction := float32(1) - ItemDrag
	if e.OnGround {
		e.SpeedX = 0
		e.SpeedY = 0
		e.SpeedZ = 0
		return
	}
	e.SpeedY *= (1 - ItemDrag)
	e.SpeedX *= friction
	e.SpeedZ *= friction

	const eps = float32(0.00001)
	if abs32(e.SpeedX) < eps {
		e.SpeedX = 0
	}
	if abs32(e.SpeedY) < eps {
		e.SpeedY = 0
	}
	if abs32(e.SpeedZ) < eps {
		e.SpeedZ = 0
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
