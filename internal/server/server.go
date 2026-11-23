package server

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-mc-server/config"
	"go-mc-server/internal/access"
	"go-mc-server/internal/blockrule"
	"go-mc-server/internal/command"
	"go-mc-server/internal/entity"
	"go-mc-server/internal/item"
	"go-mc-server/internal/logger"
	"go-mc-server/internal/network"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/raknet"
	"go-mc-server/internal/storage"
	"go-mc-server/internal/world"
)

type Server struct {
	cfg    *config.Config
	rakSrv *raknet.Server
	world  *world.World
	worlds map[string]*world.World
	cmds   *command.Map

	mu              sync.RWMutex
	players         map[string]*Player
	entities        map[int64]*entity.Instance
	entityIDCounter int64

	visMu sync.Mutex // 串行化玩家互相显示/隐藏

	// blockMu 保护"读方块再改方块"的整段流程：放置、破坏，以及之后的邻居更新和 tick。
	// 没有它，两个玩家同时往同一格放方块会都通过"落点是空气"的检查。
	// 加锁顺序：blockMu 最先拿，里面再拿 mu、玩家锁、世界锁；反过来会死锁。
	blockMu sync.Mutex

	// scheduled 是排队中的方块定时任务（比如按钮弹回），值是到期的 tick 序号，blockTick 是当前序号。
	// 都只在持有 blockMu 时读写。队列不存盘，重启后丢失；停在按下状态的按钮点一下就会重新排上队。
	scheduled map[scheduledKey]int64
	blockTick int64

	stopCh   chan struct{}
	stopOnce sync.Once
	saveMu   sync.Mutex

	// stopRequested 在 /stop 或控制台要求关服时关闭，main 据此走正常的 Stop 流程。
	stopRequested   chan struct{}
	stopRequestOnce sync.Once

	// access 是管理员 / 白名单 / 封禁；whitelistOn 是运行时的白名单开关（/whitelist 会改它）。
	access      *access.Access
	whitelistOn atomic.Bool

	// kickHook 非空时代替真正的断开连接，只给测试用：真会话需要 UDP 连接。
	kickHook func(player *Player, message string)

	// timeTicker 是昼夜同步用的计数器，只在 tick 协程里读写。
	timeTicker int

	// chunkSweepBusy 后台卸载区块正在跑，防止上一次没扫完又开一次。零值可用。
	chunkSweepBusy atomic.Bool

	// chunkJobs / chunkDone 是区块包 worker 池的任务和结果通道，Start 里建好之后不再变。
	// 字面量构造的 Server 这两个是 nil，区块包在 tick 协程里同步准备。
	chunkJobs chan chunkJob
	chunkDone chan chunkResult

	// worldSaveBusy 后台自动存盘正在跑，防止上一次没存完又开一次。零值可用。
	worldSaveBusy atomic.Bool

	// 合成配方包全服共用一份，首次需要时编码。
	craftOnce sync.Once
	craftRaw  []byte
	craftErr  error
}

func New(cfg *config.Config) *Server {
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
	}
	if cfg.AutosaveSeconds <= 0 {
		cfg.AutosaveSeconds = 30
	}
	if cfg.PlayerSaveSeconds <= 0 {
		cfg.PlayerSaveSeconds = 600
	}
	defaultName := cfg.WorldName
	if defaultName == "" {
		defaultName = "world"
	}
	worlds := make(map[string]*world.World)
	openWorld := func(def storage.WorldDefaults) *world.World {
		nw, err := storage.OpenWorld(cfg.DataDir, def)
		if err != nil {
			logger.Fatal("server", "打开世界 %s 失败: %v", def.Name, err)
		}
		worlds[nw.Name] = nw
		return nw
	}
	w := openWorld(storage.WorldDefaults{
		Name:      defaultName,
		Seed:      int32(cfg.LevelSeed),
		SpawnX:    int32(cfg.SpawnX),
		SpawnY:    int32(cfg.SpawnY),
		SpawnZ:    int32(cfg.SpawnZ),
		Generator: world.GeneratorFlat,
	})
	openTemplate := func(name, gen string) {
		openWorld(storage.WorldDefaults{
			Name:      name,
			Seed:      12345,
			SpawnX:    8,
			SpawnY:    64,
			SpawnZ:    8,
			Generator: gen,
		})
	}
	openTemplate("sky", world.GeneratorSky)
	openTemplate("ocean", world.GeneratorOcean)
	openTemplate("nether", world.GeneratorNether)
	openTemplate("end", world.GeneratorEnd)
	srv := &Server{
		cfg:           cfg,
		world:         w,
		worlds:        worlds,
		players:       make(map[string]*Player),
		entities:      make(map[int64]*entity.Instance),
		stopCh:        make(chan struct{}),
		stopRequested: make(chan struct{}),
		cmds:          command.NewMap(),
		access:        openAccess(cfg.ServerDir),
	}
	srv.whitelistOn.Store(cfg.Whitelist)
	srv.cmds.Register(command.GiveCommand{})
	srv.cmds.Register(command.SummonCommand{})
	srv.cmds.Register(command.GamemodeCommand{})
	srv.cmds.Register(command.WorldCommand{})
	srv.cmds.Register(command.CoordCommand{})
	var adminCommands []command.Command = command.AdminCommands()
	var adminIndex int = 0
	for adminIndex = 0; adminIndex < len(adminCommands); adminIndex++ {
		srv.cmds.Register(adminCommands[adminIndex])
	}
	srv.cmds.Register(command.HelpCommand{Map: srv.cmds})
	var _, craftErr = srv.craftingDataBytes()
	if craftErr != nil {
		logger.Fatal("server", "配方表加载失败: %v", craftErr)
	}
	srv.rakSrv = raknet.NewServer(cfg, srv.onNewRakSession, serverAdmin{srv: srv})
	for _, nw := range worlds {
		list, err := storage.LoadDroppedItems(cfg.DataDir, nw.Name)
		if err != nil {
			logger.Error("server", "读取世界 %s 掉落物失败: %v", nw.Name, err)
			continue
		}
		srv.restoreDroppedItems(nw.Name, list)
	}
	return srv
}

func (srv *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", srv.cfg.Host, srv.cfg.Port)
	logger.Info("server", "正在监听 %s (协议 %d / %s) 数据目录=%s 世界=%s",
		addr, protocol.CurrentProtocol, protocol.MinecraftVersionNetwork, srv.cfg.DataDir, srv.world.Name)
	if err := srv.rakSrv.Listen(addr); err != nil {
		return err
	}
	srv.startChunkWorkers()
	logger.Go("server", srv.tickLoop)
	return nil
}

func (srv *Server) tickLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	worldAuto := time.NewTicker(time.Duration(srv.cfg.AutosaveSeconds) * time.Second)
	defer worldAuto.Stop()
	playerAuto := time.NewTicker(time.Duration(srv.cfg.PlayerSaveSeconds) * time.Second)
	defer playerAuto.Stop()
	sweepAuto := time.NewTicker(chunkSweepInterval)
	defer sweepAuto.Stop()
	var coordsTick int = 0
	for {
		select {
		case <-srv.stopCh:
			return
		case <-ticker.C:
			srv.tickDroppedItems()
			srv.tickScheduled()
			srv.tickFallingBlocks()
			srv.tickRandom()
			srv.tickFurnaces()
			srv.tickHoppers()
			srv.tickDayNight()
			srv.tickChunkSend()
			srv.tickLife()
			coordsTick++
			if coordsTick >= coordsRefreshTicks {
				coordsTick = 0
				srv.tickCoords()
			}
		case <-worldAuto.C:
			srv.startWorldAutosave()
		case <-playerAuto.C:
			srv.savePlayers("player-autosave")
		case <-sweepAuto.C:
			srv.startChunkSweep()
		}
	}
}

// RequestStop 请求关服（/stop 命令用）。只发信号不直接停：命令是在网络协程里执行的，
// 在那里同步 Stop 会在踢人时等自己断开，所以交给 main 去走完整的关服流程。
func (srv *Server) RequestStop() {
	srv.stopRequestOnce.Do(func() {
		close(srv.stopRequested)
	})
}

// StopRequested 在有人要求关服后可读。
func (srv *Server) StopRequested() <-chan struct{} {
	return srv.stopRequested
}

// shutdownDrainTimeout 是关服时等玩家断开的上限。
const shutdownDrainTimeout time.Duration = 2 * time.Second

// Stop 先存档，再踢人（带「服务器已关闭」提示），等玩家断开后才关监听。
// 直接关监听的话客户端只会看到连接超时，也来不及收到断开原因。
func (srv *Server) Stop() {
	srv.stopOnce.Do(func() {
		close(srv.stopCh)
		srv.saveAll("shutdown")
		srv.kickAll("服务器已关闭")
		srv.waitForPlayersToLeave(shutdownDrainTimeout)
		if srv.rakSrv != nil {
			var err error = srv.rakSrv.Close()
			if err != nil {
				logger.Error("server", "关闭网络监听失败: %v", err)
			}
		}
	})
}

func (srv *Server) kickAll(message string) {
	var list []*Player = srv.playerList()
	var index int = 0
	for index = 0; index < len(list); index++ {
		srv.kickPlayer(list[index], message)
	}
}

// waitForPlayersToLeave 轮询到所有玩家都断开或超时，给断开通知留出发送时间。
func (srv *Server) waitForPlayersToLeave(timeout time.Duration) {
	var deadline time.Time = time.Now().Add(timeout)
	for srv.PlayerCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
}

func (srv *Server) saveAll(reason string) {
	srv.savePlayers(reason)
	srv.saveWorld(reason)
}

func (srv *Server) savePlayers(reason string) {
	srv.saveMu.Lock()
	defer srv.saveMu.Unlock()
	srv.mu.RLock()
	list := make([]*Player, 0, len(srv.players))
	for _, p := range srv.players {
		list = append(list, p)
	}
	worldName := srv.world.Name
	srv.mu.RUnlock()
	for _, p := range list {
		if err := storage.SavePlayer(srv.cfg.DataDir, p.Snapshot(worldName)); err != nil {
			logger.Error("server", "保存玩家 %s 失败: %v", p.Username, err)
		}
	}
	logger.Info("server", "已保存玩家 (%s) n=%d", reason, len(list))
}

func (srv *Server) saveWorld(reason string) {
	srv.saveMu.Lock()
	defer srv.saveMu.Unlock()
	for _, w := range srv.worlds {
		if err := srv.persistWorld(w); err != nil {
			logger.Error("server", "保存世界 %s 失败 (%s): %v", w.Name, reason, err)
			return
		}
	}
	logger.Info("server", "已保存世界 (%s) n=%d", reason, len(srv.worlds))
}

// startWorldAutosave 在后台存一次世界：压缩区块和写文件不能占着 tick 协程。
// 上一次还没存完就不再开新的，返回 false。关服时的最后一次存档走 saveWorld，会等这里放开 saveMu。
func (srv *Server) startWorldAutosave() bool {
	if !srv.worldSaveBusy.CompareAndSwap(false, true) {
		return false
	}
	logger.Go("server", func() {
		defer srv.worldSaveBusy.Store(false)
		srv.saveWorld("autosave")
	})
	return true
}

func (srv *Server) saveOneWorld(name, reason string) {
	w := srv.worldByName(name)
	if w == nil {
		return
	}
	srv.saveMu.Lock()
	defer srv.saveMu.Unlock()
	if err := srv.persistWorld(w); err != nil {
		logger.Error("server", "保存世界 %s 失败 (%s): %v", w.Name, reason, err)
		return
	}
	logger.Info("server", "已保存世界 %s (%s)", w.Name, reason)
}

func (srv *Server) persistWorld(w *world.World) error {
	// 掉落物进入漏斗前后只能出现在一份快照里。锁只覆盖内存读取，不覆盖压缩和写盘。
	srv.blockMu.Lock()
	var tiles = w.SnapshotTiles()
	var drops = srv.droppedSnapshot(w.Name)
	srv.blockMu.Unlock()
	if err := storage.SaveWorldWithTiles(srv.cfg.DataDir, w, tiles); err != nil {
		return err
	}
	return storage.SaveDroppedItems(srv.cfg.DataDir, w.Name, drops)
}

func (srv *Server) droppedSnapshot(worldName string) []storage.DroppedItemData {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	out := make([]storage.DroppedItemData, 0)
	for _, e := range srv.entities {
		if !e.IsDroppedItem() {
			continue
		}
		if !sameWorldName(e.WorldName, worldName, srv.world.Name) {
			continue
		}
		out = append(out, storage.DroppedItemData{
			ID:          e.Item.ID,
			Meta:        e.Item.Meta,
			Count:       e.Item.Count,
			NBT:         e.Item.NBT,
			X:           e.X,
			Y:           e.Y,
			Z:           e.Z,
			SpeedX:      e.SpeedX,
			SpeedY:      e.SpeedY,
			SpeedZ:      e.SpeedZ,
			PickupDelay: e.PickupDelay,
			Age:         e.Age,
		})
	}
	return out
}

func (srv *Server) restoreDroppedItems(worldName string, list []storage.DroppedItemData) {
	t, ok := entity.ByID(entity.NetworkIDItem)
	if !ok {
		return
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, d := range list {
		item := storage.ItemFromDrop(d)
		if item.IsAir() {
			continue
		}
		id := atomic.AddInt64(&srv.entityIDCounter, 1)
		srv.entities[id] = &entity.Instance{
			RuntimeID:   id,
			Type:        t,
			X:           d.X,
			Y:           d.Y,
			Z:           d.Z,
			SpeedX:      d.SpeedX,
			SpeedY:      d.SpeedY,
			SpeedZ:      d.SpeedZ,
			Item:        item,
			PickupDelay: d.PickupDelay,
			Age:         d.Age,
			WorldName:   worldName,
		}
	}
}

func (srv *Server) onNewRakSession(rakSess *raknet.Session) {
	logger.Info("server", "新 RakNet 会话 %s", rakSess)
	sess := network.NewSession(rakSess)
	sess.MaxChunkRadius = int32(srv.cfg.ViewDistance)
	sess.OnLogin = func(s *network.Session) { srv.handleSessionLogin(s) }
	sess.OnChunkRadius = func(s *network.Session, radius int32) { srv.sendChunksAndSpawn(s, radius) }
	sess.OnChat = func(s *network.Session, msg string) { srv.handleChat(s, msg) }
	sess.OnMove = func(s *network.Session, x, y, z, yaw, pitch float32, onGround bool) {
		srv.handlePlayerMove(s, x, y, z, yaw, pitch, onGround)
	}
	sess.OnDisconnect = func(s *network.Session) {
		srv.removePlayer(s.ClientUUID)
		logger.Info("server", "%s 断开连接", s.Username)
	}
	sess.OnUseItem = func(s *network.Session, pkt *packets.UseItemPacket) { srv.handleUseItem(s, pkt) }
	sess.OnRemoveBlock = func(s *network.Session, pkt *packets.RemoveBlockPacket) { srv.handleRemoveBlock(s, pkt) }
	sess.OnItemFrameDrop = srv.handleItemFrameDrop
	sess.OnMobEquipment = func(s *network.Session, pkt *packets.MobEquipmentPacket) { srv.handleMobEquipment(s, pkt) }
	sess.OnPlayerAction = func(s *network.Session, pkt *packets.PlayerActionPacket) {
		srv.handlePlayerAction(s, pkt)
	}
	sess.OnContainerSetSlot = func(s *network.Session, pkt *packets.ContainerSetSlotPacket) {
		srv.handleContainerSetSlot(s, pkt)
	}
	sess.OnContainerClose = func(s *network.Session, pkt *packets.ContainerClosePacket) {
		srv.handleContainerClose(s, pkt)
	}
	sess.OnCommandStep = func(s *network.Session, line string) {
		srv.dispatchCommand(s, line)
	}
	sess.OnDropItem = func(s *network.Session, pkt *packets.DropItemPacket) {
		srv.handleDropItem(s, pkt)
	}
	sess.OnSetPlayerGameType = func(s *network.Session, pkt *packets.SetPlayerGameTypePacket) {
		srv.handleSetPlayerGameType(s, pkt)
	}
	sess.OnAnimate = func(s *network.Session, pkt *packets.AnimatePacket) { srv.handleAnimate(s, pkt) }
	sess.OnInteract = func(session *network.Session, pkt *packets.InteractPacket) { srv.handleInteract(session, pkt) }
	sess.OnEntityFall = func(session *network.Session, pkt *packets.EntityFallPacket) {
		srv.handleEntityFall(session, pkt)
	}
	sess.OnEntityEvent = func(session *network.Session, pkt *packets.EntityEventPacket) {
		srv.handleEntityEvent(session, pkt)
	}
	sess.OnCraftingEvent = func(session *network.Session, pkt *packets.CraftingEventPacket) {
		srv.handleCraftingEvent(session, pkt)
	}
	sess.OnBlockEntityData = func(session *network.Session, pkt *packets.BlockEntityDataPacket) {
		srv.handleBlockEntityData(session, pkt)
	}
}

func (srv *Server) handleSessionLogin(sess *network.Session) {
	switch sess.State() {
	case network.StateLogin:
		logger.Info("server", "登录 %s uuid=%s", sess.Username, sess.ClientUUID)
		// 封禁和白名单先于人数上限检查：被封的人不该靠「服务器已满」的提示去探测人数。
		// 只发 Disconnect，不先发 PlayStatus 失败包：后者会让客户端弹出通用的
		// 「无法连接」，盖掉下面这条带原因的提示。
		var denyReason string = srv.loginDenyReason(sess.Username, sess.RemoteIP(), time.Now())
		if denyReason != "" {
			logger.Info("server", "拒绝 %s (%s): %s", sess.Username, sess.RemoteIP(), denyReason)
			sess.Disconnect(denyReason)
			return
		}
		// 人数上限在这里拦：这是唯一还没给客户端发任何游戏状态、
		// 且玩家还没进 srv.players 的时刻，拒绝掉最干净。
		if !srv.acceptNewPlayer() {
			logger.Info("server", "拒绝 %s: 服务器已满 (%d/%d)", sess.Username, srv.PlayerCount(), srv.cfg.MaxPlayers)
			sess.SendPacket(&packets.PlayStatusPacket{Status: protocol.PlayStatusLoginFailedServer})
			sess.Disconnect("服务器已满")
			return
		}
		sess.SendLoginSequence()
	case network.StateSpawning:
		srv.spawnPlayer(sess)
	}
}

// acceptNewPlayer 报告还能不能再进人。max-players <=0 视为不限制。
// 已经在线的玩家重连时不占新坑位，所以这里按 uuid 排重。
func (srv *Server) acceptNewPlayer() bool {
	if srv.cfg.MaxPlayers <= 0 {
		return true
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	return len(srv.players) < srv.cfg.MaxPlayers
}

func (srv *Server) spawnPlayer(sess *network.Session) {
	entityID := atomic.AddInt64(&srv.entityIDCounter, 1)
	sess.EntityRuntimeID = entityID

	w := srv.world
	p := newPlayer(sess, entityID)
	p.RemoteIP = sess.RemoteIP()
	p.WorldName = w.Name
	p.Dimension = w.ProtocolDimension()
	p.X = float32(w.SpawnX) + 0.5
	p.Y = float32(w.SpawnY) + 1
	p.Z = float32(w.SpawnZ) + 0.5
	data, ok, err := storage.LoadPlayer(srv.cfg.DataDir, sess.ClientUUID)
	if err != nil {
		logger.Error("server", "读取玩家存档 %s 失败: %v", sess.Username, err)
	} else if ok {
		p.ApplySave(data)
		if p.WorldName == "" {
			p.WorldName = w.Name
		}
		logger.Info("server", "%s 读取存档 pos=(%.1f,%.1f,%.1f)", sess.Username, p.X, p.Y, p.Z)
	} else {
		p.GameMode = clampPlayableGameMode(int32(srv.cfg.GameMode))
		if err := storage.SavePlayer(srv.cfg.DataDir, p.Snapshot(w.Name)); err != nil {
			logger.Error("server", "写入新玩家存档失败: %v", err)
		}
	}
	w = srv.worldByName(p.WorldName)
	if w == nil {
		w = srv.world
		p.WorldName = w.Name
	}
	p.Dimension = w.ProtocolDimension()
	srv.mu.Lock()
	// 客户端重连后鼠标和合成格为空，先将存档中的暂存物品还给玩家。
	srv.players[sess.ClientUUID] = p
	srv.mu.Unlock()
	srv.returnFloating(p)
	sess.SendPacket(&packets.StartGamePacket{
		EntityUniqueID:          entityID,
		EntityRuntimeID:         entityID,
		PlayerGamemode:          p.GameMode,
		X:                       p.X,
		Y:                       p.Y,
		Z:                       p.Z,
		Seed:                    w.Seed,
		Dimension:               w.ProtocolDimension(),
		Generator:               startGameGenerator(w),
		WorldGamemode:           int32(srv.cfg.GameMode),
		Difficulty:              int32(srv.cfg.Difficulty),
		SpawnX:                  w.SpawnX,
		SpawnY:                  w.SpawnY,
		SpawnZ:                  w.SpawnZ,
		HasAchievementsDisabled: true,
		DayCycleStopTime:        -1,
		EduMode:                 false,
		CommandsEnabled:         true,
		IsTexturePacksRequired:  false,
		LevelID:                 "",
		WorldName:               w.Name,
	})
	sess.SendPacket(&packets.SetSpawnPositionPacket{X: w.SpawnX, Y: w.SpawnY, Z: w.SpawnZ})
	srv.sendWorldTimeTo(sess, w)
	srv.sendVitals(p)
	p.syncEffects(packets.MobEffectAdd)
	srv.sendGameModeState(p)
	srv.sendPlayerListOnJoin(p)
	srv.sendCraftingData(sess)

	logger.Info("server", "%s 初始化包已发送，等待 RequestChunkRadius", sess.Username)
}

func (srv *Server) sendChunksAndSpawn(sess *network.Session, radius int32) {
	already := sess.State() == network.StatePlaying
	srv.mu.Lock()
	p := srv.players[sess.ClientUUID]
	srv.mu.Unlock()
	if p == nil {
		return
	}
	p.mu.Lock()
	p.ViewRadius = radius
	if p.SentChunks == nil {
		p.SentChunks = make(map[string]struct{})
	}
	p.mu.Unlock()
	if already {
		srv.enqueueAroundMove(p)
		return
	}
	p.mu.RLock()
	waiting := p.NeedSpawnStatus
	p.mu.RUnlock()
	if waiting {
		srv.enqueueAroundMove(p)
		return
	}

	srv.enqueuePlayerView(p)
	px, py, pz := p.Position()
	sess.SendPacket(&packets.MovePlayerPacket{
		EntityRuntimeID: sess.EntityRuntimeID,
		X:               px,
		Y:               py,
		Z:               pz,
		Mode:            protocol.MovePlayerModeReset,
		OnGround:        true,
	})
	srv.sendPlayerInventory(p)
	srv.sendOwnArmor(p)
	srv.sendCreativeInventory(p)
	sess.SendPacket(&packets.AvailableCommandsPacket{
		CommandsJSON: srv.cmds.AvailableCommandsJSON(srv.isOp(sess.Username)),
	})
	logger.Info("server", "%s 等待区块预加载 pos=(%.1f,%.1f,%.1f) radius=%d", sess.Username, px, py, pz, radius)
}

func (srv *Server) handlePlayerMove(sess *network.Session, x, y, z, yaw, pitch float32, onGround bool) {
	srv.mu.RLock()
	p, ok := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if !ok {
		return
	}
	if sess.State() != network.StatePlaying {
		p.UpdatePosition(x, y, z, yaw, pitch, onGround)
		return
	}
	// 位置校验只在游戏中的玩家身上做；判定为非法就拉回上次合法位置，
	// 不更新服务端坐标，也不转发给其他玩家，客户端那边看到的是被弹回去。
	var now time.Time = time.Now()
	var reason string = srv.moveRejectReason(p, x, y, z, now)
	if reason != "" {
		srv.pullBack(p, reason)
		return
	}
	// 位移要在更新坐标之前取，更新后就只剩新位置了。
	var prevX, prevZ = p.horizontalPosition()
	p.UpdatePosition(x, y, z, yaw, pitch, onGround)
	p.RecordMovement(x, y, z, now)
	srv.exhaustFromMove(p, prevX, prevZ, x, z)
	srv.broadcastPlayerMove(p)
	srv.enqueueAroundMove(p)
	srv.tryPickupFor(p)
}

func (srv *Server) handleUseItem(sess *network.Session, pkt *packets.UseItemPacket) {
	if pkt.Face < 0 || pkt.Face > 5 {
		logger.Debug("server", "%s UseItem face=%d 忽略 (空气中使用)", sess.Username, pkt.Face)
		return
	}
	srv.mu.RLock()
	var player *Player = srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if player == nil || sess.State() != network.StatePlaying || player.IsDead() {
		return
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var posX, posY, posZ = player.Position()

	// 点击的方块本身要在交互距离内，否则隔着半张地图也能开箱子。
	var clickedPos blockPos = blockPos{PosX: pkt.X, PosY: pkt.Y, PosZ: pkt.Z}
	if !blockInReach(posX, posY, posZ, clickedPos, maxInteractDistance) {
		logger.Debug("server", "%s 交互被拒: 目标方块距离过远 (%d,%d,%d)", sess.Username, pkt.X, pkt.Y, pkt.Z)
		srv.notifyBlocked(player, "§c目标方块太远，够不着")
		return
	}
	var clicked byte = blockIDAt(blockWorld, pkt.X, pkt.Y, pkt.Z)
	if isContainerBlock(clicked) {
		srv.openContainer(player, pkt.X, pkt.Y, pkt.Z)
		return
	}
	if clicked == world.BlockCraftingTable {
		srv.openWorkbench(player, pkt.X, pkt.Y, pkt.Z)
		return
	}

	// 门、拉杆、按钮这类方块先吃掉点击；潜行时不吃，留给下面的放置流程。
	if srv.interactBlock(player, pkt) {
		return
	}
	// 花盆、物品展示框：种东西进去、放东西进去、转一格。
	if srv.interactDecor(player, pkt) {
		return
	}
	// 手持物一律以服务端记录为准，不信客户端包里的物品栏槽位。
	var held protocol.Item = player.heldSnapshot()
	if held.IsAir() {
		logger.Debug("server", "%s 放置忽略: 空手", sess.Username)
		return
	}
	// 锄头、骨粉这类用在方块上的物品，先于放置处理。
	if srv.useItemOnBlock(player, pkt, held) {
		return
	}
	// 其余工具、食物这类不是放方块用的物品直接忽略；种子、门这类要查表的交给放置规则。
	var _, isBlockItem = blockrule.BlockForItem(held.ID)
	if !isBlockItem {
		logger.Debug("server", "%s 放置忽略: 非方块物品 id=%d", sess.Username, held.ID)
		return
	}
	// 客户端报的物品必须和服务端手里的对得上，否则就是自己改了包。
	var mismatchReason string = heldMismatchReason(held, pkt.Item)
	if mismatchReason != "" {
		logger.Debug(
			"server",
			"%s 放置被拒: 客户端物品 %d:%d 与服务端 %d:%d 不符 (%s)",
			sess.Username,
			pkt.Item.ID,
			pkt.Item.Meta,
			held.ID,
			held.Meta,
			mismatchReason,
		)
		return
	}
	srv.placeHeldBlock(player, pkt, held)
}

func (srv *Server) placeDistance(player *Player) float32 {
	if player.gameMode() == int32(protocol.GameModeCreative) {
		return maxPlaceDistanceCreative
	}
	return maxPlaceDistanceSurvival
}

// heldMismatchReason 比对客户端上报的物品和服务端手里的方块，不符就返回原因。
// 客户端空手时报空物品是正常的（它只是省略字段），这种情况放行。
func heldMismatchReason(held protocol.Item, reported protocol.Item) string {
	if reported.IsAir() {
		return ""
	}
	if held.ID != reported.ID {
		return "id 不同"
	}
	return ""
}

// spawnProtected 判断这个方块是否落在出生点保护半径内，创造模式和旁观不受限制。
func (srv *Server) spawnProtected(player *Player, blockX, blockZ int32) bool {
	if player.gameMode() == int32(protocol.GameModeCreative) {
		return false
	}
	var radius int32 = int32(srv.cfg.SpawnProtection)
	if radius <= 0 {
		return false
	}
	var blockWorld *world.World = srv.playerWorld(player)
	var spawnX int32 = blockWorld.SpawnX
	var spawnZ int32 = blockWorld.SpawnZ
	return insideSpawnProtection(spawnX, spawnZ, radius, blockX, blockZ)
}

func (srv *Server) broadcastLevelEvent(worldName string, evid int32, x, y, z float32, data int32) {
	srv.broadcastInWorld(worldName, &packets.LevelEventPacket{
		EventID: evid,
		X:       x, Y: y, Z: z,
		Data: data,
	}, "")
}

func (srv *Server) handleMobEquipment(sess *network.Session, pkt *packets.MobEquipmentPacket) {
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if p == nil || sess.State() != network.StatePlaying || pkt.WindowID != 0 {
		return
	}
	if !p.selectHotbar(pkt.HotbarSlot, pkt.InventorySlot) {
		srv.sendPlayerInventory(p)
		return
	}
	srv.broadcastHeldItem(p)
}

func (srv *Server) handleContainerSetSlot(sess *network.Session, pkt *packets.ContainerSetSlotPacket) {
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if p == nil {
		return
	}
	if pkt.WindowID == protocol.ContainerArmor {
		srv.handleArmorSlot(p, pkt)
		return
	}
	if pkt.WindowID == 0 {
		if pkt.Item.Count > item.StackLimit(pkt.Item.ID) {
			srv.sendPlayerInventory(p)
			return
		}
		slot := protocol.InventorySlotFromWindow(pkt.Slot)
		if slot < 0 {
			return
		}
		p.mu.Lock()
		var valid = slot < len(p.Inventory) && p.exchangeSlotLocked(p.Inventory[slot], pkt.Item)
		if valid {
			p.Inventory[slot] = pkt.Item
			if slot == p.heldInventorySlotLocked() {
				p.HeldItem = pkt.Item
			}
		}
		p.mu.Unlock()
		if !valid {
			srv.sendPlayerInventory(p)
		}
		return
	}
	srv.handleContainerWindowSlot(p, pkt)
}

func (srv *Server) sendPlayerInventory(p *Player) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.RLock()
	inv := append([]protocol.Item(nil), p.Inventory...)
	mapping := p.hotbarSnapshotLocked()
	equipment := p.equipmentLocked()
	eid := p.EntityRuntimeID
	sess := p.Session
	p.mu.RUnlock()
	sess.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.ContainerInventory,
		TargetEID: eid,
		Slots:     protocol.NetworkPlayerInventory(inv),
		Hotbar:    mapping,
	})
	sess.SendPacket(equipment)
}

func (srv *Server) sendInventorySlot(p *Player, slot int, item protocol.Item) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.RLock()
	held := p.HeldSlot
	sess := p.Session
	p.mu.RUnlock()
	sess.SendPacket(&packets.ContainerSetSlotPacket{
		WindowID:   0,
		Slot:       int32(slot),
		HotbarSlot: int32(held),
		Item:       item,
		SelectSlot: held,
	})
}

func (srv *Server) broadcastBlock(worldName string, x, y, z int32, id, data uint32) {
	pkt := &packets.UpdateBlockPacket{
		X: x, Y: y, Z: z,
		BlockID:   id,
		BlockData: data,
		Flags:     protocol.UpdateBlockFlagAllPriority,
	}
	srv.broadcastInWorld(worldName, pkt, "")
}

func (srv *Server) removePlayer(uuid string) {
	srv.mu.Lock()
	p, ok := srv.players[uuid]
	delete(srv.players, uuid)
	srv.mu.Unlock()
	if !ok {
		return
	}
	srv.hidePlayerInWorld(p, false)
	p.mu.RLock()
	var announced bool = p.JoinAnnounced
	p.mu.RUnlock()
	if announced {
		srv.broadcastJoinLeave(msgPlayerLeft, p.Username)
		logger.Info("server", "%s 离开了游戏", p.Username)
	}
	if err := storage.SavePlayer(srv.cfg.DataDir, p.Snapshot(srv.playerWorldName(p))); err != nil {
		logger.Error("server", "保存玩家 %s 失败: %v", p.Username, err)
	}
	srv.saveOneWorld(srv.playerWorldName(p), "disconnect")
	srv.broadcast(&packets.PlayerListPacket{
		Action:  protocol.PlayerListRemove,
		Entries: []packets.PlayerListEntry{{UUID: uuidToBytes(uuid)}},
	}, p.UUID)
}

func (srv *Server) handleChat(sess *network.Session, msg string) {
	if command.IsCommand(msg) {
		srv.dispatchCommand(sess, command.StripPrefix(msg))
		return
	}
	srv.broadcastChat(sess.Username, msg)
}

func (srv *Server) dispatchCommand(sess *network.Session, line string) {
	logger.Info("command", "%s 执行 /%s", sess.Username, line)
	ctx := &command.Context{
		Username: sess.Username,
		Reply: func(msg string) {
			srv.sendRaw(sess, msg)
		},
		FindPlayer: func(name string) (command.InventoryPlayer, bool) {
			return srv.findCommandPlayer(sess, name)
		},
		Position: func() (x, y, z float32, ok bool) {
			srv.mu.RLock()
			p := srv.players[sess.ClientUUID]
			srv.mu.RUnlock()
			if p == nil {
				return 0, 0, 0, false
			}
			x, y, z = p.Position()
			return x, y, z, true
		},
		SpawnEntity: func(networkID int32, x, y, z float32) error {
			srv.mu.RLock()
			p := srv.players[sess.ClientUUID]
			srv.mu.RUnlock()
			return srv.spawnEntity(networkID, x, y, z, srv.playerWorldName(p))
		},
		DefaultWorld: srv.world.Name,
		IsOp:         srv.isOp(sess.Username),
		World:        srv.playerWorldName(srv.playerBySession(sess)),
		Admin:        serverAdmin{srv: srv},
	}
	srv.cmds.Dispatch(ctx, line)
}

func (srv *Server) findCommandPlayer(self *network.Session, name string) (command.InventoryPlayer, bool) {
	if strings.EqualFold(name, "@s") || strings.EqualFold(name, self.Username) {
		srv.mu.RLock()
		p := srv.players[self.ClientUUID]
		srv.mu.RUnlock()
		if p == nil {
			return nil, false
		}
		return &commandPlayer{srv: srv, p: p}, true
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, p := range srv.players {
		if strings.EqualFold(p.Username, name) {
			return &commandPlayer{srv: srv, p: p}, true
		}
	}
	return nil, false
}

func (srv *Server) sendRaw(sess *network.Session, msg string) {
	if sess == nil {
		return
	}
	sess.SendPacket(&packets.TextPacket{
		Type:    protocol.TextTypeRaw,
		Message: msg,
	})
}

func (srv *Server) spawnEntity(networkID int32, x, y, z float32, worldName string) error {
	t, ok := entity.ByID(networkID)
	if !ok {
		return fmt.Errorf("未知实体: %d", networkID)
	}
	if !t.CanSummon {
		return fmt.Errorf("本轮不能召唤 %s", t.Name)
	}
	if worldName == "" {
		worldName = srv.world.Name
	}
	id := atomic.AddInt64(&srv.entityIDCounter, 1)
	inst := &entity.Instance{
		RuntimeID: id,
		Type:      t,
		X:         x,
		Y:         y,
		Z:         z,
		WorldName: worldName,
	}
	srv.mu.Lock()
	srv.entities[id] = inst
	srv.mu.Unlock()
	srv.broadcastInWorld(worldName, addEntityPacket(inst), "")
	logger.Info("entity", "召唤 %s id=%d 于 (%.1f,%.1f,%.1f) world=%s", t.Name, id, x, y, z, worldName)
	return nil
}

func (srv *Server) sendEntitiesTo(sess *network.Session) {
	if sess == nil {
		return
	}
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	srv.sendWorldEntities(sess, srv.playerWorldName(p))
}

func addEntityPacket(e *entity.Instance) *packets.AddEntityPacket {
	return &packets.AddEntityPacket{
		EntityUniqueID:  e.RuntimeID,
		EntityRuntimeID: e.RuntimeID,
		EntityType:      uint32(e.Type.NetworkID),
		X:               e.X,
		Y:               e.Y,
		Z:               e.Z,
		Yaw:             e.Yaw,
		Pitch:           e.Pitch,
		Metadata:        entityMetadata(e),
	}
}

func addItemEntityPacket(e *entity.Instance) *packets.AddItemEntityPacket {
	return &packets.AddItemEntityPacket{
		EntityUniqueID:  e.RuntimeID,
		EntityRuntimeID: e.RuntimeID,
		Item:            e.Item,
		X:               e.X,
		Y:               e.Y,
		Z:               e.Z,
		SpeedX:          e.SpeedX,
		SpeedY:          e.SpeedY,
		SpeedZ:          e.SpeedZ,
	}
}

func entitySpawnPacket(e *entity.Instance) packets.Packet {
	if e.IsDroppedItem() {
		return addItemEntityPacket(e)
	}
	return addEntityPacket(e)
}

func (srv *Server) handleDropItem(sess *network.Session, pkt *packets.DropItemPacket) {
	item := pkt.Item
	if item.IsAir() {
		return
	}
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if p == nil || sess.State() != network.StatePlaying {
		return
	}
	if !p.payForDrop(item) {
		srv.sendPlayerInventory(p)
		return
	}
	srv.sendPlayerInventory(p)
	x, y, z := p.Position()
	yaw, pitch := p.Look()
	sx, sy, sz := dropMotion(yaw, pitch)
	ox, oy, oz := dropOrigin(x, y, z)
	srv.spawnDroppedItem(item, ox, oy, oz, sx, sy, sz, entity.PickupDelayTicks, srv.playerWorldName(p))
	logger.Info("server", "%s 丢弃 %d×%d", sess.Username, item.ID, item.Count)
}

func dropMotion(yaw, pitch float32) (x, y, z float32) {
	// PocketMine Entity::getDirectionVector()->multiply(0.4)
	yawR := float64(yaw) * math.Pi / 180
	pitchR := float64(pitch) * math.Pi / 180
	xz := math.Cos(pitchR)
	x = float32(-xz * math.Sin(yawR) * 0.4)
	y = float32(-math.Sin(pitchR) * 0.4)
	z = float32(xz * math.Cos(yawR) * 0.4)
	return
}

// dropOrigin matches PocketMine Player::dropItem → add(0, 1.3, 0) at the player's body.
func dropOrigin(x, y, z float32) (ox, oy, oz float32) {
	return x, y + 1.3, z
}

func (srv *Server) spawnDroppedItem(item protocol.Item, x, y, z, sx, sy, sz float32, delay int, worldName string) {
	t, ok := entity.ByID(entity.NetworkIDItem)
	if !ok || item.IsAir() {
		return
	}
	if worldName == "" {
		worldName = srv.world.Name
	}
	id := atomic.AddInt64(&srv.entityIDCounter, 1)
	inst := &entity.Instance{
		RuntimeID:   id,
		Type:        t,
		X:           x,
		Y:           y,
		Z:           z,
		SpeedX:      sx,
		SpeedY:      sy,
		SpeedZ:      sz,
		Item:        item,
		PickupDelay: delay,
		WorldName:   worldName,
	}
	srv.mu.Lock()
	srv.entities[id] = inst
	srv.mu.Unlock()
	srv.broadcastInWorld(worldName, addItemEntityPacket(inst), "")
}

type worldEntityID struct {
	id        int64
	worldName string
}

func (srv *Server) tickDroppedItems() {
	var despawn []worldEntityID
	var freeze []worldEntityID
	srv.mu.Lock()
	for id, e := range srv.entities {
		if !e.IsDroppedItem() {
			continue
		}
		e.Age++
		if e.PickupDelay > 0 {
			e.PickupDelay--
		}
		wn := e.WorldName
		entity.StepDroppedItem(e, func(x, y, z float32) bool {
			return srv.itemSolidIn(wn, x, y, z)
		})
		if e.Y < -16 || e.Age > entity.ItemDespawnTicks {
			despawn = append(despawn, worldEntityID{id: id, worldName: wn})
			delete(srv.entities, id)
			continue
		}
		// 不发 MoveEntity：位置瞬移会和客户端抛物线打架。
		// 等抛出动画走完，只清零速度并标记 immobile，停住弹跳。
		if !e.ClientFrozen && e.Age >= entity.ItemClientFreezeTicks {
			e.ClientFrozen = true
			freeze = append(freeze, worldEntityID{id: e.RuntimeID, worldName: wn})
		}
	}
	srv.mu.Unlock()
	for _, d := range despawn {
		srv.broadcastInWorld(d.worldName, &packets.RemoveEntityPacket{EntityUniqueID: d.id}, "")
	}
	for _, f := range freeze {
		srv.broadcastInWorld(f.worldName, &packets.SetEntityMotionPacket{EntityRuntimeID: f.id}, "")
		srv.broadcastInWorld(f.worldName, &packets.SetEntityDataPacket{
			EntityRuntimeID: f.id,
			Flags:           packets.EntityFlagImmobile,
			SendFlags:       true,
		}, "")
	}

	srv.mu.RLock()
	players := make([]*Player, 0, len(srv.players))
	for _, p := range srv.players {
		players = append(players, p)
	}
	srv.mu.RUnlock()
	for _, p := range players {
		if p.Session != nil && p.Session.State() == network.StatePlaying {
			srv.tryPickupFor(p)
		}
	}
}

func (srv *Server) itemSolidIn(worldName string, x, y, z float32) bool {
	by := int32(math.Floor(float64(y)))
	if by < 0 || by > 255 {
		return false
	}
	w := srv.worldByName(worldName)
	// 只看已经在内存里的区块，不能因为一个掉落物把没人去的区块读盘或生成出来（那样卸载就白做了）。
	// 区块没加载就当实心：东西停在原地等区块回来，不会掉进虚空。
	var id, _, loaded = w.Peek(int32(math.Floor(float64(x))), by, int32(math.Floor(float64(z))))
	if !loaded {
		return true
	}
	switch id {
	case world.BlockAir, 8, 9, 10, 11: // air, water, lava
		return false
	default:
		return true
	}
}

func (srv *Server) tryPickupFor(p *Player) {
	if p == nil || p.Session == nil || p.Session.State() != network.StatePlaying {
		return
	}
	px, py, pz := p.Position()
	collector := p.EntityRuntimeID
	wn := srv.playerWorldName(p)
	p.mu.RLock()
	gm := p.GameMode
	p.mu.RUnlock()
	if gm == int32(protocol.GameModeSpectator) {
		return
	}
	creative := gm == int32(protocol.GameModeCreative)

	type hit struct {
		id   int64
		item protocol.Item
	}
	var hits []hit

	srv.mu.Lock()
	for id, e := range srv.entities {
		if !e.IsDroppedItem() || e.PickupDelay > 0 {
			continue
		}
		if !sameWorldName(e.WorldName, wn, srv.world.Name) {
			continue
		}
		if !entity.CanPickupItem(px, py, pz, e.X, e.Y, e.Z) {
			continue
		}
		if !creative && !p.CanAddItem(e.Item) {
			continue
		}
		hits = append(hits, hit{id: id, item: e.Item})
		delete(srv.entities, id)
	}
	srv.mu.Unlock()

	added := false
	for _, h := range hits {
		srv.broadcastInWorld(wn, &packets.TakeItemEntityPacket{
			TargetEntityID: h.id,
			CollectorEID:   collector,
		}, "")
		srv.broadcastInWorld(wn, &packets.RemoveEntityPacket{EntityUniqueID: h.id}, "")
		if !creative {
			p.AddItem(h.item)
			added = true
		}
		logger.Info("server", "%s 捡起 %d×%d", p.Username, h.item.ID, h.item.Count)
	}
	if added {
		srv.sendPlayerInventory(p)
	}
}

type commandPlayer struct {
	srv *Server
	p   *Player
}

func (c *commandPlayer) Name() string { return c.p.Username }

func (c *commandPlayer) AddItem(item protocol.Item) (protocol.Item, bool) {
	return c.p.AddItem(item)
}

func (c *commandPlayer) SyncInventory() {
	c.srv.sendPlayerInventory(c.p)
}

func (c *commandPlayer) Reply(msg string) {
	c.srv.sendRaw(c.p.Session, msg)
}

func (c *commandPlayer) SetGameMode(gm int32) bool {
	return c.srv.applyGameMode(c.p, gm)
}

func (c *commandPlayer) CoordsEnabled() bool {
	return c.p.CoordsEnabled()
}

// SetCoordsEnabled 立刻推一次，让开关当场生效；关掉时发空串清屏。
func (c *commandPlayer) SetCoordsEnabled(enabled bool) {
	c.p.SetCoordsEnabled(enabled)
	c.srv.sendCoords(c.p)
}

func clampPlayableGameMode(gm int32) int32 {
	if gm == protocol.GameModeCreative {
		return protocol.GameModeCreative
	}
	return protocol.GameModeSurvival
}

func (srv *Server) applyGameMode(p *Player, gm int32) bool {
	if p == nil || (gm != protocol.GameModeSurvival && gm != protocol.GameModeCreative) {
		return false
	}
	p.mu.Lock()
	if p.GameMode == gm {
		p.mu.Unlock()
		return false
	}
	p.GameMode = gm
	p.mu.Unlock()
	srv.sendGameModeState(p)
	return true
}

func (srv *Server) sendGameModeState(p *Player) {
	srv.sendGameModeAndSettings(p)
	srv.sendCreativeInventory(p)
}

func (srv *Server) sendGameModeAndSettings(p *Player) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.RLock()
	gm := clampPlayableGameMode(p.GameMode)
	sess := p.Session
	p.mu.RUnlock()
	sess.SendPacket(&packets.SetPlayerGameTypePacket{GameMode: gm})
	var permission uint32 = protocol.PermissionNormal
	if srv.isOp(p.Username) {
		permission = protocol.PermissionOperator
	}
	adv := &packets.AdventureSettingsPacket{
		AutoJump:       true,
		WorldBuilder:   true,
		UserPermission: permission,
	}
	if gm == protocol.GameModeCreative {
		adv.AllowFlight = true
	} else {
		adv.AllowFlight = false
		adv.IsFlying = false
	}
	sess.SendPacket(adv)
}

func (srv *Server) sendCreativeInventory(p *Player) {
	if p == nil || p.Session == nil {
		return
	}
	p.mu.RLock()
	eid := p.EntityRuntimeID
	sess := p.Session
	p.mu.RUnlock()
	sess.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  protocol.ContainerCreative,
		TargetEID: eid,
		Slots:     item.CreativeItems(),
	})
}

func (srv *Server) handleSetPlayerGameType(sess *network.Session, pkt *packets.SetPlayerGameTypePacket) {
	srv.mu.RLock()
	p := srv.players[sess.ClientUUID]
	srv.mu.RUnlock()
	if p == nil {
		return
	}
	p.mu.RLock()
	gm := p.GameMode
	p.mu.RUnlock()
	if pkt.GameMode != gm {
		srv.sendGameModeAndSettings(p)
	}
}

func (srv *Server) broadcastChat(sender, msg string) {
	logger.Info("chat", "<%s> %s", sender, msg)
	srv.broadcast(&packets.TextPacket{
		Type:    protocol.TextTypeChat,
		Source:  sender,
		Message: msg,
	}, "")
}

func (srv *Server) broadcast(p packets.Packet, skipUUID string) {
	raw, err := packets.EncodePacket(p)
	if err != nil {
		logger.Error("server", "广播编码失败 %T: %v", p, err)
		return
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, player := range srv.players {
		if skipUUID != "" && player.UUID == skipUUID {
			continue
		}
		player.Session.SendBatch(raw)
	}
}

func (srv *Server) PlayerCount() int {
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	return len(srv.players)
}

func uuidToBytes(uuid string) [16]byte {
	var b [16]byte
	hex := make([]byte, 0, 32)
	for i := 0; i < len(uuid); i++ {
		c := uuid[i]
		if c != '-' {
			hex = append(hex, c)
		}
	}
	if len(hex) == 32 {
		for i := 0; i < 16; i++ {
			b[i] = (hexVal(hex[i*2]) << 4) | hexVal(hex[i*2+1])
		}
	} else {
		copy(b[:], []byte(uuid))
	}
	return b
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
