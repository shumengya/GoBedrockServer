package storage

import "fmt"
import "os"
import "path/filepath"
import "strings"

const DirPlayers string = "players"
const DirWorlds string = "worlds"

const WorldMetaFile string = "world.json"

// 旧版把区块放在 worlds/<世界名>/<维度>/ 下，而 world.json 放在世界根上，
// 一个世界被拆成两层。这些是历史遗留的维度子目录名，打开世界时会自动收纳回一层。
var legacyDimensionDirs []string = []string{"overworld", "nether", "the_end"}

func PlayersDir(dataDir string) string {
	return filepath.Join(dataDir, DirPlayers)
}

func WorldsDir(dataDir string) string {
	return filepath.Join(dataDir, DirWorlds)
}

// WorldDir 是一个世界的唯一目录：元数据、方块实体、掉落物、区域文件全在这一层。
// 主世界 / 地狱 / 末地 / 空岛 / 海洋都是平级的世界，各自一个文件夹。
func WorldDir(dataDir string, worldName string) string {
	return filepath.Join(dataDir, DirWorlds, sanitizeName(worldName))
}

func PlayerPath(dataDir string, uuid string) string {
	var name string = strings.ToLower(strings.TrimSpace(uuid))
	if name == "" {
		name = "unknown"
	}
	return filepath.Join(PlayersDir(dataDir), sanitizeName(name)+".json")
}

func WorldMetaPath(dataDir string, worldName string) string {
	return filepath.Join(WorldDir(dataDir, worldName), WorldMetaFile)
}

func RegionPath(worldDir string, regionX int32, regionZ int32) string {
	var fileName string = fmt.Sprintf("r.%d.%d.gmr", regionX, regionZ)
	return filepath.Join(worldDir, fileName)
}

func sanitizeName(name string) string {
	if name == "" {
		return "world"
	}
	var builder strings.Builder = strings.Builder{}
	var characters []rune = []rune(name)
	var index int = 0
	for index = 0; index < len(characters); index++ {
		var character rune = characters[index]
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9', character == '-', character == '_', character == '.':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	var safe string = builder.String()
	if safe == "" || safe == "." || safe == ".." {
		return "world"
	}
	return safe
}

// EnsureLayout 建出 players/ 和 worlds/<世界名>/。
func EnsureLayout(dataDir string, worldName string) error {
	var worldDir string = WorldDir(dataDir, worldName)
	var dirs []string = []string{PlayersDir(dataDir), worldDir}
	var index int = 0
	for index = 0; index < len(dirs); index++ {
		var dir string = dirs[index]
		var err error = os.MkdirAll(dir, 0o755)
		if err != nil {
			return fmt.Errorf("建目录 %s: %w", dir, err)
		}
	}
	return nil
}

// MigrateLegacyDimensions 把旧版存进维度子目录的 .gmr 搬到世界根上，返回搬移的文件数。
// 目标已存在同名文件时保留旧的、跳过该文件，避免用陈旧的区域文件覆盖新数据。
func MigrateLegacyDimensions(worldDir string) (int, error) {
	var moved int = 0
	var err error = nil
	var dimIndex int = 0
	for dimIndex = 0; dimIndex < len(legacyDimensionDirs); dimIndex++ {
		var legacyDir string = filepath.Join(worldDir, legacyDimensionDirs[dimIndex])
		var entries []os.DirEntry = nil
		entries, err = os.ReadDir(legacyDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return moved, fmt.Errorf("读取旧维度目录 %s: %w", legacyDir, err)
		}
		var entryIndex int = 0
		for entryIndex = 0; entryIndex < len(entries); entryIndex++ {
			var entry os.DirEntry = entries[entryIndex]
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gmr") {
				continue
			}
			var src string = filepath.Join(legacyDir, entry.Name())
			var dst string = filepath.Join(worldDir, entry.Name())
			var statErr error = nil
			var statInfo os.FileInfo = nil
			statInfo, statErr = os.Stat(dst)
			if statErr == nil {
				_ = statInfo
				continue
			}
			err = os.Rename(src, dst)
			if err != nil {
				return moved, fmt.Errorf("搬移区域文件 %s: %w", src, err)
			}
			moved++
		}
		// 只删空的旧目录；里面还有别的东西就留着，别替用户做主。
		var remain []os.DirEntry = nil
		remain, err = os.ReadDir(legacyDir)
		if err != nil {
			return moved, fmt.Errorf("复查旧维度目录 %s: %w", legacyDir, err)
		}
		if len(remain) == 0 {
			err = os.Remove(legacyDir)
			if err != nil {
				return moved, fmt.Errorf("删除空旧维度目录 %s: %w", legacyDir, err)
			}
		}
	}
	return moved, nil
}
