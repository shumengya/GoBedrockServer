package storage

import "encoding/binary"
import "fmt"
import "math"
import "slices"
import "strconv"
import "unicode/utf8"
import "go-mc-server/internal/item"
import "go-mc-server/internal/world"

// 物品 tag 从磁盘大端转成背包里的固定宽度小端；保留子标签顺序，让原本相同的栈仍可合并。
func (tag pmTag) littlePayload(dst []byte) []byte {
	var index = 0
	switch tag.kind {
	case 8:
		dst = binary.LittleEndian.AppendUint16(dst, uint16(len(tag.raw)))
		return append(dst, tag.raw...)
	case 9:
		dst = binary.LittleEndian.AppendUint32(append(dst, tag.element), uint32(len(tag.list)))
		for index = 0; index < len(tag.list); index++ {
			dst = tag.list[index].littlePayload(dst)
		}
		return dst
	case 10:
		for index = 0; index < len(tag.names); index++ {
			var key = tag.names[index]
			var child = tag.fields[key]
			dst = binary.LittleEndian.AppendUint16(append(dst, child.kind), uint16(len(key)))
			dst = child.littlePayload(append(dst, key...))
		}
		return append(dst, 0)
	}
	var width = len(tag.raw)
	switch tag.kind {
	case 7:
		dst = binary.LittleEndian.AppendUint32(dst, uint32(len(tag.raw)))
		return append(dst, tag.raw...)
	case 11, 12:
		width = 4
		if tag.kind == 12 {
			width = 8
		}
		dst = binary.LittleEndian.AppendUint32(dst, uint32(len(tag.raw)/width))
	}
	for index = 0; index < len(tag.raw); index += width {
		var pos = 0
		for pos = width - 1; pos >= 0; pos-- {
			dst = append(dst, tag.raw[index+pos])
		}
	}
	return dst
}

func pmSlot(tag pmTag) (SlotData, error) {
	var result = SlotData{}
	var index = 0
	for index = 0; index < len(tag.names); index++ {
		switch tag.names[index] {
		case "Slot", "id", "Count", "Damage", "tag":
		default:
			return result, fmt.Errorf("无法还原物品字段 %s", tag.names[index])
		}
	}
	var id = tag.fields["id"]
	switch id.kind {
	case 2:
		result.ID = int32(binary.BigEndian.Uint16(id.raw))
	case 8:
		var stack, err = item.FromString(string(id.raw))
		if err != nil {
			return result, err
		}
		result.ID = stack.ID
	default:
		return result, fmt.Errorf("物品 id 需要 Short 或 String")
	}
	var count, err = tag.integer("Count", 1)
	if err != nil {
		return result, err
	}
	result.Count = byte(count)
	if tag.fields["Damage"].kind != 0 {
		var damage, readErr = tag.integer("Damage", 2)
		if readErr != nil {
			return result, readErr
		}
		result.Meta = int32(damage)
	}
	var extra = tag.fields["tag"]
	if extra.kind != 0 {
		if extra.kind != 10 {
			return result, fmt.Errorf("物品 tag 不是 Compound")
		}
		result.NBT = extra.littlePayload([]byte{10, 0, 0})
		if len(result.NBT) > math.MaxUint16 {
			return result, fmt.Errorf("物品 NBT 超过背包协议长度上限")
		}
	}
	return result, nil
}

func pmTiles(list pmTag, chunkX, chunkZ int32) ([]TileRecord, error) {
	if list.kind == 0 {
		return nil, nil
	}
	if list.kind != 9 {
		return nil, fmt.Errorf("需要列表")
	}
	var records = make([]TileRecord, 0, len(list.list))
	var occupied = make(map[[3]int32]bool, len(list.list))
	var index = 0
	for index = 0; index < len(list.list); index++ {
		var tag = list.list[index]
		var pos = [3]int32{}
		var axis = 0
		var keys = [...]string{"x", "y", "z"}
		for axis = 0; axis < 3; axis++ {
			var value, err = tag.integer(keys[axis], 3)
			if err != nil {
				return nil, err
			}
			pos[axis] = int32(value)
		}
		if pos[0]>>4 != chunkX || pos[2]>>4 != chunkZ || pos[1] < 0 || pos[1] > 255 || occupied[pos] {
			return nil, fmt.Errorf("坐标越界或重复: %v", pos)
		}
		occupied[pos] = true
		var tile = TileRecord{ID: string(tag.fields["id"].raw), X: pos[0], Y: pos[1], Z: pos[2]}
		var tileErr = pmTileFields(tag, &tile)
		if tileErr != nil {
			return nil, fmt.Errorf("%s %v: %w", tile.ID, pos, tileErr)
		}
		records = append(records, tile)
	}
	return records, nil
}

func pmTileFields(tag pmTag, tile *TileRecord) error {
	var allowed = []string{"id", "x", "y", "z"}
	var err error = nil
	switch tile.ID {
	case world.TileEnderChest:
		// 末影箱库存属于玩家存档，方块实体只保存位置。
	case world.TileSign:
		tile.Lines = make([]string, 4)
		var index = 0
		for index = 0; index < 4; index++ {
			var key = "Text" + strconv.Itoa(index+1)
			allowed = append(allowed, key)
			var line = tag.fields[key]
			if line.kind != 0 && (line.kind != 8 || !utf8.Valid(line.raw)) {
				return fmt.Errorf("%s 不是 UTF-8 文本", key)
			}
			tile.Lines[index] = string(line.raw)
		}
	case world.TileBed, world.TileSkull, world.TileItemFrame:
		var names = []string{"color"}
		var fields = []*byte{&tile.Color}
		if tile.ID == world.TileBed {
			tile.Color = 14
		}
		if tile.ID == world.TileSkull {
			names, fields = []string{"SkullType", "Rot"}, []*byte{&tile.SkullType, &tile.Rotation}
		}
		if tile.ID == world.TileItemFrame {
			names, fields = []string{"ItemRotation"}, []*byte{&tile.Rotation}
			allowed = append(allowed, "Item", "ItemDropChance")
			var chance = tag.fields["ItemDropChance"]
			if chance.kind != 0 && (chance.kind != 5 || math.Float32frombits(binary.BigEndian.Uint32(chance.raw)) != 1) {
				return fmt.Errorf("不支持非默认 ItemDropChance")
			}
			if tag.fields["Item"].kind != 0 {
				var stack, readErr = pmSlot(tag.fields["Item"])
				if readErr != nil {
					return readErr
				}
				tile.Items = []SlotData{stack}
			}
		}
		allowed = append(allowed, names...)
		var index = 0
		for index = 0; index < len(names); index++ {
			if tag.fields[names[index]].kind == 0 {
				continue
			}
			var value, readErr = tag.integer(names[index], 1)
			if readErr != nil {
				return readErr
			}
			*fields[index] = byte(value)
		}
	case world.TileFlowerPot:
		allowed = append(allowed, "item", "mData")
		var plant = SlotData{Count: 1}
		if tag.fields["item"].kind != 0 {
			var value, readErr = tag.integer("item", 2)
			if readErr != nil {
				return readErr
			}
			plant.ID = int32(uint16(value))
		}
		if tag.fields["mData"].kind != 0 {
			var value, readErr = tag.integer("mData", 3)
			if readErr != nil {
				return readErr
			}
			plant.Meta = int32(value)
		}
		if plant.ID != 0 {
			tile.Items = []SlotData{plant}
		}
	case world.TileChest:
		allowed = append(allowed, "Items", "pairx", "pairz")
		if tag.fields["pairx"].kind != 0 || tag.fields["pairz"].kind != 0 {
			var otherX, xErr = tag.integer("pairx", 3)
			var otherZ, zErr = tag.integer("pairz", 3)
			if xErr != nil || zErr != nil {
				return fmt.Errorf("双箱需要成对的整型 pairx、pairz")
			}
			tile.Paired, tile.PairX, tile.PairZ = true, int32(otherX), int32(otherZ)
		}
	case world.TileHopper:
		allowed = append(allowed, "Items", "TransferCooldown")
		if tag.fields["TransferCooldown"].kind != 0 {
			var ticks, readErr = tag.integer("TransferCooldown", 3)
			if readErr != nil || ticks < 0 || ticks > 255 {
				return fmt.Errorf("无效的 TransferCooldown")
			}
			tile.TransferCooldown = byte(ticks)
		}
	case world.TileFurnace:
		allowed = append(allowed, "Items", "BurnTime", "CookTime", "MaxTime", "BurnTicks")
		var timers = [...]string{"BurnTime", "CookTime", "MaxTime"}
		var values = [...]*int32{&tile.BurnTime, &tile.CookTime, &tile.MaxTime}
		var index = 0
		for index = 0; index < len(timers); index++ {
			if tag.fields[timers[index]].kind == 0 {
				continue
			}
			var ticks, readErr = tag.integer(timers[index], 2)
			if readErr != nil {
				return readErr
			}
			*values[index] = int32(ticks)
		}
		if tag.fields["MaxTime"].kind == 0 {
			tile.MaxTime = tile.BurnTime
		}
	default:
		return fmt.Errorf("尚不支持这种方块实体")
	}
	var index = 0
	for index = 0; index < len(tag.names); index++ {
		if !slices.Contains(allowed, tag.names[index]) {
			return fmt.Errorf("尚不能还原字段 %s", tag.names[index])
		}
	}
	var items = tag.fields["Items"]
	if items.kind == 0 {
		return nil
	}
	if items.kind != 9 {
		return fmt.Errorf("Items 需要列表")
	}
	tile.Items = make([]SlotData, world.SlotCount(tile.ID))
	var used uint64 = 0
	for index = 0; index < len(items.list); index++ {
		var entry = items.list[index]
		var slot, readErr = entry.integer("Slot", 1)
		if readErr != nil || slot < 0 || slot >= int64(len(tile.Items)) || used&(1<<slot) != 0 {
			return fmt.Errorf("无效或重复的槽号 %d", slot)
		}
		used |= 1 << slot
		tile.Items[slot], err = pmSlot(entry)
		if err != nil {
			return fmt.Errorf("第 %d 格: %w", slot, err)
		}
	}
	return nil
}
