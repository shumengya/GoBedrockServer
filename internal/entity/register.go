package entity

import (
	"fmt"
	"strconv"
	"strings"
)

// Type is a registered entity stub. AI / behavior methods will be added later.
type Type struct {
	NetworkID int32
	Name      string
	Aliases   []string
	// CanSummon is false for types that need a special spawn packet this round
	// (dropped Item → AddItemEntity, Painting → AddPainting).
	CanSummon bool
}

var (
	byID   = map[int32]*Type{}
	byName = map[string]*Type{}
)

// Register adds an entity type. Duplicate network IDs merge aliases.
func Register(t *Type) {
	if existing, ok := byID[t.NetworkID]; ok {
		mergeAliases(existing, t)
		if !t.CanSummon {
			existing.CanSummon = false
		}
		indexNames(existing)
		return
	}
	byID[t.NetworkID] = t
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
	t, ok := byID[id]
	return t, ok
}

func ByName(name string) (*Type, bool) {
	t, ok := byName[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// FromString matches PocketMine Entity::createEntity type lookup (single entity).
func FromString(spec string) (*Type, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("实体不能为空")
	}
	spec = strings.ReplaceAll(spec, " ", "_")
	spec = strings.TrimPrefix(strings.ToLower(spec), "minecraft:")

	if id, err := strconv.ParseInt(spec, 10, 32); err == nil {
		if t, ok := ByID(int32(id)); ok {
			return t, nil
		}
		return nil, fmt.Errorf("未知实体: %s", spec)
	}
	if t, ok := ByName(spec); ok {
		return t, nil
	}
	return nil, fmt.Errorf("未知实体: %s", spec)
}
