package blockrule

// 树冠、树枝与树干可能重合。每格只发最后的方块；探到未加载区块时整棵树留待下次生长。
type growingTree struct {
	view                View
	root                Pos
	wood, leaf, variant byte
	changes             []Change
	index               map[Pos]int
	incomplete          bool
}

func (tree *growingTree) available(pos Pos) bool {
	if pos.PosY < 0 || pos.PosY > 255 || !tree.view.Loaded(pos) {
		tree.incomplete = true
		return false
	}
	switch IDAt(tree.view, pos) {
	case BlockAir, BlockSapling, BlockSnowLayer, BlockLeaves, BlockLeaves2:
		return true
	}
	return false
}

func (tree *growingTree) put(pos Pos, id byte) {
	if !tree.available(pos) {
		return
	}
	if id == tree.leaf && IsSolid(IDAt(tree.view, pos)) {
		return
	}
	var edit = Change{Pos: pos, ID: id, Meta: tree.variant, Quiet: true}
	var index, exists = tree.index[pos]
	if exists {
		if id == tree.wood {
			tree.changes[index] = edit
		}
		return
	}
	tree.index[pos] = len(tree.changes)
	tree.changes = append(tree.changes, edit)
}

// GrowTree 为树苗生长和世界生成计算同一棵树的方块，不修改视图。
func GrowTree(view View, root Pos, species byte, random Random) Update {
	if root.PosY < 1 || !supportedAt(view, BlockSapling, root) || species > 5 {
		return Update{}
	}
	var tree = growingTree{view: view, root: root, wood: BlockLog, leaf: BlockLeaves, variant: species, index: make(map[Pos]int)}
	if species >= 4 {
		tree.wood, tree.leaf, tree.variant = BlockWood2, BlockLeaves2, species-4
	}
	// PHP 先按构造时的树高检查净空，普通橡树、白桦、云杉随后才抽取实际高度。
	var clearance int32 = 8
	switch species {
	case 0, 2:
		clearance = 7
	case 1:
		clearance = 10
	}
	var radius, level, east, south int32 = 0, 0, 0, 0
	for level = 0; level < clearance+3; level++ {
		if level == 1 || level == clearance {
			radius++
		}
		for east = -radius; east <= radius; east++ {
			for south = -radius; south <= radius; south++ {
				if !tree.available(root.Add(east, level, south)) {
					return Update{}
				}
			}
		}
	}
	var soil = root.Add(0, -1, 0)
	tree.changes = []Change{{Pos: soil, ID: BlockDirt}}
	if species == 0 && random.IntN(10) == 0 {
		tree.bigOak(random)
	} else {
		tree.straightTree(species, random)
	}
	if tree.incomplete {
		return Update{}
	}
	return Update{Changes: tree.changes}
}

func (tree *growingTree) straightTree(species byte, random Random) {
	var height int32 = 8
	switch species {
	case 0:
		height = 4 + int32(random.IntN(3))
	case 1:
		height = 6 + int32(random.IntN(4))
	case 2:
		var tall = random.IntN(39) == 0
		height = 5 + int32(random.IntN(3))
		if tall {
			height += 5
		}
	}
	var trunk = height - 1
	if species == 1 {
		var depth = height - 1 - int32(random.IntN(2))
		var widest = 2 + int32(random.IntN(2))
		trunk = height - int32(random.IntN(3))
		var radius = int32(random.IntN(2))
		var maximum, reset int32 = 1, 0
		var layer int32 = 0
		for layer = 0; layer <= depth; layer++ {
			tree.leafSquare(height-layer, radius, nil)
			if radius >= maximum {
				radius, reset, maximum = reset, 1, min(widest, maximum+1)
			} else {
				radius++
			}
		}
	} else {
		var offset int32 = 0
		for offset = -3; offset <= 0; offset++ {
			var corners Random = random
			if offset == 0 {
				corners = nil
			}
			tree.leafSquare(height+offset, 1-offset/2, corners)
		}
	}
	var level int32 = 0
	for level = 0; level < trunk; level++ {
		var pos = tree.root.Add(0, level, 0)
		if !tree.available(pos) {
			tree.incomplete = true
			return
		}
		tree.put(pos, tree.wood)
	}
}

// 阔叶树下三层独立抽取四角；顶层和云杉剪去四角。半径零保留树尖。
func (tree *growingTree) leafSquare(level, radius int32, corners Random) {
	var east, south int32 = 0, 0
	for east = -radius; east <= radius; east++ {
		for south = -radius; south <= radius; south++ {
			if radius > 0 && (east == radius || east == -radius) && (south == radius || south == -radius) && (corners == nil || corners.IntN(2) == 0) {
				continue
			}
			tree.put(tree.root.Add(east, level, south), tree.leaf)
		}
	}
}
