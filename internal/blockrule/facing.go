package blockrule

import "math"

// Direction 是玩家朝向的四个水平方向，数值对齐参考端 Entity::getDirection：
// 0 朝东(+X)、1 朝南(+Z)、2 朝西(-X)、3 朝北(-Z)。
// 参考端注释把 0~3 标成南西北东，但按它的方向向量公式算出来是上面这样，以向量为准。
type Direction int

const DirectionEast Direction = 0
const DirectionSouth Direction = 1
const DirectionWest Direction = 2
const DirectionNorth Direction = 3

// 方块面编号，和 world.OffsetByFace 一致。
const FaceDown int32 = 0
const FaceUp int32 = 1
const FaceNorth int32 = 2
const FaceSouth int32 = 3
const FaceWest int32 = 4
const FaceEast int32 = 5

// DirectionFromYaw 把玩家偏航角换成朝向。yaw 为 0 时看向 +Z（南），顺时针增大看向 -X（西）。
func DirectionFromYaw(yaw float32) Direction {
	var rotation float64 = math.Mod(float64(yaw)-90, 360)
	if rotation < 0 {
		rotation += 360
	}
	if rotation < 45 || rotation >= 315 {
		return DirectionWest
	}
	if rotation < 135 {
		return DirectionNorth
	}
	if rotation < 225 {
		return DirectionEast
	}
	return DirectionSouth
}

// Opposite 返回反方向。
func (direction Direction) Opposite() Direction {
	switch direction {
	case DirectionEast:
		return DirectionWest
	case DirectionWest:
		return DirectionEast
	case DirectionSouth:
		return DirectionNorth
	}
	return DirectionSouth
}

// Step 返回朝这个方向走一格的 X、Z 偏移。
func (direction Direction) Step() (int32, int32) {
	switch direction {
	case DirectionEast:
		return 1, 0
	case DirectionWest:
		return -1, 0
	case DirectionSouth:
		return 0, 1
	}
	return 0, -1
}

// Face 返回这个方向对应的方块面编号。
func (direction Direction) Face() int32 {
	switch direction {
	case DirectionEast:
		return FaceEast
	case DirectionWest:
		return FaceWest
	case DirectionSouth:
		return FaceSouth
	}
	return FaceNorth
}
