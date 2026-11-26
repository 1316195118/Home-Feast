// 撤菜实时回归：菜单行、首页封面和统计必须随同一次操作立即更新。
// 用法：启动服务端后 node scripts/_ui/order-realtime-verify.mjs
// 测试会临时加入一道未点过的菜，结束后无论成功失败都会尝试清理。
import { spawn } from 'node:child_process'
import path from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const CLIENT_ID = `realtime-remove-${Date.now()}`

async function api(pathname, options = {}) {
  const response = await fetch(`${BASE}${pathname}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Client-ID': CLIENT_ID,
      ...(options.headers || {}),
    },
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || `${pathname} 请求失败 (${response.status})`)
  return data
}

const initial = await api('/api/bootstrap')
const date = initial.today
const member = initial.members[0]
if (!member) throw new Error('没有可用的家庭成员')
const existingIds = new Set((initial.plan?.orders || []).map((order) => order.dishId))
const dish = initial.dishes.find((item) => !existingIds.has(item.id))
if (!dish) throw new Error('没有可用于测试的菜品')

let orderId = ''
let proc
let ws
try {
  const created = await api(`/api/plans/${date}/orders`, {
    method: 'POST',
    body: JSON.stringify({ dishId: dish.id, userId: member.id }),
  })
  orderId = created.orders.find((order) => order.dishId === dish.id)?.id || ''
  if (!orderId) throw new Error('临时订单没有创建成功')

  const port = 9800 + Math.floor(Math.random() * 100)
  const profile = path.join(process.env.TEMP || '.', `fm-real-remove-${Date.now()}`)
  proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })

  let targets
  for (let i = 0; i < 70; i++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch {}
    await sleep(200)
  }
  if (!targets) throw new Error('Edge 调试端口未就绪')

  ws = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve)
    ws.addEventListener('error', reject)
  })

  let seq = 0
  const pending = new Map()
  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data)
    if (!message.id || !pending.has(message.id)) return
    const { resolve, reject } = pending.get(message.id)
    pending.delete(message.id)
    if (message.error) reject(new Error(message.error.message))
    else resolve(message.result)
  })
  const cdp = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq
    pending.set(id, { resolve, reject })
    ws.send(JSON.stringify({ id, method, params }))
  })
  const evaluate = async (expression) => {
    const result = await cdp('Runtime.evaluate', { expression, returnByValue: true })
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.text || '页面脚本执行失败')
    return result.result?.value
  }

  await cdp('Page.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 390, height: 844, deviceScaleFactor: 2, mobile: true,
    screenWidth: 390, screenHeight: 844,
  })
  await cdp('Page.addScriptToEvaluateOnNewDocument', {
    source: `(() => {
      const originalFetch = window.fetch.bind(window);
      window.fetch = async (...args) => {
        const response = await originalFetch(...args);
        const request = args[0] instanceof Request ? args[0] : new Request(args[0], args[1]);
        if (request.method === 'DELETE' && request.url.includes('/orders/')) {
          response.clone().json().then((data) => { window.__lastDeleteResponse = data }).catch(() => {});
        }
        return response;
      };
    })()`,
  })
  await cdp('Page.navigate', { url: `${BASE}/?realtime=${Date.now()}#today` })
  await sleep(2800)
  await evaluate(`document.querySelector('.user-pick-item')?.click()`)
  for (let i = 0; i < 40; i++) {
    if (await evaluate(`!!document.querySelector('main h1')`)) break
    await sleep(100)
  }

  const before = JSON.parse(await evaluate(`JSON.stringify((()=>{
    const rows=[...document.querySelectorAll('.order-row')];
    const row=rows.find((item)=>item.querySelector('.order-name')?.textContent.trim()===${JSON.stringify(dish.name)});
    return {
      found:!!row,
      count:document.querySelector('.stat-item .stat-num')?.textContent.trim()||'',
      photos:[...document.querySelectorAll('.hero-photo-card img')].map((img)=>img.getAttribute('src')),
      suggestions:[...document.querySelectorAll('.suggest-name')].map((el)=>el.textContent.trim()),
      heroTitle:document.querySelector('.hero-title')?.textContent.trim()||'',
      heroSub:document.querySelector('.hero-sub')?.textContent.trim()||'',
    };
  })())`))
  if (!before.found) throw new Error(`页面没有显示临时订单：${dish.name}`)

  await evaluate(`(()=>{
    const rows=[...document.querySelectorAll('.order-row')];
    const row=rows.find((item)=>item.querySelector('.order-name')?.textContent.trim()===${JSON.stringify(dish.name)});
    window.__removeStarted=performance.now();
    row?.querySelector('.order-actions button:last-child')?.click();
  })()`)

  let rowGoneMs = null
  let heroUpdatedMs = null
  for (let i = 0; i < 80; i++) {
    const snapshot = JSON.parse(await evaluate(`JSON.stringify((()=>{
      const rows=[...document.querySelectorAll('.order-row')];
      const photos=[...document.querySelectorAll('.hero-photo-card img')].map((img)=>img.getAttribute('src'));
      const suggestions=[...document.querySelectorAll('.suggest-name')].map((el)=>el.textContent.trim());
      return {
        rowGone:!rows.some((item)=>item.querySelector('.order-name')?.textContent.trim()===${JSON.stringify(dish.name)}),
        photos,
        suggestions,
        elapsed:performance.now()-window.__removeStarted,
      };
    })())`))
    if (rowGoneMs === null && snapshot.rowGone) rowGoneMs = Math.round(snapshot.elapsed)
    const photosChanged = JSON.stringify(snapshot.photos) !== JSON.stringify(before.photos)
    const suggestionsChanged = JSON.stringify(snapshot.suggestions) !== JSON.stringify(before.suggestions)
    if (heroUpdatedMs === null && (photosChanged || suggestionsChanged)) {
      heroUpdatedMs = Math.round(snapshot.elapsed)
    }
    if (rowGoneMs !== null && heroUpdatedMs !== null) break
    await sleep(50)
  }

  const after = JSON.parse(await evaluate(`JSON.stringify((()=>{
    const rows=[...document.querySelectorAll('.order-row')];
    return {
      rowGone:!rows.some((item)=>item.querySelector('.order-name')?.textContent.trim()===${JSON.stringify(dish.name)}),
      count:document.querySelector('.stat-item .stat-num')?.textContent.trim()||'',
      photos:[...document.querySelectorAll('.hero-photo-card img')].map((img)=>img.getAttribute('src')),
      suggestions:[...document.querySelectorAll('.suggest-name')].map((el)=>el.textContent.trim()),
      heroTitle:document.querySelector('.hero-title')?.textContent.trim()||'',
      heroSub:document.querySelector('.hero-sub')?.textContent.trim()||'',
      toast:document.querySelector('.toast')?.textContent.trim()||'',
      deleteResponse:window.__lastDeleteResponse||null,
    };
  })())`))
  const server = await api('/api/bootstrap')
  const serverGone = !(server.plan?.orders || []).some((order) => order.id === orderId)

  console.log(JSON.stringify({
    dish: dish.name,
    rowGoneMs,
    heroUpdatedMs,
    before,
    after,
    serverGone,
  }, null, 2))

  const failures = []
  if (!after.rowGone) failures.push('菜单行未移除')
  if (rowGoneMs === null || rowGoneMs > 500) failures.push('菜单行没有在 500ms 内更新')
  if (after.count !== '0') failures.push(`菜单统计没有归零：${after.count}`)
  if (after.heroSub === before.heroSub) failures.push('首页首屏状态没有随撤菜更新')
  if (!Array.isArray(after.deleteResponse?.orders)) failures.push('撤菜接口仍返回 orders: null')
  if (!serverGone) failures.push('服务端订单未移除')
  if (failures.length) {
    console.error(`FAIL: ${failures.join('；')}`)
    process.exitCode = 1
  } else {
    console.log('PASS: 撤菜后菜单行、首页封面与服务端状态已同步更新')
  }
} finally {
  try {
    if (orderId) {
      const latest = await api('/api/bootstrap')
      if ((latest.plan?.orders || []).some((order) => order.id === orderId)) {
        await api(`/api/plans/${date}/orders/${orderId}`, { method: 'DELETE' })
      }
    }
  } catch {}
  ws?.close()
  killProcessTree(proc)
}
