package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

type yawCase struct {
	Name   string
	Yaw    float32
	Expect blockrule.Direction
}

// 偏航角 0 看向 +Z（南），顺时针增大依次转向西、北、东。
// 每条分界线两侧各取一个值，确认分界位置在 45°、135°、225°、315°。
func TestDirectionFromYaw(test *testing.T) {
	var cases []yawCase = []yawCase{
		{Name: "0 度朝南", Yaw: 0, Expect: blockrule.DirectionSouth},
		{Name: "90 度朝西", Yaw: 90, Expect: blockrule.DirectionWest},
		{Name: "180 度朝北", Yaw: 180, Expect: blockrule.DirectionNorth},
		{Name: "270 度朝东", Yaw: 270, Expect: blockrule.DirectionEast},
		{Name: "负 90 度等于 270 度朝东", Yaw: -90, Expect: blockrule.DirectionEast},
		{Name: "360 度回到朝南", Yaw: 360, Expect: blockrule.DirectionSouth},
		{Name: "44 度仍朝南", Yaw: 44, Expect: blockrule.DirectionSouth},
		{Name: "46 度转朝西", Yaw: 46, Expect: blockrule.DirectionWest},
		{Name: "134 度仍朝西", Yaw: 134, Expect: blockrule.DirectionWest},
		{Name: "136 度转朝北", Yaw: 136, Expect: blockrule.DirectionNorth},
		{Name: "224 度仍朝北", Yaw: 224, Expect: blockrule.DirectionNorth},
		{Name: "226 度转朝东", Yaw: 226, Expect: blockrule.DirectionEast},
		{Name: "314 度仍朝东", Yaw: 314, Expect: blockrule.DirectionEast},
		{Name: "316 度转朝南", Yaw: 316, Expect: blockrule.DirectionSouth},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item yawCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var actual blockrule.Direction = blockrule.DirectionFromYaw(item.Yaw)
			if actual != item.Expect {
				sub.Fatalf("yaw=%v 期望 %d，实际 %d", item.Yaw, item.Expect, actual)
			}
		})
	}
}

// 朝向的偏移、方块面和反方向三套定义必须互相一致，放楼梯、门、床都依赖它们。
func TestDirectionHelpersAgree(test *testing.T) {
	var directions []blockrule.Direction = []blockrule.Direction{
		blockrule.DirectionEast,
		blockrule.DirectionSouth,
		blockrule.DirectionWest,
		blockrule.DirectionNorth,
	}
	var origin blockrule.Pos = blockrule.Pos{PosX: 0, PosY: 0, PosZ: 0}
	var index int = 0
	for index = 0; index < len(directions); index++ {
		var direction blockrule.Direction = directions[index]
		var stepX, stepZ = direction.Step()
		var byStep blockrule.Pos = origin.Add(stepX, 0, stepZ)
		var byFace, faceOK = origin.Side(direction.Face())
		if !faceOK || byFace != byStep {
			test.Fatalf("方向 %d: Step 得到 %+v，Face 得到 %+v", direction, byStep, byFace)
		}
		var opposite blockrule.Direction = direction.Opposite()
		var backX, backZ = opposite.Step()
		if backX != -stepX || backZ != -stepZ {
			test.Fatalf("方向 %d 的反方向偏移应为 (%d,%d)，实际 (%d,%d)", direction, -stepX, -stepZ, backX, backZ)
		}
		if opposite.Opposite() != direction {
			test.Fatalf("方向 %d 的反方向再取反应回到自己，实际 %d", direction, opposite.Opposite())
		}
	}
}

type sideCase struct {
	Name   string
	Face   int32
	Expect blockrule.Pos
}

// 面编号和 world.OffsetByFace 一致：0 下、1 上、2 北(-Z)、3 南(+Z)、4 西(-X)、5 东(+X)。
func TestPosSide(test *testing.T) {
	var origin blockrule.Pos = blockrule.Pos{PosX: 10, PosY: 20, PosZ: 30}
	var cases []sideCase = []sideCase{
		{Name: "下", Face: blockrule.FaceDown, Expect: blockrule.Pos{PosX: 10, PosY: 19, PosZ: 30}},
		{Name: "上", Face: blockrule.FaceUp, Expect: blockrule.Pos{PosX: 10, PosY: 21, PosZ: 30}},
		{Name: "北", Face: blockrule.FaceNorth, Expect: blockrule.Pos{PosX: 10, PosY: 20, PosZ: 29}},
		{Name: "南", Face: blockrule.FaceSouth, Expect: blockrule.Pos{PosX: 10, PosY: 20, PosZ: 31}},
		{Name: "西", Face: blockrule.FaceWest, Expect: blockrule.Pos{PosX: 9, PosY: 20, PosZ: 30}},
		{Name: "东", Face: blockrule.FaceEast, Expect: blockrule.Pos{PosX: 11, PosY: 20, PosZ: 30}},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item sideCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var actual, ok = origin.Side(item.Face)
			if !ok || actual != item.Expect {
				sub.Fatalf("面 %d 期望 %+v，实际 %+v ok=%v", item.Face, item.Expect, actual, ok)
			}
		})
	}
	var _, invalidOK = origin.Side(6)
	if invalidOK {
		test.Fatal("面编号 6 不合法，应返回 false")
	}
	var _, negativeOK = origin.Side(-1)
	if negativeOK {
		test.Fatal("面编号 -1 不合法，应返回 false")
	}
}
