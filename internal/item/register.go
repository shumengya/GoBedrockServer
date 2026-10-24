package item

import (
	"fmt"
	"strconv"
	"strings"

	"go-mc-server/internal/block"
	"go-mc-server/internal/protocol"
)

// Type is a registered item stub. Behavior methods will be added later.
type Type struct {
	ID       int32
	Name     string
	Aliases  []string
	MaxStack int
}

var (
	byID   = map[int32]*Type{}
	byName = map[string]*Type{}
)

// unstackableIDs 生成文件把所有物品的 MaxStack 都写成 64；工具和武器带耐久，
// 实际一格只能放一个，这里统一修正，免得重新生成时被覆盖。
var unstackableIDs map[int32]bool = map[int32]bool{
	256: true, 257: true, 258: true, 267: true, 268: true, 269: true, 270: true, 271: true,
	272: true, 273: true, 274: true, 275: true, 276: true, 277: true, 278: true, 279: true,
	283: true, 284: true, 285: true, 286: true,
	290: true, 291: true, 292: true, 293: true, 294: true,
}

// Register adds an item type. Duplicate IDs merge aliases.
func Register(t *Type) {
	if t.MaxStack <= 0 {
		t.MaxStack = 64
	}
	if unstackableIDs[t.ID] {
		t.MaxStack = 1
	}
	if existing, ok := byID[t.ID]; ok {
		mergeAliases(existing, t)
		indexNames(existing)
		return
	}
	byID[t.ID] = t
	indexNames(t)
}

func mergeAliases(dst, src *Type) {
	seen := map[string]struct{}{}
	for _, a := range dst.Aliases {
		seen[strings.ToLower(a)] = struct{}{}
	}
	for _, a := range src.Aliases {
		k := strings.ToLower(a)
		if _, ok := seen[k]; ok {
			continue
		}
		dst.Aliases = append(dst.Aliases, k)
		seen[k] = struct{}{}
	}
}

func indexNames(t *Type) {
	names := make([]string, 0, 2+len(t.Aliases))
	names = append(names, strings.ToLower(strings.ReplaceAll(t.Name, " ", "_")))
	names = append(names, t.Aliases...)
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		byName[n] = t
	}
}

func ByID(id int32) (*Type, bool) {
	if t, ok := byID[id]; ok {
		return t, true
	}
	return fromBlock(id)
}

func ByName(name string) (*Type, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if t, ok := byName[key]; ok {
		return t, true
	}
	bt, ok := block.ByName(key)
	if !ok {
		return nil, false
	}
	return fromBlock(bt.ID)
}

// FromString matches PocketMine Item::fromString (single item).
func FromString(spec string) (protocol.Item, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return protocol.Air(), fmt.Errorf("物品不能为空")
	}
	spec = strings.ReplaceAll(spec, " ", "_")
	spec = strings.TrimPrefix(strings.ToLower(spec), "minecraft:")

	idPart := spec
	meta := int32(0)
	if i := strings.IndexByte(spec, ':'); i >= 0 {
		idPart = spec[:i]
		m, err := strconv.ParseInt(spec[i+1:], 10, 32)
		if err != nil {
			return protocol.Air(), fmt.Errorf("无效物品 meta: %s", spec)
		}
		meta = int32(m)
	}

	if id, err := strconv.ParseInt(idPart, 10, 32); err == nil {
		return protocol.Item{ID: int32(id) & 0xffff, Meta: meta}, nil
	}

	if t, ok := ByName(idPart); ok {
		return protocol.Item{ID: t.ID, Meta: meta}, nil
	}
	return protocol.Air(), fmt.Errorf("未知物品: %s", spec)
}
