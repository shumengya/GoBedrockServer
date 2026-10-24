package server

import "testing"

import "go-mc-server/config"
import "go-mc-server/internal/access"
import "go-mc-server/internal/command"
import "go-mc-server/internal/entity"
import "go-mc-server/internal/network"
import "go-mc-server/internal/world"

type kickRecord struct {
	Name    string
	Message string
}

// adminFixture 是一台不联网的服务器：世界、命令、访问控制都是真的，网络层是空壳，
// 踢人通过 kickHook 记录下来而不是真的断开。
type adminFixture struct {
	srv   *Server
	kicks []kickRecord
}

func newAdminFixture(test *testing.T) *adminFixture {
	var overworld *world.World = world.New("world")
	var nether *world.World = world.New("nether")
	nether.Generator = world.GeneratorNether
	var controls, _, err = access.Open(test.TempDir())
	if err != nil {
		test.Fatalf("打开访问控制: %v", err)
	}
	var fixture *adminFixture = &adminFixture{}
	fixture.srv = &Server{
		cfg:           &config.Config{DataDir: test.TempDir(), PVP: true, ViewDistance: 4},
		world:         overworld,
		worlds:        map[string]*world.World{"world": overworld, "nether": nether},
		players:       make(map[string]*Player),
		entities:      make(map[int64]*entity.Instance),
		stopCh:        make(chan struct{}),
		stopRequested: make(chan struct{}),
		cmds:          command.NewMap(),
		access:        controls,
	}
	var commands []command.Command = command.AdminCommands()
	var index int = 0
	for index = 0; index < len(commands); index++ {
		fixture.srv.cmds.Register(commands[index])
	}
	fixture.srv.kickHook = func(player *Player, message string) {
		fixture.kicks = append(fixture.kicks, kickRecord{Name: player.Username, Message: message})
	}
	return fixture
}

// join 让一个玩家以「已进入游戏」的状态上线，位置在世界原点、脚下 y=64。
func (fixture *adminFixture) join(name string, address string, worldName string) *Player {
	var sess *network.Session = &network.Session{Username: name, ClientUUID: "uuid-" + name}
	sess.SetState(network.StatePlaying)
	var player *Player = newPlayer(sess, int64(len(fixture.srv.players)+1))
	player.RemoteIP = address
	player.WorldName = worldName
	player.X, player.Y, player.Z = 0, 64+playerEyeHeight, 0
	fixture.srv.players[sess.ClientUUID] = player
	return player
}

func (fixture *adminFixture) admin() serverAdmin {
	return serverAdmin{srv: fixture.srv}
}

func (fixture *adminFixture) kickedNames() []string {
	var names []string = make([]string, 0, len(fixture.kicks))
	var index int = 0
	for index = 0; index < len(fixture.kicks); index++ {
		names = append(names, fixture.kicks[index].Name)
	}
	return names
}
