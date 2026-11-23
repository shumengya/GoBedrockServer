package blockrule

// 液体方块的 meta：低 3 位是水位，0 是源头，1~7 离源头越远越浅；bit 3 表示这是一格往下掉的水柱。
const liquidFalling int = 8

// 水和岩浆各隔多少 tick 评估一次。
const waterTickRate int = 5
const lavaTickRate int = 30

// 流向代价搜索里一格的状态：被挡住、能往旁边流、脚下有空位能往下掉。
const flowStatusBlocked int = -1
const flowStatusSide int = 0
const flowStatusDown int = 1

// flowCostNone 一个方向找不到出路时的代价。
const flowCostNone int = 1000

// flowFaces 参考端把水平四个方向按 西、东、北、南 编成 0~3，异或 1 就是反方向。
var flowFaces [4]int32 = [4]int32{FaceWest, FaceEast, FaceNorth, FaceSouth}

func isLava(id byte) bool {
	return id == BlockFlowingLava || id == BlockStillLava
}

// sameLiquid 是不是同一种液体。静水和流动的水在存档里分成两个 ID，但对流动来说是同一种东西，
// 参考端在算水位时只认完全相同的 ID，水域边上混着两种 ID 就会互相不认，这里按水、岩浆两大类算。
func sameLiquid(left byte, right byte) bool {
	return (isWater(left) && isWater(right)) || (isLava(left) && isLava(right))
}

// decayPerBlock 水每流一格水位加 1，岩浆加 2。参考端注释说下界里岩浆也该是 1，但它没有区分维度。
func decayPerBlock(id byte) int {
	if isLava(id) {
		return 2
	}
	return 1
}

func liquidTickRate(id byte) int {
	if isLava(id) {
		return lavaTickRate
	}
	return waterTickRate
}

// liquidReader 包一层 View，把"世界之外和没加载的区块"统一当成墙：
// 液体只在已经加载的区块里流，不能因为流动把相邻的空区块生成出来。
type liquidReader struct {
	view View
}

func (reader liquidReader) reachable(spot Pos) bool {
	return spot.PosY >= 0 && spot.PosY <= 255 && reader.view.Loaded(spot)
}

// flowable 对应参考端的 canBeFlowedInto：空气、液体、花草火把这类不实心的方块。
func (reader liquidReader) flowable(spot Pos) bool {
	if !reader.reachable(spot) {
		return false
	}
	var id, _ = reader.view.Block(spot)
	return CanFlowInto(id)
}

// canFlowInto 对应参考端 Liquid 的 canFlowInto：在 flowable 之上，源头液体不能被流进去。
func (reader liquidReader) canFlowInto(spot Pos) bool {
	if !reader.flowable(spot) {
		return false
	}
	var id, meta = reader.view.Block(spot)
	return !(isLiquid(id) && meta == 0)
}

func (reader liquidReader) flowStatus(spot Pos) int {
	if !reader.canFlowInto(spot) {
		return flowStatusBlocked
	}
	if reader.flowable(spot.Add(0, -1, 0)) {
		return flowStatusDown
	}
	return flowStatusSide
}

// flowCost 从 spot 出发，往旁边最多再走到 maxCost 格，找最近的一个"脚下有空位"的格子，
// 返回要走几步；找不到返回 flowCostNone。originOpposite 是起始方向的反方向，lastOpposite 是上一步的反方向，
// 这两个方向不回头。
func (reader liquidReader) flowCost(
	visited map[Pos]int,
	spot Pos,
	accumulated int,
	maxCost int,
	originOpposite int,
	lastOpposite int,
) int {
	var cost int = flowCostNone
	var dir int = 0
	for dir = 0; dir < 4; dir++ {
		if dir == originOpposite || dir == lastOpposite {
			continue
		}
		var next, _ = spot.Side(flowFaces[dir])
		var status, seen = visited[next]
		if !seen {
			status = reader.flowStatus(next)
			visited[next] = status
		}
		if status == flowStatusBlocked {
			continue
		}
		if status == flowStatusDown {
			return accumulated
		}
		if accumulated >= maxCost {
			continue
		}
		var real int = reader.flowCost(visited, next, accumulated+1, maxCost, originOpposite, dir^1)
		if real < cost {
			cost = real
		}
	}
	return cost
}

// optimalDirections 四个水平方向里哪些是"最该流过去"的：脚下有空位的方向代价是 0，
// 其余按到最近落差要走几步算；代价最小的方向都算。四个方向都没出路时代价全是 flowCostNone，
// 于是四个方向都算，水会往四周铺开。
func (reader liquidReader) optimalDirections(origin Pos, perBlock int) [4]bool {
	var costs [4]int = [4]int{flowCostNone, flowCostNone, flowCostNone, flowCostNone}
	var maxCost int = 4 / perBlock
	var visited map[Pos]int = map[Pos]int{}
	var dir int = 0
	for dir = 0; dir < 4; dir++ {
		var next, _ = origin.Side(flowFaces[dir])
		if !reader.canFlowInto(next) {
			visited[next] = flowStatusBlocked
			continue
		}
		if reader.flowable(next.Add(0, -1, 0)) {
			visited[next] = flowStatusDown
			costs[dir] = 0
			maxCost = 0
			continue
		}
		if maxCost > 0 {
			visited[next] = flowStatusSide
			costs[dir] = reader.flowCost(visited, next, 1, maxCost, dir^1, dir^1)
			maxCost = min(maxCost, costs[dir])
		}
	}
	var minCost int = min(costs[0], costs[1], costs[2], costs[3])
	var flags [4]bool
	for dir = 0; dir < 4; dir++ {
		flags[dir] = costs[dir] == minCost
	}
	return flags
}

// targetDecay 一格流动的液体按四周重新算它该有的水位：取水平邻居里最小的水位加一格；
// 超过 7 或者根本没有邻居就是 -1（该干了）；正上方有同种液体就变成往下掉的水柱；
// 水旁边有两个源头、脚下是实心方块或水源，就地变成新的源头。
func (reader liquidReader) targetDecay(spot Pos, id byte, perBlock int) int {
	var smallest int = -100
	var sources int = 0
	var dir int = 0
	for dir = 0; dir < 4; dir++ {
		var side, _ = spot.Side(flowFaces[dir])
		var sideID, sideMeta = reader.view.Block(side)
		if !sameLiquid(sideID, id) {
			continue
		}
		var sideDecay int = int(sideMeta)
		if sideDecay == 0 {
			sources++
		}
		if sideDecay >= liquidFalling {
			sideDecay = 0
		}
		if smallest >= 0 && sideDecay >= smallest {
			continue
		}
		smallest = sideDecay
	}
	var newDecay int = smallest + perBlock
	if newDecay >= 8 || smallest < 0 {
		newDecay = -1
	}
	var aboveID, aboveMeta = reader.view.Block(spot.Add(0, 1, 0))
	if sameLiquid(aboveID, id) {
		newDecay = int(aboveMeta) | liquidFalling
	}
	if sources >= 2 && isWater(id) {
		var belowID, belowMeta = reader.view.Block(spot.Add(0, -1, 0))
		if IsSolid(belowID) || (isWater(belowID) && belowMeta == 0) {
			newDecay = 0
		}
	}
	return newDecay
}

// flowInto 液体流进 target 这一格。能被流进的是空气和花草火把这类不实心的方块，
// 被流进的方块会被冲掉并按空手结算掉落（Drop）；不会流进别的液体。
// 岩浆流到水上，水那一格变成石头（参考端 Lava 的 flowIntoBlock）。
func (reader liquidReader) flowInto(changes []Change, target Pos, id byte, decay byte) []Change {
	if !reader.reachable(target) {
		return changes
	}
	var targetID, _ = reader.view.Block(target)
	if isLava(id) && isWater(targetID) {
		return append(changes, Change{Pos: target, ID: BlockStone, Meta: 0})
	}
	if !reader.canFlowInto(target) || isLiquid(targetID) {
		return changes
	}
	return append(changes, Change{Pos: target, ID: id, Meta: decay, Drop: targetID != BlockAir})
}

// flows 这一格液体往外流：先往下，脚下流不动（或者本身是源头）再往最该去的水平方向铺开。
// 往外铺的水位是自己加一格（水）或加两格（岩浆），掉落中的水柱落地后从 1 开始铺。
func (reader liquidReader) flows(spot Pos, id byte, decay int, perBlock int) []Change {
	var changes []Change = nil
	var bottom Pos = spot.Add(0, -1, 0)
	changes = reader.flowInto(changes, bottom, id, byte(decay|liquidFalling))
	if decay != 0 && reader.flowable(bottom) {
		return changes
	}
	var adjacent int = 1
	if decay < liquidFalling {
		adjacent = decay + perBlock
	}
	if adjacent >= 8 {
		return changes
	}
	var flags [4]bool = reader.optimalDirections(spot, perBlock)
	var dir int = 0
	for dir = 0; dir < 4; dir++ {
		if flags[dir] {
			var side, _ = spot.Side(flowFaces[dir])
			changes = reader.flowInto(changes, side, id, byte(adjacent))
		}
	}
	return changes
}

// lavaHardened 岩浆旁边（不含正下方）有水就硬化：源头变黑曜石，水位 1~4 的流动岩浆变圆石，
// 更浅的不变。只看遇到的第一块水。
func lavaHardened(view View, spot Pos, decay int) (Change, bool) {
	var face int32 = 0
	for face = FaceUp; face <= FaceEast; face++ {
		var side, _ = spot.Side(face)
		var sideID, _ = view.Block(side)
		if !isWater(sideID) {
			continue
		}
		if decay == 0 {
			return Change{Pos: spot, ID: BlockObsidian, Meta: 0}, true
		}
		if decay <= 4 {
			return Change{Pos: spot, ID: BlockCobblestone, Meta: 0}, true
		}
		return Change{}, false
	}
	return Change{}, false
}

// liquidScheduled 排队的时间到了，评估一格液体：先重算自己的水位，再往下流、往旁边流，
// 最后岩浆看要不要硬化。所有读都来自评估开始时的快照，改动统一返回，由调用方落地。
func liquidScheduled(view View, spot Pos, id byte, meta byte) Update {
	var reader liquidReader = liquidReader{view: view}
	var perBlock int = decayPerBlock(id)
	var decay int = int(meta)
	var own []Change = nil
	var update Update = Update{}
	if decay > 0 {
		var newDecay int = reader.targetDecay(spot, id, perBlock)
		if newDecay != decay {
			decay = newDecay
			own = []Change{{Pos: spot, ID: BlockAir, Meta: 0}}
			if decay >= 0 {
				own = []Change{{Pos: spot, ID: id, Meta: byte(decay)}}
				update.Schedules = append(update.Schedules, Schedule{Pos: spot, Delay: liquidTickRate(id)})
			}
		}
	}
	if decay < 0 {
		update.Changes = own
		return update
	}
	update.Changes = reader.flows(spot, id, decay, perBlock)
	if isLava(id) {
		var hardened, hardens = lavaHardened(view, spot, decay)
		if hardens {
			own = []Change{hardened}
		}
	}
	update.Changes = append(own, update.Changes...)
	return update
}

// liquidNeighborUpdate 液体旁边有方块变了：岩浆先看要不要硬化，不硬化就排一次定时评估。
func liquidNeighborUpdate(view View, spot Pos) Update {
	var id, meta = view.Block(spot)
	if !isLiquid(id) {
		return Update{}
	}
	if isLava(id) {
		var hardened, hardens = lavaHardened(view, spot, int(meta))
		if hardens {
			return Update{Changes: []Change{hardened}}
		}
	}
	return Update{Schedules: []Schedule{{Pos: spot, Delay: liquidTickRate(id)}}}
}

// OnNeighborUpdate 是 OnNeighborChange 的完整版：除了"要不要掉下来"，还会给液体排定时评估。
// 服务端的级联引擎用它。
func OnNeighborUpdate(view View, spot Pos) Update {
	var changes []Change = OnNeighborChange(view, spot)
	if len(changes) > 0 {
		return Update{Changes: changes}
	}
	if !view.Loaded(spot) {
		return Update{}
	}
	if IDAt(view, spot) == BlockFire {
		return fireWake(view, spot, fireDelay+10)
	}
	return liquidNeighborUpdate(view, spot)
}
