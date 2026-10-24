package server

import "errors"
import "testing"

import "go-mc-server/internal/command"
import "go-mc-server/internal/network"

// 完整名字优先（忽略大小写），其次才是唯一前缀；前缀不唯一必须拒绝，免得踢错人。
func TestLookupOnlineExactPrefixAndAmbiguity(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Steve", "1.1.1.1", "world")
	fixture.join("Stella", "1.1.1.2", "world")
	fixture.join("Alex", "1.1.1.3", "world")
	var admin serverAdmin = fixture.admin()

	var player, err = admin.lookupOnline("STEVE")
	if err != nil || player.Username != "Steve" {
		test.Fatalf("完整名字应忽略大小写: %v %v", player, err)
	}
	player, err = admin.lookupOnline("al")
	if err != nil || player.Username != "Alex" {
		test.Fatalf("唯一前缀应命中 Alex: %v %v", player, err)
	}
	_, err = admin.lookupOnline("Ste")
	if !errors.Is(err, command.ErrAmbiguousName) {
		test.Fatalf("Ste 同时匹配 Steve 和 Stella，应报歧义: %v", err)
	}
	_, err = admin.lookupOnline("Ghost")
	if !errors.Is(err, command.ErrPlayerNotFound) {
		test.Fatalf("不存在的玩家应报未找到: %v", err)
	}
	_, err = admin.lookupOnline("   ")
	if !errors.Is(err, command.ErrPlayerNotFound) {
		test.Fatalf("空名字应报未找到: %v", err)
	}
}

// 玩家恰好叫 Ste 时，输入 Ste 就是他本人，不能因为还有 Steve 就判歧义。
func TestLookupOnlineExactBeatsPrefix(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.join("Ste", "1.1.1.1", "world")
	fixture.join("Steve", "1.1.1.2", "world")
	var player, err = fixture.admin().lookupOnline("ste")
	if err != nil || player.Username != "Ste" {
		test.Fatalf("完整匹配应压过前缀匹配: %v %v", player, err)
	}
}

// 还在加载区块的玩家：能被找到（可以踢、可以封），但没有位置可传、也收不了私聊。
func TestLookupPlayingRequiresPlayingState(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	var loading *Player = fixture.join("Loader", "1.1.1.1", "world")
	loading.Session.SetState(network.StateSpawning)
	fixture.join("Steve", "1.1.1.2", "world")
	var admin serverAdmin = fixture.admin()

	var _, onlineErr = admin.lookupOnline("Loader")
	if onlineErr != nil {
		test.Fatalf("加载中的玩家应能被找到: %v", onlineErr)
	}
	var _, playingErr = admin.lookupPlaying("Loader")
	if !errors.Is(playingErr, command.ErrPlayerNotFound) {
		test.Fatalf("加载中的玩家不该被当作可传送 / 可私聊: %v", playingErr)
	}
	var names []string = admin.OnlinePlayers()
	if len(names) != 1 || names[0] != "Steve" {
		test.Fatalf("在线列表只应包含已进入游戏的玩家: %v", names)
	}
}

func TestMaxPlayersFollowsConfig(test *testing.T) {
	var fixture *adminFixture = newAdminFixture(test)
	fixture.srv.cfg.MaxPlayers = 12
	if fixture.admin().MaxPlayers() != 12 {
		test.Fatal("应返回配置里的人数上限")
	}
}
