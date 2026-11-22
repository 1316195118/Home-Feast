// 动态接口压力与故障输入测试。
// 只读检查默认可跑；写测试必须显式 WRITE=1，并且目标应是临时数据副本。
//
// 用法：
//   BASE=http://127.0.0.1:18090 node scripts/stress-test.mjs
//   BASE=http://127.0.0.1:18090 WRITE=1 DATA_DIR=<临时 data> CONFIG_DIR=<临时 config> node scripts/stress-test.mjs
import fs from 'node:fs/promises'
import path from 'node:path'
import { performance } from 'node:perf_hooks'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const WRITE = process.env.WRITE === '1'
const DATA_DIR = process.env.DATA_DIR || ''
const CONFIG_DIR = process.env.CONFIG_DIR || ''
const CODE = process.env.CODE || ''
const clientId = `stress-${Date.now()}`

const results = []
function check(label, ok, detail = '') {
  results.push({ label, ok: Boolean(ok), detail })
  console.log(`${ok ? '  ✓' : '  ✗'} ${label}${detail ? `：${detail}` : ''}`)
}

async function request(urlPath, options = {}) {
  const headers = {
    Accept: 'application/json, text/plain, */*',
    'X-Client-ID': clientId,
    ...(CODE ? { 'X-Access-Code': CODE } : {}),
    ...(options.headers || {}),
  }
  const started = performance.now()
  const response = await fetch(BASE + urlPath, { ...options, headers })
  const body = Buffer.from(await response.arrayBuffer())
  const elapsed = performance.now() - started
  let data = null
  const type = response.headers.get('content-type') || ''
  if (body.length && (type.includes('json') || !options.rawBody)) {
    try { data = JSON.parse(body.toString('utf8')) } catch {}
  }
  return { response, status: response.status, headers: response.headers, body, data, elapsed }
}

async function jsonRequest(urlPath, options = {}) {
  const result = await request(urlPath, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })
  return result
}

function formFile(name, bytes, type = 'application/octet-stream') {
  const form = new FormData()
  form.append('file', new Blob([bytes], { type }), name)
  return form
}

async function waitForSSE(trigger) {
  const controller = new AbortController()
  const response = await fetch(`${BASE}/api/events?clientId=stress-listener-${Date.now()}`, {
    headers: CODE ? { 'X-Access-Code': CODE } : {},
    signal: controller.signal,
  })
  if (response.status !== 200 || !response.body) {
    controller.abort()
    throw new Error(`SSE 连接失败：HTTP ${response.status}`)
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let text = ''
  const readUntil = async (pattern, timeoutMs) => {
    const deadline = Date.now() + timeoutMs
    while (!pattern.test(text)) {
      const left = deadline - Date.now()
      if (left <= 0) throw new Error(`等待 SSE 超时，已收到 ${JSON.stringify(text.slice(-120))}`)
      const chunk = await Promise.race([
        reader.read(),
        sleep(left).then(() => { throw new Error('SSE 读取超时') }),
      ])
      if (chunk.done) break
      text += decoder.decode(chunk.value, { stream: true })
    }
  }
  try {
    await readUntil(/: connected/, 2000)
    const triggered = trigger()
    await readUntil(/data: \{"v":\d+\}/, 3000)
    await triggered
    return text
  } finally {
    controller.abort()
    await reader.cancel().catch(() => {})
  }
}

async function parseJSONFile(file) {
  const raw = await fs.readFile(file, 'utf8')
  return JSON.parse(raw)
}

const created = { dishes: [], members: [], orders: [] }
let originalPlan = null
let today = ''
let firstMeal = ''

try {
  console.log(`\n目标：${BASE}  ${WRITE ? '写测试已启用' : '只读检查'}`)

  const ping = await request('/api/ping')
  check('健康检查', ping.status === 200 && ping.data?.ok === true, `HTTP ${ping.status}`)

  const boot0 = await request('/api/bootstrap', { headers: { 'Accept-Encoding': 'gzip' } })
  const boot = boot0.data
  check('bootstrap 返回完整数据', boot0.status === 200 && boot?.dishes?.length >= 50)
  // Node fetch 会自动解压 body，但响应头仍保留 gzip；内容必须是可直接解析的单层 JSON。
  check('bootstrap 预压缩未二次压缩', boot0.headers.get('content-encoding') === 'gzip'
    && boot0.body[0] === 0x7b)
  check('bootstrap 带强 ETag', /^"[0-9a-f]{24}-gzip"$/.test(boot0.headers.get('etag') || ''))

  const cached = await request('/api/bootstrap', {
    headers: { 'Accept-Encoding': 'gzip', 'If-None-Match': boot0.headers.get('etag') },
  })
  check('ETag 命中返回空 304', cached.status === 304 && cached.body.length === 0)

  const index = await request('/', { headers: { 'Accept-Encoding': 'gzip' } })
  const html = index.body.toString('utf8')
  const asset = html.match(/<script[^>]+src="([^"]+\.js)"/)?.[1]
  check('首页返回构建入口', index.status === 200 && Boolean(asset))
  if (asset) {
    const js = await request(asset, { headers: { 'Accept-Encoding': 'gzip' } })
    check('静态资源启用 gzip 与长缓存', js.status === 200
      && js.headers.get('content-encoding') === 'gzip'
      && /immutable/.test(js.headers.get('cache-control') || ''), `${js.body.length} 字节`)
  }

  check('列表数据结构正确', Array.isArray(boot.members) && boot.members.length > 0
    && Array.isArray(boot.plans) && Array.isArray(boot.cuisines))
  today = boot.today
  firstMeal = boot.meals?.[0]?.key || 'lunch'
  originalPlan = boot.plan
  const memberId = boot.members[0].id
  const dishId = boot.dishes[0].id

  const badJSON = await request('/api/dishes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    rawBody: true,
    body: '{"name":',
  })
  check('畸形 JSON 被拒绝', badJSON.status === 400, `HTTP ${badJSON.status}`)

  const wrongRoot = await request('/api/dishes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    rawBody: true,
    body: '[1,2,3]',
  })
  check('错误 JSON 根类型被拒绝', wrongRoot.status === 400, `HTTP ${wrongRoot.status}`)

  const emptyName = await jsonRequest('/api/dishes', {
    method: 'POST', body: { name: ' \u0000 ', cuisine: 'sichuan' },
  })
  check('纯空白菜名被拒绝', emptyName.status === 400)

  const crossSite = await jsonRequest('/api/dishes', {
    method: 'POST',
    headers: { Origin: 'https://evil.example' },
    body: { name: '跨站菜', cuisine: 'sichuan' },
  })
  check('跨站写请求被拒绝', crossSite.status === 403, `HTTP ${crossSite.status}`)

  const missingDish = await jsonRequest(`/api/plans/${today}/orders`, {
    method: 'POST', body: { dishId: 'missing-dish', userId: memberId, meal: firstMeal },
  })
  check('不存在菜品被拒绝', missingDish.status === 404)

  const missingMember = await jsonRequest(`/api/plans/${today}/orders`, {
    method: 'POST', body: { dishId, userId: 'missing-member', meal: firstMeal },
  })
  check('不存在成员被拒绝', missingMember.status === 400)

  const badDate = await jsonRequest('/api/plans/2026-02-30', {
    method: 'PUT', body: { note: '非法日期' },
  })
  check('不存在的日历日期被拒绝', badDate.status === 400)

  const badMeal = await jsonRequest(`/api/plans/${today}/orders`, {
    method: 'POST', body: { dishId, userId: memberId, meal: 'invalid-meal' },
  })
  check('未知餐次被拒绝', badMeal.status === 400)

  const traversal = await request('/uploads/..%2fconfig%2fsettings.json', { rawBody: true })
  check('上传目录路径穿越被拒绝', [400, 403, 404].includes(traversal.status), `HTTP ${traversal.status}`)

  const fakeImage = await request('/api/upload', { method: 'POST', body: formFile('fake.png', Buffer.from('not an image')) })
  check('伪装图片被拒绝', fakeImage.status === 400)

  const corruptZip = await request('/api/backup', {
    method: 'POST',
    body: formFile('bad.zip', Buffer.from('not a zip'), 'application/zip'),
  })
  check('损坏备份包被拒绝', corruptZip.status === 400)

  if (!WRITE) {
    console.log('\n只读检查完成；设置 WRITE=1 才会执行写入、并发和数据文件测试。')
  } else {
    const stamp = Date.now()
    const dish = await jsonRequest('/api/dishes', {
      method: 'POST',
      body: {
        name: `压力测试菜-${stamp}`,
        cuisine: 'sichuan',
        desc: '自动化测试临时数据\u0000',
        tags: ['四川', '测试', '四川'],
        spicy: 99,
        minutes: -5,
        ingredients: ['鸡腿肉 800g', '生抽 30ml'],
        steps: ['大火炒熟'],
      },
    })
    if (dish.data?.id) created.dishes.push(dish.data.id)
    check('新增菜品并清洗边界值', dish.status === 201
      && dish.data?.spicy === 3 && dish.data?.minutes === 0
      && dish.data?.tags?.filter((x) => x === '四川').length === 1
      && !dish.data?.desc?.includes('\u0000'))

    const member = await jsonRequest('/api/members', {
      method: 'POST',
      body: {
        name: `测试成员-${stamp}`,
        avatar: '<img src=x onerror=alert(1)>',
        color: 'red; background:url(//evil)',
        avoid: ['花生', '花生', '\u0000'],
      },
    })
    if (member.data?.id) created.members.push(member.data.id)
    check('新增成员并回退危险头像/颜色', member.status === 201
      && member.data?.avatar === '🙂'
      && /^#[0-9a-f]{6}$/i.test(member.data?.color || '')
      && !JSON.stringify(member.data).includes('\u0000'))

    const testMemberId = member.data.id
    const testDishId = dish.data.id
    const totalOrders = 24
    const occupied = new Set((originalPlan?.orders || []).map((item) => item.dishId))
    const orderDishIDs = boot.dishes
      .map((item) => item.id)
      .filter((id) => id !== testDishId && !occupied.has(id))
      .slice(0, totalOrders)
    if (orderDishIDs.length < totalOrders) {
      throw new Error(`可用测试菜品不足：${orderDishIDs.length}/${totalOrders}`)
    }
    const writeStarted = performance.now()
    const writes = await Promise.all(orderDishIDs.map((orderDishID, i) => {
      return jsonRequest(`/api/plans/${today}/orders`, {
        method: 'POST',
        headers: { 'X-Client-ID': `stress-order-${stamp}-${i}` },
        body: {
          dishId: orderDishID,
          userId: testMemberId,
          meal: firstMeal,
          note: `并发临时订单 ${i}`,
        },
      })
    }))
    const writeElapsed = performance.now() - writeStarted
    const orderIDs = writes.flatMap((r, i) => {
      if (r.status !== 201) return []
      const order = r.data?.orders?.find((item) => item.dishId === orderDishIDs[i]
        && item.note === `并发临时订单 ${i}`)
      return order?.id ? [order.id] : []
    })
    created.orders.push(...orderIDs)
    check('24 并发点菜无 5xx', writes.every((r) => r.status >= 200 && r.status < 300),
      `${writes.length} 次，${writeElapsed.toFixed(0)} ms`)
    check('24 并发点菜订单 ID 唯一', orderIDs.length === totalOrders && new Set(orderIDs).size === totalOrders,
      `${new Set(orderIDs).size}/${totalOrders}`)

    const duty = await jsonRequest(`/api/plans/${today}`, {
      method: 'PUT',
      body: {
        dutyMeal: firstMeal,
        chefIds: [testMemberId, testMemberId, memberId],
        dishwasherIds: [testMemberId],
        note: `压力测试备注-${stamp}`,
      },
    })
    check('多人分工去重并正常保存', duty.status === 200
      && duty.data?.mealDuties?.[firstMeal]?.chefIds?.length === 2)

    const backup = await request('/api/backup', { rawBody: true })
    check('备份导出为有效 ZIP 头', backup.status === 200
      && backup.body.length > 1000 && backup.body.subarray(0, 2).toString() === 'PK')

    let probeOrderId = ''
    try {
      await waitForSSE(async () => {
        const probe = await jsonRequest(`/api/plans/${today}/orders`, {
          method: 'POST',
          headers: { 'X-Client-ID': `stress-sse-writer-${stamp}` },
          body: {
            dishId: testDishId,
            userId: testMemberId,
            meal: firstMeal,
            note: `SSE 临时订单 ${stamp}`,
          },
        })
        if (probe.status !== 201) throw new Error(`SSE 触发写失败：HTTP ${probe.status}`)
        probeOrderId = probe.data.orders.at(-1)?.id || ''
      })
      check('其他客户端写入会收到 SSE 实时事件', Boolean(probeOrderId))
    } finally {
      if (probeOrderId) {
        await request(`/api/plans/${today}/orders/${probeOrderId}`, { method: 'DELETE' }).catch(() => {})
      }
    }

    const deleteCodes = await Promise.all(orderIDs.map((id) => {
      return request(`/api/plans/${today}/orders/${id}`, { method: 'DELETE' })
    }))
    check('24 并发撤单无 5xx', deleteCodes.every((r) => r.status < 500),
      `HTTP ${[...new Set(deleteCodes.map((r) => r.status))].join(',')}`)
    created.orders.length = 0

    // 批量接口只写盘一次：用独立一批菜验证新增与全部上桌，再逐条清理临时订单。
    const batchDishIDs = boot.dishes
      .map((item) => item.id)
      .filter((id) => id !== testDishId && !occupied.has(id))
      .slice(0, 16)
    const batchStarted = performance.now()
    const batch = await jsonRequest(`/api/plans/${today}/orders/batch`, {
      method: 'POST',
      headers: { 'X-Client-ID': `stress-batch-${stamp}` },
      body: {
        userId: testMemberId,
        items: batchDishIDs.map((dishId, i) => ({
          dishId,
          meal: firstMeal,
          note: `批量临时订单 ${i}`,
        })),
      },
    })
    const batchElapsed = performance.now() - batchStarted
    const batchOrderIDs = (batch.data?.plan?.orders || [])
      .filter((item) => batchDishIDs.includes(item.dishId))
      .map((item) => item.id)
    check('16 道菜单事务批量点菜', batch.status === 200
      && batch.data?.added === 16 && batchOrderIDs.length === 16,
    `${batchElapsed.toFixed(0)} ms`)

    const batchDone = await jsonRequest(`/api/plans/${today}/orders/batch`, {
      method: 'PUT',
      headers: { 'X-Client-ID': `stress-batch-done-${stamp}` },
      body: { orderIds: batchOrderIDs, done: true },
    })
    check('16 道菜单事务批量上桌', batchDone.status === 200
      && batchDone.data?.updated === 16
      && (batchDone.data?.plan?.orders || [])
        .filter((item) => batchOrderIDs.includes(item.id))
        .every((item) => item.done === true))
    const batchDelete = await Promise.all(batchOrderIDs.map((id) => {
      return request(`/api/plans/${today}/orders/${id}`, { method: 'DELETE' })
    }))
    check('批量测试订单清理完成', batchDelete.every((r) => r.status === 200))

    if (originalPlan) {
      const restore = await jsonRequest(`/api/plans/${today}`, {
        method: 'PUT',
        body: {
          dutyMeal: firstMeal,
          chefIds: originalPlan.mealDuties?.[firstMeal]?.chefIds || [],
          dishwasherIds: originalPlan.mealDuties?.[firstMeal]?.dishwasherIds || [],
          note: originalPlan.note || '',
        },
      })
      check('恢复测试前分工与备注', restore.status === 200,
        `HTTP ${restore.status}${restore.data?.error ? ` ${restore.data.error}` : ''}`)
    }

    for (const id of created.members.splice(0)) {
      const removed = await request(`/api/members/${id}`, { method: 'DELETE' })
      check('删除临时成员', removed.status === 200)
    }
    for (const id of created.dishes.splice(0)) {
      const removed = await request(`/api/dishes/${id}`, { method: 'DELETE' })
      check('删除临时菜品', removed.status === 200)
    }

    const bootAfter = await request('/api/bootstrap')
    check('写操作后 bootstrap 可重新生成', bootAfter.status === 200
      && !bootAfter.data?.dishes?.some((d) => d.id === testDishId)
      && !bootAfter.data?.members?.some((m) => m.id === testMemberId))

    if (DATA_DIR) {
      const files = ['dishes.json', 'members.json', 'plans.json', 'pins.json']
      const parsed = {}
      for (const name of files) parsed[name] = await parseJSONFile(path.join(DATA_DIR, name))
      const dishIDs = parsed['dishes.json'].map((d) => d.id)
      const memberIDs = parsed['members.json'].map((m) => m.id)
      const orderIDs = parsed['plans.json'].flatMap((p) => (p.orders || []).map((o) => o.id))
      check('写后数据文件均为合法 JSON', true,
        `${dishIDs.length} 道菜 / ${memberIDs.length} 位成员 / ${parsed['plans.json'].length} 天计划`)
      check('菜品、成员、订单 ID 无重复', new Set(dishIDs).size === dishIDs.length
        && new Set(memberIDs).size === memberIDs.length
        && new Set(orderIDs).size === orderIDs.length)
    }
    if (CONFIG_DIR) {
      const settings = await parseJSONFile(path.join(CONFIG_DIR, 'settings.json'))
      check('配置文件为合法 JSON', settings && typeof settings.familyName === 'string')
    }
  }
} catch (error) {
  check('测试执行未中断', false, error.stack || error.message)
} finally {
  if (WRITE) {
    for (const id of created.orders) {
      await request(`/api/plans/${today}/orders/${id}`, { method: 'DELETE' }).catch(() => {})
    }
    for (const id of created.members) {
      await request(`/api/members/${id}`, { method: 'DELETE' }).catch(() => {})
    }
    for (const id of created.dishes) {
      await request(`/api/dishes/${id}`, { method: 'DELETE' }).catch(() => {})
    }
    await sleep(50)
  }
}

const failed = results.filter((item) => !item.ok)
console.log(`\n结果：${results.length - failed.length}/${results.length} 通过，${failed.length} 失败`)
if (failed.length) {
  for (const item of failed) console.log(`  ✗ ${item.label}：${item.detail}`)
  process.exitCode = 1
}
