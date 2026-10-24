package crafting

import "encoding/json"
import "fmt"

import "go-mc-server/internal/protocol"

// toItem 把配方文件里的物品转成协议物品；id 或数量为 0 的是空格子。
// 材料的数量原样保留（和参考端一样发给客户端），服务端校验时每格只按 1 个算。
func toItem(raw rawItem) protocol.Item {
	if raw.ID <= 0 || raw.Count <= 0 {
		return protocol.Air()
	}
	var count int32 = min(raw.Count, 255)
	return protocol.Item{ID: raw.ID, Meta: raw.Damage, Count: byte(count)}
}

func toItems(raws []rawItem) []protocol.Item {
	var out []protocol.Item = make([]protocol.Item, len(raws))
	var index int = 0
	for index = 0; index < len(raws); index++ {
		out[index] = toItem(raws[index])
	}
	return out
}

// convertRecipe 把一条原始配方转成 Recipe；第二个返回值为 false 表示类型不支持，直接跳过。
func convertRecipe(raw rawRecipe) (Recipe, bool, error) {
	switch raw.Type {
	case rawTypeShapeless:
		return convertCrafting(raw, KindShapeless)
	case rawTypeShaped:
		return convertCrafting(raw, KindShaped)
	case rawTypeFurnace, rawTypeFurnaceData:
		return convertFurnace(raw)
	default:
		return Recipe{}, false, nil
	}
}

func convertCrafting(raw rawRecipe, kind Kind) (Recipe, bool, error) {
	var outputs []rawItem = nil
	var err error = json.Unmarshal(raw.Output, &outputs)
	if err != nil {
		return Recipe{}, false, fmt.Errorf("解析输出: %w", err)
	}
	var id [16]byte = [16]byte{}
	id, err = ParseUUID(raw.UUID)
	if err != nil {
		return Recipe{}, false, fmt.Errorf("读取配方 id: %w", err)
	}
	var recipe Recipe = Recipe{
		Kind:   kind,
		ID:     id,
		Input:  toItems(raw.Input),
		Output: toItems(outputs),
	}
	if len(recipe.Input) == 0 {
		return Recipe{}, false, fmt.Errorf("没有材料")
	}
	if len(recipe.Output) == 0 || recipe.Output[0].IsAir() {
		return Recipe{}, false, fmt.Errorf("没有产物")
	}
	if kind != KindShaped {
		return recipe, true, nil
	}
	if raw.Width <= 0 || raw.Height <= 0 || int(raw.Width*raw.Height) != len(raw.Input) {
		return Recipe{}, false, fmt.Errorf("有序配方尺寸 %dx%d 与材料数 %d 不符", raw.Width, raw.Height, len(raw.Input))
	}
	recipe.Width = raw.Width
	recipe.Height = raw.Height
	return recipe, true, nil
}

// convertFurnace 熔炼配方的 output 是单个对象而不是数组，没有 uuid。
// 种类按输入 meta 判定（对齐参考端）：不限 meta 走类型 2，指定 meta 走类型 3。
func convertFurnace(raw rawRecipe) (Recipe, bool, error) {
	var output rawItem = rawItem{}
	var err error = json.Unmarshal(raw.Output, &output)
	if err != nil {
		return Recipe{}, false, fmt.Errorf("解析熔炼产物: %w", err)
	}
	var meta int32 = MetaAny
	var kind Kind = KindFurnace
	if raw.Type == rawTypeFurnaceData && raw.InputDamage >= 0 {
		meta = raw.InputDamage
		kind = KindFurnaceData
	}
	var result protocol.Item = toItem(output)
	if result.IsAir() || raw.InputID <= 0 {
		return Recipe{}, false, fmt.Errorf("熔炼配方缺少输入或产物")
	}
	var recipe Recipe = Recipe{
		Kind:   kind,
		Input:  []protocol.Item{{ID: raw.InputID, Meta: meta, Count: 1}},
		Output: []protocol.Item{result},
	}
	return recipe, true, nil
}
