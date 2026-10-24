package network

import (
	"fmt"

	"go-mc-server/internal/logger"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/raknet"
	"sync"
)

type SessionState int

const (
	StateConnecting SessionState = iota
	StateLogin
	StateResourcePacks
	StateSpawning
	StatePlaying
	StateDisconnected
)

type Session struct {
	rakSess *raknet.Session

	mu    sync.Mutex
	state SessionState

	Username        string
	ClientUUID      string
	XboxID          string
	ClientID        int64
	SkinID          string
	SkinData        []byte
	EntityRuntimeID int64

	OnLogin             func(s *Session)
	OnChunkRadius       func(s *Session, radius int32)
	OnChat              func(s *Session, msg string)
	OnMove              func(s *Session, x, y, z, yaw, pitch float32, onGround bool)
	OnDisconnect        func(s *Session)
	OnUseItem           func(s *Session, pkt *packets.UseItemPacket)
	OnRemoveBlock       func(s *Session, pkt *packets.RemoveBlockPacket)
	OnMobEquipment      func(s *Session, pkt *packets.MobEquipmentPacket)
	OnPlayerAction      func(s *Session, pkt *packets.PlayerActionPacket)
	OnContainerSetSlot  func(s *Session, pkt *packets.ContainerSetSlotPacket)
	OnContainerClose    func(s *Session, pkt *packets.ContainerClosePacket)
	OnCommandStep       func(s *Session, line string)
	OnDropItem          func(s *Session, pkt *packets.DropItemPacket)
	OnSetPlayerGameType func(s *Session, pkt *packets.SetPlayerGameTypePacket)
	OnAnimate           func(s *Session, pkt *packets.AnimatePacket)
	OnInteract          func(s *Session, pkt *packets.InteractPacket)
	OnEntityFall        func(s *Session, pkt *packets.EntityFallPacket)
	OnEntityEvent       func(s *Session, pkt *packets.EntityEventPacket)
	OnCraftingEvent     func(s *Session, pkt *packets.CraftingEventPacket)

	// MaxChunkRadius 是客户端视距请求的上限，来自 server.properties 的 view-distance。
	// 0 表示没配置，按 defaultMaxChunkRadius 处理。
	MaxChunkRadius int32
}

// 视距的下限和未配置时的上限。
const minChunkRadius int32 = 2
const defaultMaxChunkRadius int32 = 4

// RemoteIP 返回客户端 IP（不含端口）。没有传输层的会话（测试里的零值）返回空串。
func (s *Session) RemoteIP() string {
	if s.rakSess == nil {
		return ""
	}
	return s.rakSess.RemoteIP()
}

func NewSession(rakSess *raknet.Session) *Session {
	s := &Session{
		rakSess: rakSess,
		state:   StateConnecting,
	}
	rakSess.SetCallbacks(s.handleRawPayload, s.onRakClose)
	return s
}

func (s *Session) who() string {
	if s.Username != "" {
		return fmt.Sprintf("%s/%s", s.Username, s.rakSess)
	}
	return s.rakSess.String()
}

func (s *Session) handleRawPayload(data []byte) {
	defer logger.Recover("network")
	if len(data) == 0 {
		return
	}
	if data[0] != protocol.IDBatch {
		logger.Warn("network", "%s 非 Batch 载荷 %s len=%d hex=%s", s.who(), protocol.PacketLabel(data[0]), len(data), logger.HexDump(data, 24))
		return
	}
	inner, err := protocol.DecodeBatch(data[1:])
	if err != nil {
		logger.Error("network", "%s Batch 解码失败: %v hex=%s", s.who(), err, logger.HexDump(data, 24))
		return
	}
	for _, pkt := range inner {
		if err := s.handlePacket(pkt); err != nil {
			logger.Error("network", "%s 处理包失败: %v", s.who(), err)
		}
	}
}

func (s *Session) handlePacket(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	id := data[0]
	label := protocol.PacketLabel(id)
	pkt := packets.Lookup(id)
	if pkt == nil {
		logger.Warn("network", "%s 未知包 %s len=%d hex=%s", s.who(), label, len(data), logger.HexDump(data, 24))
		return nil
	}
	if err := pkt.Decode(protocol.NewStream(data[1:])); err != nil {
		logger.Error("network", "%s 解码失败 %s: %v hex=%s", s.who(), label, err, logger.HexDump(data, 24))
		return err
	}
	return s.dispatch(pkt)
}

func (s *Session) dispatch(pkt packets.Packet) error {
	if s.dispatchExtra(pkt) {
		return nil
	}
	switch p := pkt.(type) {
	case *packets.LoginPacket:
		return s.handleLogin(p)
	case *packets.ResourcePackClientResponsePacket:
		return s.handleResourcePackResponse(p)
	case *packets.TextPacket:
		if p.Type == protocol.TextTypeChat && s.OnChat != nil {
			s.OnChat(s, p.Message)
		}
		return nil
	case *packets.MovePlayerPacket:
		if s.OnMove != nil {
			s.OnMove(s, p.X, p.Y, p.Z, p.Yaw, p.Pitch, p.OnGround)
		}
		return nil
	case *packets.PlayerActionPacket:
		logger.Debug("network", "%s PlayerAction %s pos=(%d,%d,%d) face=%d", s.who(), packets.ActionName(p.Action), p.X, p.Y, p.Z, p.Face)
		if s.OnPlayerAction != nil {
			s.OnPlayerAction(s, p)
		}
		return nil
	case *packets.RequestChunkRadiusPacket:
		return s.handleRequestChunkRadius(p)
	case *packets.UseItemPacket:
		logger.Debug("network", "%s UseItem face=%d block=(%d,%d,%d) item=%d", s.who(), p.Face, p.X, p.Y, p.Z, p.Item.ID)
		if s.OnUseItem != nil {
			s.OnUseItem(s, p)
		}
		return nil
	case *packets.RemoveBlockPacket:
		logger.Debug("network", "%s RemoveBlock (%d,%d,%d)", s.who(), p.X, p.Y, p.Z)
		if s.OnRemoveBlock != nil {
			s.OnRemoveBlock(s, p)
		}
		return nil
	case *packets.MobEquipmentPacket:
		if s.OnMobEquipment != nil {
			s.OnMobEquipment(s, p)
		}
		return nil
	case *packets.LevelSoundEventPacket:
		logger.Debug("network", "%s LevelSoundEvent %s pos=(%.1f,%.1f,%.1f) extra=%d",
			s.who(), packets.SoundName(p.SoundID), p.X, p.Y, p.Z, p.ExtraData)
		return nil
	case *packets.ContainerSetSlotPacket:
		logger.Debug("network", "%s ContainerSetSlot window=%d slot=%d hotbar=%d item=%d×%d select=%d",
			s.who(), p.WindowID, p.Slot, p.HotbarSlot, p.Item.ID, p.Item.Count, p.SelectSlot)
		if s.OnContainerSetSlot != nil {
			s.OnContainerSetSlot(s, p)
		}
		return nil
	case *packets.CommandStepPacket:
		line := p.CommandLine()
		logger.Debug("network", "%s CommandStep line=%q", s.who(), line)
		if s.OnCommandStep != nil && line != "" {
			s.OnCommandStep(s, line)
		}
		return nil
	case *packets.ContainerClosePacket:
		logger.Debug("network", "%s ContainerClose window=%d", s.who(), p.WindowID)
		if s.OnContainerClose != nil {
			s.OnContainerClose(s, p)
		}
		return nil
	case *packets.DropItemPacket:
		logger.Debug("network", "%s DropItem type=%d item=%d×%d", s.who(), p.Type, p.Item.ID, p.Item.Count)
		if s.OnDropItem != nil {
			s.OnDropItem(s, p)
		}
		return nil
	case *packets.SetPlayerGameTypePacket:
		logger.Debug("network", "%s SetPlayerGameType gm=%d", s.who(), p.GameMode)
		if s.OnSetPlayerGameType != nil {
			s.OnSetPlayerGameType(s, p)
		}
		return nil
	case *packets.AnimatePacket:
		if s.OnAnimate != nil {
			s.OnAnimate(s, p)
		}
		return nil
	case *packets.InteractPacket:
		if s.OnInteract != nil {
			s.OnInteract(s, p)
		}
		return nil
	case *packets.EntityFallPacket:
		if s.OnEntityFall != nil {
			s.OnEntityFall(s, p)
		}
		return nil
	case *packets.SetEntityDataPacket:
		return nil
	default:
		logger.Debug("network", "%s 未处理包 %s", s.who(), protocol.PacketLabel(pkt.ID()))
		return nil
	}
}

func (s *Session) handleLogin(pkt *packets.LoginPacket) error {
	s.mu.Lock()
	if s.state != StateConnecting {
		s.mu.Unlock()
		return fmt.Errorf("unexpected LoginPacket in state %d", s.state)
	}
	s.state = StateLogin
	s.mu.Unlock()

	accepted := false
	for _, v := range protocol.AcceptedProtocols {
		if pkt.Protocol == int32(v) {
			accepted = true
			break
		}
	}
	if !accepted {
		logger.Warn("network", "%s 拒绝协议 %d (接受 %v)", s.who(), pkt.Protocol, protocol.AcceptedProtocols)
		s.SendPacket(&packets.PlayStatusPacket{Status: protocol.PlayStatusLoginFailedClient})
		s.Disconnect("Outdated client")
		return fmt.Errorf("unsupported protocol %d", pkt.Protocol)
	}

	s.Username = pkt.Username
	s.ClientUUID = pkt.ClientUUID
	s.XboxID = pkt.XUID
	s.ClientID = pkt.ClientID
	s.SkinID = pkt.SkinID
	s.SkinData = pkt.SkinData
	logger.Info("network", "%s 登录 protocol=%d edition=%d uuid=%s", s.who(), pkt.Protocol, pkt.GameEdition, pkt.ClientUUID)

	if s.OnLogin != nil {
		logger.Go("network", func() { s.OnLogin(s) })
	}
	return nil
}

func (s *Session) handleResourcePackResponse(pkt *packets.ResourcePackClientResponsePacket) error {
	switch pkt.Status {
	case protocol.ResourcePackResponseHaveAllPacks:
		s.SendPacket(&packets.ResourcePackStackPacket{MustAccept: false})
	case protocol.ResourcePackResponseCompleted:
		s.mu.Lock()
		s.state = StateSpawning
		s.mu.Unlock()
		if s.OnLogin != nil {
			logger.Go("network", func() { s.OnLogin(s) })
		}
	default:
		logger.Info("network", "%s 资源包响应 status=%d", s.who(), pkt.Status)
	}
	return nil
}

func (s *Session) handleRequestChunkRadius(pkt *packets.RequestChunkRadiusPacket) error {
	var radius int32 = pkt.ChunkRadius
	var maxRadius int32 = s.MaxChunkRadius
	if maxRadius <= 0 {
		maxRadius = defaultMaxChunkRadius
	}
	radius = max(minChunkRadius, min(radius, maxRadius))
	s.SendPacket(&packets.ChunkRadiusUpdatedPacket{ChunkRadius: radius})
	if s.OnChunkRadius != nil {
		logger.Go("network", func() { s.OnChunkRadius(s, radius) })
	}
	return nil
}

func (s *Session) SendPacket(p packets.Packet) {
	raw, err := packets.EncodePacket(p)
	if err != nil {
		logger.Error("network", "%s 编码 %s 失败: %v", s.who(), protocol.PacketLabel(p.ID()), err)
		return
	}
	s.SendBatch(raw)
}

func (s *Session) SendBatch(rawPackets ...[]byte) {
	// 没有传输层（测试里的零值会话）时发出去的包直接丢弃，而不是空指针崩溃。
	if s.rakSess == nil {
		return
	}
	batch, err := protocol.EncodeBatch(rawPackets)
	if err != nil {
		logger.Error("network", "%s 编码 Batch 失败: %v", s.who(), err)
		return
	}
	s.rakSess.SendPayload(batch)
}

func (s *Session) SendLoginSequence() {
	s.mu.Lock()
	s.state = StateResourcePacks
	s.mu.Unlock()
	s.SendPacket(&packets.PlayStatusPacket{Status: protocol.PlayStatusLoginSuccess})
	s.SendPacket(&packets.ResourcePacksInfoPacket{MustAccept: false})
}

// Disconnect 先把断开原因发给客户端，再等发送队列排空后才真正关连接。
// 直接关会让还在队列里的 Disconnect 包发不出去，客户端只看到「连接超时」而不是踢出原因。
func (s *Session) Disconnect(reason string) {
	s.SendPacket(&packets.DisconnectPacket{Message: reason})
	if s.rakSess == nil {
		return
	}
	s.rakSess.CloseAfterFlush()
}

func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) SetState(st SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
}

func (s *Session) onRakClose() {
	s.mu.Lock()
	s.state = StateDisconnected
	s.mu.Unlock()
	if s.OnDisconnect != nil {
		s.OnDisconnect(s)
	}
}
