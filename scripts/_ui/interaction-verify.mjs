// 交互回归：食材搜索/分类跳转/清空、骰子弹窗去重与高度、设置页版权折叠。
// 用法：启动服务端后 node scripts/_ui/interaction-verify.mjs（可设 BASE 指向线上）。
// 只改 UI 草稿，不保存设置；结束不影响线上数据。
// 验证本轮新增交互：食材搜索/分类跳转/清空、骰子弹窗去重文本、设置页折叠
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const PORT = 9961
const proc = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`, `--user-data-dir=C:/Users/Administrator/AppData/Local/Temp/fm-vn-${Date.now()}`, 'about:blank',
], { stdio: 'ignore' })
let targets
for (let i = 0; i < 60; i++) {
  try { const r = await fetch(`http://127.0.0.1:${PORT}/json/list`); targets = await r.json(); if (targets.some((t) => t.type === 'page')) break } catch {}
  await sleep(250)
}
const ws = new WebSocket(targets.find((t) => t.type === 'page').webSocketDebuggerUrl)
await new Promise((r) => ws.addEventListener('open', r))
let seq = 0
const cdp = (m, p = {}) => new Promise((r) => {
  const id = ++seq
  const f = (e) => { const j = JSON.parse(e.data); if (j.id !== id) return; ws.removeEventListener('message', f); r(j.result) }
  ws.addEventListener('message', f); ws.send(JSON.stringify({ id, method: m, params: p }))
})
const ev = async (x) => (await cdp('Runtime.evaluate', { expression: x, returnByValue: true }))?.result?.value
const errors = []
ws.addEventListener('message', (e) => { const j = JSON.parse(e.data); if (j.method === 'Runtime.exceptionThrown') errors.push('exc'); if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') errors.push('console') })
await cdp('Page.enable'); await cdp('Runtime.enable')
await cdp('Emulation.setDeviceMetricsOverride', { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false })
await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#menu` })
await sleep(2800); await ev(`document.querySelector('.user-pick-item')?.click()`); await sleep(500)

// 食材面板
await ev(`[...document.querySelectorAll('.btn')].find(b=>b.textContent.includes('冰箱食材'))?.click()`); await sleep(700)
const base = await ev(`JSON.stringify({groups:document.querySelectorAll('.pantry-section').length,cards:document.querySelectorAll('.pantry-card').length,jumps:document.querySelectorAll('.pantry-jumps .chip').length,fb:document.querySelector('.pantry-feedback')?.textContent.trim()})`)
console.log('食材初始', base)
// 搜索「排骨」
await ev(`(()=>{const i=document.querySelector('.pantry-toolbar .input');const s=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;s.call(i,'排骨');i.dispatchEvent(new Event('input',{bubbles:true}))})()`)
await sleep(500)
console.log('搜索排骨', await ev(`JSON.stringify({groups:document.querySelectorAll('.pantry-section').length,cards:document.querySelectorAll('.pantry-card').length,names:[...document.querySelectorAll('.pantry-card-name')].map(x=>x.textContent)})`))
// 分类跳转（应清空搜索）
await ev(`document.querySelectorAll('.pantry-jumps .chip')[2]?.click()`); await sleep(600)
console.log('跳转蔬菜后搜索框', await ev(`document.querySelector('.pantry-toolbar .input')?.value`))
// 清空已选
await ev(`[...document.querySelectorAll('.pantry-toolbar .btn')].find(b=>b.textContent.includes('清空'))?.click()`); await sleep(400)
console.log('清空后反馈', await ev(`document.querySelector('.pantry-feedback')?.textContent.trim()`))
await ev(`document.querySelector('.pantry-modal .icon-btn')?.click()`); await sleep(300)

// 骰子弹窗去重
await ev(`window.location.hash='#today'`); await sleep(700)
await ev(`document.querySelector('.pk-cta-btn')?.click()`); await sleep(700)
console.log('骰子弹窗', await ev(`JSON.stringify((()=>{const m=document.querySelector('.pk-modal');const t=m?.textContent||'';return {h:Math.round(m?.getBoundingClientRect().height||0),hasWho:t.includes('正在摇'),hasSwitchHint:t.includes('点右上角「我是」切到'),desc:(m?.querySelector('.setting-desc')?.textContent||'').trim(),rows:document.querySelectorAll('.pk-row').length}})())`))
await ev(`document.querySelector('.pk-modal .icon-btn:last-child')?.click()`); await sleep(300)

// 设置页折叠
await ev(`window.location.hash='#settings'`); await sleep(700)
console.log('关于折叠', await ev(`JSON.stringify({details:!!document.querySelector('.setting-details'),open:document.querySelector('.setting-details')?.open,sub:document.querySelector('.settings-grid .section-sub')?.textContent.trim()})`))
console.log('JS 异常', errors.length)
ws.close(); proc.kill(); process.exit(0)
