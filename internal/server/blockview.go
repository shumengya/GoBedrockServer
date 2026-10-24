package server

import "go-mc-server/internal/blockrule"
import "go-mc-server/internal/world"

// loadingView 是玩家主动放置时给规则用的世界视图：读到没加载的区块会把它加载出来。
// 玩家点的方块在他视野里，放在视野边缘时落点可能落在还没加载的区块上，这时加载是对的。
// 邻居更新、随机刻这类没人主动触发的逻辑不能用它，要用不会加载区块的视图，
// 否则一次更新会把相邻的一大片区块都生成出来。
type loadingView struct {
	target *world.World
}

func (view loadingView) Block(pos blockrule.Pos) (byte, byte) {
	return view.target.GetBlock(pos.PosX, pos.PosY, pos.PosZ)
}

func (view loadingView) Loaded(pos blockrule.Pos) bool {
	return true
}

// peekView 是邻居更新、随机刻用的视图：只读内存里已有的区块，不会触发生成或读盘。
type peekView struct {
	target *world.World
}

func (view peekView) Block(pos blockrule.Pos) (byte, byte) {
	var id, meta, _ = view.target.Peek(pos.PosX, pos.PosY, pos.PosZ)
	return id, meta
}

func (view peekView) Loaded(pos blockrule.Pos) bool {
	var _, _, loaded = view.target.Peek(pos.PosX, pos.PosY, pos.PosZ)
	return loaded
}
