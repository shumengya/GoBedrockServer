package raknet

import "bytes"
import "net"
import "testing"
import "time"

import "go-mc-server/internal/protocol"

// 排队中的包必须在连接关闭前发出去。踢人原因就是靠这个送达的：
// 之前 Disconnect 紧接着 Close，发送循环被立刻停掉，原因包还在队列里就丢了。
func TestCloseAfterFlushDeliversQueuedPayload(test *testing.T) {
	var loopback net.IP = net.IPv4(127, 0, 0, 1)
	var serverConn, serverErr = net.ListenUDP("udp4", &net.UDPAddr{IP: loopback, Port: 0})
	if serverErr != nil {
		test.Fatalf("监听服务端: %v", serverErr)
	}
	defer serverConn.Close()
	var clientConn, clientErr = net.ListenUDP("udp4", &net.UDPAddr{IP: loopback, Port: 0})
	if clientErr != nil {
		test.Fatalf("监听客户端: %v", clientErr)
	}
	defer clientConn.Close()
	var clientAddr, isUDP = clientConn.LocalAddr().(*net.UDPAddr)
	if !isUDP {
		test.Fatal("客户端地址不是 UDP 地址")
	}

	var srv *Server = NewServer(nil, nil, nil)
	srv.conn = serverConn
	var sess *Session = newSession(srv, clientAddr, 1400)
	srv.sessions[clientAddr.String()] = sess
	var closed chan struct{} = make(chan struct{})
	sess.SetCallbacks(nil, func() { close(closed) })
	go sess.run()

	var marker []byte = []byte{0xfe, 0xA1, 0xB2, 0xC3, 0xD4, 0xE5}
	sess.SendPayload(marker)
	sess.CloseAfterFlush()

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		test.Fatal("CloseAfterFlush 后会话没有关闭")
	}

	// 收齐关闭前后发出的数据报，按序号比先后，不依赖到达顺序。
	var markerSeq int64 = -1
	var otherSeq int64 = -1
	var buffer []byte = make([]byte, 2048)
	var deadline time.Time = time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		var deadlineErr error = clientConn.SetReadDeadline(deadline)
		if deadlineErr != nil {
			test.Fatalf("设置读超时: %v", deadlineErr)
		}
		var length, _, readErr = clientConn.ReadFromUDP(buffer)
		if readErr != nil {
			break
		}
		if length < 4 || buffer[0] < 0x80 || buffer[0] > 0x8f {
			continue
		}
		var seq int64 = int64(protocol.ReadTriadLE(buffer[1:4]))
		if bytes.Contains(buffer[:length], marker) {
			markerSeq = seq
			continue
		}
		if otherSeq < 0 || seq > otherSeq {
			otherSeq = seq
		}
	}
	if markerSeq < 0 {
		test.Fatal("排队的载荷没有在关闭前发出去")
	}
	if otherSeq < 0 {
		test.Fatal("关闭时应当发出断开通知")
	}
	if markerSeq >= otherSeq {
		test.Fatalf("载荷序号 %d 应早于断开通知序号 %d", markerSeq, otherSeq)
	}
}

// RemoteIP 只返回 IP，不带端口，封禁列表按这个比对。
func TestRemoteIPHasNoPort(test *testing.T) {
	var sess *Session = newSession(NewServer(nil, nil, nil), &net.UDPAddr{IP: net.IPv4(10, 1, 2, 3), Port: 19132}, 1400)
	if sess.RemoteIP() != "10.1.2.3" {
		test.Fatalf("RemoteIP=%q", sess.RemoteIP())
	}
}
