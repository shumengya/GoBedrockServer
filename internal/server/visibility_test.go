package server

import "testing"

import "go-mc-server/internal/network"
import "go-mc-server/internal/protocol/packets"
import "go-mc-server/internal/world"

func TestAddPlayerPacketUsesFeetY(test *testing.T) {
	var player *Player = &Player{
		UUID:            "uuid",
		Username:        "Alex",
		EntityRuntimeID: 3,
		X:               1,
		Y:               6.62,
		Z:               2,
		Yaw:             45,
	}
	var pkt *packets.AddPlayerPacket = addPlayerPacket(player)
	var feetDelta float32 = pkt.PosY - 5
	if feetDelta > 1e-4 || feetDelta < -1e-4 {
		test.Fatalf("脚底 y = %v，应为 5", pkt.PosY)
	}
	if pkt.Username != "Alex" || pkt.EntityRuntimeID != 3 || pkt.HeadYaw != 45 {
		test.Fatalf("got %+v", pkt)
	}
}

func TestViewersOfFiltersWorldAndVisibility(test *testing.T) {
	var self *Player = &Player{UUID: "self", WorldName: "world", Visible: true, Session: &network.Session{}}
	var srv *Server = &Server{
		world: world.New("world"),
		players: map[string]*Player{
			"self":   self,
			"same":   {UUID: "same", WorldName: "", Visible: true, Session: &network.Session{}}, // 空世界名视为默认世界
			"hidden": {UUID: "hidden", WorldName: "world", Visible: false, Session: &network.Session{}},
			"other":  {UUID: "other", WorldName: "sky", Visible: true, Session: &network.Session{}},
		},
	}
	var viewers []*Player = srv.viewersOf(self)
	if len(viewers) != 1 || viewers[0].UUID != "same" {
		test.Fatalf("viewers 数量 = %d，应只有 same", len(viewers))
	}
}
