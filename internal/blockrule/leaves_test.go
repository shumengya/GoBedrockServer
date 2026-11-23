package blockrule_test

import (
	"testing"

	"go-mc-server/internal/blockrule"
)

// 两组树叶只寻找对应的原木，最远三步；未知的区块不能作为“没有原木”的证据。
func TestLeafDecaySupport(test *testing.T) {
	var spot = abovePos
	for _, sample := range []struct {
		name                  string
		leaf, log             byte
		distance              int32
		below, unknown, decay bool
	}{
		{"三步原木", 18, 17, 3, false, false, false},
		{"四步太远", 18, 17, 4, false, false, true},
		{"两步叶下的原木", 161, 162, 2, true, false, false},
		{"不同木组", 161, 17, 1, false, false, true},
		{"边缘待查", 18, 17, 2, false, true, false},
	} {
		test.Run(sample.name, func(sub *testing.T) {
			var view = newMapView().set(spot, sample.leaf, 9)
			for index := int32(1); index < sample.distance; index++ {
				view.set(spot.Add(index, 0, 0), sample.leaf, 0)
			}
			var end = spot.Add(sample.distance, 0, 0)
			if sample.below {
				view.set(end, sample.leaf, 0)
				end = end.Add(0, -1, 0)
			}
			view.set(end, sample.log, 0)
			if sample.unknown {
				view.unload(end)
			}
			var changes = blockrule.OnRandomTick(view, spot, nil).Changes
			if sample.unknown {
				if len(changes) != 0 {
					sub.Fatalf("期望保留待检查位，实际 %+v", changes)
				}
			} else if sample.decay {
				if len(changes) != 1 || !changes[0].Drop || changes[0].ID != 0 {
					sub.Fatalf("期望腐烂并结算掉落，实际 %+v", changes)
				}
			} else if len(changes) != 1 || changes[0].ID != sample.leaf || changes[0].Meta != 1 || !changes[0].Quiet {
				sub.Fatalf("期望仅清除检查位，实际 %+v", changes)
			}
		})
	}
}

// 玩家放置的树叶即使没有原木也保留树种且不参与腐烂。
func TestPlacedLeavesPersist(test *testing.T) {
	var view = groundView()
	var changes, placed = blockrule.Place(view, request(18, 2, groundPos, blockrule.FaceUp, 0.5, 0))
	if !placed || len(changes) != 1 || changes[0].Meta != 6 {
		test.Fatalf("期望持久白桦叶 meta 6，实际 %+v", changes)
	}
	view.set(abovePos, 18, 6)
	if len(blockrule.OnNeighborChange(view, abovePos)) != 0 || len(blockrule.OnRandomTick(view, abovePos, nil).Changes) != 0 {
		test.Fatal("期望手放的叶子保持不变")
	}
}
