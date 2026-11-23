package world

import (
	"math/rand/v2"

	"go-mc-server/internal/blockrule"
)

// 树形规则只读未装饰地形。这里的邻区是本次计算用的列，不进入世界缓存，不会触发递归生成。
type treeTerrain struct {
	seed, centerX, centerZ int32
	columns                map[[2]int32]*Chunk
}

func (view *treeTerrain) column(east, south int32) *Chunk {
	var key = [2]int32{east, south}
	var column = view.columns[key]
	if column == nil {
		column = newChunk()
		column.fillNormal(view.seed, view.centerX+east, view.centerZ+south)
		view.columns[key] = column
	}
	return column
}

func (view *treeTerrain) Loaded(pos blockrule.Pos) bool { return pos.PosY >= 0 && pos.PosY < 256 }

func (view *treeTerrain) Block(pos blockrule.Pos) (byte, byte) {
	return view.column(pos.PosX>>4, pos.PosZ>>4).GetBlock(int(pos.PosX&15), int(pos.PosY), int(pos.PosZ&15))
}

type biomePlants struct {
	species               byte
	trees, grass, flowers int
}

var plantCounts = [28]biomePlants{
	0: {grass: 5}, 1: {grass: 12, flowers: 2}, 3: {trees: 1, grass: 1}, 4: {trees: 5, grass: 3},
	5: {species: 1, trees: 10, grass: 1}, 6: {flowers: 8}, 7: {grass: 5}, 12: {grass: 5},
	20: {trees: 1, grass: 1}, 27: {species: 2, trees: 5, grass: 3},
}

var plainsFlowers = [...][2]byte{{37, 0}, {38, 0}, {38, 3}, {38, 4}, {38, 5}, {38, 6}, {38, 7}, {38, 8}}

func (column *Chunk) fillVegetation(seed, chunkX, chunkZ int32) {
	var _, temp, rain = terrainNoise(seed)
	var view = treeTerrain{seed, chunkX, chunkZ, map[[2]int32]*Chunk{{0, 0}: column}}
	var changes = []blockrule.Change(nil)
	var east, south int32 = 0, 0
	for east = -1; east <= 1; east++ {
		for south = -1; south <= 1; south++ {
			var sourceX, sourceZ = chunkX + east, chunkZ + south
			var centerX, centerZ = float64(sourceX)*16 + 7, float64(sourceZ)*16 + 7
			var plants = plantCounts[selectClimate(temp.climate(centerX, centerZ), rain.climate(centerX, centerZ)).id]
			if plants.trees == 0 {
				continue
			}
			var random = rand.New(rand.NewPCG(uint64(uint32(seed))<<1|1, chunkKey(sourceX, sourceZ)))
			var attempts = plants.trees + random.IntN(2)
			var index = 0
			for index = 0; index < attempts; index++ {
				var col, row = random.IntN(16), random.IntN(16)
				// 单棵树的随机流独立；跳过伸不到目标列的树不能改变同一源区块里其他树的形状。
				var shapeSeed = random.Uint64()
				var posX, posZ = east*16 + int32(col), south*16 + int32(row)
				var reach int32 = 8
				if plants.species != 0 {
					reach = 3
				}
				if posX < -reach || posX > 15+reach || posZ < -reach || posZ > 15+reach {
					continue
				}
				var base = view.column(east, south)
				var level = int(base.heightMap[row*16+col])
				var floor, _ = base.GetBlock(col, level-1, row)
				if floor == blockrule.BlockSnowLayer {
					level--
				}
				var root = blockrule.Pos{PosX: posX, PosY: int32(level), PosZ: posZ}
				var tree = blockrule.GrowTree(&view, root, plants.species, rand.New(rand.NewPCG(shapeSeed, 0)))
				var edit = 0
				for edit = 0; edit < len(tree.Changes); edit++ {
					var change = tree.Changes[edit]
					if change.Pos.PosX>>4 == 0 && change.Pos.PosZ>>4 == 0 {
						changes = append(changes, change)
					}
				}
			}
		}
	}
	// 先收集再落地，每个区块都看到同样的树根地形；相交的树冠不能吞掉树干。
	var edit = 0
	for edit = 0; edit < len(changes); edit++ {
		var change = changes[edit]
		var col, level, row = int(change.Pos.PosX), int(change.Pos.PosY), int(change.Pos.PosZ)
		var old, _ = column.GetBlock(col, level, row)
		if old == BlockLog && change.ID == blockrule.BlockLeaves {
			continue
		}
		column.setBlockLocked(col, level, row, change.ID, change.Meta)
	}
	var biome = column.biomes[7*16+7]
	var plants = plantCounts[biome]
	var random = rand.New(rand.NewPCG(uint64(uint32(seed))<<2|2, chunkKey(chunkX, chunkZ)))
	column.scatterPlants(random, biome, plants.grass, false)
	column.scatterPlants(random, biome, plants.flowers, true)
	column.dirty = false
}

func (column *Chunk) scatterPlants(random *rand.Rand, biome byte, count int, flowers bool) {
	if count == 0 {
		return
	}
	var attempts = count + random.IntN(2)
	var index = 0
	for index = 0; index < attempts; index++ {
		var col, row = random.IntN(16), random.IntN(16)
		var level = int(column.heightMap[row*16+col]) - 1
		var floor, _ = column.GetBlock(col, level, row)
		for floor == BlockAir || floor == blockrule.BlockLeaves || floor == blockrule.BlockSnowLayer {
			level--
			floor, _ = column.GetBlock(col, level, row)
		}
		if floor != BlockGrass {
			continue
		}
		level++
		var old, _ = column.GetBlock(col, level, row)
		if old != BlockAir && old != blockrule.BlockSnowLayer {
			continue
		}
		var id, meta = blockrule.BlockTallGrass, byte(1)
		if flowers {
			if biome == 6 {
				id, meta = blockrule.BlockPoppy, 1
			} else {
				var choice = plainsFlowers[random.IntN(len(plainsFlowers))]
				id, meta = choice[0], choice[1]
			}
		}
		column.setBlockLocked(col, level, row, id, meta)
	}
}
