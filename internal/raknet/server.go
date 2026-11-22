package raknet

import (
	crand "crypto/rand"
	"fmt"
	"go-mc-server/config"
	"math/rand"
	"net"
	"sync"
	"time"

	"go-mc-server/internal/logger"
)

type listing interface {
	OnlinePlayers() []string
	WhitelistEnabled() bool
}

// SessionHandler is called by the RakNet server whenever a new fully-connected
// session is ready for MCPE traffic.
type SessionHandler func(sess *Session)

// Server is the RakNet UDP server. It handles offline messages directly and
// routes per-client datagrams to their Session objects.
type Server struct {
	conn     *net.UDPConn
	cfg      *config.Config
	listing  listing
	queryKey [32]byte
	serverID int64

	mu       sync.RWMutex
	sessions map[string]*Session

	onNewSession SessionHandler
}

// NewServer creates a new RakNet Server but does not start listening.
func NewServer(cfg *config.Config, onNewSession SessionHandler, players listing) *Server {
	var srv = &Server{
		listing:      players,
		cfg:          cfg,
		serverID:     rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		sessions:     make(map[string]*Session),
		onNewSession: onNewSession,
	}
	crand.Read(srv.queryKey[:])
	return srv
}

// Listen starts the UDP server on the configured host:port.
func (srv *Server) Listen(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return fmt.Errorf("raknet: resolve %s: %w", addr, err)
	}
	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("raknet: listen %s: %w", addr, err)
	}
	srv.conn = conn
	go srv.readLoop()
	return nil
}

// readLoop is the main receive loop; it runs in its own goroutine.
func (srv *Server) readLoop() {
	defer logger.Recover("raknet")
	buf := make([]byte, 1500)
	for {
		n, addr, err := srv.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		logger.Go("raknet", func() { srv.handlePacket(data, addr) })
	}
}

// handlePacket routes an incoming UDP packet to the right handler.
func (srv *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	defer logger.Recover("raknet")
	if len(data) == 0 {
		return
	}
	id := data[0]

	// Offline / handshake messages are handled before a session exists.
	switch id {
	case 0xfe:
		if srv.cfg.Query {
			if reply := srv.queryReply(data, addr.IP, time.Now()); reply != nil {
				_, _ = srv.conn.WriteToUDP(reply, addr)
			}
		}
		return
	case IDUnconnectedPing, IDUnconnectedPingOpenConnections,
		IDOpenConnectionRequest1, IDOpenConnectionRequest2:
		srv.handleOfflinePacket(data, addr)
		return
	}

	// Route to existing session.
	key := addr.String()
	srv.mu.RLock()
	sess, ok := srv.sessions[key]
	srv.mu.RUnlock()

	if !ok {
		// Unknown sender – ignore.
		return
	}

	sess.HandlePacket(data)
}

// removeSession cleans up a terminated session.
func (srv *Server) removeSession(key string) {
	srv.mu.Lock()
	delete(srv.sessions, key)
	srv.mu.Unlock()
}

// notifyNewSession is called by the offline handler (in handleOpenConnectionRequest2)
// after a session is registered so the MCPE layer can attach callbacks.
func (srv *Server) notifyNewSession(sess *Session) {
	if srv.onNewSession != nil {
		logger.Go("raknet", func() { srv.onNewSession(sess) })
	}
}

// Close shuts down the UDP listener.
func (srv *Server) Close() error {
	if srv.conn != nil {
		return srv.conn.Close()
	}
	return nil
}
