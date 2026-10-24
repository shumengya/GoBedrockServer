package blockrule

import "math"

const itemSign int32 = 323

// isSign 立牌和贴墙牌两种方块。
func isSign(id byte) bool {
	return id == BlockSignPost || id == BlockWallSign
}

// signRotation 立牌朝向，0~15 共 16 个方向，公式照参考端 SignPost::place：
// 玩家偏航角加 180 度之后按 22.5 度一格四舍五入，负角度也要折回 0~15。
func signRotation(yaw float32) byte {
	var steps float64 = math.Floor((float64(yaw)+180)*16/360 + 0.5)
	return byte(int(steps) & 15)
}

// placeSign 放告示牌：点顶面放立牌，朝向跟着玩家；点侧面放贴墙牌，meta 就是被点击的面；点底面放不了。
// 靠着的那一格（立牌是脚下，贴墙牌是背后）是空气就放不了，这样放下去的牌不会下一刻就被邻居更新掉下来。
// 点到高草这类被顶替的方块时，背后那一格是被顶替方块背后的方块，不是被点的那一个。
func placeSign(view View, req PlaceRequest, spot Pos) ([]Change, bool) {
	if req.Face == FaceDown {
		return nil, false
	}
	var support, _ = spot.Side(req.Face ^ 1)
	if IDAt(view, support) == BlockAir {
		return nil, false
	}
	if req.Face == FaceUp {
		return []Change{{Pos: spot, ID: BlockSignPost, Meta: signRotation(req.Yaw)}}, true
	}
	return []Change{{Pos: spot, ID: BlockWallSign, Meta: byte(req.Face)}}, true
}

// signPostBroken 立牌脚下是空气就掉下来。只有空气才算，水、火把、花都撑得住，和参考端一致。
func signPostBroken(view View, pos Pos) bool {
	var belowID, loaded = neighborBlock(view, pos, FaceDown)
	return loaded && belowID == BlockAir
}

// wallSignBroken 贴墙牌背后是空气就掉下来。meta 是被点击的面，背后在它的对面；
// meta 不在 2~5 说明数据不对，不动它。
func wallSignBroken(view View, pos Pos, meta byte) bool {
	if meta < 2 || meta > 5 {
		return false
	}
	var backID, loaded = neighborBlock(view, pos, int32(meta)^1)
	return loaded && backID == BlockAir
}
