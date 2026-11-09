// 接口端到端冒烟测试：对着一个跑起来的实例执行，尽量不留痕迹。
// 用法：BASE=http://127.0.0.1:8090 node scripts/api-test.mjs
import assert from 'node:assert/strict'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const HEADERS = { 'Content-Type': 'application/json' }
if (process.env.CODE) HEADERS['X-Access-Code'] = process.env.CODE

let passed = 0
function ok(label) {
  passed++
  console.log(`  ✓ ${label}`)
}

async function call(path, { method = 'GET', body, raw = false } = {}) {
  const res = await fetch(BASE + path, {
    method,
    headers: HEADERS,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (raw) {
    assert.ok(res.ok, `${path} 应返回 2xx，实际 ${res.status}`)
    return Buffer.from(await res.arrayBuffer())
  }
  const data = await res.json()
  assert.ok(res.ok, `${path} 应返回 2xx，实际 ${res.status}：${data.error || ''}`)
  return data
}

const createdDishes = []
const createdOrders = []

try {
  console.log(`\n测试目标：${BASE}\n`)

  const ping = await call('/api/ping')
  assert.equal(ping.ok, true)
  ok('健康检查 /api/ping')

  const cfg = await call('/api/config')
  assert.ok(cfg.version)
  ok(`版本信息 v${cfg.version}`)

  const boot = await call('/api/bootstrap')
  assert.ok(boot.dishes.length > 50, '内置菜品应超过 50 道')
  assert.ok(boot.members.length >= 1, '至少有一位示例成员')
  assert.ok(boot.cuisines.length >= 10, '菜系分类应超过 10 类')
  ok(`首屏数据：${boot.dishes.length} 道菜 / ${boot.cuisines.length} 个分类 / ${boot.members.length} 位成员`)

  const today = boot.today
  const originalPlan = boot.plan
  const memberId = boot.members[0].id
  const stamp = Date.now()

  const dish = await call('/api/dishes', {
    method: 'POST',
    body: {
      name: `接口测试菜-${stamp}`, cuisine: 'homestyle', desc: '测试用，稍后删除',
      tags: ['测试'], spicy: 1, minutes: 12,
    },
  })
  createdDishes.push(dish.id)
  assert.equal(dish.builtin, false)
  ok('添加自定义菜品')

  const renamed = await call(`/api/dishes/${dish.id}`, {
    method: 'PUT', body: { favorite: true, desc: '改过的描述' },
  })
  assert.equal(renamed.favorite, true)
  assert.equal(renamed.desc, '改过的描述')
  ok('编辑菜品 / 收藏')

  const planned = await call(`/api/plans/${today}`, {
    method: 'PUT', body: { chefId: memberId, dishwasherId: memberId, note: `接口测试备注-${stamp}` },
  })
  assert.equal(planned.chefId, memberId)
  assert.equal(planned.dishwasherId, memberId)
  ok('指定今天谁掌勺 / 谁洗碗 + 写备注')

  const withOrder = await call(`/api/plans/${today}/orders`, {
    method: 'POST', body: { dishId: dish.id, userId: memberId, note: '少放辣' },
  })
  const order = withOrder.orders.at(-1)
  createdOrders.push(order.id)
  assert.equal(order.dishName, dish.name)
  assert.equal(order.userName, boot.members[0].name)
  assert.equal(order.done, false)
  ok('点菜并写入菜名 / 成员快照')

  const toggled = await call(`/api/plans/${today}/orders/${order.id}`, { method: 'PUT' })
  assert.equal(toggled.orders.find((o) => o.id === order.id).done, true)
  ok('标记已上桌')

  const plans = await call('/api/plans')
  assert.ok(plans.some((p) => p.date === today))
  ok('历史记录列表')

  const zip = await call('/api/backup', { raw: true })
  assert.equal(zip.subarray(0, 2).toString(), 'PK', '备份应是 zip')
  assert.ok(zip.length > 1000, '备份应包含数据')
  ok(`导出备份 zip（${(zip.length / 1024).toFixed(1)} KB）`)

  await call(`/api/plans/${today}/orders/${order.id}`, { method: 'DELETE' })
  createdOrders.length = 0
  ok('撤掉点菜')

  await call(`/api/plans/${today}`, {
    method: 'PUT', body: {
      chefId: originalPlan.chefId || '',
      dishwasherId: originalPlan.dishwasherId || '',
      note: originalPlan.note || '',
    },
  })
  ok('还原今天的掌勺人 / 洗碗人与备注')

  await call(`/api/dishes/${dish.id}`, { method: 'DELETE' })
  createdDishes.length = 0
  ok('删除测试菜品')

  let rejected = false
  try {
    await call(`/api/plans/not-a-date`, { method: 'PUT', body: { note: 'x' } })
  } catch {
    rejected = true
  }
  assert.ok(rejected, '非法日期应被拒绝')
  ok('非法日期被拒绝')

  console.log(`\n全部通过，共 ${passed} 项。\n`)
} catch (err) {
  console.error(`\n✗ 测试失败：${err.message}\n`)
  process.exitCode = 1
} finally {
  // 即使中途失败，也尽量清掉测试数据。
  for (const id of createdOrders) {
    try { await call(`/api/plans/${(await call('/api/bootstrap')).today}/orders/${id}`, { method: 'DELETE' }) } catch {}
  }
  for (const id of createdDishes) {
    try { await call(`/api/dishes/${id}`, { method: 'DELETE' }) } catch {}
  }
}
