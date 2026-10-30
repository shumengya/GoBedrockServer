package world_test

import "reflect"
import "testing"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/world"

// chestTestWorld 放好一组箱子，不配对。每条用例各自建，免得互相污染。
func chestTestWorld(positions ...[3]int32) *world.World {
	var target *world.World = world.New("chests")
	var index int = 0
	for index = 0; index < len(positions); index++ {
		target.SetTile(world.NewChestTile(positions[index][0], positions[index][1], positions[index][2]))
	}
	return target
}

type pairCase struct {
	Name   string
	Setup  func() *world.World
	Args   [5]int32
	Expect bool
}

// 只有水平相邻（曼哈顿距离正好 1）、都是箱子、都还没配对的两个箱子能配成大箱子；其余一律什么都不改。
func TestPairChests(test *testing.T) {
	var cases []pairCase = []pairCase{
		{
			Name:   "沿 Z 相邻",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}, [3]int32{0, 5, 1}) },
			Args:   [5]int32{0, 5, 0, 0, 1},
			Expect: true,
		},
		{
			Name:   "沿 X 相邻",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}, [3]int32{1, 5, 0}) },
			Args:   [5]int32{0, 5, 0, 1, 0},
			Expect: true,
		},
		{
			Name:   "隔了一格",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}, [3]int32{0, 5, 2}) },
			Args:   [5]int32{0, 5, 0, 0, 2},
			Expect: false,
		},
		{
			Name:   "斜对角",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}, [3]int32{1, 5, 1}) },
			Args:   [5]int32{0, 5, 0, 1, 1},
			Expect: false,
		},
		{
			Name:   "自己和自己",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}) },
			Args:   [5]int32{0, 5, 0, 0, 0},
			Expect: false,
		},
		{
			Name:   "另一个位置没有方块实体",
			Setup:  func() *world.World { return chestTestWorld([3]int32{0, 5, 0}) },
			Args:   [5]int32{0, 5, 0, 0, 1},
			Expect: false,
		},
		{
			Name: "另一个是熔炉",
			Setup: func() *world.World {
				var target *world.World = chestTestWorld([3]int32{0, 5, 0})
				target.SetTile(world.NewFurnaceTile(0, 5, 1))
				return target
			},
			Args:   [5]int32{0, 5, 0, 0, 1},
			Expect: false,
		},
		{
			Name: "其中一个已经配过对了",
			Setup: func() *world.World {
				var target *world.World = chestTestWorld([3]int32{0, 5, 0}, [3]int32{0, 5, 1}, [3]int32{0, 5, 2})
				target.PairChests(0, 5, 1, 0, 2)
				return target
			},
			Args:   [5]int32{0, 5, 0, 0, 1},
			Expect: false,
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item pairCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var target *world.World = item.Setup()
			var before []world.Tile = target.SnapshotTiles()
			var paired bool = target.PairChests(item.Args[0], item.Args[1], item.Args[2], item.Args[3], item.Args[4])
			if paired != item.Expect {
				sub.Fatalf("配对结果期望 %v，实际 %v", item.Expect, paired)
			}
			if item.Expect {
				var first, _ = target.TileAt(item.Args[0], item.Args[1], item.Args[2])
				var second, _ = target.TileAt(item.Args[3], item.Args[1], item.Args[4])
				if !first.Paired || first.PairX != item.Args[3] || first.PairZ != item.Args[4] {
					sub.Fatalf("第一个箱子应指向第二个，实际 %+v", first)
				}
				if !second.Paired || second.PairX != item.Args[0] || second.PairZ != item.Args[2] {
					sub.Fatalf("第二个箱子应指向第一个，实际 %+v", second)
				}
				return
			}
			var after []world.Tile = target.SnapshotTiles()
			if len(before) != len(after) {
				sub.Fatalf("没配成时方块实体数不该变，期望 %d，实际 %d", len(before), len(after))
			}
			var tileIndex int = 0
			for tileIndex = 0; tileIndex < len(before); tileIndex++ {
				var current, _ = target.TileAt(before[tileIndex].X, before[tileIndex].Y, before[tileIndex].Z)
				if current.Paired != before[tileIndex].Paired {
					sub.Fatalf("没配成时不该动配对状态，(%d,%d,%d) 期望 %v，实际 %v",
						current.X, current.Y, current.Z, before[tileIndex].Paired, current.Paired)
				}
			}
		})
	}
}

type partnerCase struct {
	Name   string
	Tiles  []world.Tile
	Expect bool
}

// 搭档必须两边互相指着对方才算：只有一边记着，就当没配对。
func TestChestPartnerRequiresMutualPointing(test *testing.T) {
	var cases []partnerCase = []partnerCase{
		{
			Name: "互相指着",
			Tiles: []world.Tile{
				{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
				{ID: world.TileChest, X: 0, Y: 5, Z: 1, Paired: true, PairX: 0, PairZ: 0},
			},
			Expect: true,
		},
		{
			Name: "另一半没记配对",
			Tiles: []world.Tile{
				{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
				{ID: world.TileChest, X: 0, Y: 5, Z: 1},
			},
			Expect: false,
		},
		{
			Name: "另一半指着别处",
			Tiles: []world.Tile{
				{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
				{ID: world.TileChest, X: 0, Y: 5, Z: 1, Paired: true, PairX: 0, PairZ: 2},
			},
			Expect: false,
		},
		{
			Name: "另一半不存在",
			Tiles: []world.Tile{
				{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
			},
			Expect: false,
		},
		{
			Name: "另一半是熔炉",
			Tiles: []world.Tile{
				{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
				{ID: world.TileFurnace, X: 0, Y: 5, Z: 1, Paired: true, PairX: 0, PairZ: 0},
			},
			Expect: false,
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item partnerCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var target *world.World = world.New("partner")
			target.LoadTiles(item.Tiles)
			var partner, found = target.ChestPartner(0, 5, 0)
			if found != item.Expect {
				sub.Fatalf("找到搭档期望 %v，实际 %v", item.Expect, found)
			}
			if found && (partner.X != 0 || partner.Z != 1) {
				sub.Fatalf("搭档应在 (0,5,1)，实际 (%d,%d,%d)", partner.X, partner.Y, partner.Z)
			}
		})
	}
}

type orderCase struct {
	Name  string
	Left  world.Tile
	Right world.Tile
	Front world.Tile
}

// 双箱两半在窗口里的顺序只取决于位置（照参考端 x + (z<<15) 大的在前），和传参顺序无关。
func TestOrderChestHalves(test *testing.T) {
	var cases []orderCase = []orderCase{
		{
			Name:  "沿 Z：z 大的在前",
			Left:  world.Tile{X: 0, Z: 0},
			Right: world.Tile{X: 0, Z: 1},
			Front: world.Tile{X: 0, Z: 1},
		},
		{
			Name:  "沿 X：x 大的在前",
			Left:  world.Tile{X: 4, Z: 2},
			Right: world.Tile{X: 5, Z: 2},
			Front: world.Tile{X: 5, Z: 2},
		},
		{
			Name:  "负坐标",
			Left:  world.Tile{X: -3, Z: -1},
			Right: world.Tile{X: -3, Z: -2},
			Front: world.Tile{X: -3, Z: -1},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item orderCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var frontA, backA = world.OrderChestHalves(item.Left, item.Right)
			var frontB, backB = world.OrderChestHalves(item.Right, item.Left)
			if frontA.X != item.Front.X || frontA.Z != item.Front.Z {
				sub.Fatalf("前面那半期望 (%d,%d)，实际 (%d,%d)", item.Front.X, item.Front.Z, frontA.X, frontA.Z)
			}
			if frontB.X != frontA.X || frontB.Z != frontA.Z || backB.X != backA.X || backB.Z != backA.Z {
				sub.Fatalf("传参顺序不该影响结果，正传 前(%d,%d) 后(%d,%d)，反传 前(%d,%d) 后(%d,%d)",
					frontA.X, frontA.Z, backA.X, backA.Z, frontB.X, frontB.Z, backB.X, backB.Z)
			}
		})
	}
}

// 拆掉大箱子的一半，剩下那半自动变回单箱；返回的是删之前的样子，配对信息还在。
func TestRemoveTileUnpairsPartner(test *testing.T) {
	var target *world.World = chestTestWorld([3]int32{0, 5, 0}, [3]int32{0, 5, 1})
	if !target.PairChests(0, 5, 0, 0, 1) {
		test.Fatal("夹具没配成对")
	}
	var removed, found = target.RemoveTile(0, 5, 0)
	if !found || !removed.Paired || removed.PairZ != 1 {
		test.Fatalf("返回的应是删之前带配对信息的样子，实际 找到=%v %+v", found, removed)
	}
	var remaining, remainingFound = target.TileAt(0, 5, 1)
	if !remainingFound {
		test.Fatal("另一半不该被一起删掉")
	}
	if remaining.Paired || remaining.PairX != 0 || remaining.PairZ != 0 {
		test.Fatalf("剩下那半应解除配对，实际 %+v", remaining)
	}
}

// 解除配对只动互相指着的两个：另一半早就指着别处时，它自己的状态不能被误清。
func TestUnpairChest(test *testing.T) {
	var target *world.World = world.New("unpair")
	target.LoadTiles([]world.Tile{
		{ID: world.TileChest, X: 0, Y: 5, Z: 0, Paired: true, PairX: 0, PairZ: 1},
		{ID: world.TileChest, X: 0, Y: 5, Z: 1, Paired: true, PairX: 0, PairZ: 2},
		{ID: world.TileChest, X: 0, Y: 5, Z: 2, Paired: true, PairX: 0, PairZ: 1},
		{ID: world.TileChest, X: 4, Y: 5, Z: 4},
	})
	if !target.UnpairChest(0, 5, 0) {
		test.Fatal("记着配对的箱子应能解除")
	}
	var first, _ = target.TileAt(0, 5, 0)
	var second, _ = target.TileAt(0, 5, 1)
	if first.Paired {
		test.Fatalf("自己应解除，实际 %+v", first)
	}
	if !second.Paired || second.PairZ != 2 {
		test.Fatalf("指着别处的那个不该被动，实际 %+v", second)
	}
	if target.UnpairChest(4, 5, 4) {
		test.Fatal("没配过对的箱子解除应返回 false")
	}
	if target.UnpairChest(9, 5, 9) {
		test.Fatal("不存在的箱子解除应返回 false")
	}
}

// 配对信息要跟着方块实体走完整个存取：快照出来再装回去，配对不丢。
func TestPairSurvivesSnapshotAndLoad(test *testing.T) {
	var source *world.World = chestTestWorld([3]int32{2, 5, 2}, [3]int32{3, 5, 2})
	source.PairChests(2, 5, 2, 3, 2)
	source.SetTileSlot(2, 5, 2, 0, protocol.Item{ID: 1, Count: 3})
	var restored *world.World = world.New("restored")
	restored.LoadTiles(source.SnapshotTiles())
	var partner, found = restored.ChestPartner(2, 5, 2)
	if !found || partner.X != 3 {
		test.Fatalf("装回去后搭档应还在 (3,5,2)，实际 找到=%v (%d)", found, partner.X)
	}
	var first, _ = restored.TileAt(2, 5, 2)
	if !reflect.DeepEqual(first.Items[0], protocol.Item{ID: 1, Count: 3}) {
		test.Fatalf("物品应保留，实际 %+v", first.Items[0])
	}
}
