package access

import "errors"
import "fmt"
import "os"
import "slices"
import "strings"
import "sync"
import "time"

// banTimeLayout 对应参考端的 "Y-m-d H:i:s O"，保证两边的 banned-*.txt 能互相读。
const banTimeLayout string = "2006-01-02 15:04:05 -0700"

const defaultBanSource string = "(Unknown)"
const defaultBanReason string = "Banned by an operator."

// BanEntry 是一条封禁。Key 是小写名字或 IP；Expires 为零值表示永久。
type BanEntry struct {
	Key     string
	Created time.Time
	Source  string
	Expires time.Time
	Reason  string
}

// Expired 报告在 now 这一刻封禁是否已经到期。
func (entry BanEntry) Expired(now time.Time) bool {
	return !entry.Expires.IsZero() && now.After(entry.Expires)
}

// sanitizeField 去掉会破坏文件格式的字符：| 是分隔符，换行会把一条拆成两条。
func sanitizeField(text string) string {
	var cleaned string = strings.NewReplacer("|", "/", "\r", " ", "\n", " ").Replace(text)
	return strings.TrimSpace(cleaned)
}

func (entry BanEntry) line() string {
	var expires string = "Forever"
	if !entry.Expires.IsZero() {
		expires = entry.Expires.Format(banTimeLayout)
	}
	return strings.Join([]string{
		entry.Key,
		entry.Created.Format(banTimeLayout),
		entry.Source,
		expires,
		entry.Reason,
	}, "|")
}

// parseBanLine 解析一行。缺失的后几段按默认值补，对齐参考端 BanEntry::fromString。
func parseBanLine(line string) (BanEntry, error) {
	var parts []string = strings.SplitN(line, "|", 5)
	var entry BanEntry = BanEntry{
		Key:     NormalizeName(parts[0]),
		Created: time.Now(),
		Source:  defaultBanSource,
		Reason:  defaultBanReason,
	}
	if entry.Key == "" {
		return BanEntry{}, fmt.Errorf("缺少名字")
	}
	var err error = nil
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		entry.Created, err = time.Parse(banTimeLayout, strings.TrimSpace(parts[1]))
		if err != nil {
			return BanEntry{}, fmt.Errorf("创建时间 %q: %w", parts[1], err)
		}
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		entry.Source = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		var expire string = strings.TrimSpace(parts[3])
		if expire != "" && !strings.EqualFold(expire, "forever") {
			entry.Expires, err = time.Parse(banTimeLayout, expire)
			if err != nil {
				return BanEntry{}, fmt.Errorf("到期时间 %q: %w", expire, err)
			}
		}
	}
	if len(parts) > 4 && strings.TrimSpace(parts[4]) != "" {
		entry.Reason = strings.TrimSpace(parts[4])
	}
	return entry, nil
}

// BanList 是一份封禁名单（玩家名或 IP），并发安全，每次改动立刻落盘。
type BanList struct {
	mu      sync.RWMutex
	path    string
	entries map[string]BanEntry
}

func NewBanList(path string) *BanList {
	return &BanList{path: path, entries: make(map[string]BanEntry)}
}

// Load 重新读取文件，文件不存在视为空。返回因格式错误被跳过的行，调用方记日志：
// 一行写坏不该让整个封禁列表失效，但也不能悄悄吞掉。
func (list *BanList) Load() ([]string, error) {
	var raw, err = os.ReadFile(list.path)
	if errors.Is(err, os.ErrNotExist) {
		list.mu.Lock()
		list.entries = make(map[string]BanEntry)
		list.mu.Unlock()
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", list.path, err)
	}
	var loaded map[string]BanEntry = make(map[string]BanEntry)
	var skipped []string = nil
	var lines []string = strings.Split(strings.TrimPrefix(string(raw), "\ufeff"), "\n")
	var index int = 0
	for index = 0; index < len(lines); index++ {
		var line string = strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var entry, parseErr = parseBanLine(line)
		if parseErr != nil {
			skipped = append(skipped, fmt.Sprintf("%s 第 %d 行: %v", list.path, index+1, parseErr))
			continue
		}
		loaded[entry.Key] = entry
	}
	list.mu.Lock()
	list.entries = loaded
	list.mu.Unlock()
	return skipped, nil
}

// Find 查一条封禁；已到期的当作没有。now 由调用方传入，测试可以固定时间。
func (list *BanList) Find(key string, now time.Time) (BanEntry, bool) {
	list.mu.RLock()
	defer list.mu.RUnlock()
	var entry, found = list.entries[NormalizeName(key)]
	if !found || entry.Expired(now) {
		return BanEntry{}, false
	}
	return entry, true
}

// Entries 返回未到期的全部封禁，按 Key 排序。
func (list *BanList) Entries(now time.Time) []BanEntry {
	list.mu.RLock()
	defer list.mu.RUnlock()
	var out []BanEntry = make([]BanEntry, 0, len(list.entries))
	var key string = ""
	for key = range list.entries {
		if !list.entries[key].Expired(now) {
			out = append(out, list.entries[key])
		}
	}
	slices.SortFunc(out, func(left BanEntry, right BanEntry) int {
		return strings.Compare(left.Key, right.Key)
	})
	return out
}

// saveLocked 落盘时顺手丢掉已到期的条目。
func (list *BanList) saveLocked(now time.Time) error {
	var kept []BanEntry = make([]BanEntry, 0, len(list.entries))
	var key string = ""
	for key = range list.entries {
		if list.entries[key].Expired(now) {
			delete(list.entries, key)
			continue
		}
		kept = append(kept, list.entries[key])
	}
	slices.SortFunc(kept, func(left BanEntry, right BanEntry) int {
		return strings.Compare(left.Key, right.Key)
	})
	var builder strings.Builder = strings.Builder{}
	builder.WriteString("# 名字|创建时间|来源|到期时间(Forever 为永久)|原因\n")
	var index int = 0
	for index = 0; index < len(kept); index++ {
		builder.WriteString(kept[index].line() + "\n")
	}
	return writeFileAtomic(list.path, []byte(builder.String()))
}

// Add 加一条封禁；已有且未到期的返回 false，不覆盖原来的原因和来源。expires 零值表示永久。
func (list *BanList) Add(key string, source string, reason string, expires time.Time, now time.Time) (bool, error) {
	var normalized string = NormalizeName(key)
	if normalized == "" || strings.ContainsAny(normalized, "\r\n|") {
		return false, fmt.Errorf("封禁对象不合法: %q", key)
	}
	list.mu.Lock()
	defer list.mu.Unlock()
	var existing, found = list.entries[normalized]
	if found && !existing.Expired(now) {
		return false, nil
	}
	var entry BanEntry = BanEntry{
		Key:     normalized,
		Created: now,
		Source:  defaultBanSource,
		Expires: expires,
		Reason:  defaultBanReason,
	}
	if sanitizeField(source) != "" {
		entry.Source = sanitizeField(source)
	}
	if sanitizeField(reason) != "" {
		entry.Reason = sanitizeField(reason)
	}
	list.entries[normalized] = entry
	var err error = list.saveLocked(now)
	if err != nil {
		delete(list.entries, normalized)
		return false, fmt.Errorf("保存封禁列表: %w", err)
	}
	return true, nil
}

// Remove 解除封禁，返回是否真的有这一条。
func (list *BanList) Remove(key string, now time.Time) (bool, error) {
	var normalized string = NormalizeName(key)
	list.mu.Lock()
	defer list.mu.Unlock()
	var existing, found = list.entries[normalized]
	if !found {
		return false, nil
	}
	delete(list.entries, normalized)
	var err error = list.saveLocked(now)
	if err != nil {
		list.entries[normalized] = existing
		return false, fmt.Errorf("保存封禁列表: %w", err)
	}
	return !existing.Expired(now), nil
}
