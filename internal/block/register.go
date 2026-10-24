package block

import "strings"

// Type is a registered block stub. Behavior methods will be added later.
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

// Register adds a block type. Duplicate IDs merge aliases.
func Register(t *Type) {
	if t.MaxStack <= 0 {
		t.MaxStack = 64
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
	t, ok := byID[id]
	return t, ok
}

func ByName(name string) (*Type, bool) {
	t, ok := byName[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}
