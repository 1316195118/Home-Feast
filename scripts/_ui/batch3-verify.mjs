// 第三批 + 优先推荐验收：实时重排 / 佐料不参与 / SSE 多端同步 / 输入保护 / 自动备份。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { pathToFileURL } from 'node:url'

const ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'

async function api(method, path, body) {
  const r = await fetch(BASE + path, {
    method, headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  return { status: r.status, data: await r.json().catch(() => ({})) }
}

async function openPage(port, width, height, hash) {
  const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-b3-${port}-${Date.now()}`
  const proc = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${PROFILE}`, 'about:blank',
  ], { stdio: 'ignore' })
  let targets
  for (let i = 0; i < 60; i++) {
    try { const r = await fetch(`http://127.0.0.1:${port}/json/list`); targets = await r.json(); if (targets.some(t => t.type === 'page')) break } catch {}
    await sleep(250)
  }
  const ws = new WebSocket(targets.find(t => t.type === 'page').webSocketDebuggerUrl)
  await new Promise((res, rej) => { ws.addEventListener('open', res); ws.addEventListener('error', rej) })
  let seq = 0
  const cdp = (m, p = {}) => new Promise((r) => {
    const id = ++seq
    const f = (e) => { const j = JSON.parse(e.data); if (j.id !== id) return; ws.removeEventListener('message', f); r(j.result) }
    ws.addEventListener('message', f)
    ws.send(JSON.stringify({ id, method: m, params: p }))
  })
  const ev = async (x) => (await cdp('Runtime.evaluate', { expression: x, returnByValue: true }))?.result?.value
  await cdp('Page.enable')
  const errors = []
  ws.addEventListener('message', (e) => {
    const j = JSON.parse(e.data)
    if (j.method === 'Runtime.exceptionThrown') errors.push('exception')
    if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') errors.push('console.error')
  })
  await cdp('Runtime.enable')
  await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: width < 500, screenWidth: width, screenHeight: height })
  await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#${hash}` })
  await sleep(3200)
  await ev(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(500)
  return { ev, errors, close: () => { ws.close(); proc.kill() } }
}

// ---------- 1. 纯函数：佐料不参与 ----------
const { pantryMatches } = await import(pathToFileURL(ROOT + 'web/src/utils.js').href)
const unit = pantryMatches({ ingredients: ['鸡蛋 2 个', '番茄 2 个', '盐 少许', '葱 1 根'] }, ['鸡蛋', '盐', '葱'])
console.log('佐料过滤单测:', JSON.stringify(unit), '| 只命中主料:', unit.hits.length === 1 && unit.hits[0] === '鸡蛋')

const boot = (await api('GET', '/api/bootstrap')).data
const today = boot.today
const memberId = boot.members[0]?.id || ''
const oldPantry = boot.settings.pantry || []
const dishA = boot.dishes.find(d => d.name === '宫保鸡丁') || boot.dishes[0]
const dishB = boot.dishes.find(d => d.name === '麻婆豆腐') || boot.dishes[1]
const beforeIds = new Set(((await api('GET', `/api/plans/${today}`)).data.orders || []).map(o => o.id))
const added = []

try {
  const A = await openPage(9961, 1280, 900, 'menu')
  const before = await A.ev(`[...document.querySelectorAll('.dish-name-link')].slice(0,5).map(x=>x.textContent.trim()).join(',')`)
  await A.ev(`[...document.querySelectorAll('.toolbar .chip')].find(b=>b.textContent.includes('优先推荐'))?.click()`)
  await sleep(500)
  const on = await A.ev(`JSON.stringify((()=>({
    pressed: document.querySelector('.toolbar .chip.active')?.textContent.trim()||'',
    tagged: document.querySelectorAll('.tag-pantry').length,
    first: [...document.querySelectorAll('.dish-name-link')].slice(0,5).map(x=>x.textContent.trim()).join(','),
  }))())`)
  console.log('\n优先推荐 默认前5:', before)
  console.log('开启后:', on)

  // 通过 API 改冰箱食材（模拟另一台设备），页面应经 SSE 自动重排，无需手动刷新
  await api('PUT', '/api/settings', { pantry: ['鸡蛋', '番茄', '土豆', '盐'] })
  await sleep(1800)
  const live = await A.ev(`JSON.stringify((()=>({
    tagged: document.querySelectorAll('.tag-pantry').length,
    firstTags: [...document.querySelectorAll('.dish-card')].slice(0,4).map(c=>c.querySelector('.tag-pantry')?.textContent.trim()||'-'),
    stored: document.querySelector('.menu-head-actions .btn')?.textContent.replace(/\\s+/g,' ').trim(),
  }))())`)
  console.log('改食材后实时变化:', live)

  // ---------- 2. SSE 多端同步 ----------
  const B = await openPage(9962, 390, 844, 'today')
  const ordersBefore = await B.ev(`document.querySelectorAll('.order-row').length`)
  await api('POST', `/api/plans/${today}/orders`, { dishId: dishA.id, userId: memberId, meal: 'dinner' })
  const afterPost = (await api('GET', `/api/plans/${today}`)).data
  for (const o of afterPost.orders || []) if (!beforeIds.has(o.id)) added.push(o.id)
  await sleep(2000)
  const ordersAfter = await B.ev(`document.querySelectorAll('.order-row').length`)
  console.log('\nSSE 另一设备点菜 -> B 页订单数', ordersBefore, '=>', ordersAfter, '| 自动同步:', ordersAfter > ordersBefore)

  // 输入保护：正在输入留言时，远端写操作不能覆盖草稿
  await B.ev(`(()=>{const i=document.querySelector('.note-row input'); if(i){i.focus(); i.value='测试草稿未保存'; i.dispatchEvent(new Event('input',{bubbles:true}));}})()`)
  await sleep(200)
  await api('POST', `/api/plans/${today}/orders`, { dishId: dishB.id, userId: memberId, meal: 'dinner' })
  const afterPost2 = (await api('GET', `/api/plans/${today}`)).data
  for (const o of afterPost2.orders || []) if (!beforeIds.has(o.id) && !added.includes(o.id)) added.push(o.id)
  await sleep(2000)
  const draft = await B.ev(`document.querySelector('.note-row input')?.value || ''`)
  console.log('远端写入时保留未保存草稿:', draft === '测试草稿未保存', '| 实际:', JSON.stringify(draft))
  console.log('B 页 JS 异常:', B.errors.length)
  A.close(); B.close()

  // ---------- 3. 自动备份 ----------
  const run = await api('POST', '/api/backups/run')
  const list = await api('GET', '/api/backups')
  console.log('\n自动备份 立即执行:', run.status, run.data.name, Math.round((run.data.size || 0) / 1024), 'KB')
  console.log('  备份列表:', (list.data || []).length, '份 | familyId:', boot.familyId || '(旧缓存)')
  const boot2 = (await api('GET', '/api/bootstrap')).data
  console.log('  bootstrap familyId:', boot2.familyId, '| autoBackup:', boot2.settings.autoBackup, '| keep:', boot2.settings.backupKeep)

  // 设置页卡片（桌面 + 手机）
  for (const [w, h, tag] of [[1280, 900, 'desktop'], [390, 844, 'phone']]) {
    const P = await openPage(9970 + (w < 500 ? 1 : 0), w, h, 'settings')
    await sleep(700)
    const s = JSON.parse(await P.ev(`JSON.stringify((()=>({
      backupCard: [...document.querySelectorAll('.setting-card')].some(c=>c.textContent.includes('自动备份')),
      runBtn: [...document.querySelectorAll('.btn')].some(b=>b.textContent.includes('立即备份')),
      realtime: [...document.querySelectorAll('.setting-card')].some(c=>c.textContent.includes('多人实时同步')),
      inputFont: getComputedStyle(document.querySelector('.setting-row-inline input')||document.body).fontSize,
      overflow: document.documentElement.scrollWidth>document.documentElement.clientWidth,
    }))())`))
    console.log(`${tag} 设置页 自动备份卡 ${s.backupCard} 立即备份按钮 ${s.runBtn} 实时同步状态 ${s.realtime} 输入字号 ${s.inputFont} 溢出 ${s.overflow} JS异常 ${P.errors.length}`)
    P.close()
  }
} finally {
  for (const id of added) await api('DELETE', `/api/plans/${today}/orders/${id}`)
  await api('PUT', '/api/settings', { pantry: oldPantry })
  console.log('\n临时订单与 pantry 已恢复')
}
console.log('DONE')
