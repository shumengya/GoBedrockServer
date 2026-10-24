package access

import "errors"
import "fmt"
import "os"
import "slices"
import "strings"
import "sync"

// NormalizeName 是名字比对的唯一口径：去首尾空白、转小写。
// 离线模式下大小写不同的名字是同一个人，不统一的话封禁和 OP 都能靠改大小写绕过。
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// writeFileAtomic 先写临时文件再改名，写到一半断电也不会留下半截名单。
func writeFileAtomic(path string, data []byte) error {
	var tmp string = path + ".tmp"
	var err error = os.WriteFile(tmp, data, 0o644)
	if err != nil {
		return fmt.Errorf("写入 %s: %w", tmp, err)
	}
	err = os.Rename(tmp, path)
	if err != nil {
		return fmt.Errorf("替换 %s: %w", path, err)
	}
	return nil
}

// NameList 是一份「每行一个名字」的名单，ops.txt 和 white-list.txt 都用它，格式和参考端一致。
type NameList struct {
	mu    sync.RWMutex
	path  string
	names map[string]struct{}
}

func NewNameList(path string) *NameList {
	return &NameList{path: path, names: make(map[string]struct{})}
}

// Path 返回名单文件的路径。
func (list *NameList) Path() string {
	return list.path
}

// Load 从文件重新读取，文件不存在视为空名单。
func (list *NameList) Load() error {
	var raw, err = os.ReadFile(list.path)
	if errors.Is(err, os.ErrNotExist) {
		list.mu.Lock()
		list.names = make(map[string]struct{})
		list.mu.Unlock()
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 %s: %w", list.path, err)
	}
	var loaded map[string]struct{} = make(map[string]struct{})
	var lines []string = strings.Split(strings.TrimPrefix(string(raw), "\ufeff"), "\n")
	var index int = 0
	for index = 0; index < len(lines); index++ {
		var name string = NormalizeName(lines[index])
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		loaded[name] = struct{}{}
	}
	list.mu.Lock()
	list.names = loaded
	list.mu.Unlock()
	return nil
}

// Has 按名字查（大小写不敏感）。
func (list *NameList) Has(name string) bool {
	list.mu.RLock()
	defer list.mu.RUnlock()
	var _, found = list.names[NormalizeName(name)]
	return found
}

// Names 返回排好序的全部名字。
func (list *NameList) Names() []string {
	list.mu.RLock()
	defer list.mu.RUnlock()
	var out []string = make([]string, 0, len(list.names))
	var name string = ""
	for name = range list.names {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (list *NameList) saveLocked() error {
	var names []string = make([]string, 0, len(list.names))
	var name string = ""
	for name = range list.names {
		names = append(names, name)
	}
	slices.Sort(names)
	var text string = strings.Join(names, "\n")
	if text != "" {
		text += "\n"
	}
	return writeFileAtomic(list.path, []byte(text))
}

// Add 加入名单，返回是否真的新增了。名字里不能带换行，否则一条会变两条。
func (list *NameList) Add(name string) (bool, error) {
	var key string = NormalizeName(name)
	if key == "" || strings.ContainsAny(key, "\r\n") {
		return false, fmt.Errorf("名字不合法: %q", name)
	}
	list.mu.Lock()
	defer list.mu.Unlock()
	var _, exists = list.names[key]
	if exists {
		return false, nil
	}
	list.names[key] = struct{}{}
	var err error = list.saveLocked()
	if err != nil {
		delete(list.names, key)
		return false, fmt.Errorf("保存名单: %w", err)
	}
	return true, nil
}

// Remove 移出名单，返回是否真的移除了。
func (list *NameList) Remove(name string) (bool, error) {
	var key string = NormalizeName(name)
	list.mu.Lock()
	defer list.mu.Unlock()
	var _, exists = list.names[key]
	if !exists {
		return false, nil
	}
	delete(list.names, key)
	var err error = list.saveLocked()
	if err != nil {
		list.names[key] = struct{}{}
		return false, fmt.Errorf("保存名单: %w", err)
	}
	return true, nil
}
