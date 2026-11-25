package storage

import "encoding/binary"
import "fmt"
import "math"

import "go-mc-server/internal/entity"

// Pos 和 Motion 使用大端 Double；Rotation 使用 Float。本服运行时坐标精度为 float32。
func pmVector(tag pmTag, name string, kind byte, out []float32) error {
	var list = tag.fields[name]
	if list.kind != 9 || list.element != kind || len(list.list) != len(out) {
		return fmt.Errorf("%s 需要 %d 个类型 %d 的分量", name, len(out), kind)
	}
	var index = 0
	for index = 0; index < len(out); index++ {
		var raw = list.list[index].raw
		if kind == 6 {
			out[index] = float32(math.Float64frombits(binary.BigEndian.Uint64(raw)))
		} else {
			out[index] = math.Float32frombits(binary.BigEndian.Uint32(raw))
		}
		if math.IsNaN(float64(out[index])) || math.IsInf(float64(out[index]), 0) {
			return fmt.Errorf("%s[%d] 无法表示为有限的 float32", name, index)
		}
	}
	return nil
}

func pmDrop(tag pmTag, chunkX, chunkZ int64) (EntityData, error) {
	var record = EntityData{NetworkID: entity.NetworkIDItem}
	var id = tag.fields["id"]
	if id.kind != 8 || string(id.raw) != "Item" {
		return record, fmt.Errorf("尚不能还原实体 %q", id.raw)
	}
	var pos, motion = [3]float32{}, [3]float32{}
	var angles = [2]float32{}
	var err = pmVector(tag, "Pos", 6, pos[:])
	if err != nil {
		return record, err
	}
	if math.Floor(float64(pos[0])/16) != float64(chunkX) || math.Floor(float64(pos[2])/16) != float64(chunkZ) {
		return record, fmt.Errorf("掉落物位置 %v 与所属区块不符", pos)
	}
	record.X, record.Y, record.Z = pos[0], pos[1], pos[2]
	if tag.fields["Motion"].kind != 0 {
		err = pmVector(tag, "Motion", 6, motion[:])
		if err != nil {
			return record, err
		}
	}
	record.SpeedX, record.SpeedY, record.SpeedZ = motion[0], motion[1], motion[2]
	err = pmVector(tag, "Rotation", 5, angles[:])
	if err != nil {
		return record, err
	}
	record.Yaw, record.Pitch = angles[0], angles[1]
	var stack, stackErr = pmSlot(tag.fields["Item"])
	if stackErr != nil {
		return record, fmt.Errorf("Item: %w", stackErr)
	}
	if stack.ID == 0 || stack.Count == 0 {
		return record, fmt.Errorf("掉落物的物品栈为空")
	}
	record.ID, record.Meta, record.Count, record.NBT = stack.ID, stack.Meta, stack.Count, stack.NBT
	var index = 0
	for index = 0; index < len(tag.names); index++ {
		var key = tag.names[index]
		var field = tag.fields[key]
		switch key {
		case "id", "Pos", "Motion", "Rotation", "Item":
		case "PickupDelay", "Age", "Health", "Air", "Fire":
			var value, readErr = tag.integer(key, 2)
			if readErr != nil {
				return record, readErr
			}
			switch key {
			case "PickupDelay":
				record.PickupDelay = int(value)
			case "Age":
				record.Age = int(value)
			case "Health", "Air", "Fire":
				if (key == "Health" && value != 5) || (key == "Air" && value != 300) || (key == "Fire" && value != 0) {
					return record, fmt.Errorf("不能还原掉落物 %s=%d", key, value)
				}
			}
		case "OnGround", "Invulnerable":
			var value, readErr = tag.integer(key, 1)
			if readErr != nil {
				return record, readErr
			}
			if key == "Invulnerable" && value != 0 {
				return record, fmt.Errorf("不能还原 Invulnerable=%d", value)
			}
			if key == "OnGround" {
				record.OnGround = value != 0
			}
		case "FallDistance":
			// 参考端的掉落物不接受跌落伤害；移动只需要位置、速度和着地状态。
			if field.kind != 5 {
				return record, fmt.Errorf("FallDistance 需要 Float")
			}
		default:
			return record, fmt.Errorf("尚不能还原掉落物字段 %s", key)
		}
	}
	return record, nil
}
