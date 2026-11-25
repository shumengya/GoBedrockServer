package storage

import "bytes"
import "encoding/binary"
import "fmt"
import "io"
import "math"
import "os"
import "path/filepath"
import "regexp"
import "strconv"
import "strings"

import "go-mc-server/internal/world"

var pmRegionName = regexp.MustCompile(`^r\.(-?[0-9]+)\.(-?[0-9]+)\.(mca|mcr|mcapm)$`)

func pmMeta(path, name string) (WorldMeta, error) {
	var file, err = os.Open(path)
	if err != nil {
		return WorldMeta{}, err
	}
	defer file.Close()
	var tag, readErr = readPMRoot(file, 1, "Data")
	if readErr != nil {
		return WorldMeta{}, readErr
	}
	var generators = map[string]string{"normal": world.GeneratorNormal, "default": world.GeneratorNormal, "flat": world.GeneratorFlat, "hell": world.GeneratorNetherTerrain, "ender": world.GeneratorEndTerrain}
	var generator = string(tag.fields["generatorName"].raw)
	var meta = WorldMeta{Name: name, Generator: generators[strings.ToLower(generator)]}
	if meta.Generator == "" || len(tag.fields["generatorOptions"].raw) != 0 {
		return meta, fmt.Errorf("不能还原生成器 %q 或其自定义预设", generator)
	}
	var seed, seedErr = tag.integer("RandomSeed", 4)
	if seedErr != nil {
		return meta, seedErr
	}
	if seed < math.MinInt32 || seed > math.MaxInt32 {
		return meta, fmt.Errorf("种子 %d 超出本服 32 位范围", seed)
	}
	meta.Seed = int32(seed)
	var names = [...]string{"SpawnX", "SpawnY", "SpawnZ"}
	var index = 0
	for index = 0; index < len(names); index++ {
		var val, fieldErr = tag.integer(names[index], 3)
		if fieldErr != nil {
			return meta, fieldErr
		}
		meta.Spawn[index] = int32(val)
	}
	var ticks, timeErr = tag.integer("Time", 4)
	if timeErr != nil {
		return meta, timeErr
	}
	if ticks < math.MinInt32 || ticks > math.MaxInt32 {
		ticks %= int64(world.TimeFull)
	}
	meta.Time = int32(ticks)
	return meta, nil
}

// ImportPMWorld 离线导入到尚不存在的目录。先在临时目录完成全部解析，坏列不会发布成一张看似完整的图。
func ImportPMWorld(source, dest string) (int, error) {
	var absolute, err = filepath.Abs(dest)
	if err != nil {
		return 0, err
	}
	dest = filepath.Clean(absolute)
	var name = filepath.Base(dest)
	if name != sanitizeName(name) || strings.Contains(name, ".") || !filepath.IsLocal(name) {
		return 0, fmt.Errorf("无效的目标世界目录名 %q", name)
	}
	if _, err = os.Lstat(dest); err == nil {
		return 0, fmt.Errorf("目标目录已存在: %s", dest)
	} else if !os.IsNotExist(err) {
		return 0, err
	}
	var dir, dirErr = filepath.EvalSymlinks(source)
	if dirErr != nil {
		return 0, dirErr
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return 0, err
	}
	var parent, parentErr = filepath.EvalSymlinks(filepath.Dir(dest))
	if parentErr != nil {
		return 0, parentErr
	}
	var relative, relErr = filepath.Rel(dir, filepath.Join(parent, name))
	if relErr == nil && (relative == "." || filepath.IsLocal(relative)) {
		return 0, fmt.Errorf("目标不能位于源世界内")
	}
	var meta, metaErr = pmMeta(filepath.Join(dir, "level.dat"), name)
	if metaErr != nil {
		return 0, fmt.Errorf("level.dat: %w", metaErr)
	}
	var entries, listErr = os.ReadDir(filepath.Join(dir, "region"))
	if listErr != nil && !os.IsNotExist(listErr) {
		return 0, listErr
	}
	var _, dbErr = os.Stat(filepath.Join(dir, "db"))
	if dbErr == nil {
		return 0, fmt.Errorf("LevelDB 世界不能作为区域文件导入")
	} else if !os.IsNotExist(dbErr) {
		return 0, dbErr
	}
	var stage, stageErr = os.MkdirTemp("", "gomc-world-import-")
	if stageErr != nil {
		return 0, stageErr
	}
	defer os.RemoveAll(stage)
	var tiles []TileRecord = nil
	var entities []EntityData = nil
	var columns, index = 0, 0
	for index = 0; index < len(entries); index++ {
		var filename = entries[index].Name()
		var match = pmRegionName.FindStringSubmatch(filename)
		if match == nil {
			return 0, fmt.Errorf("不能识别区域条目 %s", filename)
		}
		var regionX, xErr = strconv.ParseInt(match[1], 10, 32)
		var regionZ, zErr = strconv.ParseInt(match[2], 10, 32)
		if xErr != nil || zErr != nil || regionX < math.MinInt32/32 || regionX > math.MaxInt32/32 || regionZ < math.MinInt32/32 || regionZ > math.MaxInt32/32 {
			return 0, fmt.Errorf("区域坐标越界: %s", filename)
		}
		var output = RegionPath(stage, int32(regionX), int32(regionZ))
		if _, err = os.Stat(output); err == nil {
			return 0, fmt.Errorf("重复区域: %s", filename)
		}
		var count, regionErr = importPMRegion(filepath.Join(dir, "region", filename), output, match[3], int32(regionX), int32(regionZ), &tiles, &entities)
		if regionErr != nil {
			return 0, fmt.Errorf("%s: %w", filename, regionErr)
		}
		columns += count
	}
	if len(tiles) > 0 {
		if err = saveTileRecords(filepath.Join(stage, TilesFile), tiles); err != nil {
			return 0, err
		}
	}
	if len(entities) > 0 {
		if err = saveEntityRecords(filepath.Join(stage, EntitiesFile), entities); err != nil {
			return 0, err
		}
	}
	if err = SaveWorldMeta(filepath.Join(stage, WorldMetaFile), meta); err != nil {
		return 0, err
	}
	if err = os.Mkdir(dest, 0755); err != nil {
		return 0, err
	}
	if err = os.CopyFS(dest, os.DirFS(stage)); err != nil {
		return 0, fmt.Errorf("复制未完成，目标目录 %s 不完整: %w", dest, err)
	}
	return columns, nil
}

func importPMRegion(input, output, format string, regionX, regionZ int32, tiles *[]TileRecord, entities *[]EntityData) (int, error) {
	var file, err = os.Open(input)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var header = [8192]byte{}
	if _, err = io.ReadFull(file, header[:]); err != nil {
		return 0, err
	}
	var info, statErr = file.Stat()
	if statErr != nil {
		return 0, statErr
	}
	var region = regionMem{path: output}
	var slot, count = 0, 0
	for slot = 0; slot < 1024; slot++ {
		var location = binary.BigEndian.Uint32(header[slot*4:])
		if location == 0 {
			continue
		}
		var start = int64(location>>8) * 4096
		var size = int64(location&255) * 4096
		if start < 8192 || size == 0 || start+size > info.Size() {
			return 0, fmt.Errorf("槽 %d 的扇区范围无效", slot)
		}
		var record = io.NewSectionReader(file, start, size)
		var prefix = [5]byte{}
		if _, err = io.ReadFull(record, prefix[:]); err != nil {
			return 0, err
		}
		var length = int64(binary.BigEndian.Uint32(prefix[:4]))
		if length <= 1 || length > size-4 {
			return 0, fmt.Errorf("槽 %d 的记录长度无效: %d", slot, length)
		}
		var level, readErr = readPMRoot(io.LimitReader(record, length-1), prefix[4], "Level")
		if readErr != nil {
			return 0, fmt.Errorf("槽 %d: %w", slot, readErr)
		}
		var chunkX, xErr = level.integer("xPos", 3)
		var chunkZ, zErr = level.integer("zPos", 3)
		if xErr != nil || zErr != nil || chunkX != int64(regionX)*32+int64(slot&31) || chunkZ != int64(regionZ)*32+int64(slot>>5) {
			return 0, fmt.Errorf("槽 %d 的区块坐标不匹配", slot)
		}
		var column, columnErr = pmColumn(level, format)
		if columnErr != nil {
			return 0, fmt.Errorf("区块 (%d,%d): %w", chunkX, chunkZ, columnErr)
		}
		var added, tileErr = pmTiles(level.fields["TileEntities"], int32(chunkX), int32(chunkZ))
		if tileErr != nil {
			return 0, fmt.Errorf("区块 (%d,%d) TileEntities: %w", chunkX, chunkZ, tileErr)
		}
		*tiles = append(*tiles, added...)
		var group = level.fields["Entities"]
		if group.kind != 0 && group.kind != 9 {
			return 0, fmt.Errorf("区块 (%d,%d) Entities 需要列表", chunkX, chunkZ)
		}
		var index = 0
		for index = 0; index < len(group.list); index++ {
			var record, entityErr = pmDrop(group.list[index], chunkX, chunkZ)
			if entityErr != nil {
				return 0, fmt.Errorf("区块 (%d,%d) Entities[%d]: %w", chunkX, chunkZ, index, entityErr)
			}
			*entities = append(*entities, record)
		}
		region.slots[slot], err = deflate(column.EncodeDisk())
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, region.flush()
}

func pmColumn(level pmTag, format string) (*world.Chunk, error) {
	var index = 0
	var ticks = level.fields["TileTicks"]
	if ticks.kind != 0 && (ticks.kind != 9 || len(ticks.list) > 0) {
		return nil, fmt.Errorf("尚不能转换 TileTicks，已停止导入")
	}
	var sections [16][]byte
	if format == "mcr" {
		var ids = level.fields["Blocks"].raw
		var meta = level.fields["Data"].raw
		if level.fields["Blocks"].kind != 7 || level.fields["Data"].kind != 7 || len(ids) != 32768 || len(meta) != 16384 {
			return nil, fmt.Errorf("McRegion 方块或 meta 数组无效")
		}
		var section = 0
		for section = 0; section < 8; section++ {
			var buf = make([]byte, 6144)
			var column = 0
			for column = 0; column < 256; column++ {
				copy(buf[column*16:], ids[column*128+section*16:column*128+section*16+16])
				copy(buf[4096+column*8:], meta[column*64+section*8:column*64+section*8+8])
			}
			sections[section] = buf
		}
	} else {
		var list = level.fields["Sections"]
		if list.kind != 9 {
			return nil, fmt.Errorf("缺少 Sections 列表")
		}
		for index = 0; index < len(list.list); index++ {
			var section = list.list[index]
			var height, err = section.integer("Y", 1)
			if err != nil || height < 0 || height >= 16 || sections[height] != nil {
				return nil, fmt.Errorf("无效或重复的节号 %d", height)
			}
			var ids = section.fields["Blocks"].raw
			var meta = section.fields["Data"].raw
			if section.fields["Blocks"].kind != 7 || section.fields["Data"].kind != 7 || len(ids) != 4096 || len(meta) != 2048 {
				return nil, fmt.Errorf("节 %d 方块或 meta 数组无效", height)
			}
			if len(bytes.Trim(section.fields["Add"].raw, "\x00")) > 0 {
				return nil, fmt.Errorf("节 %d 含超过 255 的方块 ID", height)
			}
			var buf = make([]byte, 6144)
			if format == "mcapm" {
				copy(buf, ids)
				copy(buf[4096:], meta)
			} else {
				var target = 0
				for target = 0; target < 4096; target++ {
					var source = (target&15)<<8 | target&240 | target>>8
					buf[target] = ids[source]
					var nibble = meta[source>>1] >> uint((source&1)*4) & 15
					buf[4096+target/2] |= nibble << uint((target&1)*4)
				}
			}
			sections[height] = buf
		}
	}
	var biomes = level.fields["Biomes"].raw
	var colors = level.fields["BiomeColors"]
	if colors.kind != 0 {
		if colors.kind != 11 || len(colors.raw) != 1024 {
			return nil, fmt.Errorf("BiomeColors 长度或类型错误")
		}
		biomes = make([]byte, 256)
		for index = 0; index < 256; index++ {
			biomes[index] = colors.raw[index*4]
		}
	} else if level.fields["Biomes"].kind == 0 {
		biomes = make([]byte, 256)
	}
	if len(biomes) != 256 {
		return nil, fmt.Errorf("Biomes 长度错误")
	}
	return world.ImportColumn(sections, biomes), nil
}
