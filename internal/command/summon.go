package command

import (
	"fmt"
	"strconv"
	"strings"

	"go-mc-server/internal/entity"
)

// SummonCommand implements /summon <entity> [x y z].
type SummonCommand struct{}

func (SummonCommand) Name() string        { return "summon" }
func (SummonCommand) Aliases() []string   { return nil }
func (SummonCommand) Usage() string       { return "/summon <实体名|id> [x y z]" }
func (SummonCommand) Description() string { return "在指定位置生成实体" }

func (c SummonCommand) Execute(ctx *Context, args []string) error {
	if len(args) != 1 && len(args) != 4 {
		return fmt.Errorf("用法: %s", c.Usage())
	}
	if ctx.SpawnEntity == nil {
		return fmt.Errorf("命令上下文未就绪")
	}

	t, err := entity.FromString(args[0])
	if err != nil {
		return err
	}
	if !t.CanSummon {
		return fmt.Errorf("本轮不能召唤 %s", t.Name)
	}

	var x, y, z float32
	if len(args) == 1 {
		var ok bool
		x, y, z, ok = ctx.pos()
		if !ok {
			return fmt.Errorf("必须指定生成坐标")
		}
	} else {
		ox, oy, oz, hasOrigin := ctx.pos()
		x, err = parseCoord(args[1], ox, hasOrigin, false)
		if err != nil {
			return fmt.Errorf("无效坐标 x: %s", args[1])
		}
		y, err = parseCoord(args[2], oy, hasOrigin, true)
		if err != nil {
			return fmt.Errorf("无效坐标 y: %s", args[2])
		}
		z, err = parseCoord(args[3], oz, hasOrigin, false)
		if err != nil {
			return fmt.Errorf("无效坐标 z: %s", args[3])
		}
	}

	if err := ctx.SpawnEntity(t.NetworkID, x, y, z); err != nil {
		return err
	}
	if ctx.Reply != nil {
		ctx.Reply(fmt.Sprintf("已生成 %s 于 (%.1f, %.1f, %.1f)", t.Name, x, y, z))
	}
	return nil
}

func (ctx *Context) pos() (x, y, z float32, ok bool) {
	if ctx == nil || ctx.Position == nil {
		return 0, 0, 0, false
	}
	return ctx.Position()
}

func parseCoord(spec string, origin float32, hasOrigin bool, clampY bool) (float32, error) {
	spec = strings.TrimSpace(spec)
	if spec == "~" || strings.HasPrefix(spec, "~") {
		if !hasOrigin {
			return 0, fmt.Errorf("相对坐标需要玩家位置")
		}
		off := spec[1:]
		v := origin
		if off != "" {
			n, err := strconv.ParseFloat(off, 32)
			if err != nil {
				return 0, err
			}
			v = origin + float32(n)
		}
		if clampY {
			if v < 0 {
				v = 0
			}
			if v > 128 {
				v = 128
			}
		}
		return v, nil
	}
	n, err := strconv.ParseFloat(spec, 32)
	if err != nil {
		return 0, err
	}
	return float32(n), nil
}
