package raknet

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"

	"go-mc-server/internal/protocol"
)

// 挑战值绑定来源 IP 和分钟窗口，不为未建立会话的地址分配状态。
func (srv *Server) queryToken(ip net.IP, minute int64) uint32 {
	var mac = hmac.New(sha256.New, srv.queryKey[:])
	mac.Write(ip.To16())
	mac.Write(binary.BigEndian.AppendUint64(nil, uint64(minute)))
	return binary.BigEndian.Uint32(mac.Sum(nil))
}

func (srv *Server) queryReply(data []byte, ip net.IP, now time.Time) []byte {
	if len(data) < 7 || data[0] != 0xfe || data[1] != 0xfd {
		return nil
	}
	var minute = now.Unix() / 60
	var reply = append([]byte(nil), data[2:7]...)
	switch data[2] {
	case 9:
		if len(data) != 7 {
			return nil
		}
		return append(strconv.AppendInt(reply, int64(int32(srv.queryToken(ip, minute))), 10), 0)
	case 0:
		if len(data) != 11 && len(data) != 15 {
			return nil
		}
		var token = binary.BigEndian.Uint32(data[7:11])
		if token != srv.queryToken(ip, minute) && token != srv.queryToken(ip, minute-1) {
			return nil
		}
	default:
		return nil
	}
	var cfg = srv.cfg
	var names = srv.listing.OnlinePlayers()
	var count = strconv.Itoa(len(names))
	var maximum = strconv.Itoa(cfg.MaxPlayers)
	var mode = "SMP"
	if cfg.GameMode&1 != 0 {
		mode = "CMP"
	}
	var address = cfg.Host
	if address == "" {
		address = "0.0.0.0"
	}
	var writeFields = func(fields ...string) {
		for _, field := range fields {
			reply = append(reply, strings.ReplaceAll(field, "\x00", "")...)
			reply = append(reply, 0)
		}
	}
	if len(data) == 11 {
		writeFields(cfg.MOTD, mode, cfg.WorldName, count, maximum)
		reply = binary.LittleEndian.AppendUint16(reply, uint16(cfg.Port))
		writeFields(address)
		return reply
	}
	var whitelist = "off"
	if srv.listing.WhitelistEnabled() {
		whitelist = "on"
	}
	writeFields("splitnum", "\x80", "hostname", cfg.MOTD, "gametype", mode, "game_id", "MINECRAFTPE",
		"version", protocol.MinecraftVersionNetwork, "server_engine", "Go-MC-Server", "plugins", "Go-MC-Server",
		"map", cfg.WorldName, "numplayers", count, "maxplayers", maximum, "whitelist", whitelist,
		"hostip", address, "hostport", strconv.Itoa(cfg.Port))
	reply = append(reply, "\x00\x01player_\x00\x00"...)
	writeFields(names...)
	return append(reply, 0)
}
