// 本轮 UI 调整验收：设置卡片统一 / 排班弹窗 / 冰箱食材面板与效果图 /
// 骰子 CTA / 购物清单用量 / 双视口布局。会临时改设置与订单，结束时恢复。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
async function api(method, path, body) {
  const r = await fetch(BASE + path, {
    method, headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  return { status: r.status, data: await r.json().catch(() => ({})) }
}

const boot = (await api('GET', '/api/bootstrap')).data
const today = boot.today
const memberId = boot.members[0]?.id || ''
const dish = boot.dishes.find((d) => d.name === '宫保鸡丁') || boot.dishes[0]
const before = (await api('GET', `/api/plans/${today}`)).data
const beforeIds = new Set((before.orders || []).map((o) => o.id))
const oldPantry = boot.settings.pantry || []
const added = []
console.log(`今天 ${today} | 临时测试菜 ${dish.name}`)

try {
  // 造一条订单，让购物清单有用量可显示
  await api('POST', `/api/plans/${today}/orders`, { dishId: dish.id, userId: memberId, meal: 'dinner' })
  const after = (await api('GET', `/api/plans/${today}`)).data
  for (const o of after.orders || []) if (!beforeIds.has(o.id)) added.push(o.id)
  console.log('临时订单', added.length ? added.join(',') : '(该菜今天已点，复用现有订单)')

  async function run(width, height, tag, mobile) {
    const PORT = 9900 + (mobile ? 40 : 0) + Math.floor(Math.random() * 30)
    const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-r3-${tag}-${Date.now()}`
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
      if (j.method === 'Runtime.exceptionThrown') errors.push('exception')
      if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') errors.push('console.error')
    })
    await cdp('Runtime.enable')
    await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height })
    await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
    await sleep(3000)
    await ev(`document.querySelector('.user-pick-item')?.click()`)
    await sleep(450)

    console.log(`\n=== ${tag} ${width}x${height} ===`)
    // 骰子 CTA
    let s = JSON.parse(await ev(`JSON.stringify((()=>({
      cta: !!document.querySelector('.pk-cta'),
      btn: document.querySelector('.pk-cta-btn')?.textContent.trim() || '',
      inWashLabel: [...document.querySelectorAll('.duty-label')].some(x=>x.textContent.includes('摇骰子')),
    }))())`))
    console.log('骰子 CTA 独立存在:', s.cta, '| 按钮:', s.btn, '| 已不在「谁洗碗」标题里:', !s.inWashLabel)

    // 购物清单用量
    await ev(`[...document.querySelectorAll('.btn')].find(b=>b.textContent.includes('购物清单'))?.click()`)
    await sleep(600)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      open: !!document.querySelector('.shopping-item'),
      rows: document.querySelectorAll('.shopping-item').length,
      amounts: [...document.querySelectorAll('.shopping-amount')].map(x=>x.textContent.trim()).slice(0,8),
    }))())`))
    console.log('购物清单 行数', s.rows, '| 用量', JSON.stringify(s.amounts))
    await ev(`[...document.querySelectorAll('.setting-actions .btn')].find(b=>b.textContent.includes('完成'))?.click()`)
    await sleep(300)

    // 菜谱页：冰箱食材入口 + 效果图卡片
    await ev(`window.location.hash='#menu'`)
    await sleep(700)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      entry: [...document.querySelectorAll('.menu-head-actions .btn')].some(b=>b.textContent.includes('冰箱食材')),
    }))())`))
    console.log('菜谱标题后冰箱食材入口:', s.entry)
    await ev(`[...document.querySelectorAll('.menu-head-actions .btn')].find(b=>b.textContent.includes('冰箱食材'))?.click()`)
    await sleep(500)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      cards: document.querySelectorAll('.pantry-card').length,
      withArt: document.querySelectorAll('.pantry-card .pantry-art').length,
      icons: [...document.querySelectorAll('.pantry-art-icon')].slice(0,4).map(x=>x.textContent.trim()),
    }))())`))
    console.log('食材卡', s.cards, '| 带效果图', s.withArt, '| 图标样例', JSON.stringify(s.icons))
    // 点选一张并保存
    await ev(`document.querySelector('.pantry-card')?.click()`)
    await sleep(150)
    await ev(`[...document.querySelectorAll('.pantry-modal .btn')].find(b=>b.textContent.trim()==='保存')?.click()`)
    await sleep(700)
    const saved = (await api('GET', '/api/bootstrap')).data
    console.log('  保存后 pantry:', JSON.stringify(saved.settings.pantry || []))
    await api('PUT', '/api/settings', { pantry: oldPantry })

    // 设置页：卡片尺寸统一 + 排班弹窗
    await ev(`window.location.hash='#settings'`)
    await sleep(700)
    s = JSON.parse(await ev(`JSON.stringify((()=>{
      const cs=[...document.querySelectorAll('.setting-card')];
      const ws=[...new Set(cs.map(c=>Math.round(c.getBoundingClientRect().width)))];
      const mh=[...new Set(cs.map(c=>getComputedStyle(c).minHeight))];
      return {
        count: cs.length, widthKinds: ws.length, widths: ws, minHeights: mh,
        wideCard: cs.some(c=>Math.round(c.getBoundingClientRect().width)>Math.min(...ws)+4),
      };
    })())`))
    console.log('设置页 卡片数', s.count, '| 宽度种类', s.widthKinds, JSON.stringify(s.widths), '| min-height', JSON.stringify(s.minHeights), '| 有跨列大卡:', s.wideCard)
    await ev(`[...document.querySelectorAll('.setting-card .btn')].find(b=>b.textContent.includes('管理每周排班'))?.click()`)
    await sleep(500)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      rows: document.querySelectorAll('.roster-grid .roster-row:not(.roster-head)').length,
      selects: document.querySelectorAll('.roster-select').length,
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      smallBtns: [...document.querySelectorAll('.btn,.chip,.tab')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.width<40||r.height<36)}).length,
    }))())`))
    console.log('排班弹窗 行数', s.rows, '| 下拉', s.selects, '| 横向溢出', s.overflow, '| 过小控件', s.smallBtns, '| JS 异常', errors.length)
    ws.close(); proc.kill()
  }
  await run(1280, 900, 'desktop', false)
  await run(390, 844, 'phone', true)
} finally {
  for (const id of added) await api('DELETE', `/api/plans/${today}/orders/${id}`)
  await api('PUT', '/api/settings', { pantry: oldPantry })
  console.log('\n临时订单与 pantry 已恢复')
}
console.log('DONE')
