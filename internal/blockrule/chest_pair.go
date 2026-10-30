package blockrule

// ChestPairCandidates 一个箱子可以和哪两格的箱子配成大箱子，按参考端的查找顺序排。
// 箱子正面朝东西（meta 4、5）就并排在南北两侧，正面朝南北（meta 2、3）就并排在东西两侧：
// 只有并排的才配，正对着或背对着的不配。meta 不是这四个值返回空。
// 这里只给位置，那一格是不是同朝向、还没配对的箱子由调用方查。
func ChestPairCandidates(pos Pos, meta byte) []Pos {
	switch meta {
	case 4, 5:
		return []Pos{pos.Add(0, 0, -1), pos.Add(0, 0, 1)}
	case 2, 3:
		return []Pos{pos.Add(-1, 0, 0), pos.Add(1, 0, 0)}
	}
	return nil
}
