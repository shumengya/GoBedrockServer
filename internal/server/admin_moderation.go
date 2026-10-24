package server

import "errors"
import "fmt"
import "net"
import "strings"
import "time"

import "go-mc-server/config"
import "go-mc-server/internal/command"

const defaultBanText string = "被管理员封禁"

// kickPlayer 把玩家踢下线并告诉他原因。kickHook 是测试用的替换点，
// 真实会话需要 UDP 连接，单元测试里造不出来。
func (srv *Server) kickPlayer(player *Player, message string) {
	if srv.kickHook != nil {
		srv.kickHook(player, message)
		return
	}
	if player.Session == nil {
		return
	}
	player.Session.Disconnect(message)
}

func kickMessage(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "你被管理员踢出了游戏"
	}
	return "你被管理员踢出了游戏。原因: " + strings.TrimSpace(reason)
}

func (admin serverAdmin) Kick(name string, reason string) (string, error) {
	var player, err = admin.lookupOnline(name)
	if err != nil {
		return "", err
	}
	admin.srv.kickPlayer(player, kickMessage(reason))
	return player.Username, nil
}

// forceKill 直接置死，不走 takeDamage：/kill 在创造模式也要生效，无敌时间也不该挡住它。
func (player *Player) forceKill() bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.combat.Dead {
		return false
	}
	player.combat.Health = 0
	player.combat.Dead = true
	player.combat.LastCause = DamageCauseGeneric
	return true
}

func (srv *Server) slayPlayer(player *Player) bool {
	if !player.forceKill() {
		return false
	}
	srv.sendHealth(player, 0)
	srv.killPlayer(player, nil, DamageCauseGeneric)
	return true
}

func (admin serverAdmin) Kill(name string) (string, error) {
	var player, err = admin.lookupPlaying(name)
	if err != nil {
		return "", err
	}
	if !admin.srv.slayPlayer(player) {
		return "", fmt.Errorf("%s 已经死亡", player.Username)
	}
	return player.Username, nil
}

func (admin serverAdmin) accessReady() error {
	if admin.srv.access == nil {
		return fmt.Errorf("访问控制未就绪")
	}
	return nil
}

// Ban 玩家不在线也能封，按字面名字写进名单；名字有多个前缀匹配时拒绝，免得封错人。
func (admin serverAdmin) Ban(name string, source string, reason string) (string, bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return "", false, err
	}
	var online *Player = nil
	online, err = admin.lookupOnline(name)
	if errors.Is(err, command.ErrAmbiguousName) {
		return "", false, err
	}
	var target string = strings.TrimSpace(name)
	if online != nil {
		target = online.Username
	}
	var why string = strings.TrimSpace(reason)
	if why == "" {
		why = defaultBanText
	}
	var added bool = false
	added, err = admin.srv.access.BannedNames.Add(target, source, why, time.Time{}, time.Now())
	if err != nil {
		return "", false, fmt.Errorf("写入封禁名单: %w", err)
	}
	if online != nil {
		admin.srv.kickPlayer(online, fmt.Sprintf(denyBannedName, why))
	}
	return target, added, nil
}

// resolveBanIP 把「IP 或在线玩家名」解析成规范的 IP 字符串。
// 先判断是不是 IP：玩家名恰好是某个 IP 的前缀时，不能被当成玩家名匹配走。
func (admin serverAdmin) resolveBanIP(target string) (string, error) {
	var parsed net.IP = net.ParseIP(strings.TrimSpace(target))
	if parsed != nil {
		return parsed.String(), nil
	}
	var online, err = admin.lookupOnline(target)
	if err != nil {
		return "", fmt.Errorf("%s 不是有效的 IP，也不是在线玩家: %w", target, err)
	}
	parsed = net.ParseIP(online.RemoteIP)
	if parsed == nil {
		return "", fmt.Errorf("拿不到 %s 的 IP", online.Username)
	}
	return parsed.String(), nil
}

// BanIP 封一个 IP，并把当前同 IP 的所有在线玩家一起踢掉。
func (admin serverAdmin) BanIP(target string, source string, reason string) (string, bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return "", false, err
	}
	var address string = ""
	address, err = admin.resolveBanIP(target)
	if err != nil {
		return "", false, err
	}
	var why string = strings.TrimSpace(reason)
	if why == "" {
		why = defaultBanText
	}
	var added bool = false
	added, err = admin.srv.access.BannedIPs.Add(address, source, why, time.Time{}, time.Now())
	if err != nil {
		return "", false, fmt.Errorf("写入封禁名单: %w", err)
	}
	var list []*Player = admin.srv.playerList()
	var index int = 0
	for index = 0; index < len(list); index++ {
		if net.ParseIP(list[index].RemoteIP).String() == address {
			admin.srv.kickPlayer(list[index], fmt.Sprintf(denyBannedIP, why))
		}
	}
	return address, added, nil
}

func (admin serverAdmin) Pardon(name string) (bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return false, err
	}
	var removed bool = false
	removed, err = admin.srv.access.BannedNames.Remove(name, time.Now())
	if err != nil {
		return false, fmt.Errorf("更新封禁名单: %w", err)
	}
	return removed, nil
}

func (admin serverAdmin) PardonIP(address string) (bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return false, err
	}
	var key string = strings.TrimSpace(address)
	var parsed net.IP = net.ParseIP(key)
	if parsed != nil {
		key = parsed.String()
	}
	var removed bool = false
	removed, err = admin.srv.access.BannedIPs.Remove(key, time.Now())
	if err != nil {
		return false, fmt.Errorf("更新封禁名单: %w", err)
	}
	return removed, nil
}

// changeOps 按 enable 选择加入或移出管理员名单。
func (admin serverAdmin) changeOps(target string, enable bool) (bool, error) {
	if enable {
		return admin.srv.access.Ops.Add(target)
	}
	return admin.srv.access.Ops.Remove(target)
}

func (admin serverAdmin) SetOp(name string, enable bool) (string, bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return "", false, err
	}
	var online *Player = nil
	online, err = admin.lookupOnline(name)
	if errors.Is(err, command.ErrAmbiguousName) {
		return "", false, err
	}
	var target string = strings.TrimSpace(name)
	if online != nil {
		target = online.Username
	}
	var changed bool = false
	changed, err = admin.changeOps(target, enable)
	if err != nil {
		return "", false, fmt.Errorf("更新管理员名单: %w", err)
	}
	if changed && online != nil {
		admin.srv.refreshPermissions(online, enable)
	}
	return target, changed, nil
}

func (admin serverAdmin) WhitelistEnabled() bool {
	return admin.srv.whitelistOn.Load()
}

// SetWhitelist 开关立即生效，并写回 server.properties 让重启后保持。
// 写文件失败时开关已经生效，错误信息要把这点说清楚，管理员才知道重启会还原。
func (admin serverAdmin) SetWhitelist(enabled bool) error {
	admin.srv.whitelistOn.Store(enabled)
	if admin.srv.cfg == nil || admin.srv.cfg.PropertiesPath == "" {
		return nil
	}
	var err error = config.SetProperty(admin.srv.cfg.PropertiesPath, "white-list", config.OnOff(enabled))
	if err != nil {
		return fmt.Errorf("开关已生效，但没能写入 server.properties（重启后会还原）: %w", err)
	}
	return nil
}

func (admin serverAdmin) WhitelistAdd(name string) (bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return false, err
	}
	return admin.srv.access.Whitelist.Add(name)
}

func (admin serverAdmin) WhitelistRemove(name string) (bool, error) {
	var err error = admin.accessReady()
	if err != nil {
		return false, err
	}
	return admin.srv.access.Whitelist.Remove(name)
}

func (admin serverAdmin) WhitelistNames() []string {
	if admin.srv.access == nil {
		return nil
	}
	return admin.srv.access.Whitelist.Names()
}

func (admin serverAdmin) WhitelistReload() error {
	var err error = admin.accessReady()
	if err != nil {
		return err
	}
	err = admin.srv.access.Whitelist.Load()
	if err != nil {
		return fmt.Errorf("读取白名单: %w", err)
	}
	return nil
}
