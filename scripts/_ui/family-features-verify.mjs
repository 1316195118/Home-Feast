// 七项家庭协作功能回归：分量换算、过敏排除/确认、整周采购、共享状态、
// 烹饪时间线、临期食材和口味周报。
// 用法：BASE=http://127.0.0.1:18090 node scripts/_ui/family-features-verify.mjs
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { createRequire } from 'node:module'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const ROOT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const stamp = Date.now()

async function api(method, path, body) {
  const response = await fetch(BASE + path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${data.error || ''}`)
  return data
}

function localDate(offset = 0) {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

const require = createRequire(import.meta.url)
const esbuild = require(ROOT + 'web/node_modules/esbuild')
const bundledUtils = await esbuild.build({
  entryPoints: [ROOT + 'web/src/utils.js'],
  bundle: true,
  write: false,
  format: 'esm',
  platform: 'neutral',
  logLevel: 'silent',
})
const utils = await import(`data:text/javascript;base64,${Buffer.from(bundledUtils.outputFiles[0].contents).toString('base64')}`)
assert.equal(utils.dishServings({}), 2, '旧菜谱应兼容为 2 人份')
assert.equal(utils.scaleIngredient('鸡腿肉 800g', 1.5), '鸡腿肉 1200g', '配料应按人数换算')
const deducted = utils.applyPantryStock(
  [{ text: '番茄', amounts: ['200g'], amount: '200g' }],
  { 番茄: { quantity: 100, unit: 'g' } },
)[0]
assert.equal(deducted.amount, '100g', '库存应从采购量中扣除')
assert.equal(deducted.pantryDeducted, true, '库存扣减应有明确标记')
assert.equal(
  utils.allergyConflicts(
    { name: '凉拌花生', ingredients: ['花生米 30g'] },
    [{ id: 'u1', name: '甲', allergies: ['花生'] }],
  ).length,
  1,
  '过敏原应按成员硬性命中',
)
console.log('✓ 纯函数：份数默认值、人数缩放、库存扣减、过敏检测')

const boot = await api('GET', '/api/bootstrap')
const today = boot.today
const member = boot.members[0]
const originalAllergies = [...(member.allergies || [])]
const originalPantry = [...(boot.settings.pantry || [])]
const originalStock = JSON.parse(JSON.stringify(boot.settings.pantryStock || {}))
const originalOrderIds = new Set((boot.plan.orders || []).map((order) => order.id))
const createdDishes = []

async function createDish(payload) {
  const dish = await api('POST', '/api/dishes', payload)
  createdDishes.push(dish.id)
  return dish
}

const allergyDish = await createDish({
  name: `过敏测试菜-${stamp}`,
  cuisine: 'homestyle',
  desc: '自动化测试菜品，结束后删除',
  tags: ['家常', '凉菜'],
  spicy: 0,
  minutes: 8,
  servings: 2,
  ingredients: ['花生米 30g', '黄瓜 120g'],
  steps: ['黄瓜切丝', '花生米拌入黄瓜'],
})
const orderDish = await createDish({
  name: `时间线测试菜-${stamp}`,
  cuisine: 'homestyle',
  desc: '自动化测试菜品，结束后删除',
  tags: ['家常', '素菜'],
  spicy: 0,
  minutes: 12,
  servings: 2,
  ingredients: ['番茄 200g', '鸡蛋 2个'],
  steps: ['番茄切块', '鸡蛋打散炒熟', '番茄下锅煮 3 分钟'],
})

await api('PUT', `/api/members/${member.id}`, { allergies: [...new Set([...originalAllergies, '花生'])] })
await api('PUT', '/api/settings', {
  pantry: [...new Set([...originalPantry, '番茄'])],
  pantryStock: {
    ...originalStock,
    番茄: { quantity: 100, unit: 'g', added: today, expires: localDate(1) },
  },
})
const withOrder = await api('POST', `/api/plans/${today}/orders`, {
  dishId: orderDish.id,
  userId: member.id,
  meal: 'dinner',
})
const addedOrderIds = (withOrder.orders || [])
  .filter((order) => !originalOrderIds.has(order.id))
  .map((order) => order.id)

const PORT = 9870 + Math.floor(Math.random() * 60)
const profile = `C:/Users/Administrator/AppData/Local/Temp/fm-family-features-${PORT}-${Date.now()}`
const browser = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`, `--user-data-dir=${profile}`, 'about:blank',
], { stdio: 'ignore' })

let targets
for (let attempt = 0; attempt < 60; attempt++) {
  try {
    const response = await fetch(`http://127.0.0.1:${PORT}/json/list`)
    targets = await response.json()
    if (targets.some((target) => target.type === 'page')) break
  } catch {}
  await sleep(250)
}
if (!targets?.some((target) => target.type === 'page')) {
  killProcessTree(browser)
  throw new Error('无法连接 Edge 调试端口')
}

const socket = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  socket.addEventListener('open', resolve)
  socket.addEventListener('error', reject)
})

let sequence = 0
const runtimeErrors = []
socket.addEventListener('message', (event) => {
  const message = JSON.parse(event.data)
  if (message.method === 'Runtime.exceptionThrown') {
    runtimeErrors.push(message.params?.exceptionDetails?.text || 'runtime exception')
  }
})
const cdp = (method, params = {}) => new Promise((resolve, reject) => {
  const id = ++sequence
  const listener = (event) => {
    const payload = JSON.parse(event.data)
    if (payload.id !== id) return
    socket.removeEventListener('message', listener)
    if (payload.error) reject(new Error(payload.error.message))
    else resolve(payload.result)
  }
  socket.addEventListener('message', listener)
  socket.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression, awaitPromise = false) => {
  const result = await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise })
  if (result?.exceptionDetails) throw new Error(result.exceptionDetails.text)
  return result?.result?.value
}
async function waitFor(expression, timeout = 5000) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    if (await evaluate(expression)) return true
    await sleep(100)
  }
  return false
}
async function clickByText(selector, text) {
  return evaluate(`(()=>{
    const el=[...document.querySelectorAll(${JSON.stringify(selector)})]
      .find(x=>x.textContent.replace(/\\s+/g,' ').includes(${JSON.stringify(text)}));
    if(!el)return false; el.click(); return true;
  })()`)
}
async function inputValue(selector, value) {
  return evaluate(`(()=>{
    const el=document.querySelector(${JSON.stringify(selector)});
    if(!el)return false;
    const setter=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;
    setter.call(el,${JSON.stringify(value)});
    el.dispatchEvent(new Event('input',{bubbles:true}));
    return true;
  })()`)
}

let passed = 0
function check(label, condition, detail = '') {
  assert.ok(condition, `${label}${detail ? `：${detail}` : ''}`)
  passed++
  console.log(`✓ ${label}`)
}

try {
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 390, height: 844, deviceScaleFactor: 2, mobile: true, screenWidth: 390, screenHeight: 844,
  })
  await cdp('Page.navigate', { url: `${BASE}/?feature-verify=${stamp}#today` })
  check('手机端首页加载', await waitFor(`!!document.querySelector('.schedule-switcher')`, 8000))
  await evaluate(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(500)

  check('临期食材提示出现', await waitFor(`!!document.querySelector('.pantry-expiry-banner')`))
  const expiryText = await evaluate(`document.querySelector('.pantry-expiry-banner')?.textContent || ''`)
  check('临期提示包含番茄', expiryText.includes('番茄'), expiryText.replace(/\s+/g, ' ').trim())

  check('做饭时间线入口可用', await clickByText('button', '做饭时间线'))
  check('时间线面板打开', await waitFor(`!!document.querySelector('.meal-timeline-modal')`))
  const timeline = JSON.parse(await evaluate(`JSON.stringify((()=>({
    dishes:document.querySelectorAll('.timeline-dish').length,
    steps:document.querySelectorAll('.timeline-steps li').length,
    text:document.querySelector('.meal-timeline-modal')?.textContent||''
  }))())`))
  check('时间线按订单生成步骤', timeline.dishes > 0 && timeline.steps > 0, JSON.stringify(timeline))
  check('时间线显示人数换算', timeline.text.includes('人份'))
  await evaluate(`document.querySelector('.meal-timeline-modal .icon-btn')?.click()`)
  await sleep(250)

  check('每日购物清单入口可用', await clickByText('.panel-head-actions button', '购物清单'))
  check('每日购物清单打开', await waitFor(`!!document.querySelector('.modal[aria-label="购物清单"]')`))
  const shopping = JSON.parse(await evaluate(`JSON.stringify((()=>({
    items:document.querySelectorAll('.modal[aria-label="购物清单"] .shopping-item').length,
    progress:!!document.querySelector('.modal[aria-label="购物清单"] .shopping-progress'),
    text:document.querySelector('.modal[aria-label="购物清单"]')?.textContent||''
  }))())`))
  check('购物清单显示进度反馈', shopping.progress && shopping.items > 0, JSON.stringify({ items: shopping.items }))
  const firstName = await evaluate(`document.querySelector('.modal[aria-label="购物清单"] .shopping-name')?.textContent || ''`)
  if (firstName) {
    check('采购项可以认领', await evaluate(`(()=>{const b=document.querySelector('.shopping-claim');if(!b)return false;b.click();return true})()`))
    await sleep(500)
    const marks = (await api('GET', '/api/bootstrap')).settings.shoppingMarks || {}
    const markKey = `${today}::${firstName}`
    check('共享采购状态写入服务端', marks[markKey]?.status === 'claimed', markKey)
    await api('POST', '/api/shopping/marks', {
      date: today, userId: member.id, items: [{ key: markKey, status: '' }],
    })
  }
  await evaluate(`document.querySelector('.modal[aria-label="购物清单"] .modal-head .icon-btn')?.click()`)
  await sleep(250)

  await evaluate(`window.location.hash='#menu'`)
  check('菜谱页加载', await waitFor(`!!document.querySelector('.dish-grid')`, 8000))
  await inputValue('.search-box input', allergyDish.name)
  await sleep(450)
  const allergyCard = JSON.parse(await evaluate(`JSON.stringify((()=>{
    const card=[...document.querySelectorAll('.dish-card')].find(x=>x.textContent.includes(${JSON.stringify(allergyDish.name)}));
    return {found:!!card,tag:card?.querySelector('.tag-allergy')?.textContent||''};
  })())`))
  check('过敏菜品有明显警示', allergyCard.found && allergyCard.tag.includes('花生'), JSON.stringify(allergyCard))
  await clickByText('.dish-card .btn-soft', '点这道')
  check('手动点过敏菜二次确认', await waitFor(`!!document.querySelector('.confirm-modal')`))
  const confirmText = await evaluate(`document.querySelector('.confirm-modal')?.textContent || ''`)
  check('确认框说明具体过敏人', confirmText.includes('花生') && confirmText.includes(member.name))
  await clickByText('.confirm-modal .btn', '换一道')
  await sleep(250)

  await evaluate(`window.location.hash='#today'`)
  await waitFor(`!!document.querySelector('.meal-plan')`, 5000)
  await clickByText('button', '生成一周菜单')
  await waitFor(`!!document.querySelector('.modal[aria-label="本周菜单"]')`)
  await clickByText('.modal[aria-label="本周菜单"] button', '生成本周菜单')
  check('整周菜单生成', await waitFor(`document.querySelectorAll('.week-day').length === 7`))
  const weekText = await evaluate(`[...document.querySelectorAll('.week-dish')].map(x=>x.textContent.trim()).join('|')`)
  check('自动周菜单排除过敏菜', !weekText.includes(allergyDish.name))
  check('整周采购清单入口可用', await clickByText('.modal[aria-label="本周菜单"] button', '整周采购清单'))
  check('整周采购清单打开', await waitFor(`!!document.querySelector('.modal[aria-label="购物清单"]')`))
  const weekShopping = JSON.parse(await evaluate(`JSON.stringify((()=>{
    const root=document.querySelector('.modal[aria-label="购物清单"]');
    return {items:root?.querySelectorAll('.shopping-item').length||0,text:root?.textContent||''};
  })())`))
  check('整周采购覆盖多道菜', weekShopping.items > 0 && weekShopping.text.includes('本周采购'))
  await evaluate(`document.querySelector('.modal[aria-label="购物清单"] .modal-head .icon-btn')?.click()`)
  await sleep(250)
  await evaluate(`document.querySelector('.modal[aria-label="本周菜单"] .modal-head .icon-btn')?.click()`)

  await evaluate(`window.location.hash='#family'`)
  check('家庭口味周报出现', await waitFor(`!!document.querySelector('.taste-report')`, 6000))
  const taste = JSON.parse(await evaluate(`JSON.stringify((()=>({
    kpis:document.querySelectorAll('.taste-kpis > div').length,
    insights:document.querySelectorAll('.taste-insights p').length,
    text:document.querySelector('.taste-report')?.textContent||''
  }))())`))
  check('口味周报包含指标与建议', taste.kpis === 4 && taste.insights > 0, JSON.stringify({ kpis: taste.kpis, insights: taste.insights }))

  const layout = JSON.parse(await evaluate(`JSON.stringify((()=>({
    viewport:document.documentElement.clientWidth,
    scrollWidth:document.documentElement.scrollWidth,
    overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth
  }))())`))
  check('390px 页面无横向溢出', !layout.overflow, JSON.stringify(layout))
  check('运行期间无 JS 异常', runtimeErrors.length === 0, runtimeErrors.slice(0, 3).join(' | '))
  console.log(`\n全部通过，共 ${passed} 项。`)
} finally {
  socket.close()
  killProcessTree(browser)
  for (const id of addedOrderIds) {
    try { await api('DELETE', `/api/plans/${today}/orders/${id}`) } catch {}
  }
  for (const dish of createdDishes) {
    try { await api('DELETE', `/api/dishes/${dish}`) } catch {}
  }
  try { await api('PUT', `/api/members/${member.id}`, { allergies: originalAllergies }) } catch {}
  try {
    await api('PUT', '/api/settings', {
      pantry: originalPantry,
      pantryStock: originalStock,
    })
  } catch {}
}
