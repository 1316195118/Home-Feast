// 日期/餐次联动、购物清单全选、微信分享与微信内降级回归。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
let failures = 0
function check(ok, label, detail = '') {
  if (!ok) failures++
  console.log(`${ok ? 'PASS' : 'FAIL'}  ${label}${detail ? ` · ${detail}` : ''}`)
}

async function api(method, path, body) {
  const response = await fetch(BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(data)}`)
  return data
}

function dateShift(date, days) {
  const d = new Date(`${date}T12:00:00`)
  d.setDate(d.getDate() + days)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function startEdge(port, profile) {
  return spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })
}

async function connectCdp(port, setupScript) {
  let targets
  for (let i = 0; i < 80; i++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch { /* Chrome 还在启动 */ }
    await sleep(200)
  }
  const page = targets?.find((target) => target.type === 'page')
  if (!page) throw new Error('浏览器调试页没有启动')
  const ws = new WebSocket(page.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve)
    ws.addEventListener('error', reject)
  })
  let seq = 0
  const cdp = (method, params = {}) => new Promise((resolve) => {
    const id = ++seq
    const listener = (event) => {
      const message = JSON.parse(event.data)
      if (message.id !== id) return
      ws.removeEventListener('message', listener)
      resolve(message.result)
    }
    ws.addEventListener('message', listener)
    ws.send(JSON.stringify({ id, method, params }))
  })
  const evaluate = async (expression) => (
    await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
  )?.result?.value
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  if (setupScript) await cdp('Page.addScriptToEvaluateOnNewDocument', { source: setupScript })
  return { ws, cdp, evaluate }
}

const boot = await api('GET', '/api/bootstrap')
const tomorrow = dateShift(boot.today, 1)
const lunch = boot.meals.find((meal) => meal.label === '午餐') || boot.meals[1] || boot.meals[0]
const member = boot.members[0]
const todayDish = boot.dishes[0]
const tomorrowDish = boot.dishes[1]
const probeDish = boot.dishes[2]

await api('POST', `/api/plans/${boot.today}/orders`, {
  dishId: todayDish.id, userId: member.id, meal: lunch.key,
})
await api('POST', `/api/plans/${tomorrow}/orders`, {
  dishId: tomorrowDish.id, userId: member.id, meal: lunch.key,
})

const shareStub = `
  Object.defineProperty(navigator, 'canShare', { configurable: true, value: () => true });
  Object.defineProperty(navigator, 'share', { configurable: true, value: async (data) => {
    window.__shareProbe = {
      title: data.title || '',
      text: data.text || '',
      files: data.files?.length || 0,
      type: data.files?.[0]?.type || ''
    };
  }});
`
const noShareStub = `
  Object.defineProperty(navigator, 'canShare', { configurable: true, value: () => false });
  Object.defineProperty(navigator, 'share', { configurable: true, value: undefined });
`
// 固定到当天 16:00，确保午餐已经结束，同时仍处于当天而不是跨日。
const expiredLunchClockStub = `
  (() => {
    const RealDate = Date;
    const fixed = new RealDate(${JSON.stringify(`${boot.today}T16:00:00+08:00`)}).getTime();
    class FixedDate extends RealDate {
      constructor(...args) { super(...(args.length ? args : [fixed])); }
      static now() { return fixed; }
    }
    window.Date = FixedDate;
  })();
`

async function run(width, height, tag, {
  mobile = false, wechat = false, stubShare = false, noShare = false,
} = {}) {
  const port = 9900 + Math.floor(Math.random() * 80)
  const profile = `C:/Users/Administrator/AppData/Local/Temp/fm-day-share-${tag}-${Date.now()}`
  const proc = startEdge(port, profile)
  const errors = []
  try {
    const setupScript = [expiredLunchClockStub, stubShare ? shareStub : noShare ? noShareStub : ''].join('\n')
    const { ws, cdp, evaluate } = await connectCdp(port, setupScript)
    ws.addEventListener('message', (event) => {
      const message = JSON.parse(event.data)
      if (message.method === 'Runtime.exceptionThrown') errors.push('exception')
      if (message.method === 'Runtime.consoleAPICalled' && message.params?.type === 'error') {
        errors.push('console.error')
      }
    })
    await cdp('Emulation.setDeviceMetricsOverride', {
      width, height, deviceScaleFactor: 1, mobile,
      screenWidth: width, screenHeight: height,
    })
    if (wechat) {
      await cdp('Emulation.setUserAgentOverride', {
        userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 MicroMessenger/8.0.50',
        platform: 'iPhone',
      })
    }
    await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
    await sleep(2800)

    await evaluate(`document.querySelector('.user-pick-item')?.click()`)
    await sleep(300)

    const initial = JSON.parse(await evaluate(`JSON.stringify({
      dates: document.querySelectorAll('.schedule-date').length,
      selected: document.querySelector('.schedule-date.active')?.textContent || '',
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth
    })`))
    check(initial.dates === 7, `${tag} 日期栏 7 天`, String(initial.dates))
    check(initial.selected.includes('今天'), `${tag} 默认显示今天`, initial.selected)
    check(!initial.overflow, `${tag} 首屏无横向溢出`)

    const lunchChip = await evaluate(`(() => {
      const chip = [...document.querySelectorAll('.schedule-meal-chips .chip')].find((el) => el.textContent.includes(${JSON.stringify(lunch.label)}));
      return { text: chip?.textContent || '', title: chip?.title || '' };
    })()`)
    check(lunchChip.text.includes('次日'), `${tag} 已过午餐标记为次日`, lunchChip.text)
    check(lunchChip.title.includes('安排到明天'), `${tag} 次日午餐有明确说明`, lunchChip.title)
    await evaluate(`[...document.querySelectorAll('.schedule-meal-chips .chip')].find((el) => el.textContent.includes(${JSON.stringify(lunch.label)}))?.click()`)
    await sleep(650)

    const filtered = JSON.parse(await evaluate(`JSON.stringify({
      selectedDate: document.querySelector('.schedule-date.active')?.textContent || '',
      panel: document.querySelector('#today-menu-panel .panel-title')?.textContent || '',
      hint: document.querySelector('.schedule-meal-hint')?.textContent || '',
      toast: document.querySelector('.toast')?.textContent || '',
      meals: [...document.querySelectorAll('.shown-meal-marker, .meal-group-title')].map((el) => el.textContent.trim()),
      text: document.querySelector('#today-menu-panel')?.textContent || '',
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth
    })`))
    check(filtered.selectedDate.includes('明天'), `${tag} 点午餐自动切到明天`, filtered.selectedDate)
    check(filtered.hint.includes('今天午餐已过') && filtered.hint.includes('明天午餐'), `${tag} 次日午餐有持续标识`, filtered.hint)
    check(filtered.toast.includes('已切到明天午餐'), `${tag} 自动顺延有即时反馈`, filtered.toast)
    check(filtered.panel.includes('明天') && filtered.panel.includes(lunch.label), `${tag} 菜单标题显示明天午餐`, filtered.panel)
    check(filtered.text.includes(tomorrowDish.name), `${tag} 显示明天午餐菜品`, tomorrowDish.name)
    check(!filtered.text.includes(todayDish.name), `${tag} 不串入今天午餐菜品`, todayDish.name)
    check(!filtered.overflow, `${tag} 切换后无横向溢出`)

    // 从菜谱页真正点一道菜，必须写到「明天 · 午餐」而不是今天。
    await evaluate(`[...document.querySelectorAll('.tab')].find((el) => el.textContent.includes('菜谱'))?.click()`)
    await sleep(650)
    const context = await evaluate(`document.querySelector('.menu-order-context')?.textContent || ''`)
    check(context.includes('明天') && context.includes(lunch.label), `${tag} 菜谱页提示正在点明天午餐`, context)

    await evaluate(`[...document.querySelectorAll('.menu-head-actions .btn')].find((el) => el.textContent.includes('冰箱食材'))?.click()`)
    await sleep(400)
    const pantryUi = JSON.parse(await evaluate(`JSON.stringify({
      open: !!document.querySelector('.pantry-modal'),
      orderFab: !!document.querySelector('.order-fab'),
      fridgeWidth: Math.round(document.querySelector('.real-fridge')?.getBoundingClientRect().width || 0),
      doors: [...document.querySelectorAll('.rf-door')].map((door) => ({
        open: door.classList.contains('open'),
        width: Math.round(door.getBoundingClientRect().width)
      }))
    })`))
    check(pantryUi.open, `${tag} 冰箱里有什么弹窗已打开`)
    check(!pantryUi.orderFab, `${tag} 冰箱里不显示已下单悬浮`)
    check(
      pantryUi.doors.length === 2
        && pantryUi.doors.every((door) => door.open)
        && pantryUi.doors.every((door) => door.width <= pantryUi.fridgeWidth * 0.12),
      `${tag} 冰箱双门已充分展开`,
      JSON.stringify(pantryUi.doors),
    )
    await evaluate(`document.querySelector('.pantry-modal .modal-head .icon-btn')?.click()`)
    await sleep(350)
    check(await evaluate(`!!document.querySelector('.order-fab')`), `${tag} 关闭冰箱后恢复已下单入口`)

    await evaluate(`(() => {
      const card = [...document.querySelectorAll('.dish-card')].find((el) => el.textContent.includes(${JSON.stringify(probeDish.name)}));
      card?.querySelector('.dish-foot .btn')?.click();
    })()`)
    await sleep(500)
    const tomorrowPlan = await api('GET', `/api/plans/${tomorrow}`)
    const todayPlan = await api('GET', `/api/plans/${boot.today}`)
    check(
      tomorrowPlan.orders.some((order) => order.dishName === probeDish.name && order.meal === lunch.key),
      `${tag} 点菜写入明天午餐`, probeDish.name,
    )
    check(
      !todayPlan.orders.some((order) => order.dishName === probeDish.name && order.meal === lunch.key),
      `${tag} 点菜没有误写今天`,
    )
    await evaluate(`[...document.querySelectorAll('.tab')].find((el) => el.textContent.includes('今天'))?.click()`)
    await sleep(450)

    await evaluate(`[...document.querySelectorAll('#today-menu-panel .btn')].find((el) => el.textContent.includes('购物清单'))?.click()`)
    await sleep(250)
    const before = JSON.parse(await evaluate(`JSON.stringify({
      groups: document.querySelectorAll('.shopping-group').length,
      checked: document.querySelectorAll('.shopping-item input:checked').length,
      total: document.querySelectorAll('.shopping-item input').length
    })`))
    check(before.groups > 0, `${tag} 购物清单已打开`, `${before.groups} 组`)
    await evaluate(`document.querySelector('.shopping-group .shopping-select-all')?.click()`)
    await sleep(220)
    const afterGroup = JSON.parse(await evaluate(`JSON.stringify({
      checked: document.querySelectorAll('.shopping-item input:checked').length,
      summary: document.querySelector('.shopping-list-summary')?.textContent || '',
      toast: document.querySelector('.toast')?.textContent || ''
    })`))
    check(afterGroup.checked > before.checked, `${tag} 分组全选生效`, `${before.checked} → ${afterGroup.checked}`)
    check(afterGroup.toast.includes('已全部勾选'), `${tag} 分组全选有视觉反馈`, afterGroup.toast)
    await evaluate(`[...document.querySelectorAll('.shopping-list-summary .btn')].find((el) => el.textContent.includes('全选全部'))?.click()`)
    await sleep(220)
    const afterAll = JSON.parse(await evaluate(`JSON.stringify({
      checked: document.querySelectorAll('.shopping-item input:checked').length,
      total: document.querySelectorAll('.shopping-item input').length,
      summary: document.querySelector('.shopping-list-summary')?.textContent || ''
    })`))
    check(afterAll.checked === afterAll.total && afterAll.total > 0, `${tag} 清单全选全部生效`, `${afterAll.checked}/${afterAll.total}`)

    await evaluate(`document.querySelector('.modal-head .icon-btn')?.click()`)
    await sleep(220)
    await evaluate(`[...document.querySelectorAll('#today-menu-panel .btn')].find((el) => el.textContent.includes('分享'))?.click()`)
    await sleep(350)
    const shareUi = JSON.parse(await evaluate(`JSON.stringify({
      button: document.querySelector('.wechat-share-btn')?.textContent || '',
      image: document.querySelector('.shopping-preview img')?.src?.slice(0, 22) || ''
    })`))
    check(shareUi.button.includes('分享到微信'), `${tag} 分享图有微信按钮`, shareUi.button)
    check(shareUi.image.startsWith('data:image/png'), `${tag} 分享图已生成`)
    await evaluate(`document.querySelector('.wechat-share-btn')?.click()`)
    await sleep(350)

    if (stubShare) {
      const probe = JSON.parse(await evaluate(`JSON.stringify(window.__shareProbe || null)`))
      check(probe?.files === 1 && probe?.type === 'image/png', `${tag} 原生文件分享被正确调用`, JSON.stringify(probe))
    } else {
      const guide = await evaluate(`document.querySelector('.wechat-share-guide')?.textContent || ''`)
      check(guide.length > 0, `${tag} 不支持原生分享时显示降级指引`, guide.slice(0, 60))
      if (wechat) check(guide.includes('长按上方图片'), `${tag} 微信内置浏览器提示长按发送`)
    }

    if (tag === 'desktop') {
      await evaluate(`document.querySelector('.modal-head .icon-btn')?.click()`)
      await sleep(200)
      await evaluate(`[...document.querySelectorAll('.tab')].find((el) => el.textContent.includes('设置'))?.click()`)
      await sleep(700)
      await evaluate(`[...document.querySelectorAll('button')].find((el) => el.textContent.includes('管理菜系显示'))?.click()`)
      await sleep(250)
      await evaluate(`[...document.querySelectorAll('.modal button')].find((el) => el.textContent.trim() === '全部隐藏')?.click()`)
      await sleep(420)
      const hidden = JSON.parse(await evaluate(`JSON.stringify({
        checked: document.querySelectorAll('.cuisine-toggle input:checked').length,
        total: document.querySelectorAll('.cuisine-toggle input').length,
        count: document.querySelector('.cuisine-foot-count')?.textContent || ''
      })`))
      check(hidden.checked === 0 && hidden.count.includes('0 /'), `${tag} 设置页全部隐藏生效`, hidden.count)
      await evaluate(`[...document.querySelectorAll('.modal button')].find((el) => el.textContent.trim() === '全选显示')?.click()`)
      await sleep(420)
      const shown = JSON.parse(await evaluate(`JSON.stringify({
        checked: document.querySelectorAll('.cuisine-toggle input:checked').length,
        total: document.querySelectorAll('.cuisine-toggle input').length,
        count: document.querySelector('.cuisine-foot-count')?.textContent || '',
        toast: document.querySelector('.toast')?.textContent || ''
      })`))
      check(shown.checked === shown.total && shown.total > 0, `${tag} 设置页全选显示生效`, `${shown.checked}/${shown.total}`)
      check(shown.toast.includes('已显示全部'), `${tag} 设置页全选显示有反馈`, shown.toast)
    }
    check(errors.length === 0, `${tag} 无 JS 异常`, errors.join(', '))
  } finally {
    proc.kill()
    killProcessTree(proc)
  }
}

await run(1280, 900, 'desktop', { stubShare: true })
await run(390, 844, 'phone', { mobile: true, noShare: true })
await run(320, 720, 'wechat', { mobile: true, wechat: true, noShare: true })

console.log(`\n${failures ? `FAILED ${failures}` : 'ALL PASS'}`)
if (failures) process.exitCode = 1
