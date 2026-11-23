package blockrule

import "math"

// 大橡树沿主干上半段抽取叶簇；每簇五层，较远的枝从主干较低处斜向连入。
func (tree *growingTree) bigOak(random Random) {
	var level int32 = 0
	var height = 5 + int32(random.IntN(12))
	for level = 0; level < height; level++ {
		if !tree.available(tree.root.Add(0, level, 0)) {
			if level <= 5 {
				tree.incomplete = true
				return
			}
			height = level
			break
		}
	}
	var trunk = int32(float64(height) * 0.618)
	type cluster struct {
		center Pos
		start  int32
	}
	var groups = []cluster{{center: tree.root.Add(0, height-5, 0), start: trunk}}
	var half = float64(height) / 2
	var perLayer = int(1.382 + math.Pow(float64(height)/13, 2))
	for level = height - 5; float64(level) >= float64(height)/3; level-- {
		var width = math.Sqrt(half*half-math.Pow(float64(level)-half, 2)) / 2
		if float64(level) == half {
			width = half / 4
		}
		var count int = 0
		for count = 0; count < perLayer; count++ {
			var reach = width * (float64(random.IntN(1<<24))/(1<<24) + 0.328)
			var angle = 2 * math.Pi * float64(random.IntN(1<<24)) / (1 << 24)
			var east, south = int32(math.Round(reach * math.Cos(angle))), int32(math.Round(reach * math.Sin(angle)))
			var center = tree.root.Add(east, level, south)
			var clear = true
			var offset int32 = 0
			for offset = 0; offset <= 5; offset++ {
				if !tree.available(center.Add(0, offset, 0)) {
					clear = false
					break
				}
			}
			var start = min(trunk, int32(float64(level)-math.Hypot(float64(east), float64(south))*0.381))
			if clear && tree.oakBranch(tree.root.Add(0, start, 0), center, false) {
				groups = append(groups, cluster{center, start})
			}
		}
	}
	var index int = 0
	for index = 0; index < len(groups); index++ {
		var group = groups[index]
		for level = 0; level < 5; level++ {
			var radius int32 = 3
			if level == 4 {
				radius = 2
			}
			var east, south int32 = 0, 0
			for east = -radius; east <= radius; east++ {
				for south = -radius; south <= radius; south++ {
					var offsetX, offsetZ = math.Abs(float64(east)) + 0.5, math.Abs(float64(south)) + 0.5
					if offsetX*offsetX+offsetZ*offsetZ <= float64(radius*radius) {
						tree.put(group.center.Add(east, level, south), tree.leaf)
					}
				}
			}
		}
		if group.start*5 >= height {
			tree.oakBranch(tree.root.Add(0, group.start, 0), group.center, true)
		}
	}
	tree.oakBranch(tree.root, tree.root.Add(0, trunk, 0), true)
}

// 沿方块中心之间的线段逐面走，斜枝不只在棱角处相接。检查净空和落木走同一条路径。
func (tree *growingTree) oakBranch(start, end Pos, write bool) bool {
	var pos = start
	var delta = [3]int32{end.PosX - start.PosX, end.PosY - start.PosY, end.PosZ - start.PosZ}
	var crossed = [3]int32{}
	for {
		if !tree.available(pos) {
			return false
		}
		if write {
			tree.put(pos, tree.wood)
		}
		if pos == end {
			return true
		}
		var axis int = -1
		var nearest = math.Inf(1)
		var index int = 0
		for index = 0; index < 3; index++ {
			var distance = math.Abs(float64(delta[index]))
			if float64(crossed[index]) < distance {
				var next = (float64(crossed[index]) + 0.5) / distance
				if next < nearest {
					axis, nearest = index, next
				}
			}
		}
		crossed[axis]++
		var step int32 = 1
		if delta[axis] < 0 {
			step = -1
		}
		switch axis {
		case 0:
			pos.PosX += step
		case 1:
			pos.PosY += step
		case 2:
			pos.PosZ += step
		}
	}
}
