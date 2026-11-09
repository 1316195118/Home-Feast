<script setup>
import { inject, computed } from 'vue'
import { friendlyDate, memberOf, cuisineIcon, mealOf } from '../utils'

const { state } = inject('app')

const plans = computed(() => (state.plans || []).filter(
  (p) => (p.orders?.length || 0) > 0 || p.chefId || p.dishwasherId || p.note,
))
const totalDishes = computed(() => plans.value.reduce((n, p) => n + (p.orders?.length || 0), 0))
const thisMonth = computed(() => {
  const prefix = (state.today || '').slice(0, 7)
  return plans.value.filter((p) => p.date.startsWith(prefix)).reduce((n, p) => n + (p.orders?.length || 0), 0)
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

    <div class="stat-strip" style="margin: 0 0 18px">
      <div class="stat-item"><span class="stat-num">{{ plans.length }}</span> 天有记录</div>
      <div class="stat-item"><span class="stat-num">{{ totalDishes }}</span> 道菜累计</div>
      <div class="stat-item"><span class="stat-num">{{ thisMonth }}</span> 道本月</div>
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
        <p v-if="p.note" class="tiny muted" style="margin-top: 9px">📝 {{ p.note }}</p>
      </article>
    </div>
    <div v-else class="empty">
      <span class="empty-icon">🍽️</span>
      还没有历史记录，今天先点起来
    </div>
  </section>
</template>
