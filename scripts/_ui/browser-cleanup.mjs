import { spawn } from 'node:child_process'

// Edge/Chrome 会派生 GPU、渲染和网络子进程。只结束父进程会留下后台树，
// 连续跑性能用例时会叠加上百个进程，最后把被测页面本身拖慢。
export function killProcessTree(proc) {
  if (!proc?.pid || proc.killed) return
  if (process.platform === 'win32') {
    spawn('taskkill', ['/pid', String(proc.pid), '/T', '/F'], {
      stdio: 'ignore',
      windowsHide: true,
    })
    return
  }
  proc.kill('SIGKILL')
}
