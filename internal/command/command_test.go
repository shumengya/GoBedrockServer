package command

import (
	"strings"
	"testing"

	"go-mc-server/internal/protocol"
)

func TestIsCommandAndStripPrefix(t *testing.T) {
	if !IsCommand("/give Steve 1") {
		t.Fatal("expected command")
	}
	if IsCommand("hello") {
		t.Fatal("plain chat is not a command")
	}
	if got := StripPrefix(" /give Steve 1 "); got != "give Steve 1" {
		t.Fatalf("strip=%q", got)
	}
}

func TestParseItem(t *testing.T) {
	it, err := ParseItem("1")
	if err != nil || it.ID != 1 || it.Meta != 0 {
		t.Fatalf("1 -> %+v err=%v", it, err)
	}
	it, err = ParseItem("35:2")
	if err != nil || it.ID != 35 || it.Meta != 2 {
		t.Fatalf("35:2 -> %+v err=%v", it, err)
	}
	it, err = ParseItem("stone")
	if err != nil || it.ID != 1 {
		t.Fatalf("stone -> %+v err=%v", it, err)
	}
	it, err = ParseItem("minecraft:wool")
	if err != nil || it.ID != 35 {
		t.Fatalf("minecraft:wool -> %+v err=%v", it, err)
	}
	if _, err := ParseItem("nope"); err == nil {
		t.Fatal("unknown name must fail")
	}
}

type memPlayer struct {
	name  string
	inv   []protocol.Item
	msgs  []string
	syncs int
	gm    int32
	world string
	coord bool
}

func (m *memPlayer) Name() string { return m.name }

func (m *memPlayer) AddItem(item protocol.Item) (protocol.Item, bool) {
	// Minimal merge for give tests.
	for i := range m.inv {
		if m.inv[i].IsAir() {
			m.inv[i] = item
			return protocol.Air(), true
		}
		if m.inv[i].ID == item.ID && m.inv[i].Meta == item.Meta && m.inv[i].Count < MaxStack {
			space := byte(MaxStack) - m.inv[i].Count
			if item.Count <= space {
				m.inv[i].Count += item.Count
				return protocol.Air(), true
			}
			m.inv[i].Count = MaxStack
			item.Count -= space
		}
	}
	return item, false
}

func (m *memPlayer) SyncInventory()   { m.syncs++ }
func (m *memPlayer) Reply(msg string) { m.msgs = append(m.msgs, msg) }
func (m *memPlayer) SetGameMode(gm int32) bool {
	if m.gm == gm {
		return false
	}
	m.gm = gm
	return true
}

func (m *memPlayer) TransferWorld(name string) bool {
	if m.world == name {
		return false
	}
	m.world = name
	return true
}

func (m *memPlayer) CoordsEnabled() bool { return m.coord }

func (m *memPlayer) SetCoordsEnabled(enabled bool) { m.coord = enabled }

func TestGiveCommand(t *testing.T) {
	target := &memPlayer{name: "Steve", inv: make([]protocol.Item, 36)}
	var replies []string
	m := NewMap()
	m.Register(GiveCommand{})
	ctx := &Context{
		Username: "Admin",
		IsOp:     true,
		Reply:    func(msg string) { replies = append(replies, msg) },
		FindPlayer: func(name string) (InventoryPlayer, bool) {
			if name == "Steve" || name == "@s" {
				return target, true
			}
			return nil, false
		},
	}
	m.Dispatch(ctx, "give Steve stone 10")
	if target.inv[0].ID != 1 || target.inv[0].Count != 10 {
		t.Fatalf("inventory %+v", target.inv[0])
	}
	if target.syncs != 1 {
		t.Fatalf("syncs=%d", target.syncs)
	}
	if len(replies) == 0 || len(target.msgs) == 0 {
		t.Fatalf("replies=%v targetMsgs=%v", replies, target.msgs)
	}

	m.Dispatch(ctx, "give")
	if len(replies) < 2 {
		t.Fatal("usage reply expected")
	}
}

func TestDispatchUnknown(t *testing.T) {
	var replies []string
	m := NewMap()
	m.Dispatch(&Context{
		Username: "Steve",
		Reply:    func(msg string) { replies = append(replies, msg) },
	}, "nope")
	if len(replies) != 1 {
		t.Fatalf("replies=%v", replies)
	}
}

func TestAvailableCommandsJSON(t *testing.T) {
	m := NewMap()
	m.Register(GiveCommand{})
	m.Register(SummonCommand{})
	m.Register(GamemodeCommand{})
	m.Register(WorldCommand{})
	m.Register(HelpCommand{Map: m})
	raw := m.AvailableCommandsJSON(true)
	if !strings.Contains(raw, `"give"`) || !strings.Contains(raw, `"help"`) || !strings.Contains(raw, `"summon"`) || !strings.Contains(raw, `"gamemode"`) || !strings.Contains(raw, `"world"`) {
		t.Fatalf("json=%s", raw)
	}
	if !strings.Contains(raw, `"permission":"any"`) {
		t.Fatalf("missing permission any: %s", raw)
	}
	// 普通玩家的命令补全里不该出现管理员专用命令。
	plain := m.AvailableCommandsJSON(false)
	if strings.Contains(plain, `"give"`) || strings.Contains(plain, `"summon"`) || strings.Contains(plain, `"gamemode"`) {
		t.Fatalf("非管理员不该看到管理员命令: %s", plain)
	}
	if !strings.Contains(plain, `"help"`) || !strings.Contains(plain, `"world"`) {
		t.Fatalf("非管理员应仍能看到公共命令: %s", plain)
	}
}

func TestSummonCommand(t *testing.T) {
	var gotID int32
	var gotX, gotY, gotZ float32
	var calls int
	var replies []string
	m := NewMap()
	m.Register(SummonCommand{})
	ctx := &Context{
		Username: "Steve",
		IsOp:     true,
		Reply:    func(msg string) { replies = append(replies, msg) },
		Position: func() (float32, float32, float32, bool) {
			return 10, 64, 20, true
		},
		SpawnEntity: func(networkID int32, x, y, z float32) error {
			calls++
			gotID, gotX, gotY, gotZ = networkID, x, y, z
			return nil
		},
	}
	m.Dispatch(ctx, "summon zombie")
	if calls != 1 || gotID != 32 || gotX != 10 || gotY != 64 || gotZ != 20 {
		t.Fatalf("summon zombie id=%d pos=(%v,%v,%v) calls=%d replies=%v", gotID, gotX, gotY, gotZ, calls, replies)
	}

	m.Dispatch(ctx, "summon zombie ~ ~1 ~")
	if calls != 2 || gotY != 65 {
		t.Fatalf("relative y=%v calls=%d", gotY, calls)
	}

	before := calls
	m.Dispatch(ctx, "summon painting")
	if calls != before {
		t.Fatal("painting must not spawn this round")
	}
	if len(replies) == 0 {
		t.Fatal("expected painting error reply")
	}

	m.Dispatch(ctx, "summon nope")
	if calls != before {
		t.Fatal("unknown entity must not spawn")
	}
}

func TestParseGameMode(t *testing.T) {
	gm, err := ParseGameMode("1")
	if err != nil || gm != protocol.GameModeCreative {
		t.Fatalf("1 -> %d %v", gm, err)
	}
	gm, err = ParseGameMode("creative")
	if err != nil || gm != protocol.GameModeCreative {
		t.Fatalf("creative -> %d %v", gm, err)
	}
	gm, err = ParseGameMode("s")
	if err != nil || gm != protocol.GameModeSurvival {
		t.Fatalf("s -> %d %v", gm, err)
	}
	if _, err := ParseGameMode("2"); err == nil {
		t.Fatal("adventure must be rejected this round")
	}
	if _, err := ParseGameMode("spectator"); err == nil {
		t.Fatal("spectator must be rejected this round")
	}
}

func TestGamemodeCommand(t *testing.T) {
	target := &memPlayer{name: "Steve"}
	other := &memPlayer{name: "Alex"}
	var replies []string
	m := NewMap()
	m.Register(GamemodeCommand{})
	ctx := &Context{
		Username: "Steve",
		IsOp:     true,
		Reply:    func(msg string) { replies = append(replies, msg) },
		FindPlayer: func(name string) (InventoryPlayer, bool) {
			switch name {
			case "@s", "Steve":
				return target, true
			case "Alex":
				return other, true
			default:
				return nil, false
			}
		},
	}
	m.Dispatch(ctx, "gamemode")
	if len(replies) == 0 {
		t.Fatal("missing args should reply")
	}
	m.Dispatch(ctx, "gm 1")
	if target.gm != protocol.GameModeCreative {
		t.Fatalf("gm=%d", target.gm)
	}
	found := false
	for _, r := range replies {
		if r == "已将游戏模式设为创造" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("self reply=%v", replies)
	}
	before := len(replies)
	m.Dispatch(ctx, "gamemode creative")
	if len(replies) <= before {
		t.Fatal("same mode should reply")
	}
	m.Dispatch(ctx, "gamemode 2")
	if other.gm != 0 {
		t.Fatal("adventure must not apply")
	}
	m.Dispatch(ctx, "gamemode 1 Alex")
	if other.gm != protocol.GameModeCreative {
		t.Fatalf("alex gm=%d", other.gm)
	}
	if len(other.msgs) == 0 {
		t.Fatal("target should be notified")
	}
}

func TestParseWorldName(t *testing.T) {
	cases := []struct {
		in, def, want string
	}{
		{"sky", "world", "sky"},
		{"空岛", "world", "sky"},
		{"overworld", "world", "world"},
		{"world", "world", "world"},
		{"flat", "custom", "custom"},
		{"超平坦", "world", "world"},
		{"海洋", "world", "ocean"},
		{"ocean", "world", "ocean"},
		{"nether", "world", "nether"},
		{"地狱", "world", "nether"},
		{"下界", "world", "nether"},
		{"末地", "world", "end"},
		{"end", "world", "end"},
		{"主世界", "world", "world"},
	}
	for _, c := range cases {
		got, err := ParseWorldName(c.in, c.def, []string{c.def, "sky", "ocean", "nether", "end"})
		if err != nil || got != c.want {
			t.Fatalf("%s def=%s -> %s %v want %s", c.in, c.def, got, err, c.want)
		}
	}
	if _, err := ParseWorldName("nope", "world", []string{"world"}); err == nil {
		t.Fatal("unknown world must be rejected")
	}
}

func TestWorldCommand(t *testing.T) {
	self := &memPlayer{name: "Steve", world: "world"}
	var replies []string
	m := NewMap()
	m.Register(WorldCommand{})
	ctx := &Context{
		Username:     "Steve",
		DefaultWorld: "world",
		Admin:        &fakeAdmin{worlds: []string{"world", "sky", "ocean", "nether", "end"}},
		Reply:        func(msg string) { replies = append(replies, msg) },
		FindPlayer: func(name string) (InventoryPlayer, bool) {
			if name == "@s" || name == "Steve" {
				return self, true
			}
			return nil, false
		},
	}
	m.Dispatch(ctx, "world")
	if len(replies) == 0 {
		t.Fatal("missing args should list worlds")
	}
	m.Dispatch(ctx, "world sky")
	if self.world != "sky" {
		t.Fatalf("world=%s", self.world)
	}
	before := len(replies)
	m.Dispatch(ctx, "world 空岛")
	if len(replies) <= before {
		t.Fatal("same world should reply")
	}
	m.Dispatch(ctx, "world 海洋")
	if self.world != "ocean" {
		t.Fatalf("ocean world=%s", self.world)
	}
	m.Dispatch(ctx, "world 地狱")
	if self.world != "nether" {
		t.Fatalf("nether world=%s", self.world)
	}
	m.Dispatch(ctx, "world 末地")
	if self.world != "end" {
		t.Fatalf("end world=%s", self.world)
	}
	m.Dispatch(ctx, "world overworld")
	if self.world != "world" {
		t.Fatalf("overworld world=%s", self.world)
	}
	m.Dispatch(ctx, "world nope")
	if self.world != "world" {
		t.Fatal("unknown world must not transfer")
	}
	ctx.Admin = &fakeAdmin{worlds: []string{"Lobby", "Quarry"}}
	ctx.DefaultWorld = "Lobby"
	m.Dispatch(ctx, "world")
	if replies[len(replies)-1] != "可用世界: Lobby, Quarry" {
		t.Fatalf("期望列出已加载目录，实际 %s", replies[len(replies)-1])
	}
	m.Dispatch(ctx, "world quarry")
	m.Dispatch(ctx, "world 下界")
	if self.world != "Quarry" || !strings.Contains(replies[len(replies)-1], "未知世界") {
		t.Fatalf("期望传送到 Quarry 后拒绝未加载的下界，实际 %s / %v", self.world, replies)
	}
}

func TestCoordCommand(test *testing.T) {
	var self *memPlayer = &memPlayer{name: "Steve", coord: true}
	var replies []string = nil
	var commands *Map = NewMap()
	commands.Register(CoordCommand{})
	var ctx *Context = &Context{
		Username: "Steve",
		Reply:    func(msg string) { replies = append(replies, msg) },
		FindPlayer: func(name string) (InventoryPlayer, bool) {
			if name == "@s" || name == "Steve" {
				return self, true
			}
			return nil, false
		},
	}
	// 默认开着，不带参数应该关掉。
	commands.Dispatch(ctx, "coord")
	if self.coord {
		test.Fatal("bare /coord should toggle the default-on display off")
	}
	if len(replies) == 0 {
		test.Fatal("toggling should reply")
	}
	commands.Dispatch(ctx, "coord")
	if !self.coord {
		test.Fatal("toggling twice should turn it back on")
	}
	commands.Dispatch(ctx, "coord off")
	if self.coord {
		test.Fatal("/coord off should disable")
	}
	commands.Dispatch(ctx, "coord on")
	if !self.coord {
		test.Fatal("/coord on should enable")
	}
	// 别名和中文参数（参数认中文，命令名不认）。
	commands.Dispatch(ctx, "coords 关闭")
	if self.coord {
		test.Fatal("alias with chinese argument should disable")
	}
	commands.Dispatch(ctx, "pos 开启")
	if !self.coord {
		test.Fatal("pos alias should enable")
	}
	// 中文命令名已废弃，必须当未知命令拒掉，不能悄悄生效。
	commands.Dispatch(ctx, "坐标 关闭")
	if !self.coord {
		test.Fatal("chinese alias must no longer work")
	}
	var before int = len(replies)
	commands.Dispatch(ctx, "coord nope")
	if !self.coord {
		test.Fatal("bad argument must not change the state")
	}
	if len(replies) <= before {
		test.Fatal("bad argument should reply with usage")
	}
}
