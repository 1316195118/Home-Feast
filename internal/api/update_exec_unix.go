//go:build unix

package api

import "syscall"

// execBinary 用新程序原地替换当前进程：PID 不变，容器 PID1 不会退出。
func execBinary(path string, args []string, env []string) error {
	return syscall.Exec(path, args, env)
}
