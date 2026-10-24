package smelting_test

import "testing"

import "go-mc-server/internal/smelting"

type fuelCase struct {
	Name  string
	ID    int32
	Meta  int32
	Ticks int32
}

// 燃料能烧多久照参考端的表：煤一个烧 8 个物品(1600)，煤块 10 倍，木头类 300，木棍 100。
// 桶只有装岩浆的(meta 10)才算燃料；表里没有的一律是 0。
func TestFuelTicks(test *testing.T) {
	var cases []fuelCase = []fuelCase{
		{Name: "煤", ID: 263, Ticks: 1600},
		{Name: "煤块", ID: 173, Ticks: 16000},
		{Name: "原木", ID: 17, Ticks: 300},
		{Name: "金合欢原木是补的", ID: 162, Ticks: 300},
		{Name: "木板", ID: 5, Ticks: 300},
		{Name: "树苗", ID: 6, Ticks: 100},
		{Name: "木棍", ID: 280, Ticks: 100},
		{Name: "烈焰棒", ID: 369, Ticks: 2400},
		{Name: "木镐", ID: 270, Ticks: 200},
		{Name: "箱子", ID: 54, Ticks: 300},
		{Name: "金合欢楼梯是补的", ID: 163, Ticks: 300},
		{Name: "岩浆桶", ID: 325, Meta: 10, Ticks: 20000},
		{Name: "水桶不是燃料", ID: 325, Meta: 8},
		{Name: "空桶不是燃料", ID: 325, Meta: 0},
		{Name: "泥土", ID: 3},
		{Name: "铁镐", ID: 257},
		{Name: "石头", ID: 1},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item fuelCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var actual int32 = smelting.FuelTicks(stack(item.ID, item.Meta, 1))
			if actual != item.Ticks {
				sub.Fatalf("期望 %d，实际 %d", item.Ticks, actual)
			}
		})
	}
	if smelting.FuelTicks(stack(263, 0, 0)) != 0 {
		test.Fatal("数量为 0 的当空气，不是燃料")
	}
}
