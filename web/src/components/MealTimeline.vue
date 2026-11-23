<script setup>
import { computed, inject, ref } from 'vue'
import { getRecipe } from '../recipes'
import { dishServings, scaleIngredients } from '../utils'
import { useModalBehavior } from '../modal'

const props = defineProps({
  orders: { type: Array, default: () => [] },
  dishes: { type: Array, default: () => [] },
  mealLabel: { type: String, default: '这一餐' },
  serveTime: { type: String, default: '18:30' },
})
const emit = defineEmits(['close'])
const { state } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const target = ref(/^\d{2}:\d{2}$/.test(props.serveTime) ? props.serveTime : '18:30')
const people = ref(Math.max(2, state.members?.length || 3))
const byId = computed(() => new Map((props.dishes || []).map((dish) => [dish.id, dish])))
const validItems = computed(() => {
  const seen = new Set()
  return (props.orders || []).flatMap((order) => {
    if (!order?.dishId || seen.has(order.dishId)) return []
    const dish = byId.value.get(order.dishId)
    if (!dish) return []
    seen.add(order.dishId)
    const recipe = getRecipe(dish)
    const factor = people.value / dishServings(dish)
    return [{
      dish,
      order,
      minutes: Math.max(5, Number(dish.minutes) || 20),
      ingredients: scaleIngredients(recipe.ingredients, factor),
      steps: recipe.steps || [],
    }]
  })
})

function toMinutes(value) {
  const [h, m] = String(value).split(':').map(Number)
  return (Number.isFinite(h) ? h : 18) * 60 + (Number.isFinite(m) ? m : 30)
}
function clock(total) {
  const n = ((Math.round(total) % 1440) + 1440) % 1440
  return `${String(Math.floor(n / 60)).padStart(2, '0')}:${String(n % 60).padStart(2, '0')}`
}
function stepWeight(text) {
  const s = String(text || '')
  if (/腌|泡|解冻|醒发/.test(s)) return 3
  if (/炖|焖|煲|蒸|烤|烧开|慢煮/.test(s)) return 5
  if (/焯|洗|切|调|拌|准备/.test(s)) return 2
  return 1.5
}
function buildDish(item) {
  const steps = item.steps.length ? item.steps : [`按家常做法完成${item.dish.name}`]
  const weights = steps.map(stepWeight)
  const sum = weights.reduce((a, b) => a + b, 0) || 1
  const start = toMinutes(target.value) - item.minutes
  let cursor = start
  const tasks = steps.map((text, index) => {
    const duration = Math.max(1, Math.round((item.minutes * weights[index]) / sum))
    const task = {
      id: `${item.dish.id}-${index}`,
      dishName: item.dish.name,
      index,
      text,
      start: cursor,
      end: cursor + duration,
      kind: /腌|泡|解冻|醒发/.test(text) ? 'prep' : /炖|焖|煲|蒸|烤|烧开|慢煮/.test(text) ? 'heat' : 'work',
    }
    cursor += duration
    return task
  })
  if (tasks.length) tasks[tasks.length - 1].end = toMinutes(target.value)
  return { ...item, start, end: toMinutes(target.value), tasks }
}

const dishes = computed(() => validItems.value.map(buildDish).sort((a, b) => a.start - b.start || a.dish.name.localeCompare(b.dish.name, 'zh')))
const allTasks = computed(() => dishes.value.flatMap((dish) => dish.tasks).sort((a, b) => a.start - b.start || a.index - b.index))
const earliest = computed(() => Math.min(...dishes.value.map((dish) => dish.start), toMinutes(target.value)))
const activeTasks = computed(() => {
  const now = state.now || Date.now()
  const d = new Date(now)
  const minute = d.getHours() * 60 + d.getMinutes()
  return allTasks.value.filter((task) => task.start <= minute && task.end > minute)
})
const prepCount = computed(() => allTasks.value.filter((task) => task.kind === 'prep').length)
const longCount = computed(() => allTasks.value.filter((task) => task.kind === 'heat').length)
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide meal-timeline-modal" role="dialog" aria-modal="true" aria-label="做饭时间线">
      <div class="modal-head">
        <h2>⏱️ 做饭时间线</h2>
        <button class="icon-btn" type="button" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <p class="setting-desc">
        把 {{ mealLabel }} 的菜排成一条时间线：先腌、先炖的先开始，最后统一上桌。时间是按菜谱耗时推算的，可随时调整。
      </p>
      <div class="timeline-controls">
        <label>想吃的时间 <input v-model="target" class="input" type="time" /></label>
        <label>人数
          <select v-model.number="people" class="select">
            <option v-for="n in [2, 3, 4, 5, 6, 7, 8]" :key="n" :value="n">{{ n }} 人</option>
          </select>
        </label>
        <span class="timeline-window">建议 {{ clock(earliest) }} 开始 · {{ clock(toMinutes(target)) }} 上桌</span>
      </div>
      <div class="timeline-summary">
        <span><b>{{ dishes.length }}</b> 道菜</span>
        <span><b>{{ allTasks.length }}</b> 个步骤</span>
        <span><b>{{ prepCount }}</b> 个提前准备</span>
        <span><b>{{ longCount }}</b> 个需看火</span>
      </div>
      <div v-if="activeTasks.length" class="timeline-now">
        <b>现在做</b>
        <span v-for="task in activeTasks" :key="task.id">{{ task.dishName }} · {{ task.text }}</span>
      </div>
      <div v-if="dishes.length" class="timeline-list">
        <section v-for="item in dishes" :key="item.dish.id" class="timeline-dish">
          <header>
            <span class="timeline-dish-name">{{ item.dish.name }}</span>
            <span class="timeline-dish-time">{{ clock(item.start) }} → {{ clock(item.end) }}</span>
            <span class="timeline-dish-people">{{ people }} 人份 · {{ item.minutes }} 分钟</span>
          </header>
          <ol class="timeline-steps">
            <li v-for="task in item.tasks" :key="task.id" :class="`kind-${task.kind}`">
              <span class="timeline-step-time">{{ clock(task.start) }}</span>
              <span class="timeline-step-text">{{ task.text }}</span>
              <span class="timeline-step-duration">{{ Math.max(1, task.end - task.start) }} 分钟</span>
            </li>
          </ol>
          <details>
            <summary>🧺 按 {{ people }} 人换算的配料</summary>
            <p>{{ item.ingredients.join(' · ') }}</p>
          </details>
        </section>
      </div>
      <div v-else class="empty pad-18"><span class="empty-icon">🍳</span>这一餐还没有菜</div>
      <div class="modal-foot">
        <span class="tiny muted">提前准备和慢炖任务可并行；同一口锅仍需按实际火候调整。</span>
        <button class="btn btn-primary" type="button" @click="emit('close')">知道了</button>
      </div>
    </div>
  </div>
</template>
