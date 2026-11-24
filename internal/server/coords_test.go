package server

import "strings"
import "go-mc-server/internal/network"
import "testing"

// 坐标文本要能带上世界名和三位坐标，并且显示在快捷栏 tip 那一行。
func TestCoordsText(test *testing.T) {
	var player *Player = newPlayer(&network.Session{}, 1)
	player.SetCoordsEnabled(true)
	player.UpdatePosition(12.5, 64.0, -3.25, 0, 0, true)
	var text string = coordsText(player, "world", "world")
	if text == "" {
		test.Fatal("坐标文本不该为空")
	}
	if !containsAll(text, "X: 12.5", "Y: 64.0", "Z: -3.2", "主世界") {
		test.Fatalf("坐标文本不对: %q", text)
	}
}

// 世界名映射：地狱/末地各有中文名，别把坐标标成主世界。
func TestCoordsWorldLabel(test *testing.T) {
	var player *Player = newPlayer(&network.Session{}, 1)
	var netherText string = coordsText(player, "nether", "world")
	if !strings.Contains(netherText, "地狱") {
		test.Fatalf("地狱世界名不对: %q", netherText)
	}
	var endText string = coordsText(player, "end", "world")
	if !strings.Contains(endText, "末地") {
		test.Fatalf("末地世界名不对: %q", endText)
	}
}

func TestCoordsDefaultOn(test *testing.T) {
	var player *Player = newPlayer(&network.Session{}, 1)
	if !player.CoordsEnabled() {
		test.Fatal("新玩家默认应该开着坐标显示")
	}
}

// 关掉后状态要立刻变，方便 sendCoords 发空串清屏。
func TestCoordsToggle(test *testing.T) {
	var player *Player = newPlayer(&network.Session{}, 1)
	player.SetCoordsEnabled(false)
	if player.CoordsEnabled() {
		test.Fatal("关掉后不该还报开着")
	}
	player.SetCoordsEnabled(true)
	if !player.CoordsEnabled() {
		test.Fatal("重新打开后应该报开着")
	}
}

func containsAll(text string, parts ...string) bool {
	var index int = 0
	for index = 0; index < len(parts); index++ {
		if !strings.Contains(text, parts[index]) {
			return false
		}
	}
	return true
}
