package raknet

import (
	"encoding/binary"
	"fmt"
	"go-mc-server/internal/protocol"
	"net"

	"go-mc-server/internal/logger"
)

// Magic is the 16-byte RakNet offline message identifier.
var Magic = [16]byte{
	0x00, 0xff, 0xff, 0x00,
	0xfe, 0xfe, 0xfe, 0xfe,
	0xfd, 0xfd, 0xfd, 0xfd,
	0x12, 0x34, 0x56, 0x78,
}

// Offline message IDs.
const (
	IDUnconnectedPing                = 0x01
	IDUnconnectedPingOpenConnections = 0x02
	IDUnconnectedPong                = 0x1c
	IDOpenConnectionRequest1         = 0x05
	IDOpenConnectionReply1           = 0x06
	IDOpenConnectionRequest2         = 0x07
	IDOpenConnectionReply2           = 0x08
	IDConnectionRequest              = 0x09
	IDConnectionRequestAccepted      = 0x10
	IDNewIncomingConnection          = 0x13
	IDConnectedPing                  = 0x00
	IDConnectedPong                  = 0x03
	IDDisconnectionNotification      = 0x15
	IDIncompatibleProtocolVersion    = 0x19
)

// RakNetProtocol is the RakNet-level protocol version used by MCPE 1.1.x.
// Source: RakLibInterface.php → MCPE_RAKNET_PROTOCOL_VERSION = 8
const RakNetProtocol = 8

// handleOfflinePacket processes RakNet offline/handshake packets
// (those that arrive before a full session is established) and returns
// the response bytes to send back, or nil if handled elsewhere.
func (srv *Server) handleOfflinePacket(data []byte, addr *net.UDPAddr) {
	if len(data) == 0 {
		return
	}

	switch data[0] {
	case IDUnconnectedPing, IDUnconnectedPingOpenConnections:
		srv.handleUnconnectedPing(data, addr)
	case IDOpenConnectionRequest1:
		logger.Info("raknet", "[RakNet] OCR1 from %s (len=%d)", addr, len(data))
		srv.handleOpenConnectionRequest1(data, addr)
	case IDOpenConnectionRequest2:
		logger.Info("raknet", "[RakNet] OCR2 from %s", addr)
		srv.handleOpenConnectionRequest2(data, addr)
	}
}

// handleUnconnectedPing responds with the server's MOTD pong.
func (srv *Server) handleUnconnectedPing(data []byte, addr *net.UDPAddr) {
	if len(data) < 1+8+16 {
		return
	}
	pingTime := int64(binary.BigEndian.Uint64(data[1:9]))
	// Build the MOTD string (MCPE format).
	motd := fmt.Sprintf("MCPE;%s;%d;%s;%d;%d", srv.cfg.MOTD, protocol.MinimumProtocol,
		protocol.MinimumVersionNetwork, len(srv.listing.OnlinePlayers()), srv.cfg.MaxPlayers)

	resp := make([]byte, 0, 35+len(motd))
	resp = append(resp, IDUnconnectedPong)
	resp = appendInt64BE(resp, pingTime)
	resp = appendInt64BE(resp, srv.serverID)
	resp = append(resp, Magic[:]...)
	resp = appendUint16BE(resp, uint16(len(motd)))
	resp = append(resp, []byte(motd)...)
	_, _ = srv.conn.WriteToUDP(resp, addr)
}

// handleOpenConnectionRequest1 responds with OpenConnectionReply1.
func (srv *Server) handleOpenConnectionRequest1(data []byte, addr *net.UDPAddr) {
	// data: ID(1) + magic(16) + raknetVersion(1) + MTU padding
	if len(data) < 18 {
		logger.Info("raknet", "[RakNet] OCR1 too short (%d bytes) from %s", len(data), addr)
		return
	}
	if !checkMagic(data[1:17]) {
		logger.Info("raknet", "[RakNet] OCR1 bad magic from %s", addr)
		return
	}
	rakVer := data[17]
	logger.Info("raknet", "[RakNet] OCR1 raknet-protocol=%d from %s", rakVer, addr)
	if rakVer != RakNetProtocol {
		logger.Info("raknet", "[RakNet] Incompatible RakNet protocol %d (want %d) from %s", rakVer, RakNetProtocol, addr)
		srv.sendIncompatibleProtocol(addr)
		return
	}
	mtu := uint16(len(data) + 28) // 28 = UDP/IP header overhead

	logger.Info("raknet", "[RakNet] OCA1 → %s  mtu=%d", addr, mtu)
	resp := make([]byte, 0, 28)
	resp = append(resp, IDOpenConnectionReply1)
	resp = append(resp, Magic[:]...)
	resp = appendInt64BE(resp, srv.serverID)
	resp = append(resp, 0x00) // use security = false
	resp = appendUint16BE(resp, mtu)
	_, _ = srv.conn.WriteToUDP(resp, addr)
}

// handleOpenConnectionRequest2 responds with OpenConnectionReply2 and creates a session.
func (srv *Server) handleOpenConnectionRequest2(data []byte, addr *net.UDPAddr) {
	// data: ID(1) + magic(16) + serverAddress(variable) + MTU(2) + clientID(8)
	if len(data) < 1+16+7+2+8 {
		logger.Info("raknet", "[RakNet] OCR2 too short (%d bytes) from %s", len(data), addr)
		return
	}
	if !checkMagic(data[1:17]) {
		logger.Info("raknet", "[RakNet] OCR2 bad magic from %s", addr)
		return
	}
	offset := 17
	// Skip server address (IP type byte + IP + port = variable)
	addrLen := 4 + 1 + 2 // IPv4: 1 + 4 + 2
	if data[offset] == 6 {
		addrLen = 1 + 16 + 2 // IPv6
	} else {
		addrLen = 1 + 4 + 2 // IPv4
	}
	offset += addrLen
	if offset+2+8 > len(data) {
		return
	}
	mtu := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	clientID := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	_ = clientID

	// Create the session.
	srv.mu.Lock()
	key := addr.String()
	var newSess *Session
	if _, exists := srv.sessions[key]; !exists {
		newSess = newSession(srv, addr, mtu)
		srv.sessions[key] = newSess
		go newSess.run()
	}
	srv.mu.Unlock()
	if newSess != nil {
		srv.notifyNewSession(newSess)
	}

	logger.Info("raknet", "[RakNet] OCA2 → %s  mtu=%d", addr, mtu)
	resp := make([]byte, 0, 31)
	resp = append(resp, IDOpenConnectionReply2)
	resp = append(resp, Magic[:]...)
	resp = appendInt64BE(resp, srv.serverID)
	resp = appendAddress(resp, addr)
	resp = appendUint16BE(resp, mtu)
	resp = append(resp, 0x00) // encryption disabled
	_, _ = srv.conn.WriteToUDP(resp, addr)
}

func (srv *Server) sendIncompatibleProtocol(addr *net.UDPAddr) {
	resp := make([]byte, 0, 26)
	resp = append(resp, IDIncompatibleProtocolVersion)
	resp = append(resp, RakNetProtocol)
	resp = append(resp, Magic[:]...)
	resp = appendInt64BE(resp, srv.serverID)
	_, _ = srv.conn.WriteToUDP(resp, addr)
}

// ---- helpers ----

func checkMagic(b []byte) bool {
	if len(b) < 16 {
		return false
	}
	for i, v := range Magic {
		if b[i] != v {
			return false
		}
	}
	return true
}

func appendInt64BE(b []byte, v int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	return append(b, buf...)
}

func appendUint16BE(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

func appendAddress(b []byte, addr *net.UDPAddr) []byte {
	ip := addr.IP.To4()
	if ip != nil {
		b = append(b, 4)
		b = append(b, ip...)
	} else {
		b = append(b, 6)
		b = append(b, addr.IP.To16()...)
	}
	return appendUint16BE(b, uint16(addr.Port))
}
