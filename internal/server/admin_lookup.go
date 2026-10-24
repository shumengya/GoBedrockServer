package server

import "fmt"
import "strings"

import "go-mc-server/internal/command"
import "go-mc-server/internal/network"

// serverAdmin 是 command.Admin 的服务端实现。单独一个类型，免得和 Server 上已有的同名方法撞车。
type serverAdmin struct {
	srv *Server
}

// lookupOnline 按名字找在线玩家：先要完整名字（忽略大小写），找不到再认唯一的前缀。
// 前缀不唯一返回 ErrAmbiguousName，一个都没有返回 ErrPlayerNotFound。
func (admin serverAdmin) lookupOnline(name string) (*Player, error) {
	var wanted string = strings.ToLower(strings.TrimSpace(name))
	if wanted == "" {
		return nil, fmt.Errorf("名字为空: %w", command.ErrPlayerNotFound)
	}
	var list []*Player = admin.srv.playerList()
	var prefixed []*Player = nil
	var index int = 0
	for index = 0; index < len(list); index++ {
		var candidate string = strings.ToLower(list[index].Username)
		if candidate == wanted {
			return list[index], nil
		}
		if strings.HasPrefix(candidate, wanted) {
			prefixed = append(prefixed, list[index])
		}
	}
	if len(prefixed) == 1 {
		return prefixed[0], nil
	}
	if len(prefixed) > 1 {
		return nil, fmt.Errorf("%s: %w", name, command.ErrAmbiguousName)
	}
	return nil, fmt.Errorf("%s: %w", name, command.ErrPlayerNotFound)
}

// lookupPlaying 要求玩家已经进入游戏。还在加载区块的玩家没有位置可传、也收不了私聊。
func (admin serverAdmin) lookupPlaying(name string) (*Player, error) {
	var player, err = admin.lookupOnline(name)
	if err != nil {
		return nil, err
	}
	if player.Session == nil || player.Session.State() != network.StatePlaying {
		return nil, fmt.Errorf("%s 还在加载: %w", player.Username, command.ErrPlayerNotFound)
	}
	return player, nil
}

func (admin serverAdmin) OnlinePlayers() []string {
	var list []*Player = admin.srv.playerList()
	var names []string = make([]string, 0, len(list))
	var index int = 0
	for index = 0; index < len(list); index++ {
		if list[index].Session != nil && list[index].Session.State() == network.StatePlaying {
			names = append(names, list[index].Username)
		}
	}
	return names
}

func (admin serverAdmin) MaxPlayers() int {
	if admin.srv.cfg == nil {
		return 0
	}
	return admin.srv.cfg.MaxPlayers
}
