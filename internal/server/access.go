package server

import "fmt"
import "time"

import "go-mc-server/internal/access"
import "go-mc-server/internal/logger"

// 登录被拒时显示给玩家的提示。
const denyBannedName string = "你已被服务器封禁。原因: %s"
const denyBannedIP string = "你的 IP 已被服务器封禁。原因: %s"
const denyNotWhitelisted string = "服务器已开启白名单，你不在名单中"

// openAccess 读取 ops / 白名单 / 封禁四个文件。读不出来直接中止启动：
// 带着空封禁列表上线等于悄悄把所有人解封。
func openAccess(dir string) *access.Access {
	var controls, warnings, err = access.Open(dir)
	if err != nil {
		logger.Fatal("server", "加载访问控制文件失败: %v", err)
	}
	var index int = 0
	for index = 0; index < len(warnings); index++ {
		logger.Warn("server", "访问控制: 跳过无法解析的行 %s", warnings[index])
	}
	if len(controls.Ops.Names()) == 0 {
		logger.Info("server", "当前没有任何管理员；在控制台输入 op <玩家名> 添加")
	}
	return controls
}

// isOp 报告名字是否在管理员名单里；没挂访问控制（测试里直接构造的 Server）时一律不是。
// 注意：离线模式下名字由客户端自报，没有验证，任何人都能冒用管理员的名字登录。
func (srv *Server) isOp(name string) bool {
	return srv.access != nil && srv.access.Ops.Has(name)
}

// loginDenyReason 返回拒绝登录的原因，空串表示放行。
// 检查顺序：名字封禁 → IP 封禁 → 白名单；管理员不受白名单限制，但仍受封禁约束。
func (srv *Server) loginDenyReason(name string, ip string, now time.Time) string {
	if srv.access == nil {
		return ""
	}
	var nameBan, nameBanned = srv.access.BannedNames.Find(name, now)
	if nameBanned {
		return fmt.Sprintf(denyBannedName, nameBan.Reason)
	}
	var ipBan, ipBanned = srv.access.BannedIPs.Find(ip, now)
	if ipBanned {
		return fmt.Sprintf(denyBannedIP, ipBan.Reason)
	}
	if srv.whitelistOn.Load() && !srv.access.Whitelist.Has(name) && !srv.access.Ops.Has(name) {
		return denyNotWhitelisted
	}
	return ""
}
