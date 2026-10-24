package command

import "fmt"
import "math"

// 传送坐标的合法范围。水平取 3000 万（原版世界边界），竖直下限留出虚空，上限留出建筑高度。
const teleportLimitHorizontal float64 = 30000000
const teleportLimitLow float64 = -64
const teleportLimitHigh float64 = 512

// ValidateTeleportTarget 拒绝非有限数和越界坐标，命令层和服务端桥接共用这一份规则。strconv 会把 "nan"、"inf" 当合法浮点解析出来，
// 传送过去玩家位置变成 NaN，之后所有距离判定都会失效。
func ValidateTeleportTarget(posX float32, posY float32, posZ float32) error {
	var coords [3]float64 = [3]float64{float64(posX), float64(posY), float64(posZ)}
	var index int = 0
	for index = 0; index < len(coords); index++ {
		if math.IsNaN(coords[index]) || math.IsInf(coords[index], 0) {
			return fmt.Errorf("坐标必须是有限的数字")
		}
	}
	if math.Abs(coords[0]) > teleportLimitHorizontal || math.Abs(coords[2]) > teleportLimitHorizontal {
		return fmt.Errorf("坐标超出世界范围 ±%.0f", teleportLimitHorizontal)
	}
	if coords[1] < teleportLimitLow || coords[1] > teleportLimitHigh {
		return fmt.Errorf("高度必须在 %.0f 到 %.0f 之间", teleportLimitLow, teleportLimitHigh)
	}
	return nil
}

const teleportUsage string = "用法: /tp <目的玩家> | /tp <玩家> <目的玩家> | /tp <x> <y> <z> | /tp <玩家> <x> <y> <z>"

func runTeleport(ctx *Context, args []string) error {
	var admin, err = ctx.requireAdmin()
	if err != nil {
		return fmt.Errorf("传送: %w", err)
	}
	switch len(args) {
	case 1:
		return teleportToPlayer(ctx, admin, ctx.Username, args[0])
	case 2:
		return teleportToPlayer(ctx, admin, args[0], args[1])
	case 3:
		return teleportToCoords(ctx, admin, ctx.Username, args)
	case 4:
		return teleportToCoords(ctx, admin, args[0], args[1:])
	default:
		return fmt.Errorf(teleportUsage)
	}
}

// subjectNeeded 控制台没有「自己」，省略传送对象时必须报错，不能拿控制台的名字去查玩家。
func subjectNeeded(ctx *Context, subject string) error {
	if ctx.IsConsole && subject == ctx.Username {
		return fmt.Errorf("控制台必须指定要传送的玩家")
	}
	return nil
}

func teleportToPlayer(ctx *Context, admin Admin, subject string, target string) error {
	var err error = subjectNeeded(ctx, subject)
	if err != nil {
		return err
	}
	var movedName string = ""
	var targetName string = ""
	movedName, targetName, err = admin.TeleportToPlayer(subject, target)
	if err != nil {
		return fmt.Errorf("传送: %w", err)
	}
	ctx.respond(fmt.Sprintf("已将 %s 传送到 %s", movedName, targetName))
	return nil
}

func teleportToCoords(ctx *Context, admin Admin, subject string, specs []string) error {
	var err error = subjectNeeded(ctx, subject)
	if err != nil {
		return err
	}
	// 相对坐标 (~) 以被传送者当前脚下为基准，不是执行者。
	var originX, originY, originZ, online = admin.PlayerPosition(subject)
	var posX float32 = 0
	var posY float32 = 0
	var posZ float32 = 0
	posX, err = parseCoord(specs[0], originX, online, false)
	if err != nil {
		return fmt.Errorf("无效坐标 x: %s", specs[0])
	}
	posY, err = parseCoord(specs[1], originY, online, false)
	if err != nil {
		return fmt.Errorf("无效坐标 y: %s", specs[1])
	}
	posZ, err = parseCoord(specs[2], originZ, online, false)
	if err != nil {
		return fmt.Errorf("无效坐标 z: %s", specs[2])
	}
	err = ValidateTeleportTarget(posX, posY, posZ)
	if err != nil {
		return fmt.Errorf("传送: %w", err)
	}
	var name string = ""
	name, err = admin.TeleportToPosition(subject, posX, posY, posZ)
	if err != nil {
		return fmt.Errorf("传送: %w", err)
	}
	ctx.respond(fmt.Sprintf("已将 %s 传送到 (%.1f, %.1f, %.1f)", name, posX, posY, posZ))
	return nil
}
