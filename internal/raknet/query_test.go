package raknet

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"go-mc-server/config"
)

type testListing []string

func (names testListing) OnlinePlayers() []string { return names }
func (names testListing) WhitelistEnabled() bool  { return true }

// 同一个 UDP 端口分别响应游戏列表、挑战、简略和完整查询，逐字段核对端序与在线名单。
func TestQueryAndPongUDP(test *testing.T) {
	var cfg = config.Default()
	cfg.MOTD = "Demo"
	var srv = NewServer(cfg, nil, testListing{"Alex", "Steve"})
	if err := srv.Listen("127.0.0.1:0"); err != nil {
		test.Fatal(err)
	}
	defer srv.Close()
	var conn, err = net.DialUDP("udp4", nil, srv.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		test.Fatal(err)
	}
	defer conn.Close()
	var exchange = func(request []byte) []byte {
		test.Helper()
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(request); err != nil {
			test.Fatal(err)
		}
		var buf [4096]byte
		var size, err = conn.Read(buf[:])
		if err != nil {
			test.Fatal(err)
		}
		return buf[:size]
	}
	var pong = exchange(append([]byte{1, 0, 0, 0, 0, 0, 0, 0, 1}, Magic[:]...))
	if len(pong) < 35 || pong[0] != 0x1c || string(pong[35:]) != "MCPE;Demo;110;1.1.0;2;20" {
		test.Fatalf("期望 Pong 报告 2/20 和最低协议版本，实际 %q", pong)
	}
	var request = []byte{0xfe, 0xfd, 9, 0xaa, 0xbb, 0xcc, 0xdd}
	var challenge = exchange(request)
	if len(challenge) < 7 || !bytes.Equal(challenge[:5], request[2:]) || challenge[len(challenge)-1] != 0 {
		test.Fatalf("期望握手回显会话标识与挑战值，实际 %x", challenge)
	}
	var token, parseErr = strconv.ParseInt(string(challenge[5:len(challenge)-1]), 10, 32)
	if parseErr != nil {
		test.Fatal(parseErr)
	}
	request[2] = 0
	request = binary.BigEndian.AppendUint32(request, uint32(token))
	var short = exchange(request)
	var want = append([]byte{0, 0xaa, 0xbb, 0xcc, 0xdd}, "Demo\x00SMP\x00world\x002\x0020\x00\xbc\x4a0.0.0.0\x00"...)
	if !bytes.Equal(short, want) {
		test.Fatalf("期望 %x，实际 %x", want, short)
	}
	var full = exchange(append(request, 0, 0, 0, 0))
	if !bytes.HasPrefix(full, append(request[2:7:7], "splitnum\x00\x80\x00"...)) || !bytes.Contains(full, []byte("numplayers\x002\x00maxplayers\x0020\x00whitelist\x00on\x00")) || !bytes.HasSuffix(full, []byte("\x00\x01player_\x00\x00Alex\x00Steve\x00\x00")) {
		test.Fatalf("期望完整统计与玩家名单，实际 %q", full)
	}
}

// 跨分钟的在途请求仍可完成，其他来源和过期挑战均不能取走统计信息。
func TestQueryChallengeScope(test *testing.T) {
	var srv = NewServer(config.Default(), nil, testListing{})
	var addr = net.IPv4(127, 0, 0, 1)
	var request = binary.BigEndian.AppendUint32([]byte{0xfe, 0xfd, 0, 1, 2, 3, 4}, srv.queryToken(addr, 20))
	if srv.queryReply(request, addr, time.Unix(21*60, 0)) == nil ||
		srv.queryReply(request, addr, time.Unix(22*60, 0)) != nil ||
		srv.queryReply(request, net.IPv4(127, 0, 0, 2), time.Unix(20*60, 0)) != nil {
		test.Fatal("期望仅原地址在当前或下一窗口内通过，实际不符")
	}
}
