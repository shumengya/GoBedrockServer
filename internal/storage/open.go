package storage

import "fmt"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/world"

// OpenWorld 建好目录、读取或新建 world.json，然后挂上区域文件存储。
func OpenWorld(dataDir string, definitions WorldDefaults) (*world.World, error) {
	var err error = EnsureLayout(dataDir, definitions.Name)
	if err != nil {
		return nil, fmt.Errorf("建世界目录 %s: %w", definitions.Name, err)
	}
	var worldDir string = WorldDir(dataDir, definitions.Name)
	var moved int = 0
	moved, err = MigrateLegacyDimensions(worldDir)
	if err != nil {
		return nil, fmt.Errorf("迁移旧维度目录 %s: %w", worldDir, err)
	}
	if moved > 0 {
		logger.Info("storage", "世界 %s 收纳了 %d 个区域文件(旧维度子目录 → 世界根)", definitions.Name, moved)
	}
	var meta WorldMeta = WorldMeta{}
	meta, err = LoadWorldMeta(WorldMetaPath(dataDir, definitions.Name), definitions)
	if err != nil {
		return nil, fmt.Errorf("读取世界元数据 %s: %w", definitions.Name, err)
	}
	var loaded *world.World = world.New(meta.Name)
	loaded.Seed = meta.Seed
	loaded.SetTimeTicks(meta.Time)
	loaded.SetTimeStopped(meta.TimeStopped)
	loaded.SpawnX = meta.Spawn[0]
	loaded.SpawnY = meta.Spawn[1]
	loaded.SpawnZ = meta.Spawn[2]
	loaded.Generator = meta.Generator
	if loaded.Generator == "" {
		loaded.Generator = world.GeneratorFlat
	}
	loaded.SetStore(NewRegionStore(worldDir))
	var tiles []TileRecord = nil
	tiles, err = LoadTiles(dataDir, loaded.Name)
	if err != nil {
		return nil, fmt.Errorf("读取世界 %s 方块实体: %w", loaded.Name, err)
	}
	loaded.LoadTiles(WorldTilesFromRecords(tiles))
	return loaded, nil
}

func SaveWorldFrom(dataDir string, loaded *world.World) error {
	return SaveWorldWithTiles(dataDir, loaded, loaded.SnapshotTiles())
}

func SaveWorldWithTiles(dataDir string, loaded *world.World, tiles []world.Tile) error {
	var err error = loaded.SaveDirty()
	if err != nil {
		return fmt.Errorf("保存世界 %s 区块: %w", loaded.Name, err)
	}
	err = SaveTiles(dataDir, loaded.Name, TilesFromWorld(tiles))
	if err != nil {
		return fmt.Errorf("保存世界 %s 方块实体: %w", loaded.Name, err)
	}
	var gen string = loaded.Generator
	if gen == "" {
		gen = world.GeneratorFlat
	}
	var meta WorldMeta = WorldMeta{
		Name:        loaded.Name,
		Seed:        loaded.Seed,
		Spawn:       [3]int32{loaded.SpawnX, loaded.SpawnY, loaded.SpawnZ},
		Time:        loaded.TimeTicks(),
		TimeStopped: loaded.TimeStopped(),
		Generator:   gen,
	}
	err = SaveWorldMeta(WorldMetaPath(dataDir, loaded.Name), meta)
	if err != nil {
		return fmt.Errorf("保存世界元数据 %s: %w", loaded.Name, err)
	}
	return nil
}
