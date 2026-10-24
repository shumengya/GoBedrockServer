package server

import "math"

// insideSpawnProtection 判断某个方块是否落在世界出生点的保护半径内。
// 半径取配置的 spawn-protection（默认 16），<=0 表示关闭。
// 只比 XZ 平面距离，参考端 Level::checkSpawnProtection 也是这么算的，不看 Y。
func insideSpawnProtection(spawnX int32, spawnZ int32, radius int32, blockX int32, blockZ int32) bool {
	if radius <= 0 {
		return false
	}
	var diffX float64 = float64(blockX) - float64(spawnX)
	var diffZ float64 = float64(blockZ) - float64(spawnZ)
	return math.Sqrt(diffX*diffX+diffZ*diffZ) <= float64(radius)
}
