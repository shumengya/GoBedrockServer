package crafting

import "encoding/hex"
import "fmt"
import "strings"

import "go-mc-server/internal/protocol"

// Kind 是配方种类，数值就是 CraftingData 包里的 entry type。
type Kind int32

const KindShapeless Kind = 0
const KindShaped Kind = 1
const KindFurnace Kind = 2
const KindFurnaceData Kind = 3

// MetaAny 表示配方材料不限 meta（任意木板、任意羊毛都行）。
const MetaAny int32 = -1

// Recipe 是一条可合成/可熔炼的配方。
//
// 有序配方：Input 按行优先排成 Width×Height，空格是空气。
// 无序配方：Input 是材料清单。
// 熔炼配方：Input 只有一项；Meta 为 MetaAny 表示不限 meta。
type Recipe struct {
	Kind   Kind
	ID     [16]byte
	Width  int32
	Height int32
	Input  []protocol.Item
	Output []protocol.Item
}

// ParseUUID 把带横线的标准写法转成 16 字节，字节序和 putUUID 的入参一致。
func ParseUUID(text string) ([16]byte, error) {
	var out [16]byte = [16]byte{}
	var clean string = strings.ReplaceAll(strings.TrimSpace(text), "-", "")
	if len(clean) != 32 {
		return out, fmt.Errorf("uuid 长度不对: %q", text)
	}
	var raw, err = hex.DecodeString(clean)
	if err != nil {
		return out, fmt.Errorf("解析 uuid %q: %w", text, err)
	}
	copy(out[:], raw)
	return out, nil
}

// NeedsBench 报告这条配方是否必须开工作台：超过 2×2 的格子放不进背包合成区。
func (recipe Recipe) NeedsBench() bool {
	if recipe.Kind == KindShaped {
		return recipe.Width > 2 || recipe.Height > 2
	}
	if recipe.Kind == KindShapeless {
		return len(recipe.Input) > 4
	}
	return false
}

// Craftable 报告这条配方能不能在合成格里做（熔炼配方不行）。
func (recipe Recipe) Craftable() bool {
	return recipe.Kind == KindShaped || recipe.Kind == KindShapeless
}
