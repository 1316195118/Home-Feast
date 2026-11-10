// 分工认领验收：「我来掌勺 / 我来洗碗」两个按钮用当前身份认领、再点取消、文字动画反馈。
// 用法：先启动服务端，再 node scripts/_ui/duty-verify.mjs；可设 BASE 指向线上。
// 脚本先记下当前身份与分工，测完恢复，不影响线上数据。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
async function api(method, path, body) {
  const r = await fetch(BASE + path, {
    method, headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  return (await r.json().catch(() => ({})))
}
const boot = await api('GET', '/api/bootstrap')
const today = boot.today
const beforePlan = await api('GET', `/api/plans/${today}`)
const backup = { chefId: beforePlan.chefId || '', dishwasherId: beforePlan.dishwasherId || '' }
console.log('测试前分工:', JSON.stringify(backup))

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
  me: document.querySelector('.user-switch select')?.value || document.querySelector('.user-switch')?.textContent.trim().slice(0,12),
  claims:[...document.querySelectorAll('.duty-claim')].map(c=>({
    label:c.querySelector('b')?.textContent.trim(),
    status:c.querySelector('.duty-claim-status')?.textContent.trim(),
    active:c.className.includes('active'),
    mine:!!c.querySelector('.duty-claim-check'),
  })),
  pulse:document.querySelector('.duty-pulse')?.textContent.trim()||'',
  summary:document.querySelector('.duty-summary')?.textContent.trim(),
}))())`)

console.log('初始:', await snap())
await ev(`document.querySelector('.duty-claim-chef')?.click()`)
await sleep(350)
console.log('点我来掌勺:', await snap())
await ev(`document.querySelector('.duty-claim-wash')?.click()`)
await sleep(350)
console.log('点我来洗碗:', await snap())
await ev(`document.querySelector('.duty-claim-chef')?.click()`)
await sleep(350)
console.log('再点掌勺取消:', await snap())
console.log('JS 异常:', errors.length)

// 恢复分工
await api('PUT', `/api/plans/${today}`, { chefId: backup.chefId, dishwasherId: backup.dishwasherId })
console.log('已恢复分工')
ws.close(); proc.kill()
process.exit(0)
