package blockrule_test

import "testing"

import "go-mc-server/internal/blockrule"

type propsCase struct {
	Name        string
	ID          byte
	Solid       bool
	Transparent bool
	Replaceable bool
	FlowInto    bool
}

// 属性表来自参考端各方块类，这里抽查后面规则最依赖的几种方块。
func TestPropsOfKnownBlocks(test *testing.T) {
	var cases []propsCase = []propsCase{
		{Name: "空气", ID: blockrule.BlockAir, Solid: false, Transparent: true, Replaceable: true, FlowInto: true},
		{Name: "石头", ID: blockrule.BlockStone, Solid: true, Transparent: false, Replaceable: false, FlowInto: false},
		{Name: "沙子", ID: blockrule.BlockSand, Solid: true, Transparent: false, Replaceable: false, FlowInto: false},
		{Name: "火把", ID: blockrule.BlockTorch, Solid: false, Transparent: true, Replaceable: false, FlowInto: true},
		{Name: "流动水", ID: blockrule.BlockFlowingWater, Solid: false, Transparent: true, Replaceable: true, FlowInto: true},
		{Name: "岩浆", ID: blockrule.BlockFlowingLava, Solid: false, Transparent: true, Replaceable: true, FlowInto: true},
		{Name: "高草", ID: blockrule.BlockTallGrass, Solid: false, Transparent: true, Replaceable: true, FlowInto: true},
		{Name: "玻璃", ID: blockrule.BlockGlass, Solid: true, Transparent: true, Replaceable: false, FlowInto: false},
		{Name: "树叶", ID: blockrule.BlockLeaves, Solid: true, Transparent: true, Replaceable: false, FlowInto: false},
		{Name: "耕地", ID: blockrule.BlockFarmland, Solid: true, Transparent: true, Replaceable: false, FlowInto: false},
		{Name: "仙人掌", ID: blockrule.BlockCactus, Solid: true, Transparent: true, Replaceable: false, FlowInto: false},
		{Name: "甘蔗", ID: blockrule.BlockSugarcane, Solid: false, Transparent: true, Replaceable: false, FlowInto: true},
		{Name: "火", ID: blockrule.BlockFire, Solid: false, Transparent: true, Replaceable: true, FlowInto: true},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item propsCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var props blockrule.Props = blockrule.PropsOf(item.ID)
			if props.Solid != item.Solid {
				sub.Fatalf("Solid 期望 %v，实际 %v", item.Solid, props.Solid)
			}
			if props.Transparent != item.Transparent {
				sub.Fatalf("Transparent 期望 %v，实际 %v", item.Transparent, props.Transparent)
			}
			if props.Replaceable != item.Replaceable {
				sub.Fatalf("Replaceable 期望 %v，实际 %v", item.Replaceable, props.Replaceable)
			}
			if props.FlowInto != item.FlowInto {
				sub.Fatalf("FlowInto 期望 %v，实际 %v", item.FlowInto, props.FlowInto)
			}
		})
	}
}

// 表里没有的 ID 当成普通实心方块，不能因为缺项就被当成空气或可顶替。
func TestPropsOfUnknownIsSolid(test *testing.T) {
	var props blockrule.Props = blockrule.PropsOf(255)
	if !props.Solid || props.Transparent || props.Replaceable || props.FlowInto {
		test.Fatalf("未知 ID 应为实心不透明不可顶替，实际 %+v", props)
	}
	if props.BurnChance != 0 || props.BurnAbility != 0 {
		test.Fatalf("未知 ID 不该可燃，实际 %+v", props)
	}
}

type replaceCase struct {
	Name   string
	ID     byte
	Meta   byte
	Expect bool
}

// 雪片没叠满 8 层才能顶替；双层植物里只有高草和大型蕨能被顶替；其余按属性表。
func TestIsReplaceableByMeta(test *testing.T) {
	var cases []replaceCase = []replaceCase{
		{Name: "雪片 1 层可顶替", ID: blockrule.BlockSnowLayer, Meta: 0, Expect: true},
		{Name: "雪片 7 层可顶替", ID: blockrule.BlockSnowLayer, Meta: 6, Expect: true},
		{Name: "雪片 8 层不可顶替", ID: blockrule.BlockSnowLayer, Meta: 7, Expect: false},
		{Name: "向日葵不可顶替", ID: blockrule.BlockDoublePlant, Meta: 0, Expect: false},
		{Name: "双层高草可顶替", ID: blockrule.BlockDoublePlant, Meta: 2, Expect: true},
		{Name: "大型蕨可顶替", ID: blockrule.BlockDoublePlant, Meta: 3, Expect: true},
		{Name: "玫瑰丛不可顶替", ID: blockrule.BlockDoublePlant, Meta: 4, Expect: false},
		{Name: "空气可顶替", ID: blockrule.BlockAir, Meta: 0, Expect: true},
		{Name: "石头不可顶替", ID: blockrule.BlockStone, Meta: 0, Expect: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item replaceCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var actual bool = blockrule.IsReplaceable(item.ID, item.Meta)
			if actual != item.Expect {
				sub.Fatalf("IsReplaceable(%d,%d) 期望 %v，实际 %v", item.ID, item.Meta, item.Expect, actual)
			}
		})
	}
}

type burnCase struct {
	Name    string
	ID      byte
	Meta    byte
	Chance  int
	Ability int
}

// 燃烧值决定火的蔓延。台阶按 meta 区分木头和石头，158 号木台阶无论 meta 都可燃。
func TestBurnValues(test *testing.T) {
	var cases []burnCase = []burnCase{
		{Name: "木板", ID: blockrule.BlockPlanks, Meta: 0, Chance: 5, Ability: 20},
		{Name: "树叶", ID: blockrule.BlockLeaves, Meta: 0, Chance: 30, Ability: 60},
		{Name: "高草", ID: blockrule.BlockTallGrass, Meta: 1, Chance: 60, Ability: 100},
		{Name: "石头不可燃", ID: blockrule.BlockStone, Meta: 0, Chance: 0, Ability: 0},
		{Name: "石台阶不可燃", ID: blockrule.BlockSlab, Meta: 0, Chance: 0, Ability: 0},
		{Name: "石台阶里的木头种类可燃", ID: blockrule.BlockSlab, Meta: 2, Chance: 5, Ability: 5},
		{Name: "上半石台阶的木头种类可燃", ID: blockrule.BlockSlab, Meta: 10, Chance: 5, Ability: 5},
		{Name: "木台阶橡木", ID: blockrule.BlockWoodSlab, Meta: 0, Chance: 5, Ability: 20},
		{Name: "木台阶云杉", ID: blockrule.BlockWoodSlab, Meta: 1, Chance: 5, Ability: 20},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item burnCase = cases[index]
		test.Run(item.Name, func(sub *testing.T) {
			var chance int = blockrule.BurnChance(item.ID, item.Meta)
			var ability int = blockrule.BurnAbility(item.ID, item.Meta)
			if chance != item.Chance || ability != item.Ability {
				sub.Fatalf("期望 %d/%d，实际 %d/%d", item.Chance, item.Ability, chance, ability)
			}
		})
	}
}
