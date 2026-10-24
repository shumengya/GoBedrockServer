package main

import "flag"
import "os"
import "os/signal"
import "path/filepath"
import "syscall"

import "go-mc-server/config"
import "go-mc-server/internal/logger"
import "go-mc-server/internal/server"

// loadConfig 先读 server.properties，再让命令行里「明确传了」的参数覆盖它。
// 用 flag.Visit 而不是直接比对默认值：用户显式传了和默认值相同的数（比如 -port 19132）
// 也应该压过配置文件。
func loadConfig() *config.Config {
	var cfg *config.Config = config.Default()
	var propertiesPath *string = flag.String("properties", cfg.PropertiesPath, "server.properties 路径")
	var host *string = flag.String("host", cfg.Host, "Bind address")
	var port *int = flag.Int("port", cfg.Port, "UDP port")
	var motd *string = flag.String("motd", cfg.MOTD, "Server MOTD")
	var maxPlayers *int = flag.Int("max-players", cfg.MaxPlayers, "Maximum players, 0 for unlimited")
	var spawnGuard *int = flag.Int("spawn-protection", cfg.SpawnProtection, "Spawn protection radius in blocks, 0 disables")
	flag.Parse()

	cfg.PropertiesPath = *propertiesPath
	cfg.ServerDir = filepath.Dir(*propertiesPath)
	var warnings, err = config.LoadProperties(*propertiesPath, cfg)
	if err != nil {
		logger.Fatal("main", "读取配置失败: %v", err)
	}
	var index int = 0
	for index = 0; index < len(warnings); index++ {
		logger.Warn("main", "配置: %s", warnings[index])
	}

	flag.Visit(func(set *flag.Flag) {
		switch set.Name {
		case "host":
			cfg.Host = *host
		case "port":
			cfg.Port = *port
		case "motd":
			cfg.MOTD = *motd
		case "max-players":
			cfg.MaxPlayers = *maxPlayers
		case "spawn-protection":
			cfg.SpawnProtection = *spawnGuard
		}
	})
	return cfg
}

func main() {
	var cfg *config.Config = loadConfig()
	var srv *server.Server = server.New(cfg)
	var startErr error = srv.Start()
	if startErr != nil {
		logger.Fatal("main", "启动失败: %v", startErr)
	}

	// 控制台读 stdin 执行命令。没有交互终端时读到 EOF 就结束，不影响服务器运行。
	logger.Go("console", func() {
		var consoleErr error = srv.RunConsole(os.Stdin)
		if consoleErr != nil {
			logger.Warn("console", "控制台输入中断: %v", consoleErr)
		}
	})

	var sig chan os.Signal = make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		logger.Info("main", "收到退出信号，正在关闭...")
	case <-srv.StopRequested():
		logger.Info("main", "收到 /stop，正在关闭...")
	}
	srv.Stop()
}
