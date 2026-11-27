// 家庭点菜系统性能基准：只读模式可安全跑线上，WRITE=1 时请确保目标是临时数据副本。
//
// 用法：
//   BASE=http://127.0.0.1:18090 node scripts/perf-test.mjs
//   BASE=http://127.0.0.1:18090 WRITE=1 node scripts/perf-test.mjs
import { performance } from 'node:perf_hooks'

const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const WRITE = process.env.WRITE === '1'
const targetURL = new URL(BASE)
if (WRITE && process.env.I_KNOW_THIS_IS_PRODUCTION !== '1' &&
    (targetURL.port === '8090' || targetURL.hostname === '10.10.10.251')) {
  throw new Error('WRITE=1 检测到生产地址：确需写入生产请先设 I_KNOW_THIS_IS_PRODUCTION=1')
}
const HEADERS = {
  Accept: 'application/json, text/plain, */*',
  'Accept-Encoding': 'br, gzip',
}
if (process.env.CODE) HEADERS['X-Access-Code'] = process.env.CODE

function percentile(values, p) {
  if (!values.length) return 0
  const sorted = values.slice().sort((a, b) => a - b)
  return sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * p) - 1)]
}

function stats(values) {
  const total = values.reduce((n, value) => n + value, 0)
  return {
    n: values.length,
    p50: percentile(values, 0.5),
    p95: percentile(values, 0.95),
    p99: percentile(values, 0.99),
    mean: total / Math.max(1, values.length),
    max: Math.max(0, ...values),
  }
}

function printStats(label, values, bytes = 0) {
  const s = stats(values)
  const rate = s.mean > 0 ? 1000 / s.mean : 0
  const byteText = bytes ? `  ${(bytes / 1024).toFixed(1)} KB` : ''
  console.log(
    `${label.padEnd(24)} n=${String(s.n).padStart(3)}  `
    + `p50=${s.p50.toFixed(1)}ms p95=${s.p95.toFixed(1)}ms p99=${s.p99.toFixed(1)}ms `
    + `mean=${s.mean.toFixed(1)}ms max=${s.max.toFixed(1)}ms ${rate.toFixed(1)} req/s${byteText}`,
  )
}

async function timedFetch(path, options = {}) {
  const start = performance.now()
  const response = await fetch(BASE + path, {
    ...options,
    headers: { ...HEADERS, ...(options.headers || {}) },
  })
  const body = await response.arrayBuffer()
  return { response, body, ms: performance.now() - start }
}

async function jsonRequest(path, options = {}) {
  const result = await timedFetch(path, options)
  const data = result.body.byteLength ? JSON.parse(new TextDecoder().decode(result.body)) : {}
  if (!result.response.ok) {
    throw new Error(`${options.method || 'GET'} ${path} -> ${result.response.status}: ${data.error || ''}`)
  }
  return { ...result, data }
}

async function concurrencyRound(limit, requests) {
  const values = []
  let next = 0
  const started = performance.now()
  async function worker() {
    while (next < requests) {
      next += 1
      const result = await timedFetch('/api/bootstrap')
      if (!result.response.ok) throw new Error(`bootstrap -> ${result.response.status}`)
      values.push(result.ms)
    }
  }
  await Promise.all(Array.from({ length: limit }, worker))
  const elapsed = performance.now() - started
  printStats(`bootstrap 并发 ${limit}`, values)
  console.log(`${'并发总吞吐'.padEnd(24)} ${(requests / (elapsed / 1000)).toFixed(1)} req/s`)
}

async function baseline() {
  const boot = await jsonRequest('/api/bootstrap')
  console.log(`\n目标 ${BASE}`)
  console.log(`首屏 ${(boot.body.byteLength / 1024).toFixed(1)} KB · ${boot.data.dishes.length} 道菜 · ${boot.data.plans.length} 天记录`)
  const rawBytes = boot.body.byteLength
  const wireBytes = Number(boot.response.headers.get('content-length')) || 0
  const etag = boot.response.headers.get('etag') || ''
  const encoding = boot.response.headers.get('content-encoding') || 'identity'
  console.log(
    `ETag ${etag || '(none)'} · ${encoding}`
    + `${wireBytes ? ` · 线上传输 ${(wireBytes / 1024).toFixed(1)} KB` : ''}`,
  )

  const warm = []
  for (let i = 0; i < 20; i++) warm.push((await timedFetch('/api/bootstrap')).ms)
  printStats('bootstrap 顺序热请求', warm, rawBytes)

  if (etag) {
    const revalidate = []
    for (let i = 0; i < 80; i++) {
      revalidate.push((await timedFetch('/api/bootstrap', {
        headers: { 'If-None-Match': etag },
      })).ms)
    }
    printStats('bootstrap ETag 304', revalidate)
  }

  for (const limit of [50, 100, 200]) await concurrencyRound(limit, limit * 2)

  const index = await timedFetch('/')
  const html = new TextDecoder().decode(index.body)
  const asset = html.match(/<script[^>]+src="([^"]+\.js)"/)?.[1]
  if (asset) {
    const times = []
    let bytes = 0
    for (let i = 0; i < 40; i++) {
      const result = await timedFetch(asset)
      times.push(result.ms)
      bytes = result.body.byteLength
    }
    printStats('首屏 JS 顺序热请求', times, bytes)
  } else {
    console.log('首屏 HTML 未找到构建后的 JS 入口')
  }

  if (!WRITE) {
    console.log('\n只读基准完成；写接口压测未启用（设置 WRITE=1 才会执行）。')
    return
  }

  const today = boot.data.today
  const memberId = boot.data.members[0]?.id
  const dishIds = boot.data.dishes.slice(0, 40).map((dish) => dish.id)
  if (!memberId || !dishIds.length) throw new Error('没有可用的成员或菜品')
  const createdOrders = []
  const writeStart = performance.now()
  const writeValues = []
  await Promise.all(dishIds.map(async (dishId, i) => {
    const result = await timedFetch(`/api/plans/${today}/orders`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Client-ID': `perf-${Date.now()}-${i}`,
      },
      body: JSON.stringify({ dishId, userId: memberId, note: '性能基准临时数据' }),
    })
    writeValues.push(result.ms)
    if (!result.response.ok) throw new Error(`点菜失败 ${result.response.status}`)
    const plan = JSON.parse(new TextDecoder().decode(result.body))
    const order = plan.orders?.find((item) => item.dishId === dishId)
    if (order?.id) createdOrders.push(order.id)
  }))
  const writeElapsed = performance.now() - writeStart
  printStats(`点菜并发 ${dishIds.length}`, writeValues)
  console.log(`${'点菜总吞吐'.padEnd(24)} ${(dishIds.length / (writeElapsed / 1000)).toFixed(1)} req/s`)

  for (const orderId of createdOrders) {
    const result = await timedFetch(`/api/plans/${today}/orders/${orderId}`, { method: 'DELETE' })
    if (!result.response.ok) throw new Error(`清理临时订单失败 ${result.response.status}`)
  }
  console.log(`已清理 ${createdOrders.length} 条临时订单。`)

  // 批量事务基准：页面「批量点菜 / 再来一桌」共用此链路。
  const occupied = new Set((boot.data.plan?.orders || []).map((order) => order.dishId))
  const batchDishIDs = boot.data.dishes
    .map((dish) => dish.id)
    .filter((id) => !occupied.has(id))
    .slice(0, 40)
  const batch = await timedFetch(`/api/plans/${today}/orders/batch`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Client-ID': `perf-batch-${Date.now()}`,
    },
    body: JSON.stringify({
      userId: memberId,
      items: batchDishIDs.map((dishId) => ({ dishId, note: '批量性能基准临时数据' })),
    }),
  })
  if (!batch.response.ok) throw new Error(`批量点菜失败 ${batch.response.status}`)
  const batchPlan = JSON.parse(new TextDecoder().decode(batch.body))
  const batchOrderIDs = (batchPlan.plan?.orders || [])
    .filter((order) => batchDishIDs.includes(order.dishId))
    .map((order) => order.id)
  console.log(`${'批量点菜 40 道'.padEnd(24)} ${batch.ms.toFixed(1)}ms（单次事务，新增 ${batchPlan.added} 道）`)

  const done = await timedFetch(`/api/plans/${today}/orders/batch`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'X-Client-ID': `perf-batch-done-${Date.now()}`,
    },
    body: JSON.stringify({ orderIds: batchOrderIDs, done: true }),
  })
  if (!done.response.ok) throw new Error(`批量上桌失败 ${done.response.status}`)
  console.log(`${'批量上桌 40 道'.padEnd(24)} ${done.ms.toFixed(1)}ms（单次事务）`)
  for (const orderId of batchOrderIDs) {
    const result = await timedFetch(`/api/plans/${today}/orders/${orderId}`, { method: 'DELETE' })
    if (!result.response.ok) throw new Error(`清理批量订单失败 ${result.response.status}`)
  }
}

baseline().catch((error) => {
  console.error(`\n性能基准失败：${error.stack || error.message}`)
  process.exitCode = 1
})
