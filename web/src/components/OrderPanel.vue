<script setup>
// 已下单抽屉：在菜谱页直接核对今天点了什么，不必回首页。
import { ref, computed, inject } from 'vue'
import { useModalBehavior } from '../modal'
import { imageSize, normalizeMeals } from '../utils'
import DishPhoto from './DishPhoto.vue'

const emit = defineEmits(['close'])
const { state, actions, pending } = inject('app')

const root = ref(null)
useModalBehavior(() => emit('close'), root)

const orders = computed(() => state.plan?.orders || [])
const meals = computed(() => normalizeMeals(state.meals))
const groups = computed(() => {
  const g = { other: [] }
  for (const m of meals.value) g[m.key] = []
  for (const o of orders.value) {
    if (o.meal && g[o.meal]) g[o.meal].push(o)
    else g.other.push(o)
  }
  const all = [...meals.value, { key: 'other', label: '其他', icon: '🍽️' }]
  return all.map((m) => ({ ...m, items: g[m.key] || [] })).filter((m) => m.items.length)
})
const pick = ref('')
const shown = computed(() => pick.value ? groups.value.filter((g) => g.key === pick.value) : groups.value)
const doneCount = computed(() => orders.value.filter((o) => o.done).length)
const allMealKeys = computed(() => [...meals.value, { key: 'other', label: '其他', icon: '🍽️' }])
function mealLabel(key) { return allMealKeys.value.find((m) => m.key === key) || { label: '其他', icon: '🍽️' } }
function togglePick(key) { pick.value = pick.value === key ? '' : key }
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div ref="root" class="modal order-panel" role="dialog" aria-modal="true" aria-label="已下单的菜">
      <div class="modal-head">
        <h2>🧾 已下单 <span class="tiny muted">{{ orders.length }} 道 · {{ doneCount }} 道已上桌</span></h2>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <div class="order-meal-tabs" role="group" aria-label="按餐次查看">
        <button
          v-for="g in groups" :key="g.key" type="button"
          class="chip" :class="{ active: pick === g.key }"
          :aria-pressed="pick === g.key" @click="togglePick(g.key)"
        >{{ g.icon }} {{ g.label }} <span class="tiny">{{ g.items.length }}</span></button>
        <button v-if="pick" class="chip" type="button" @click="pick = ''">全部</button>
      </div>
      <div class="order-panel-list">
        <div v-for="g in shown" :key="g.key" class="order-panel-group">
          <div class="order-panel-head">{{ g.icon }} {{ g.label }} · {{ g.items.length }} 道</div>
          <div v-for="o in g.items" :key="o.id" class="order-panel-row" :class="{ done: o.done }">
            <DishPhoto
              :item="o" :cuisines="state.cuisines" :dishes="state.dishes"
              :size="imageSize(state.settings.imageMode)" :alt="o.dishName" loading="lazy"
            />
            <div class="opr-main">
              <div class="opr-name">{{ o.dishName }}</div>
              <div class="opr-meta tiny muted">
                {{ o.userAvatar }} {{ o.userName }}
                <span v-if="o.note"> · 「{{ o.note }}」</span>
                <span v-else-if="!o.meal"> · {{ mealLabel(o.meal).label }}</span>
              </div>
            </div>
            <button
              class="icon-btn" :title="o.done ? '标记为没做' : '标记已上桌'"
              :aria-label="o.done ? '标记为没做' : '标记已上桌'"
              :disabled="pending(`toggle:${o.id}`)" @click="actions.toggleOrder(o.id)"
            >{{ o.done ? '✅' : '⭕' }}</button>
            <button
              class="icon-btn icon-btn-danger" title="撤掉这道" aria-label="撤掉这道"
              :disabled="pending(`remove:${o.id}`)" @click="actions.removeOrder(o.id)"
            >✕</button>
          </div>
        </div>
        <div v-if="!orders.length" class="empty pad-18">
          <span class="empty-icon">🧾</span>还没点菜，去挑几道吧
        </div>
      </div>
    </div>
  </div>
</template>
