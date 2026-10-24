package main

import (
	"bufio"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type entry struct {
	ID      int
	Primary string
	Aliases []string
	Name    string
}

func main() {
	root := findRoot()
	pm := filepath.Join(root, "pocketmine服务器参考", "src", "pocketmine")
	blocks := parseConsts(filepath.Join(pm, "block", "BlockIds.php"))
	items := parseConsts(filepath.Join(pm, "item", "ItemIds.php"))
	display := parseDisplayNames(filepath.Join(pm, "item"))
	for k, v := range parseDisplayNames(filepath.Join(pm, "block")) {
		if _, ok := display[k]; !ok {
			display[k] = v
		}
	}

	blockDir := filepath.Join(root, "internal", "block")
	itemDir := filepath.Join(root, "internal", "item")
	cleanGenerated(blockDir, map[string]struct{}{"register.go": {}, "register_test.go": {}})
	cleanGenerated(itemDir, map[string]struct{}{"register.go": {}, "register_test.go": {}})

	blockList := group(blocks, display, 0, 255)
	itemList := group(items, display, 256, 65535)
	writePackage(blockDir, "block", blockList)
	writePackage(itemDir, "item", itemList)
	fmt.Printf("generated blocks=%d items=%d\n", len(blockList), len(itemList))
}

func findRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
		return wd
	}
	return filepath.Dir(wd)
}

var constRe = regexp.MustCompile(`([A-Z][A-Z0-9_]*)\s*=\s*(\d+)`)

func parseConsts(path string) map[int][]string {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	out := map[int][]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.Contains(line, "const ") && !strings.Contains(line, "=") {
			continue
		}
		if !strings.Contains(line, "const") && !constRe.MatchString(line) {
			continue
		}
		for _, m := range constRe.FindAllStringSubmatch(line, -1) {
			id, _ := strconv.Atoi(m[2])
			name := m[1]
			if hasName(out[id], name) {
				continue
			}
			out[id] = append(out[id], name)
		}
	}
	return out
}

func hasName(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

var (
	constructRe = regexp.MustCompile(`parent::__construct\(\s*self::([A-Z][A-Z0-9_]*)\s*,[^;]*"([^"]+)"`)
	idAssignRe  = regexp.MustCompile(`\$id\s*=\s*self::([A-Z][A-Z0-9_]*)`)
	getNameRe   = regexp.MustCompile(`function getName\s*(?::\s*string)?\s*\([^)]*\)[^{]*\{\s*return\s+"([^"]+)"`)
)

func parseDisplayNames(dir string) map[string]string {
	out := map[string]string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".php") {
			return nil
		}
		if filepath.Dir(path) != dir {
			return nil
		}
		base := strings.TrimSuffix(info.Name(), ".php")
		switch base {
		case "Item", "ItemIds", "ItemBlock", "Block", "BlockIds", "Food", "FoodSource", "Tool", "Armor", "Door":
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if m := constructRe.FindSubmatch(raw); len(m) == 3 {
			out[string(m[1])] = string(m[2])
			return nil
		}
		if m := idAssignRe.FindSubmatch(raw); len(m) == 2 {
			if n := getNameRe.FindSubmatch(raw); len(n) == 2 {
				out[string(m[1])] = string(n[1])
			}
		}
		return nil
	})
	return out
}

func group(consts map[int][]string, display map[string]string, min, max int) []entry {
	ids := make([]int, 0, len(consts))
	for id := range consts {
		if id < min || id > max {
			continue
		}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]entry, 0, len(ids))
	for _, id := range ids {
		names := consts[id]
		primary := names[0]
		aliases := make([]string, 0, len(names))
		seen := map[string]struct{}{}
		for _, n := range names {
			a := strings.ToLower(n)
			if _, ok := seen[a]; ok {
				continue
			}
			seen[a] = struct{}{}
			aliases = append(aliases, a)
		}
		disp := display[primary]
		if disp == "" {
			for _, n := range names {
				if d := display[n]; d != "" {
					disp = d
					break
				}
			}
		}
		if disp == "" {
			disp = titleFromConst(primary)
		}
		out = append(out, entry{ID: id, Primary: primary, Aliases: aliases, Name: disp})
	}
	return out
}

func titleFromConst(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

func cleanGenerated(dir string, keep map[string]struct{}) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(dir, 0o755)
			return
		}
		panic(err)
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if _, ok := keep[e.Name()]; ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(raw), "Code generated by scripts/gen_pm_registry.go") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func writePackage(dir, pkg string, list []entry) {
	_ = os.MkdirAll(dir, 0o755)
	used := map[string]int{}
	for _, e := range list {
		file := uniqueFile(strings.ToLower(e.Primary), used)
		path := filepath.Join(dir, file+".go")
		aliasLit := strings.Join(quoteAll(e.Aliases), ", ")
		body := fmt.Sprintf(`// Code generated by scripts/gen_pm_registry.go; DO NOT EDIT.

package %s

func init() {
	Register(&Type{
		ID:       %d,
		Name:     %q,
		Aliases:  []string{%s},
		MaxStack: 64,
	})
}
`, pkg, e.ID, e.Name, aliasLit)
		formatted, err := format.Source([]byte(body))
		if err != nil {
			panic(fmt.Errorf("%s: %w", path, err))
		}
		if err := os.WriteFile(path, formatted, 0o644); err != nil {
			panic(err)
		}
	}
}

func uniqueFile(base string, used map[string]int) string {
	base = strings.Map(func(r rune) rune {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, base)
	if base == "" || unicode.IsDigit(rune(base[0])) || base == "register" || strings.HasSuffix(base, "_test") {
		base = "id_" + base
	}
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n)
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strconv.Quote(s)
	}
	return out
}
