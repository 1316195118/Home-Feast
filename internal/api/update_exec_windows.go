//go:build windows

package api

import "errors"

// Windows 本地开发用：进程原地替换在 Windows 上语义不同，直接拒绝更新接口调用。
func execBinary(path string, args []string, env []string) error {
	return errors.New("Windows 不支持在线自更新，请在 Linux 容器中使用")
}
