<script setup>
import { computed, inject } from 'vue'
import { classifyDish } from '../utils'
import { getRecipe } from '../recipes'

const { state } = inject('app')
const CAT = {
  meat: { label: '荤菜', color: '#c76f62' },
  veg: { label: '素菜', color: '#6f9a77' },
  soup: { label: '汤羹', color: '#c99a58' },
  staple: { label: '主食', color: '#8a7cab' },
  dessert: { label: '甜品', color: '#c68fa4' },
}

const start = computed(() => {
  const base = new Date(`${state.today}T12:00:00`)
  base.setDate(base.getDate() - 6)
  const p = (n) => String(n).padStart(2, '0')
  return `${base.getFullYear()}-${p(base.getMonth() + 1)}-${p(base.getDate())}`
})
const plans = computed(() => (state.plans || []).filter((plan) => plan.date >= start.value && plan.date <= state.today))
const orders = computed(() => plans.value.flatMap((plan) => (plan.orders || []).map((order) => ({ ...order, date: plan.date }))))
const dishById = computed(() => new Map((state.dishes || []).map((dish) => [dish.id, dish])))
const uniqueDishCount = computed(() => new Set(orders.value.map((order) => order.dishId).filter(Boolean)).size)
const repeatRate = computed(() => {
  if (!orders.value.length) return 0
  return Math.round((1 - uniqueDishCount.value / orders.value.length) * 100)
})
const categoryRows = computed(() => {
  const counts = {}
  for (const order of orders.value) {
    const dish = dishById.value.get(order.dishId)
    if (!dish) continue
    const kind = classifyDish(dish, getRecipe(dish).ingredients)
    if (CAT[kind]) counts[kind] = (counts[kind] || 0) + 1
  }
  const total = Object.values(counts).reduce((a, b) => a + b, 0) || 1
  return Object.entries(CAT).map(([key, meta]) => ({
    key, ...meta, count: counts[key] || 0, percent: Math.round(((counts[key] || 0) / total) * 100),
  }))
})
const memberRows = computed(() => {
  const map = {}
  for (const order of orders.value) {
    if (!order.userId) continue
    map[order.userId] = (map[order.userId] || 0) + 1
  }
  return (state.members || [])
    .map((member) => ({ member, count: map[member.id] || 0 }))
    .sort((a, b) => b.count - a.count || a.member.name.localeCompare(b.member.name, 'zh'))
})
const topRated = computed(() => (state.dishes || [])
  .filter((dish) => dish.rating >= 2)
  .sort((a, b) => b.rating - a.rating || a.name.localeCompare(b.name, 'zh'))
  .slice(0, 5))
const repeated = computed(() => {
  const map = {}
  for (const order of orders.value) map[order.dishName] = (map[order.dishName] || 0) + 1
  return Object.entries(map).filter(([, count]) => count > 1).sort((a, b) => b[1] - a[1]).slice(0, 5)
})
const insights = computed(() => {
  const out = []
  const meat = categoryRows.value.find((row) => row.key === 'meat')?.percent || 0
  const veg = categoryRows.value.find((row) => row.key === 'veg')?.percent || 0
  if (!orders.value.length) return ['最近 7 天还没有点菜记录，先点几顿再来看。']
  if (meat >= 60) out.push('荤菜占比偏高，下周菜单可以多安排两道清爽素菜。')
  else if (veg <= 15) out.push('素菜出现得少，建议每顿至少配一道蔬菜。')
  else out.push('荤素搭配比较均衡，继续保持。')
  if (repeatRate.value >= 35) out.push(`重复率 ${repeatRate.value}%，菜色有些集中，可以试试「换一版」周菜单。`)
  else out.push(`最近点了 ${uniqueDishCount.value} 道不同的菜，花样还不错。`)
  const topMember = memberRows.value.find((row) => row.count > 0)
  if (topMember) out.push(`${topMember.member.avatar} ${topMember.member.name} 最近点菜最积极，共 ${topMember.count} 道。`)
  return out
})
</script>

<template>
  <section class="panel taste-report" :class="{ 'is-empty': !orders.length }">
    <div class="taste-head">
      <div>
        <div class="panel-title">📊 家庭口味周报</div>
        <div class="section-sub">
          {{ orders.length ? `最近 7 天 · ${orders.length} 次点菜 · ${uniqueDishCount} 道不同的菜` : '最近 7 天还没有点菜记录，点过几顿后这里会自动汇总。' }}
        </div>
      </div>
      <span v-if="orders.length" class="taste-repeat">重复率 {{ repeatRate }}%</span>
    </div>

    <div v-if="!orders.length" class="taste-empty">
      <span class="taste-empty-icon" aria-hidden="true">🥢</span>
      <div class="taste-empty-copy">
        <b>周报还在等第一顿饭</b>
        <span>点几道菜、标一次上桌，下周就能看到全家口味、荤素搭配和重复度。</span>
      </div>
      <button class="btn btn-soft btn-sm" type="button" @click="state.tab = 'menu'">去点菜</button>
    </div>

    <template v-else>
      <div class="taste-kpis">
        <div><b>{{ orders.length }}</b><span>点菜次数</span></div>
        <div><b>{{ uniqueDishCount }}</b><span>菜品花样</span></div>
        <div><b>{{ memberRows.filter((row) => row.count > 0).length }}</b><span>参与人数</span></div>
        <div><b>{{ topRated.length }}</b><span>好评菜</span></div>
      </div>

      <div class="taste-body">
        <div class="taste-cats">
          <div v-for="row in categoryRows" :key="row.key" class="taste-cat">
            <span class="taste-cat-name">{{ row.label }}</span>
            <span class="taste-cat-track"><i :style="{ width: `${row.percent}%`, background: row.color }"></i></span>
            <span class="taste-cat-count">{{ row.count }} · {{ row.percent }}%</span>
          </div>
        </div>
        <div class="taste-rank">
          <div class="taste-subhead">谁最常点</div>
          <div v-for="row in memberRows" :key="row.member.id" class="taste-member">
            <span>{{ row.member.avatar }} {{ row.member.name }}</span>
            <b>{{ row.count }} 道</b>
          </div>
        </div>
      </div>

      <div class="taste-columns">
        <div>
          <div class="taste-subhead">最近重复最多</div>
          <div v-if="repeated.length" class="taste-tags">
            <span v-for="[name, count] in repeated" :key="name" class="tag">{{ name }} ×{{ count }}</span>
          </div>
          <p v-else class="tiny muted">最近没有重复点同一道菜。</p>
        </div>
        <div>
          <div class="taste-subhead">全家好评菜</div>
          <div v-if="topRated.length" class="taste-tags">
            <span v-for="dish in topRated" :key="dish.id" class="tag">{{ dish.rating === 3 ? '⭐' : '👍' }} {{ dish.name }}</span>
          </div>
          <p v-else class="tiny muted">还没有「好吃 / 超赞」评价。</p>
        </div>
      </div>

      <div class="taste-insights">
        <div class="taste-subhead">下周建议</div>
        <p v-for="(text, index) in insights" :key="index">💡 {{ text }}</p>
      </div>
    </template>
  </section>
</template>
