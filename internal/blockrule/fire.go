package blockrule

const fireDelay int = 30
const blockNetherrack byte = 87
const ItemFlintSteel int32 = 259

// 六个邻居的最大助燃值决定空气能否着火。未知邻居不能点火，也不能据此判定旧火失去燃料。
func fireFuel(view View, spot Pos, burned map[Pos]bool) (int, bool) {
	var chance int = 0
	var unknown bool = false
	var face int32 = 0
	for face = FaceDown; face <= FaceEast; face++ {
		var side, _ = spot.Side(face)
		if side.PosY < 0 || side.PosY > 255 || burned[side] {
			continue
		}
		if !view.Loaded(side) {
			unknown = true
			continue
		}
		var id, meta = view.Block(side)
		chance = max(chance, BurnChance(id, meta))
	}
	return chance, unknown
}

func fireBase(view View, spot Pos) (byte, byte, bool) {
	var id, meta = view.Block(spot.Add(0, -1, 0))
	return id, meta, IsSolid(id) || (id == BlockSnowLayer && meta == 7)
}

func fireWake(view View, spot Pos, delay int) Update {
	var _, _, solid = fireBase(view, spot)
	if !solid {
		var fuel, unknown = fireFuel(view, spot, nil)
		if fuel == 0 && !unknown {
			return Update{Changes: []Change{{Pos: spot, ID: BlockAir}}}
		}
	}
	return Update{Schedules: []Schedule{{Pos: spot, Delay: delay}}}
}

func ignite(view View, req ItemUseRequest, target byte, random Random) (ItemUse, bool) {
	var spot, valid = req.Target.Side(req.Face)
	if !valid || !IsSolid(target) || spot.PosY < 0 || spot.PosY > 255 || !view.Loaded(spot) || IDAt(view, spot) != BlockAir {
		return ItemUse{}, false
	}
	var _, _, solid = fireBase(view, spot)
	if !solid {
		var fuel, _ = fireFuel(view, spot, nil)
		if fuel == 0 {
			return ItemUse{}, false
		}
	}
	return ItemUse{Wear: 1, Update: Update{Changes: []Change{{Pos: spot, ID: BlockFire}}, Schedules: []Schedule{{Pos: spot, Delay: fireDelay + random.IntN(11)}}}}, true
}

func fireScheduled(view View, spot Pos, age byte, random Random, difficulty int32, spread bool) Update {
	var baseID, baseMeta, solid = fireBase(view, spot)
	var fuel, unknown = fireFuel(view, spot, nil)
	if !solid && fuel == 0 && !unknown {
		return Update{Changes: []Change{{Pos: spot, ID: BlockAir}}}
	}
	if !spread {
		return Update{}
	}
	var nextAge = age
	if age < 15 && random.IntN(3) == 0 {
		nextAge++
	}
	var update = Update{Schedules: []Schedule{{Pos: spot, Delay: fireDelay + random.IntN(11)}}}
	if baseID != blockNetherrack {
		if fuel == 0 && !unknown && age > 3 || fuel > 0 && age == 15 && BurnAbility(baseID, baseMeta) == 0 && random.IntN(5) == 0 {
			return Update{Changes: []Change{{Pos: spot, ID: BlockAir}}}
		}
	}
	if age != nextAge {
		update.Changes = append(update.Changes, Change{Pos: spot, ID: BlockFire, Meta: nextAge, Quiet: true})
	}
	if baseID != blockNetherrack && fuel == 0 {
		return update
	}
	var burned = map[Pos]bool{}
	var faces = [...]int32{FaceEast, FaceWest, FaceDown, FaceUp, FaceSouth, FaceNorth}
	var index int = 0
	for index = 0; index < len(faces); index++ {
		var face = faces[index]
		var side, _ = spot.Side(face)
		if side.PosY < 0 || side.PosY > 255 || !view.Loaded(side) {
			continue
		}
		var id, meta = view.Block(side)
		var ability = BurnAbility(id, meta)
		var bound int = 300
		if face == FaceUp || face == FaceDown {
			bound = 250
		}
		if ability == 0 || random.IntN(bound+1) >= ability {
			continue
		}
		burned[side] = true
		var edit = Change{Pos: side, ID: BlockAir}
		if random.IntN(int(age)+11) < 5 {
			edit.ID, edit.Meta = BlockFire, min(byte(15), age+byte(random.IntN(5)/4))
			update.Schedules = append(update.Schedules, Schedule{Pos: side, Delay: fireDelay})
		}
		update.Changes = append(update.Changes, edit)
	}
	// 本次已烧掉的木头不能继续给周围空气提供助燃值。
	var east, south, rise int32 = 0, 0, 0
	for east = -1; east <= 1; east++ {
		for south = -1; south <= 1; south++ {
			for rise = -1; rise <= 4; rise++ {
				var air = spot.Add(east, rise, south)
				if air.PosY < 0 || air.PosY > 255 || !view.Loaded(air) || IDAt(view, air) != BlockAir {
					continue
				}
				var chance, _ = fireFuel(view, air, burned)
				var bound = 100 * max(1, int(rise))
				if chance > 0 && random.IntN(bound+1) <= chance+40+7*int(difficulty) {
					update.Changes = append(update.Changes, Change{Pos: air, ID: BlockFire, Meta: min(byte(15), age+byte(random.IntN(6)/4))})
					update.Schedules = append(update.Schedules, Schedule{Pos: air, Delay: fireDelay})
				}
			}
		}
	}
	return update
}
