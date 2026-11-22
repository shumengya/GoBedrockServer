package server

import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// 满饥饿吃最后一颗也应传送；平坦世界所有候选列的地面都在 y=4。
func TestChorusFruitAfterEating(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var player *Player = fixture.join("Alex", "127.0.0.1", "world")
	player.ApplyInventorySlot(0, protocol.Item{ID: 432, Count: 1})
	fixture.srv.finishEating(player)
	var posX, eyeY, posZ = player.Position()
	if !player.Inventory[0].IsAir() || eyeY != 5+playerEyeHeight || posX < -7.5 || posX > 8.5 || posZ < -7.5 || posZ > 8.5 {
		test.Fatalf("期望果实耗尽、在范围内的地面落脚，实际背包=%v 坐标=(%v,%v,%v)", player.Inventory[0], posX, eyeY, posZ)
	}
}

// 第一列不能容纳玩家时必须重选；16 次都选不中时保持原位。
func TestChorusFruitRejectsBlockedLanding(test *testing.T) {
	var cases = []struct {
		name   string
		height int32
		block  byte
	}{
		{"脚下有水", 64, world.BlockStillWater}, {"头部有岩浆", 65, world.BlockFlowingLava}, {"上方净空不足", 65, world.BlockStone},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var example = cases[index]
		test.Run(example.name, func(test *testing.T) {
			var fixture *adminFixture = newAdminFixture(test)
			var player *Player = fixture.join("Alex", "127.0.0.1", "world")
			fixture.srv.world.SetBlock(0, 63, 0, world.BlockStone, 0)
			fixture.srv.world.SetBlock(0, example.height, 0, example.block, 0)
			var random *fixedRandom = &fixedRandom{values: []int{8}}
			fixture.srv.teleportAfterChorusFruit(player, random)
			var posX, eyeY, posZ = player.Position()
			if random.cursor != 16*3 || posX != 0 || eyeY != 64+playerEyeHeight || posZ != 0 {
				test.Fatalf("期望 16 次尝试后留在原位，实际随机取值 %d 次，坐标=(%v,%v,%v)", random.cursor, posX, eyeY, posZ)
			}
			random = &fixedRandom{values: []int{8, 8, 8, 9, 8, 8}}
			fixture.srv.teleportAfterChorusFruit(player, random)
			posX, eyeY, posZ = player.Position()
			if posX != 1.5 || eyeY != 5+playerEyeHeight || posZ != 0.5 {
				test.Fatalf("期望落在第二列的草地上，实际坐标=(%v,%v,%v)", posX, eyeY, posZ)
			}
		})
	}
}
