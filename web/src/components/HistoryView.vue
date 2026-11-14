<script setup>
import { inject, computed } from 'vue'
import { friendlyDate, memberOf, cuisineIcon, cuisineName, mealOf } from '../utils'

const { state } = inject('app')

const plans = computed(() => (state.plans || []).filter(
  (p) => (p.orders?.length || 0) > 0 || p.chefId || p.dishwasherId || p.note,
))
const totalDishes = computed(() => plans.value.reduce((n, p) => n + (p.orders?.length || 0), 0))
const thisMonth = computed(() => {
  const prefix = (state.today || '').slice(0, 7)
  return plans.value.filter((p) => p.date.startsWith(prefix)).reduce((n, p) => n + (p.orders?.length || 0), 0)
})

// 近 14 天每日点菜量：柱状趋势，纯 CSS 渲染，不引图表库
const trendDays = computed(() => {
  const byDate = new Map(plans.value.map((p) => [p.date, p]))
  const base = state.today ? new Date(`${state.today}T12:00:00`) : new Date()
  const out = []
  const p2 = (n) => String(n).padStart(2, '0')
  for (let i = 13; i >= 0; i--) {
    const d = new Date(base)
    d.setDate(base.getDate() - i)
    const key = `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())}`
    out.push({ date: key, label: `${d.getMonth() + 1}/${d.getDate()}`, count: byDate.get(key)?.orders?.length || 0 })
  }
  return out
})
const trendMax = computed(() => Math.max(1, ...trendDays.value.map((d) => d.count)))
const trendTotal = computed(() => trendDays.value.reduce((n, d) => n + d.count, 0))

// 菜系分布：按历史订单的菜系统计次数与占比（Top 5）
const cuisineDist = computed(() => {
  const map = new Map()
  for (const p of plans.value) {
    for (const o of (p.orders || [])) {
      const k = o.dishCuisine || 'homestyle'
      map.set(k, (map.get(k) || 0) + 1)
    }
  }
  const total = [...map.values()].reduce((a, b) => a + b, 0) || 1
  return [...map.entries()]
    .map(([k, n]) => ({ key: k, name: cuisineName(k, state.cuisines), icon: cuisineIcon(k, state.cuisines), n, pct: Math.round((n / total) * 100) }))
    .sort((a, b) => b.n - a.n)
    .slice(0, 5)
})
</script>

<template>
  <section>
    <div class="section-head">
      <div>
        <h2 class="section-title">🗓️ 吃过什么</h2>
        <div class="section-sub">每天点了什么、谁掌勺谁洗碗，都留在这里；删掉菜品也不会丢掉菜名</div>
      </div>
    </div>

    <div class="stat-strip mt-0 mb-18">
      <div class="stat-item"><span class="stat-num value-pop" :key="`days-${plans.length}`">{{ plans.length }}</span> 天有记录</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`all-${totalDishes}`">{{ totalDishes }}</span> 道菜累计</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`month-${thisMonth}`">{{ thisMonth }}</span> 道本月</div>
    </div>

    <div v-if="plans.length" class="trend-card card">
      <div class="trend-head">
        <span class="trend-title">📈 近 14 天点菜量</span>
        <span class="tiny muted">共 {{ trendTotal }} 道</span>
      </div>
      <div class="trend-chart">
        <div
          v-for="d in trendDays" :key="d.date"
          class="trend-bar" :class="{ zero: !d.count }"
          :style="{ height: `${Math.max(4, Math.round((d.count / trendMax) * 100))}%` }"
          :title="`${d.label} · ${d.count} 道`"
        ></div>
      </div>
      <div class="trend-axis">
        <span>{{ trendDays[0]?.label }}</span>
        <span>{{ trendDays[trendDays.length - 1]?.label }}</span>
      </div>
    </div>

    <div v-if="cuisineDist.length" class="dist-card card">
      <div class="trend-head">
        <span class="trend-title">🍜 菜系分布</span>
        <span class="tiny muted">按历史点菜次数</span>
      </div>
      <div v-for="c in cuisineDist" :key="c.key" class="dist-row">
        <span class="dist-name">{{ c.icon }} {{ c.name }}</span>
        <div class="dist-track"><div class="dist-fill" :style="{ width: `${c.pct}%` }"></div></div>
        <span class="dist-pct">{{ c.pct }}%</span>
      </div>
    </div>

    <div v-if="plans.length" class="history-list">
      <article v-for="p in plans" :key="p.date" class="card history-card">
        <div class="history-head">
          <div class="row">
            <span class="history-date">{{ friendlyDate(p.date) }}</span>
            <span class="tag">{{ p.orders?.length || 0 }} 道菜</span>
          </div>
          <div class="tiny muted">
            <template v-if="memberOf(p.chefId, state.members)">
              {{ memberOf(p.chefId, state.members).avatar }} {{ memberOf(p.chefId, state.members).name }} 掌勺
            </template>
            <template v-else>掌勺人未定</template>
            <template v-if="memberOf(p.dishwasherId, state.members)">
              · 🧽 {{ memberOf(p.dishwasherId, state.members).name }} 洗碗
            </template>
          </div>
        </div>
        <div v-if="p.orders?.length" class="history-dishes">
          <span v-for="o in p.orders" :key="o.id" class="history-dish">
            <span v-if="mealOf(o.meal, state.meals)" class="history-meal">
              {{ mealOf(o.meal, state.meals).icon }} {{ mealOf(o.meal, state.meals).label }}
            </span>
            {{ cuisineIcon(o.dishCuisine, state.cuisines) }} {{ o.dishName }}
            <span class="muted tiny">{{ o.userName }}</span>
          </span>
        </div>
        <p v-if="p.note" class="tiny muted mt-9">📝 {{ p.note }}</p>
      </article>
    </div>
    <div v-else class="empty">
      <span class="empty-icon">🍽️</span>
      还没有历史记录，今天先点起来
    </div>
  </section>
</template>
