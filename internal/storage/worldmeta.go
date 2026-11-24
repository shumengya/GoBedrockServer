package storage

import (
	"encoding/json"
	"os"

	"go-mc-server/internal/world"
)

type WorldMeta struct {
	Name  string   `json:"name"`
	Seed  int32    `json:"seed"`
	Spawn [3]int32 `json:"spawn"`
	Time  int32    `json:"time"`
	// TimeStopped 记录 /time stop 的状态，重启后保持冻结。老存档没有这个字段，读出来就是 false。
	TimeStopped bool   `json:"timeStopped"`
	Generator   string `json:"generator"`
}

type WorldDefaults struct {
	Name      string
	Seed      int32
	SpawnX    int32
	SpawnY    int32
	SpawnZ    int32
	Generator string
}

func LoadWorldMeta(path string, def WorldDefaults) (WorldMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			gen := def.Generator
			if gen == "" {
				gen = "flat"
			}
			meta := WorldMeta{
				Name:      def.Name,
				Seed:      def.Seed,
				Spawn:     [3]int32{def.SpawnX, def.SpawnY, def.SpawnZ},
				Time:      0,
				Generator: gen,
			}
			if gen == world.GeneratorNormal {
				meta.Spawn = world.NormalSpawn(def.Seed, def.SpawnX, def.SpawnZ)
			}
			if gen == world.GeneratorNetherTerrain || gen == world.GeneratorEndTerrain {
				meta.Spawn = world.DimensionSpawn(gen, def.Seed)
			}
			if err := SaveWorldMeta(path, meta); err != nil {
				return WorldMeta{}, err
			}
			return meta, nil
		}
		return WorldMeta{}, err
	}
	var meta WorldMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return WorldMeta{}, err
	}
	if meta.Name == "" {
		meta.Name = def.Name
	}
	if meta.Generator == "" {
		meta.Generator = "flat"
	}
	return meta, nil
}

func SaveWorldMeta(path string, meta WorldMeta) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
