// 分工选择验收：掌勺/洗碗两个 tab 切换、选中反馈动画、已定成员回显。
// 用法：先启动服务端，再 node scripts/_ui/duty-verify.mjs；可设 BASE 指向线上。
// 注意：脚本会临时设置再取消两个分工，结束时恢复为空。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const PORT = 9977
const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-duty-${Date.now()}`
const proc = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`, 'about:blank',
], { stdio: 'ignore' })
let targets
for (let i = 0; i < 60; i++) {
  try { const r = await fetch(`http://127.0.0.1:${PORT}/json/list`); targets = await r.json(); if (targets.some((t) => t.type === 'page')) break } catch {}
  await sleep(250)
}
const ws = new WebSocket(targets.find((t) => t.type === 'page').webSocketDebuggerUrl)
await new Promise((res, rej) => { ws.addEventListener('open', res); ws.addEventListener('error', rej) })
let seq = 0
const cdp = (m, p = {}) => new Promise((r) => {
  const id = ++seq
  const f = (e) => { const j = JSON.parse(e.data); if (j.id !== id) return; ws.removeEventListener('message', f); r(j.result) }
  ws.addEventListener('message', f); ws.send(JSON.stringify({ id, method: m, params: p }))
})
const ev = async (x) => (await cdp('Runtime.evaluate', { expression: x, returnByValue: true, awaitPromise: true }))?.result?.value
const errors = []
ws.addEventListener('message', (e) => { const j = JSON.parse(e.data); if (j.method === 'Runtime.exceptionThrown') errors.push('exc'); if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') errors.push('console') })
await cdp('Runtime.enable')
await cdp('Page.enable')
await cdp('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true, screenWidth: 390, screenHeight: 844 })
await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
await sleep(3200)
await ev(`document.querySelector('.user-pick-item')?.click()`)
await sleep(500)

const snap = () => ev(`JSON.stringify((()=>({
  tabs:[...document.querySelectorAll('.duty-role')].map(t=>({cls:t.className.includes('active')?'active':'',who:t.querySelector('.duty-role-who')?.textContent.trim()})),
  label:document.querySelector('.duty-member-label')?.textContent.trim().slice(0,20),
  members:[...document.querySelectorAll('.duty-member')].map(m=>m.textContent.trim()),
  pulse:document.querySelector('.duty-pulse')?.textContent.trim()||'',
}))())`)

console.log('初始:', await snap())
await ev(`document.querySelectorAll('.duty-role')[1]?.click()`)
await sleep(350)
console.log('切洗碗:', await snap())
await ev(`document.querySelector('.duty-member')?.click()`)
await sleep(350)
console.log('选洗碗的人:', await snap())
await ev(`document.querySelectorAll('.duty-role')[0]?.click()`)
await sleep(350)
await ev(`document.querySelectorAll('.duty-member')[1]?.click()`)
await sleep(350)
console.log('选掌勺的人:', await snap())
console.log('JS 异常:', errors.length)
// 清理：取消两个分工
await ev(`document.querySelector('.duty-member.active')?.click()`); await sleep(200)
await ev(`document.querySelectorAll('.duty-role')[1]?.click()`); await sleep(200)
await ev(`document.querySelector('.duty-member.active')?.click()`); await sleep(200)
console.log('已清理分工')
ws.close(); proc.kill()
process.exit(0)
