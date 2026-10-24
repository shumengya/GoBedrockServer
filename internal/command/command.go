package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-mc-server/internal/protocol"
)

// Prefix is the default chat command prefix.
const Prefix = "/"

// MaxStack is the max item count for /give in this round.
const MaxStack = 64

// Command is a slash command handler.
type Command interface {
	Name() string
	Aliases() []string
	Usage() string
	Description() string
	Execute(ctx *Context, args []string) error
}

// InventoryPlayer is the player surface commands need for inventory ops.
type InventoryPlayer interface {
	Name() string
	AddItem(item protocol.Item) (leftover protocol.Item, ok bool)
	SyncInventory()
	Reply(msg string)
	SetGameMode(gm int32) bool
	TransferWorld(name string) bool
	// CoordsEnabled 报告快捷栏上方是否正在显示实时坐标。
	CoordsEnabled() bool
	SetCoordsEnabled(enabled bool)
}

// Context is per-dispatch state for a command run by a player.
type Context struct {
	Username   string
	Reply      func(msg string)
	FindPlayer func(name string) (InventoryPlayer, bool)
	// Position is the executor's current position (for ~ relative coords).
	Position func() (x, y, z float32, ok bool)
	// SpawnEntity places a registered entity in the world (no AI).
	SpawnEntity func(networkID int32, x, y, z float32) error
	// DefaultWorld is the disk name of the overworld (usually "world").
	DefaultWorld string

	// IsConsole 为真表示命令来自服务器控制台，视为拥有全部权限。
	IsConsole bool
	// IsOp 表示执行者是管理员。
	IsOp bool
	// World 是执行者当前所在世界；控制台没有所在世界，留空时按 DefaultWorld 算。
	World string
	// Admin 是管理类命令用的服务器能力（踢人、封禁、传送……）。
	Admin Admin
}

// Map registers and dispatches commands by name/alias.
type Map struct {
	cmds map[string]Command
	list []Command
}

func NewMap() *Map {
	return &Map{cmds: make(map[string]Command)}
}

func (m *Map) Register(c Command) {
	name := strings.ToLower(c.Name())
	if _, exists := m.cmds[name]; !exists {
		m.list = append(m.list, c)
	}
	m.cmds[name] = c
	for _, a := range c.Aliases() {
		m.cmds[strings.ToLower(a)] = c
	}
}

// Commands returns registered primary commands (no alias duplicates).
func (m *Map) Commands() []Command {
	out := make([]Command, len(m.list))
	copy(out, m.list)
	return out
}

// IsCommand reports whether msg should be treated as a slash command.
func IsCommand(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), Prefix)
}

// StripPrefix removes a leading / (and surrounding spaces) from a chat line.
func StripPrefix(msg string) string {
	msg = strings.TrimSpace(msg)
	if strings.HasPrefix(msg, Prefix) {
		return strings.TrimSpace(msg[len(Prefix):])
	}
	return msg
}

// Dispatch runs a command line without the leading slash (e.g. "give Steve 1 64").
func (m *Map) Dispatch(ctx *Context, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		if ctx.Reply != nil {
			ctx.Reply("未知命令")
		}
		return
	}
	parts := splitArgs(line)
	name := strings.ToLower(parts[0])
	args := parts[1:]
	c, ok := m.cmds[name]
	if !ok {
		if ctx.Reply != nil {
			ctx.Reply(fmt.Sprintf("未知命令: /%s", name))
		}
		return
	}
	if requiresOp(c) && !ctx.HasOp() {
		if ctx.Reply != nil {
			ctx.Reply("§c你没有权限使用这个命令")
		}
		return
	}
	if err := c.Execute(ctx, args); err != nil {
		if ctx.Reply != nil {
			ctx.Reply(err.Error())
		}
	}
}

// AvailableCommandsJSON builds protocol-113 AvailableCommands payload (PocketMine style).
// isOp 为假时不列出管理员专用命令，客户端的命令补全里就看不到它们。
func (m *Map) AvailableCommandsJSON(isOp bool) string {
	type param struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Optional bool   `json:"optional"`
	}
	type input struct {
		Parameters []param `json:"parameters"`
	}
	type overload struct {
		Input  input          `json:"input"`
		Output map[string]any `json:"output"`
	}
	type version struct {
		Aliases     []string            `json:"aliases"`
		Description string              `json:"description"`
		Overloads   map[string]overload `json:"overloads"`
		Permission  string              `json:"permission"`
	}
	type entry struct {
		Versions []version `json:"versions"`
	}

	root := make(map[string]entry, len(m.list))
	for _, c := range m.list {
		if requiresOp(c) && !isOp {
			continue
		}
		aliases := c.Aliases()
		if aliases == nil {
			aliases = []string{}
		}
		desc := c.Description()
		if desc == "" {
			desc = c.Usage()
		}
		root[strings.ToLower(c.Name())] = entry{
			Versions: []version{{
				Aliases:     aliases,
				Description: desc,
				Permission:  "any",
				Overloads: map[string]overload{
					"default": {
						Input: input{
							Parameters: []param{{
								Name:     "args",
								Type:     "rawtext",
								Optional: true,
							}},
						},
						Output: map[string]any{},
					},
				},
			}},
		}
	}
	raw, err := json.Marshal(root)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func splitArgs(line string) []string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return []string{""}
	}
	return fields
}
