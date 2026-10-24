package crafting

import _ "embed"
import "encoding/json"
import "fmt"
import "sync"

// recipes.json 取自参考端 resources/recipes.json，打进二进制，运行时不依赖参考目录。
//
//go:embed recipes.json
var recipesJSON []byte

// 参考端只注册类型 0–3；4（多配方）和 5（潜影盒染色）它自己也没实现，这里同样跳过。
const rawTypeShapeless int32 = 0
const rawTypeShaped int32 = 1
const rawTypeFurnace int32 = 2
const rawTypeFurnaceData int32 = 3

type rawItem struct {
	ID     int32 `json:"id"`
	Damage int32 `json:"damage"`
	Count  int32 `json:"count"`
}

type rawRecipe struct {
	Type        int32           `json:"type"`
	Width       int32           `json:"width"`
	Height      int32           `json:"height"`
	Input       []rawItem       `json:"input"`
	Output      json.RawMessage `json:"output"`
	UUID        string          `json:"uuid"`
	InputID     int32           `json:"inputId"`
	InputDamage int32           `json:"inputDamage"`
}

// Book 是全部配方的索引，加载后只读，可以并发查。
type Book struct {
	recipes []Recipe
	byID    map[[16]byte]int
	skipped int
	smelt   smeltIndex
}

var defaultBook *Book = nil
var defaultErr error = nil
var defaultOnce sync.Once

// Default 返回内置配方书，只解析一次。
func Default() (*Book, error) {
	defaultOnce.Do(func() {
		defaultBook, defaultErr = Load(recipesJSON)
	})
	return defaultBook, defaultErr
}

// Recipes 按配方文件里的顺序返回全部已注册配方。
func (book *Book) Recipes() []Recipe {
	return book.recipes
}

// Find 按配方 UUID 查找；客户端 CraftingEvent 带的就是这个。
func (book *Book) Find(id [16]byte) (Recipe, bool) {
	var index, found = book.byID[id]
	if !found {
		return Recipe{}, false
	}
	return book.recipes[index], true
}

// Skipped 是被跳过（类型不支持）的条目数，只用于启动日志。
func (book *Book) Skipped() int {
	return book.skipped
}

// Load 解析配方 JSON。数据有问题直接报错：带着坏配方上线，玩家合成时才会发现。
func Load(data []byte) (*Book, error) {
	var raws []rawRecipe = nil
	var err error = json.Unmarshal(data, &raws)
	if err != nil {
		return nil, fmt.Errorf("解析配方 JSON: %w", err)
	}
	var book *Book = &Book{byID: make(map[[16]byte]int)}
	var index int = 0
	for index = 0; index < len(raws); index++ {
		var recipe Recipe = Recipe{}
		var keep bool = false
		recipe, keep, err = convertRecipe(raws[index])
		if err != nil {
			return nil, fmt.Errorf("第 %d 条配方: %w", index, err)
		}
		if !keep {
			book.skipped++
			continue
		}
		if recipe.Craftable() {
			var _, duplicate = book.byID[recipe.ID]
			if duplicate {
				return nil, fmt.Errorf("第 %d 条配方 uuid 重复", index)
			}
			book.byID[recipe.ID] = len(book.recipes)
		}
		book.recipes = append(book.recipes, recipe)
		book.indexSmelt(recipe)
	}
	return book, nil
}
