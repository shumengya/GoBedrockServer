package access

import "fmt"
import "path/filepath"

const FileOps string = "ops.txt"
const FileWhitelist string = "white-list.txt"
const FileBannedNames string = "banned-players.txt"
const FileBannedIPs string = "banned-ips.txt"

// Access 汇总服务器的访问控制：管理员、白名单、封禁的玩家名、封禁的 IP。
// 四个文件放在服务器根目录，名字和格式都与参考端一致，可以直接拷过来用。
type Access struct {
	Ops         *NameList
	Whitelist   *NameList
	BannedNames *BanList
	BannedIPs   *BanList
}

// Open 读取四个文件。warnings 是被跳过的坏行，调用方应记日志；
// 文件存在却读不出来是 error：带着空封禁列表上线等于悄悄解封所有人。
func Open(dir string) (*Access, []string, error) {
	var access *Access = &Access{
		Ops:         NewNameList(filepath.Join(dir, FileOps)),
		Whitelist:   NewNameList(filepath.Join(dir, FileWhitelist)),
		BannedNames: NewBanList(filepath.Join(dir, FileBannedNames)),
		BannedIPs:   NewBanList(filepath.Join(dir, FileBannedIPs)),
	}
	var warnings []string = nil
	var err error = access.Ops.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("加载管理员名单: %w", err)
	}
	err = access.Whitelist.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("加载白名单: %w", err)
	}
	var skipped []string = nil
	skipped, err = access.BannedNames.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("加载封禁玩家: %w", err)
	}
	warnings = append(warnings, skipped...)
	skipped, err = access.BannedIPs.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("加载封禁 IP: %w", err)
	}
	warnings = append(warnings, skipped...)
	return access, warnings, nil
}
