package raknet

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"go-mc-server/internal/logger"
)

// sessionState represents the lifecycle stage of a RakNet session.
type sessionState int

const (
	stateHandshaking sessionState = iota
	stateConnected
	stateDisconnecting
)

const (
	maxSplitPackets = 512
	windowSize      = 2048
	sendQueueMax    = 4096
	resendTimeout   = 500 * time.Millisecond
	sessionTimeout  = 30 * time.Second
)

// Session is a single RakNet connection to one client.
type Session struct {
	srv  *Server
	addr *net.UDPAddr
	mtu  uint16

	state sessionState
	mu    sync.Mutex

	// Receive side.
	recvSeqNum    uint32
	recvACKQueue  []uint32
	reorderBuffer map[uint32]*EncapsulatedPacket // order index → packet
	reorderExpect uint32
	splitBuffer   map[uint16]map[uint32][]byte // splitID → (index → payload)

	// Send side.
	sendSeqNum     uint32
	sendMsgIndex   uint32
	sendOrderIndex uint32
	sendSplitID    uint16

	// Reliable send window for retransmission.
	sendWindow   map[uint32]*pendingDatagram
	sendWindowMu sync.Mutex

	// Inbound payload channel: delivers reassembled MCPE payloads up to network layer.
	InboundCh chan []byte
	// Outbound channel: raw bytes enqueued for sending.
	outboundCh chan []byte

	lastRecv time.Time

	// Callback invoked when a full MCPE payload is ready.
	onPayload func([]byte)
	// Callback invoked when the session is closed.
	onClose func()

	stopCh chan struct{}
}

type pendingDatagram struct {
	data   []byte
	sentAt time.Time
	seqNum uint32
}

func newSession(srv *Server, addr *net.UDPAddr, mtu uint16) *Session {
	return &Session{
		srv:           srv,
		addr:          addr,
		mtu:           mtu,
		state:         stateHandshaking,
		reorderBuffer: make(map[uint32]*EncapsulatedPacket),
		splitBuffer:   make(map[uint16]map[uint32][]byte),
		sendWindow:    make(map[uint32]*pendingDatagram),
		InboundCh:     make(chan []byte, 256),
		outboundCh:    make(chan []byte, sendQueueMax),
		lastRecv:      time.Now(),
		stopCh:        make(chan struct{}),
	}
}

// SetCallbacks wires the session up to the network layer.
func (s *Session) SetCallbacks(onPayload func([]byte), onClose func()) {
	s.onPayload = onPayload
	s.onClose = onClose
}

// run is the main goroutine for a session. It handles the outbound send loop
// and periodic tasks (ACK flushing, retransmission, timeout).
func (s *Session) run() {
	defer logger.Recover("raknet")
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case data := <-s.outboundCh:
			if data == nil {
				// CloseAfterFlush 放进来的哨兵：队列是先进先出，走到这说明前面的包都发完了。
				s.Close()
				return
			}
			s.flushSend(data)
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick is called every 50ms for ACK flushing, retransmission and timeout.
func (s *Session) tick() {
	s.mu.Lock()
	ackQueue := s.recvACKQueue
	s.recvACKQueue = nil
	timedOut := time.Since(s.lastRecv) > sessionTimeout
	s.mu.Unlock()

	if timedOut {
		s.Close()
		return
	}

	if len(ackQueue) > 0 {
		ack := encodeACK(0xc0, ackQueue)
		_, _ = s.srv.conn.WriteToUDP(ack, s.addr)
	}

	// Retransmit unacknowledged datagrams.
	s.sendWindowMu.Lock()
	var toResend [][]byte
	for _, pd := range s.sendWindow {
		if time.Since(pd.sentAt) > resendTimeout {
			toResend = append(toResend, pd.data)
			pd.sentAt = time.Now()
		}
	}
	s.sendWindowMu.Unlock()
	for _, d := range toResend {
		_, _ = s.srv.conn.WriteToUDP(d, s.addr)
	}
}

// HandlePacket is called by the server when a datagram arrives for this session.
func (s *Session) HandlePacket(data []byte) {
	if len(data) == 0 {
		return
	}

	s.mu.Lock()
	s.lastRecv = time.Now()
	s.mu.Unlock()

	id := data[0]

	switch {
	case id == 0xc0: // ACK
		seqNums, err := decodeACK(data)
		if err != nil {
			logger.Warn("raknet", "%s ACK 解码失败: %v hex=%s", s.addr, err, logger.HexDump(data, 24))
			return
		}
		s.handleACK(seqNums)
	case id == 0xa0: // NACK
		seqNums, err := decodeACK(data)
		if err != nil {
			logger.Warn("raknet", "%s NACK 解码失败: %v hex=%s", s.addr, err, logger.HexDump(data, 24))
			return
		}
		s.handleNACK(seqNums)
	case id >= 0x80 && id <= 0x8f: // Datagram
		s.handleDatagram(data)
	case id == IDConnectedPing:
		s.handleConnectedPing(data)
	case id == IDDisconnectionNotification:
		s.Close()
	}
}

func (s *Session) handleACK(seqNums []uint32) {
	s.sendWindowMu.Lock()
	defer s.sendWindowMu.Unlock()
	for _, seq := range seqNums {
		delete(s.sendWindow, seq)
	}
}

func (s *Session) handleNACK(seqNums []uint32) {
	s.sendWindowMu.Lock()
	var toResend [][]byte
	for _, seq := range seqNums {
		if pd, ok := s.sendWindow[seq]; ok {
			toResend = append(toResend, pd.data)
			pd.sentAt = time.Now()
		}
	}
	s.sendWindowMu.Unlock()
	for _, d := range toResend {
		_, _ = s.srv.conn.WriteToUDP(d, s.addr)
	}
}

func (s *Session) handleDatagram(data []byte) {
	seqNum, packets, err := decodeDatagram(data)
	if err != nil {
		logger.Warn("raknet", "%s Datagram 解码失败: %v hex=%s", s.addr, err, logger.HexDump(data, 24))
		return
	}

	s.mu.Lock()
	s.recvACKQueue = append(s.recvACKQueue, seqNum)
	s.mu.Unlock()

	for _, ep := range packets {
		s.handleEncapsulated(ep)
	}
}

func (s *Session) handleEncapsulated(ep *EncapsulatedPacket) {
	// Reassemble split packets.
	if ep.HasSplit {
		payload := s.reassemble(ep)
		if payload == nil {
			return
		}
		ep.Payload = payload
		ep.HasSplit = false
	}

	if isOrdered(ep.Reliability) {
		s.handleOrdered(ep)
		return
	}
	s.dispatchPayload(ep)
}

func cloneEncapsulated(ep *EncapsulatedPacket) *EncapsulatedPacket {
	cp := *ep
	cp.Payload = append([]byte(nil), ep.Payload...)
	return &cp
}

func (s *Session) handleOrdered(ep *EncapsulatedPacket) {
	var ready []*EncapsulatedPacket
	s.mu.Lock()
	if ep.OrderIndex < s.reorderExpect {
		s.mu.Unlock()
		return
	}
	if ep.OrderIndex > s.reorderExpect {
		if len(s.reorderBuffer) > 512 {
			logger.Warn("raknet", "%s 有序重排缓冲区溢出, 丢弃 order=%d expect=%d", s.addr, ep.OrderIndex, s.reorderExpect)
			s.mu.Unlock()
			return
		}
		s.reorderBuffer[ep.OrderIndex] = cloneEncapsulated(ep)
		s.mu.Unlock()
		return
	}
	ready = append(ready, ep)
	s.reorderExpect++
	for {
		next, ok := s.reorderBuffer[s.reorderExpect]
		if !ok {
			break
		}
		delete(s.reorderBuffer, s.reorderExpect)
		ready = append(ready, next)
		s.reorderExpect++
	}
	s.mu.Unlock()
	for _, p := range ready {
		s.dispatchPayload(p)
	}
}

// reassemble adds a split fragment to the buffer. Returns the complete payload
// when all fragments have arrived, or nil otherwise.
func (s *Session) reassemble(ep *EncapsulatedPacket) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, ok := s.splitBuffer[ep.SplitID]
	if !ok {
		buf = make(map[uint32][]byte)
		s.splitBuffer[ep.SplitID] = buf
	}
	fragment := make([]byte, len(ep.Payload))
	copy(fragment, ep.Payload)
	buf[ep.SplitIndex] = fragment

	if uint32(len(buf)) < ep.SplitCount {
		return nil
	}
	// All fragments received.
	total := 0
	for _, f := range buf {
		total += len(f)
	}
	full := make([]byte, 0, total)
	for i := uint32(0); i < ep.SplitCount; i++ {
		full = append(full, buf[i]...)
	}
	delete(s.splitBuffer, ep.SplitID)
	return full
}

func (s *Session) dispatchPayload(ep *EncapsulatedPacket) {
	if len(ep.Payload) == 0 {
		return
	}
	id := ep.Payload[0]

	switch id {
	case IDConnectionRequest:
		logger.Info("raknet", "%s ConnectionRequest", s.addr)
		s.handleConnectionRequest(ep.Payload)
	case IDNewIncomingConnection:
		s.mu.Lock()
		s.state = stateConnected
		s.mu.Unlock()
		logger.Info("raknet", "%s NewIncomingConnection", s.addr)
	case IDConnectedPing:
		s.handleConnectedPing(ep.Payload)
	case IDDisconnectionNotification:
		logger.Info("raknet", "%s DisconnectionNotification", s.addr)
		s.Close()
	default:
		// Pass up to MCPE layer.
		logger.Debug("raknet", "%s 游戏载荷 id=0x%02x len=%d", s.addr, ep.Payload[0], len(ep.Payload))
		if s.onPayload != nil {
			cp := make([]byte, len(ep.Payload))
			copy(cp, ep.Payload)
			s.onPayload(cp)
		}
	}
}

// handleConnectionRequest replies with ConnectionRequestAccepted.
func (s *Session) handleConnectionRequest(data []byte) {
	if len(data) < 17 {
		return
	}
	clientID := int64(binary.BigEndian.Uint64(data[1:9]))
	sendPing := int64(binary.BigEndian.Uint64(data[9:17]))
	_ = clientID

	now := time.Now().UnixMilli()
	resp := make([]byte, 0, 96)
	resp = append(resp, IDConnectionRequestAccepted)
	resp = appendAddress(resp, s.addr)
	resp = append(resp, 0, 0) // systemIndex
	// 10 internal addresses (each 7 bytes for IPv4).
	for i := 0; i < 10; i++ {
		resp = append(resp, 4, 127, 0, 0, 1, 0, 0) // 127.0.0.1:0
	}
	resp = appendInt64BE(resp, sendPing)
	resp = appendInt64BE(resp, now)

	s.sendReliableOrdered(resp)
}

func (s *Session) handleConnectedPing(data []byte) {
	if len(data) < 9 {
		return
	}
	pingTime := int64(binary.BigEndian.Uint64(data[1:9]))
	resp := make([]byte, 17)
	resp[0] = IDConnectedPong
	binary.BigEndian.PutUint64(resp[1:9], uint64(pingTime))
	binary.BigEndian.PutUint64(resp[9:17], uint64(time.Now().UnixMilli()))
	s.sendReliableOrdered(resp)
}

// SendPayload sends a raw MCPE payload to the client, using RELIABLE_ORDERED delivery.
// If the outbound queue is full it waits briefly instead of silently dropping.
func (s *Session) SendPayload(data []byte) {
	cp := make([]byte, len(data))
	copy(cp, data)
	select {
	case s.outboundCh <- cp:
		return
	default:
	}
	logger.Warn("raknet", "%s outbound 队列满，等待发送 len=%d", s, len(data))
	select {
	case s.outboundCh <- cp:
	case <-s.stopCh:
	case <-time.After(2 * time.Second):
		logger.Error("raknet", "%s outbound 等待超时，丢弃 len=%d", s, len(data))
	}
}

// flushSend encodes and sends the payload as one or more datagrams (splitting if needed).
//
// CRITICAL: when fragmenting a payload, ALL fragments must share the SAME
// orderIndex (each is still given its own messageIndex). The client uses the
// orderIndex from the reassembled packet as the logical message order, so
// every fragment must agree on it; otherwise the receiver buffers the
// reassembled packet as "future" and never delivers it to the game layer.
//
// Source: raklib Session::addEncapsulatedToQueue() — orderIndex is allocated
// ONCE before splitting (line 345), then copied to each fragment (line 375).
func (s *Session) flushSend(payload []byte) {
	maxPayloadSize := int(s.mtu) - 60 // conservative overhead budget

	// Allocate the orderIndex for this logical message exactly once.
	s.mu.Lock()
	orderIndex := s.sendOrderIndex
	s.sendOrderIndex++
	s.mu.Unlock()

	if len(payload) > maxPayloadSize {
		s.mu.Lock()
		splitID := s.sendSplitID
		s.sendSplitID++
		s.mu.Unlock()

		splitCount := (len(payload) + maxPayloadSize - 1) / maxPayloadSize
		for i := 0; i < splitCount; i++ {
			start := i * maxPayloadSize
			end := start + maxPayloadSize
			if end > len(payload) {
				end = len(payload)
			}
			fep := &EncapsulatedPacket{
				Reliability:  ReliableOrdered,
				OrderChannel: 0,
				OrderIndex:   orderIndex, // shared by all fragments
				HasSplit:     true,
				SplitCount:   uint32(splitCount),
				SplitID:      splitID,
				SplitIndex:   uint32(i),
				Payload:      payload[start:end],
			}
			s.assignMsgIndex(fep) // independent per fragment
			s.sendDatagram([][]byte{encodeEncapsulated(fep)})
		}
		return
	}

	ep := &EncapsulatedPacket{
		Reliability:  ReliableOrdered,
		OrderChannel: 0,
		OrderIndex:   orderIndex,
		Payload:      payload,
	}
	s.assignMsgIndex(ep)
	s.sendDatagram([][]byte{encodeEncapsulated(ep)})
}

func (s *Session) assignMsgIndex(ep *EncapsulatedPacket) {
	s.mu.Lock()
	ep.MessageIndex = s.sendMsgIndex
	s.sendMsgIndex++
	s.mu.Unlock()
}

func (s *Session) assignOrderIndex(ep *EncapsulatedPacket) {
	s.mu.Lock()
	ep.OrderIndex = s.sendOrderIndex
	s.sendOrderIndex++
	s.mu.Unlock()
}

func (s *Session) sendDatagram(encapsulated [][]byte) {
	s.mu.Lock()
	seqNum := s.sendSeqNum
	s.sendSeqNum++
	s.mu.Unlock()

	dgram := encodeDatagram(seqNum, encapsulated)
	cp := make([]byte, len(dgram))
	copy(cp, dgram)

	s.sendWindowMu.Lock()
	s.sendWindow[seqNum] = &pendingDatagram{
		data:   cp,
		sentAt: time.Now(),
		seqNum: seqNum,
	}
	s.sendWindowMu.Unlock()

	_, _ = s.srv.conn.WriteToUDP(dgram, s.addr)
}

// sendReliableOrdered is a shorthand that skips the outbound queue (for internal
// handshake messages that must be sent immediately).
func (s *Session) sendReliableOrdered(payload []byte) {
	ep := &EncapsulatedPacket{
		Reliability:  ReliableOrdered,
		OrderChannel: 0,
		Payload:      payload,
	}
	s.assignMsgIndex(ep)
	s.assignOrderIndex(ep)
	s.sendDatagram([][]byte{encodeEncapsulated(ep)})
}

// RemoteIP 返回对端 IP（不含端口）。
func (s *Session) RemoteIP() string {
	return s.addr.IP.String()
}

// closeFlushTimeout 是等发送队列腾出位置的上限，超时就不管剩下的包强行关。
const closeFlushTimeout time.Duration = 2 * time.Second

// CloseAfterFlush 等已排队的包都发完再关连接。
// 直接 Close 会让还没发出去的包（比如踢人原因）被丢掉：Close 立刻关闭发送循环。
func (s *Session) CloseAfterFlush() {
	select {
	case s.outboundCh <- nil:
	case <-s.stopCh:
	case <-time.After(closeFlushTimeout):
		s.Close()
	}
}

// Close tears down the session and notifies the server.
func (s *Session) Close() {
	s.mu.Lock()
	if s.state == stateDisconnecting {
		s.mu.Unlock()
		return
	}
	s.state = stateDisconnecting
	s.mu.Unlock()

	// Send disconnect notification.
	disc := []byte{IDDisconnectionNotification}
	s.sendReliableOrdered(disc)

	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}

	if s.onClose != nil {
		s.onClose()
	}
	s.srv.removeSession(s.addr.String())
}

// String returns a human-readable label for logging.
func (s *Session) String() string {
	return fmt.Sprintf("RakNetSession(%s)", s.addr)
}
