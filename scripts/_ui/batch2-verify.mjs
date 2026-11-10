// 第二批功能验收：每周排班（设置/提醒/一键套用）+ 智能推荐（理由/换一批）。
// 先用 API 造好数据，再用 CDP 双视口检查 UI；BASE 可指向线上。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'

async function api(method, path, body) {
  const r = await fetch(BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  return { status: r.status, data: await r.json() }
}

const boot = (await api('GET', '/api/bootstrap')).data
const members = boot.members
const today = boot.today
const weekday = String(new Date(`${today}T12:00:00`).getDay())
const [a, b] = members
console.log(`今天 ${today} 星期${weekday} | 成员 ${members.map((m) => m.name).join('、')}`)

// 后端校验：合法排班 + 非法星期 + 不存在的成员 ID + pantry 去重限长
const put = await api('PUT', '/api/settings', {
  roster: {
    enabled: true,
    days: {
      [weekday]: { chefId: a.id, washId: b.id },
      '9': { chefId: a.id, washId: b.id },
      '2': { chefId: 'not-a-real-id', washId: b.id },
    },
  },
  pantry: ['鸡蛋', '番茄', '鸡蛋', '  ', '超长食材'.repeat(6)],
})
const saved = put.data
const days = saved.roster?.days || {}
console.log('PUT status', put.status, '| roster.enabled', saved.roster?.enabled, '| days keys', Object.keys(days).sort().join(','))
console.log('  非法星期被丢弃:', !('9' in days), '| 不存在的 chefId 被清空:', days['2']?.chefId === '', '| 合法 washId 保留:', days['2']?.washId === b.id)
console.log('  pantry 去重/去空/限长:', JSON.stringify(saved.pantry))
const check = await api('GET', '/api/bootstrap')
console.log('  持久化:', check.data.settings?.roster?.enabled === true, check.data.settings?.pantry?.length === saved.pantry.length)

async function run(width, height, tag, mobile) {
  const PORT = 9700 + (mobile ? 50 : 0) + Math.floor(Math.random() * 40)
  const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-b2-${tag}-${Date.now()}`
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
    ws.addEventListener('message', f)
    ws.send(JSON.stringify({ id, method: m, params: p }))
  })
  const ev = async (x) => (await cdp('Runtime.evaluate', { expression: x, returnByValue: true }))?.result?.value
  await cdp('Page.enable')
  const errors = []
  ws.addEventListener('message', (e) => {
    const j = JSON.parse(e.data)
    if (j.method === 'Runtime.exceptionThrown') errors.push(j.params?.exceptionDetails?.text || 'exception')
    // Vue 会捕获组件 setup 错误并打到 console.error，不触发 exceptionThrown，必须单独听
    if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') {
      errors.push((j.params.args || []).map((a) => a.value || a.description || '').join(' ').slice(0, 160))
    }
  })
  await cdp('Runtime.enable')
  await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height })
  await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
  await sleep(3000)
  await ev(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(500)

  console.log(`\n=== ${tag} ${width}x${height} ===`)
  const todayInfo = JSON.parse(await ev(`JSON.stringify((()=>({
    hint: document.querySelector('.roster-hint')?.textContent.replace(/\\s+/g,' ').trim() || '',
    applyBtn: !!document.querySelector('.roster-hint .btn'),
    suggestCards: document.querySelectorAll('.suggest-card').length,
    reasons: [...document.querySelectorAll('.suggest-reasons')].map(x=>x.textContent.trim()),
    names: [...document.querySelectorAll('.suggest-name')].map(x=>x.textContent.trim()),
  }))())`))
  console.log('今日页 排班提醒:', todayInfo.hint || '(无)')
  console.log('  一键套用按钮:', todayInfo.applyBtn, '| 推荐卡', todayInfo.suggestCards, '| 理由', JSON.stringify(todayInfo.reasons))
  const before = todayInfo.names.join(',')
  await ev(`[...document.querySelectorAll('.panel-head .btn')].find(b=>b.textContent.includes('换一批'))?.click()`)
  await sleep(500)
  const after = await ev(`[...document.querySelectorAll('.suggest-name')].map(x=>x.textContent.trim()).join(',')`)
  console.log('  换一批生效:', before !== after, '|', before, '=>', after)

  await ev(`window.location.hash='#settings'`)
  await sleep(700)
  const setInfo = JSON.parse(await ev(`JSON.stringify((()=>({
    hash: location.hash,
    cards: document.querySelectorAll('.setting-card').length,
    titles: [...document.querySelectorAll('.setting-title')].map(x=>x.textContent.replace(/\\s+/g,' ').trim()).slice(0,12),
    rosterBtn: [...document.querySelectorAll('.setting-card .btn')].some(b=>b.textContent.includes('管理每周排班')),
    pantryInSettings: [...document.querySelectorAll('.setting-card')].some(c=>c.textContent.includes('冰箱里有什么')),
    saveBtns: [...document.querySelectorAll('.setting-card .btn-primary')].map(b=>b.textContent.trim()),
    overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
    smallBtns: [...document.querySelectorAll('.btn,.chip,.tab')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.width<40||r.height<36)}).length,
  }))())`))
  console.log('设置页 hash', setInfo.hash, '| 卡片数', setInfo.cards, '| 标题', JSON.stringify(setInfo.titles))
  console.log('设置页 排班弹窗入口', setInfo.rosterBtn, '| 食材卡已移出设置页', !setInfo.pantryInSettings, '| 保存按钮', JSON.stringify(setInfo.saveBtns))
  console.log('  横向溢出', setInfo.overflow, '| 过小控件', setInfo.smallBtns, '| JS 异常', errors.length)
  ws.close(); proc.kill()
}

await run(1280, 900, 'desktop', false)
await run(390, 844, 'phone', true)
console.log('\nDONE')
