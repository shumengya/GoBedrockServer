package config

// 视距上限的合法范围（区块）。下限和协议层一致；上限防止客户端请求一个巨大的半径把服务器拖垮。
const MinViewDistance int = 2
const MaxViewDistance int = 32

// Config holds all server configuration.
type Config struct {
	Host       string
	Port       int
	MOTD       string
	MaxPlayers int
	// SpawnProtection 是出生点保护半径（方块）。<=0 关闭，对齐参考端默认 16。
	SpawnProtection int
	GameMode        int
	Difficulty      int
	// ViewDistance 是客户端视距请求的上限（区块）。
	ViewDistance int
	// PVP 关闭后玩家之间不能互相攻击。
	PVP        bool
	Query      bool
	FireSpread bool
	// Whitelist 是启动时的白名单开关；运行中由 /whitelist 改动，以 Server 里的值为准。
	Whitelist         bool
	SpawnX            int
	SpawnY            int
	SpawnZ            int
	WorldName         string
	DataDir           string
	AutosaveSeconds   int
	PlayerSaveSeconds int
	// PropertiesPath 是 server.properties 的路径；空串表示不落盘（测试用）。
	PropertiesPath string
	// ServerDir 放 ops.txt、white-list.txt、banned-*.txt，和参考端一样放在服务器根目录。
	ServerDir string
	Offline   bool // skip Xbox Live auth (offline/cracked mode)
}

// GetMOTD returns the server MOTD (satisfies raknet.cfgIface).
func (c *Config) GetMOTD() string { return c.MOTD }

// GetMaxPlayers returns the max player count (satisfies raknet.cfgIface).
func (c *Config) GetMaxPlayers() int { return c.MaxPlayers }

// GetPort returns the server port (satisfies raknet.cfgIface).
func (c *Config) GetPort() int { return c.Port }

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Host:              "0.0.0.0",
		Port:              19132,
		MOTD:              "一个由go语言开发的minecraft服务器",
		MaxPlayers:        20,
		SpawnProtection:   16,
		GameMode:          0, // Survival
		Difficulty:        1, // Easy
		ViewDistance:      4,
		PVP:               true,
		Query:             true,
		Whitelist:         false,
		SpawnX:            0,
		SpawnY:            5,
		SpawnZ:            0,
		WorldName:         "world",
		DataDir:           "data",
		AutosaveSeconds:   30,
		PlayerSaveSeconds: 600,
		PropertiesPath:    "server.properties",
		ServerDir:         ".",
		Offline:           true,
	}
}
