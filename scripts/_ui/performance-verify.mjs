// 首屏请求数 + 批量事务 + 手机端真实交互回归。
// 用法：先启动临时服务端，再 BASE=http://127.0.0.1:18092 node scripts/_ui/performance-verify.mjs
import fs from 'node:fs/promises'
import path from 'node:path'
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const PORT = 9860 + Math.floor(Math.random() * 80)
const PROFILE = path.join(process.env.TEMP || '.', `fm-perf-ui-${Date.now()}`)
const proc = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`, 'about:blank',
], { stdio: 'ignore' })

const results = []
const check = (label, ok, detail = '') => {
  results.push({ label, ok: Boolean(ok), detail })
  console.log(`${ok ? '  ✓' : '  ✗'} ${label}${detail ? `：${detail}` : ''}`)
}

let targets
for (let i = 0; i < 80; i++) {
  try {
    const response = await fetch(`http://127.0.0.1:${PORT}/json/list`)
    targets = await response.json()
    if (targets.some((target) => target.type === 'page')) break
  } catch {}
  await sleep(200)
}
if (!targets) throw new Error('Edge 调试端口未就绪')

const ws = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  ws.addEventListener('open', resolve)
  ws.addEventListener('error', reject)
})

let seq = 0
const pending = new Map()
const socketEvents = []
ws.addEventListener('message', (event) => {
  const message = JSON.parse(event.data)
  if (message.id && pending.has(message.id)) {
    pending.get(message.id)(message)
    pending.delete(message.id)
  } else if (message.method) {
    socketEvents.push(message)
  }
})
const cdp = (method, params = {}) => new Promise((resolve, reject) => {
  const id = ++seq
  pending.set(id, (message) => {
    if (message.error) reject(new Error(`${method}: ${message.error.message}`))
    else resolve(message.result)
  })
  ws.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression, awaitPromise = false) => {
  const result = await cdp('Runtime.evaluate', {
    expression, returnByValue: true, awaitPromise,
  })
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || '页面脚本执行失败')
  }
  return result.result?.value
}
const waitFor = async (expression, timeout = 8000) => {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    if (await evaluate(expression)) return true
    await sleep(100)
  }
  return false
}
const clickText = (selector, text) => evaluate(`(()=>{
  const el=[...document.querySelectorAll(${JSON.stringify(selector)})]
    .find((node)=>node.textContent.includes(${JSON.stringify(text)}));
  if(!el)return false; el.click(); return true;
})()`)
const openTab = async (label) => {
  await clickText('.tab', label)
  await waitFor(`!document.querySelector('.page-enter-active,.page-leave-active') && !!document.querySelector('main .section-title')`)
  await sleep(100)
}

const initial = await fetch(`${BASE}/api/bootstrap`).then((response) => response.json())
const initialToday = new Set((initial.plan?.orders || []).map((order) => order.id))

try {
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  await cdp('Network.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 390, height: 844, deviceScaleFactor: 2, mobile: true,
    screenWidth: 390, screenHeight: 844,
  })
  await cdp('Page.navigate', { url: `${BASE}/?verify=${Date.now()}#menu` })
  check('手机首屏可见', await waitFor(`!!document.querySelector('.tabs')`, 10000))

  if (await evaluate(`!!document.querySelector('.user-pick-item')`)) {
    await evaluate(`document.querySelector('.user-pick-item').click()`)
    await sleep(250)
  }
  await waitFor(`!!document.querySelector('.dish-grid')`)

  const requests = socketEvents
    .filter((event) => event.method === 'Network.requestWillBeSent')
    .map((event) => event.params.request)
  const bootstrapRequests = requests.filter((request) => new URL(request.url).pathname === '/api/bootstrap')
  const configRequests = requests.filter((request) => new URL(request.url).pathname === '/api/config')
  check('首屏只发一次 bootstrap', bootstrapRequests.length === 1, `${bootstrapRequests.length} 次`)
  check('首屏不再串行请求 config', configRequests.length === 0, `${configRequests.length} 次`)

  check('菜谱多选入口可用', await clickText('button', '多选'))
  await sleep(200)
  const picked = await evaluate(`(()=>{
    const photos=[...document.querySelectorAll('.dish-card.is-multi .dish-photo')].slice(0,2);
    photos.forEach((photo)=>photo.click()); return photos.length;
  })()`)
  check('手机可选中多道菜', picked === 2, `${picked} 道`)
  await sleep(250)
  console.log('  批量条状态', await evaluate(`(()=>{
    const bar=document.querySelector('.multi-bar'); const button=bar?.querySelector('.btn-primary');
    return JSON.stringify({count:bar?.querySelector('.multi-count')?.textContent.trim(),disabled:button?.disabled,text:button?.textContent.trim()})
  })()`))
  const batchReady = await evaluate(`(()=>{
    const bar=document.querySelector('.multi-bar'); const button=bar?.querySelector('.btn-primary');
    return !!bar && !!button && !button.disabled && bar.querySelector('.multi-count')?.textContent.includes('2')
  })()`)
  check('批量加入按钮已启用', batchReady)
  check('批量加入按钮可用', await clickText('.multi-bar button', '加入今日菜单'))
  const batchToast = await waitFor(`[...document.querySelectorAll('.toast')].some((node)=>node.textContent.includes('一次性'))`)
  check('批量点菜完成反馈', batchToast,
    await evaluate(`document.querySelector('.toast')?.textContent.trim()||'(无提示)'`))
  const postBatch = socketEvents.filter((event) =>
    event.method === 'Network.requestWillBeSent'
    && event.params.request.method === 'POST'
    && new URL(event.params.request.url).pathname.endsWith('/orders/batch'))
  check('多道菜只发一次批量写请求', postBatch.length === 1, `${postBatch.length} 次`)

  await openTab('今天')
  console.log('  当前 API 请求', socketEvents
    .filter((event) => event.method === 'Network.requestWillBeSent')
    .map((event) => {
      const url = new URL(event.params.request.url)
      return url.pathname.startsWith('/api/') ? `${event.params.request.method} ${url.pathname}` : ''
    })
    .filter(Boolean)
    .slice(-8)
    .join(' | '))
  const finish = await clickText('.panel-head-actions button', '全部上桌')
  check('全部上桌入口可用', finish)
  if (finish) {
    await waitFor(`[...document.querySelectorAll('.toast')].some((node)=>node.textContent.includes('全部上桌'))`)
  }
  const putBatch = socketEvents.filter((event) =>
    event.method === 'Network.requestWillBeSent'
    && event.params.request.method === 'PUT'
    && new URL(event.params.request.url).pathname.endsWith('/orders/batch'))
  check('全部上桌只发一次批量写请求', putBatch.length === 1, `${putBatch.length} 次`)

  await openTab('记录')
  // 上桌事务返回后，记录页还要完成一次响应式渲染。固定只等 100ms
  // 会在慢机器上抢跑，误报「再来一桌」不存在。
  await waitFor(`!!document.querySelector('.history-reorder')`, 5000)
  const reorder = await clickText('.history-reorder', '再来一桌')
  check('历史记录「再来一桌」可用', reorder)
  if (reorder) {
    await waitFor(`[...document.querySelectorAll('.toast')].some((node)=>node.textContent.includes('加回'))`)
  }

  const allPostBatch = socketEvents.filter((event) =>
    event.method === 'Network.requestWillBeSent'
    && event.params.request.method === 'POST'
    && new URL(event.params.request.url).pathname.endsWith('/orders/batch'))
  check('「再来一桌」也走单次事务', allPostBatch.length === 2, `累计 ${allPostBatch.length} 次`)

  const fit = JSON.parse(await evaluate(`JSON.stringify({
    width:innerWidth,
    scrollWidth:document.documentElement.scrollWidth,
    clientWidth:document.documentElement.clientWidth,
    title:document.title,
    toast:document.querySelector('.toast')?.textContent.trim()||''
  })`))
  check('390px 手机端无横向溢出', fit.scrollWidth <= fit.clientWidth, `${fit.scrollWidth}/${fit.clientWidth}`)

  const shot = await cdp('Page.captureScreenshot', { format: 'png', fromSurface: true })
  const shotPath = path.join(process.env.TEMP || '.', 'family-menu-performance-390.png')
  await fs.writeFile(shotPath, Buffer.from(shot.data, 'base64'))
  console.log(`  截图：${shotPath}`)

  const final = await fetch(`${BASE}/api/bootstrap`).then((response) => response.json())
  const added = (final.plan?.orders || []).filter((order) => !initialToday.has(order.id))
  await Promise.all(added.map((order) => fetch(
    `${BASE}/api/plans/${final.today}/orders/${order.id}`,
    { method: 'DELETE' },
  )))
  check('临时订单已清理', added.length > 0, `${added.length} 条`)

  const errors = socketEvents.filter((event) =>
    event.method === 'Runtime.exceptionThrown'
    || (event.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(event.params.type)))
  check('浏览器无 error / warn', errors.length === 0, `${errors.length} 条`)
} finally {
  const failed = results.filter((result) => !result.ok)
  console.log(`\n结果：${results.length - failed.length}/${results.length} 通过，${failed.length} 失败`)
  ws.close()
  killProcessTree(proc)
  if (failed.length) process.exitCode = 1
}
