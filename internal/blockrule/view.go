package blockrule

// Pos 是方块坐标。
type Pos struct {
	PosX int32
	PosY int32
	PosZ int32
}

// Add 返回偏移后的坐标。
func (pos Pos) Add(deltaX int32, deltaY int32, deltaZ int32) Pos {
	return Pos{PosX: pos.PosX + deltaX, PosY: pos.PosY + deltaY, PosZ: pos.PosZ + deltaZ}
}

// Side 返回某个面上的邻居。face 不在 0~5 内时第二个返回值为 false。
func (pos Pos) Side(face int32) (Pos, bool) {
	switch face {
	case FaceDown:
		return pos.Add(0, -1, 0), true
	case FaceUp:
		return pos.Add(0, 1, 0), true
	case FaceNorth:
		return pos.Add(0, 0, -1), true
	case FaceSouth:
		return pos.Add(0, 0, 1), true
	case FaceWest:
		return pos.Add(-1, 0, 0), true
	case FaceEast:
		return pos.Add(1, 0, 0), true
	}
	return pos, false
}

// View 是规则读世界的唯一入口。
type View interface {
	// Block 返回方块 ID 和 meta。没加载的区块和超出高度的位置都按空气返回。
	Block(pos Pos) (byte, byte)
	// Loaded 报告这格所在的区块是否已在内存里。规则遇到没加载的邻居要保守处理，
	// 不能因为读到"空气"就把火把、作物这类方块当成没有支撑而打掉。
	Loaded(pos Pos) bool
}

// IDAt 只取方块 ID，判定用不上 meta 时用它。
func IDAt(view View, pos Pos) byte {
	var id, _ = view.Block(pos)
	return id
}
