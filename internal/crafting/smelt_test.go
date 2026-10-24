package crafting_test

import "testing"

import "go-mc-server/internal/crafting"
import "go-mc-server/internal/protocol"

type smeltCase struct {
	Name   string
	Input  protocol.Item
	Output protocol.Item
	Found  bool
}

// 内置配方书里几条有代表性的熔炼：不限 meta 的(圆石、沙子、矿、原木)、
// 按 meta 区分的(染色黏土、海绵、石砖)，以及没有配方的。预期值取自 recipes.json。
func TestSmeltDefaultBook(test *testing.T) {
	var book, loadErr = crafting.Default()
	if loadErr != nil {
		test.Fatalf("加载配方书失败: %v", loadErr)
	}
	var cases []smeltCase = []smeltCase{
		{Name: "圆石烤石头", Input: protocol.Item{ID: 4, Count: 1}, Output: protocol.Item{ID: 1}, Found: true},
		{Name: "沙子烤玻璃", Input: protocol.Item{ID: 12, Count: 1}, Output: protocol.Item{ID: 20}, Found: true},
		{Name: "铁矿烤铁锭", Input: protocol.Item{ID: 15, Count: 1}, Output: protocol.Item{ID: 265}, Found: true},
		{Name: "原木烤木炭，任何树种", Input: protocol.Item{ID: 17, Meta: 2, Count: 1}, Output: protocol.Item{ID: 263, Meta: 1}, Found: true},
		{Name: "生猪排烤熟", Input: protocol.Item{ID: 319, Count: 1}, Output: protocol.Item{ID: 320}, Found: true},
		{Name: "湿海绵烤干海绵(meta 1)", Input: protocol.Item{ID: 19, Meta: 1, Count: 1}, Output: protocol.Item{ID: 19}, Found: true},
		{Name: "干海绵没有配方", Input: protocol.Item{ID: 19, Meta: 0, Count: 1}, Found: false},
		{Name: "白色染色黏土烤白色釉陶", Input: protocol.Item{ID: 159, Meta: 0, Count: 1}, Output: protocol.Item{ID: 220}, Found: true},
		{Name: "橙色染色黏土烤橙色釉陶", Input: protocol.Item{ID: 159, Meta: 1, Count: 1}, Output: protocol.Item{ID: 221}, Found: true},
		{Name: "石砖烤裂纹石砖", Input: protocol.Item{ID: 98, Meta: 0, Count: 1}, Output: protocol.Item{ID: 98, Meta: 2}, Found: true},
		{Name: "泥土没有配方", Input: protocol.Item{ID: 3, Count: 1}, Found: false},
		{Name: "空气没有配方", Input: protocol.Air(), Found: false},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item smeltCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var output, found = book.Smelt(item.Input)
			if found != item.Found {
				sub.Fatalf("找到期望 %v，实际 %v", item.Found, found)
			}
			if found && (output.ID != item.Output.ID || output.Meta != item.Output.Meta) {
				sub.Fatalf("产物期望 %d:%d，实际 %d:%d", item.Output.ID, item.Output.Meta, output.ID, output.Meta)
			}
		})
	}
}

// 熔炉每次只出一个，数量固定按一个算（smelting.finishSmelt）：配方数据里的熔炼产物必须真的都是一个。
func TestSmeltOutputsAreSingleItems(test *testing.T) {
	var book, loadErr = crafting.Default()
	if loadErr != nil {
		test.Fatalf("加载配方书失败: %v", loadErr)
	}
	var recipes []crafting.Recipe = book.Recipes()
	var furnaceRecipes int = 0
	var index int = 0
	for index = 0; index < len(recipes); index++ {
		var recipe crafting.Recipe = recipes[index]
		if recipe.Kind != crafting.KindFurnace && recipe.Kind != crafting.KindFurnaceData {
			continue
		}
		furnaceRecipes++
		if len(recipe.Output) != 1 || recipe.Output[0].Count != 1 {
			test.Fatalf("熔炼配方 %+v 的产物不是单个物品", recipe)
		}
	}
	if furnaceRecipes == 0 {
		test.Fatal("内置配方书里应该有熔炼配方")
	}
}

// 合成配方不能被当成熔炼配方查出来。
func TestSmeltIgnoresCraftingRecipes(test *testing.T) {
	var book, loadErr = crafting.Default()
	if loadErr != nil {
		test.Fatalf("加载配方书失败: %v", loadErr)
	}
	// 木板(5)只用来合成木棍、工作台等，没有熔炼配方。
	var _, found = book.Smelt(protocol.Item{ID: 5, Count: 1})
	if found {
		test.Fatal("木板没有熔炼配方，不该被合成配方的输入误认")
	}
}

const syntheticRecipes string = `[
	{"type":2,"inputId":50,"output":{"id":1,"damage":0,"count":1,"nbt":""}},
	{"type":3,"inputId":50,"inputDamage":3,"output":{"id":2,"damage":0,"count":1,"nbt":""}},
	{"type":2,"inputId":60,"output":{"id":3,"damage":0,"count":1,"nbt":""}},
	{"type":2,"inputId":60,"output":{"id":4,"damage":0,"count":1,"nbt":""}}
]`

// 同一个 ID 既有不限 meta 的配方、又有按 meta 的配方时，meta 对得上的优先；对不上的走不限 meta 的。
func TestSmeltExactMetaBeatsWildcard(test *testing.T) {
	var book, loadErr = crafting.Load([]byte(syntheticRecipes))
	if loadErr != nil {
		test.Fatalf("加载合成的配方失败: %v", loadErr)
	}
	var exact, exactFound = book.Smelt(protocol.Item{ID: 50, Meta: 3, Count: 1})
	if !exactFound || exact.ID != 2 {
		test.Fatalf("meta 3 应走按 meta 的配方(产物 2)，实际 %d 找到=%v", exact.ID, exactFound)
	}
	var other, otherFound = book.Smelt(protocol.Item{ID: 50, Meta: 7, Count: 1})
	if !otherFound || other.ID != 1 {
		test.Fatalf("meta 7 应走不限 meta 的配方(产物 1)，实际 %d 找到=%v", other.ID, otherFound)
	}
}

// 同一个输入出现两条配方，后面的盖掉前面的。
func TestSmeltLaterRecipeWins(test *testing.T) {
	var book, loadErr = crafting.Load([]byte(syntheticRecipes))
	if loadErr != nil {
		test.Fatalf("加载合成的配方失败: %v", loadErr)
	}
	var output, found = book.Smelt(protocol.Item{ID: 60, Count: 1})
	if !found || output.ID != 4 {
		test.Fatalf("应取后一条(产物 4)，实际 %d 找到=%v", output.ID, found)
	}
}
