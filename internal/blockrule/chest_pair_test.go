package blockrule_test

import "slices"
import "testing"

import "go-mc-server/internal/blockrule"

type candidateCase struct {
	Name   string
	Meta   byte
	Expect []blockrule.Pos
}

// 箱子只和并排的箱子配对：正面朝东西（meta 4、5）的在南北两侧找，正面朝南北（meta 2、3）的在东西两侧找，
// 顺序是先北后南、先西后东。其他 meta 不是箱子朝向，不找。
func TestChestPairCandidates(test *testing.T) {
	var origin blockrule.Pos = blockrule.Pos{PosX: 10, PosY: 64, PosZ: 20}
	var cases []candidateCase = []candidateCase{
		{
			Name:   "meta 4 在南北两侧",
			Meta:   4,
			Expect: []blockrule.Pos{{PosX: 10, PosY: 64, PosZ: 19}, {PosX: 10, PosY: 64, PosZ: 21}},
		},
		{
			Name:   "meta 5 在南北两侧",
			Meta:   5,
			Expect: []blockrule.Pos{{PosX: 10, PosY: 64, PosZ: 19}, {PosX: 10, PosY: 64, PosZ: 21}},
		},
		{
			Name:   "meta 2 在东西两侧",
			Meta:   2,
			Expect: []blockrule.Pos{{PosX: 9, PosY: 64, PosZ: 20}, {PosX: 11, PosY: 64, PosZ: 20}},
		},
		{
			Name:   "meta 3 在东西两侧",
			Meta:   3,
			Expect: []blockrule.Pos{{PosX: 9, PosY: 64, PosZ: 20}, {PosX: 11, PosY: 64, PosZ: 20}},
		},
		{Name: "meta 0 不找", Meta: 0, Expect: nil},
		{Name: "meta 1 不找", Meta: 1, Expect: nil},
		{Name: "meta 6 不找", Meta: 6, Expect: nil},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item candidateCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var got []blockrule.Pos = blockrule.ChestPairCandidates(origin, item.Meta)
			if !slices.Equal(got, item.Expect) {
				sub.Fatalf("期望 %v，实际 %v", item.Expect, got)
			}
		})
	}
}
