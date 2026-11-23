<script setup>
import { inject, computed, ref } from 'vue'
import {
  applyPantryStock, dishServings, normalizeIngredient, scaleIngredients,
  shoppingGroups, shoppingKind, shoppingList,
} from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'
import WeChatShareActions from './WeChatShareActions.vue'

const emit = defineEmits(['close'])
const props = defineProps({
  orders: { type: Array, default: null },
  label: { type: String, default: '' },
  scopeKey: { type: String, default: '' },
  people: { type: Number, default: 0 },
})
const { state, actions } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)
const activeDate = computed(() => state.selectedDate || state.today)
const scopeDate = computed(() => props.scopeKey || activeDate.value)
const sourceOrders = computed(() => props.orders || state.plan.orders || [])
const people = ref(Math.max(1, props.people || state.members?.length || 3))
const scopeTitle = computed(() => props.label || `${activeDate.value} 购物清单`)

// 内置菜的配料由 recipes.js 按「技法模板 + 主料表」动态生成，dish.ingredients 本身是空的；
// 用户在编辑器里自己填的配料优先（getRecipe 已处理），这里把它展开后再聚合。
const dishesWithIngredients = computed(() => state.dishes.map((d) => {
  const r = getRecipe(d)
  const base = r.ingredients.length ? r.ingredients : (d.ingredients || [])
  return {
    ...d,
    ingredients: scaleIngredients(base, people.value / dishServings(d)),
    precise: !!r.precise,
  }
}))
const rawList = computed(() => shoppingList(sourceOrders.value, dishesWithIngredients.value))
const stockedList = computed(() => applyPantryStock(rawList.value, state.settings.pantryStock || {}))
const list = computed(() => stockedList.value.filter((item) => !item.pantryCovered))
const coveredCount = computed(() => stockedList.value.filter((item) => item.pantryCovered).length)
const preciseCount = computed(() => {
  const byId = new Map(dishesWithIngredients.value.map((d) => [d.id, d]))
  return sourceOrders.value.filter((o) => byId.get(o.dishId)?.precise).length
})
// 冰箱里已有什么（仅主料，调味料不当作「已有」）：用来在清单上标注「🧊 冰箱有」
const pantrySet = computed(() => new Set(
  (state.settings.pantry || [])
    .map((x) => normalizeIngredient(String(x)))
    .filter((x) => shoppingKind(x) !== '佐料'),
))
// 把「是否命中冰箱」挂到每个清单项上，命中即提示不用重复买
const enriched = computed(() => list.value.map((it) => ({
  ...it,
  inPantry: pantrySet.value.has(normalizeIngredient(it.text)),
})))
// 勾选过的沉到组尾，买过的不再挡视线；checked 在下方定义，computed 惰性求值没问题
const groups = computed(() => shoppingGroups(enriched.value).map((g) => {
  const items = [...g.items]
  items.sort((a, b) => Number(checkedToday(a.text)) - Number(checkedToday(b.text)) || Number(checkedBefore(a.text)) - Number(checkedBefore(b.text)))
  return { ...g, items }
}))
// 采购状态存服务端 settings，所有设备实时共享。status 为 claimed / bought。
const marks = computed(() => state.settings.shoppingMarks || {})
const markKey = (text) => `${scopeDate.value}::${text}`
const markOf = (text) => {
  const scoped = marks.value[markKey(text)]
  if (scoped) return scoped
  // 兼容上一轮开发版没有作用域前缀的记录，只在每日清单里兜底。
  return props.scopeKey ? null : marks.value[text] || null
}
// 今天勾的才算「已买」；历史勾选只用于提示
const checkedToday = (t) => {
  const mark = markOf(t)
  return mark?.status === 'bought' && mark.date === state.today
}
const checkedBefore = (t) => {
  const mark = markOf(t)
  return mark?.status === 'bought' && mark.date && mark.date < state.today
}
const claimOf = (t) => {
  const mark = markOf(t)
  return mark?.status === 'claimed' && mark.date === state.today ? mark : null
}
async function setMarks(items) {
  return actions.setShoppingMarks(
    items.map((item) => ({ ...item, key: markKey(item.key) })),
    state.today,
  )
}
async function toggle(text) {
  await setMarks([{ key: text, status: checkedToday(text) ? '' : 'bought' }])
}
async function claim(text) {
  await setMarks([{ key: text, status: 'claimed' }])
}
// 按超市区域整组勾选；已全选时再点即整组取消
async function toggleGroup(group) {
  const items = group.items
  const allChecked = items.every((x) => checkedToday(x.text))
  await setMarks(items.map((x) => ({ key: x.text, status: allChecked ? '' : 'bought' })))
  actions.toast(
    `${group.key} ${items.length} 项${allChecked ? '已取消全选' : '已全部勾选'}`,
    allChecked ? 'info' : 'success',
  )
}
function groupAllChecked(items) {
  return items.length > 0 && items.every((x) => checkedToday(x.text))
}
const checkedCount = computed(() => groups.value.reduce(
  (sum, group) => sum + group.items.filter((item) => checkedToday(item.text)).length,
  0,
))
const allChecked = computed(() => list.value.length > 0 && checkedCount.value === list.value.length)
const checkedPercent = computed(() => list.value.length ? Math.round((checkedCount.value / list.value.length) * 100) : 0)
async function toggleAll() {
  const select = !allChecked.value
  await setMarks(list.value.map((item) => ({
    key: item.text,
    status: select ? 'bought' : (checkedToday(item.text) ? '' : markOf(item.text)?.status || ''),
  })))
  actions.toast(
    select ? `已全选 ${list.value.length} 项` : '已取消所选日期的全部勾选',
    select ? 'success' : 'info',
  )
}
async function clearChecked() {
  await setMarks([...new Set(list.value.map((item) => item.text))]
    .map((key) => ({ key, status: '' })))
  actions.toast('这份清单的采购状态已清空', 'info')
}
const previewAlt = computed(() => {
  const count = groups.value.reduce(
    (sum, group) => sum + group.items.filter((item) => checkedToday(item.text)).length,
    0,
  )
  return `${activeDate.value} 购物清单图，已选 ${count} 项`
})
async function copyList() {
  const lines = groups.value.flatMap((g) => g.items.length
    ? [`【${g.key}】`, ...g.items.map(
        (x) => `${checkedToday(x.text) ? '☑' : claimOf(x.text) ? '◉' : checkedBefore(x.text) ? '◔' : '☐'} ${x.inPantry ? '🧊 ' : ''}${x.text}${x.amount ? ` · ${x.amount}` : ''}${x.count > 1 ? ` ×${x.count}` : ''}`,
      )]
    : [])
  const text = `家里吃点啥 · ${scopeTitle.value}（${people.value} 人份）\n${lines.join('\n')}`
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    ta.remove()
  }
  actions.toast('清单已复制，可以直接发给买菜的人', 'success')
}
const shareText = computed(() => {
    const lines = groups.value.flatMap((g) => g.items.filter((x) => checkedToday(x.text)).map(
    (x) => `${x.inPantry ? '🧊 ' : ''}${x.text}${x.amount ? ` · ${x.amount}` : ''}${x.count > 1 ? ` ×${x.count}` : ''}`,
  ))
  return `家里吃点啥 · ${scopeTitle.value}（${people.value} 人份）\n${lines.join('\n') || '这份清单还没有勾选要买的食材'}\n今天也要好好吃饭～`
})

// 生成清单图：纯 Canvas 自绘，不引入任何图片素材 / 第三方库，可直接保存或发给家人
const preview = ref('')
const previewHeight = ref(0)
const FONT = '"PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif'

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

function buildImage() {
  // 清单图只画已勾选的项：没勾的就是不打算买的，不进图片
  const shown = groups.value
    .map((g) => ({ ...g, items: g.items.filter((x) => checkedToday(x.text)) }))
    .filter((g) => g.items.length)
  if (!shown.length) {
    actions.toast('先勾选要买的东西，再生成清单图', 'warn')
    return
  }
  const W = 720
  const PAD = 40
  const ROW = 46
  const GROUP_HEAD = 44
  const TOP = 150
  const FOOT = 64
  let H = TOP + 10
  for (const g of shown) H += GROUP_HEAD + g.items.length * ROW + 10
  H += FOOT

  const canvas = document.createElement('canvas')
  const dpr = 2
  canvas.width = W * dpr
  canvas.height = H * dpr
  const ctx = canvas.getContext('2d')
  ctx.scale(dpr, dpr)

  // 暖米纸底 + 细点纹
  ctx.fillStyle = '#fbf6ee'
  ctx.fillRect(0, 0, W, H)
  ctx.fillStyle = 'rgba(192, 90, 60, 0.055)'
  for (let x = 14; x < W; x += 18) {
    for (let y = 14; y < H; y += 18) {
      ctx.beginPath(); ctx.arc(x, y, 1, 0, Math.PI * 2); ctx.fill()
    }
  }

  // 标题卡
  roundRect(ctx, PAD, 30, W - PAD * 2, 96, 18)
  ctx.fillStyle = '#fffdf9'; ctx.fill()
  ctx.strokeStyle = '#e9ddca'; ctx.lineWidth = 1; ctx.stroke()
  ctx.fillStyle = '#c05a3c'
  ctx.font = `bold 30px ${FONT}`
  ctx.fillText('家里吃点啥 · 购物清单', PAD + 24, 76)
  ctx.fillStyle = '#948576'
  ctx.font = `16px ${FONT}`
    ctx.fillText(`${scopeTitle.value} · ${people.value} 人份 · 共 ${list.value.length} 项 · ${sourceOrders.value.length} 道菜`, PAD + 24, 106)

  let y = TOP
  for (const g of shown) {
    // 每个区域一个色条，逛超市时按颜色找得到
    const accent = {
      蔬果: '#6d9177', 肉蛋水产: '#c0685f', 豆制品: '#b08a5a',
      主食: '#8b7ba8', 调味料: '#cf9a55',
    }[g.key] || '#cf9a55'
    // 分组色条 + 标题
    roundRect(ctx, PAD, y + 2, 6, 24, 3)
    ctx.fillStyle = accent; ctx.fill()
    ctx.fillStyle = '#3c3229'
    ctx.font = `bold 22px ${FONT}`
    ctx.fillText(`${g.key}（${g.items.length}）`, PAD + 18, y + 22)
    y += GROUP_HEAD

    for (const item of g.items) {
      const done = checkedToday(item.text)
      roundRect(ctx, PAD, y, W - PAD * 2, ROW - 6, 10)
      ctx.fillStyle = done ? '#f0ebe2' : '#fffdf9'; ctx.fill()
      ctx.strokeStyle = '#eee3d2'; ctx.lineWidth = 1; ctx.stroke()
      // 勾选框
      ctx.strokeStyle = done ? '#6d9177' : '#dccbb2'; ctx.lineWidth = 2
      ctx.strokeRect(PAD + 14, y + 11, 18, 18)
      if (done) {
        ctx.fillStyle = '#6d9177'
        ctx.font = `bold 16px ${FONT}`
        ctx.fillText('✓', PAD + 17, y + 25)
      }
      ctx.fillStyle = done ? '#948576' : '#3c3229'
      ctx.font = `19px ${FONT}`
      const prefix = item.inPantry ? '🧊 ' : ''
      const nameW = ctx.measureText(prefix + item.text).width
      ctx.fillText(prefix + item.text, PAD + 46, y + 26)
      if (item.amount) {
        ctx.fillStyle = '#948576'
        ctx.font = `15px ${FONT}`
        ctx.fillText(`· ${item.amount}`, PAD + 46 + nameW + 10, y + 26)
      }
      if (item.count > 1) {
        ctx.fillStyle = '#c05a3c'
        ctx.font = `bold 16px ${FONT}`
        ctx.fillText(`×${item.count}`, W - PAD - 54, y + 26)
      }
      y += ROW
    }
    y += 10
  }

  ctx.textAlign = 'center'
  ctx.fillStyle = '#948576'
  ctx.font = `15px ${FONT}`
  ctx.fillText('今天也要好好吃饭  ·  家里吃点啥', W / 2, H - 26)
  previewHeight.value = H * dpr
  preview.value = canvas.toDataURL('image/png')
  actions.toast('清单图已生成，可保存到相册或直接分享', 'success')
}

</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="购物清单">
      <div class="modal-head">
        <h2>🛒 购物清单</h2>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <p class="setting-desc">
        {{ scopeTitle }}按配料自动归并，用量已按 {{ people }} 人份换算；勾选状态全家实时共享。
      </p>
      <div class="shopping-scope-bar">
        <label>人数
          <select v-model.number="people" class="select">
            <option v-for="n in [2, 3, 4, 5, 6, 7, 8]" :key="n" :value="n">{{ n }} 人</option>
          </select>
        </label>
        <span v-if="coveredCount" class="shopping-covered">🧊 冰箱库存已覆盖 {{ coveredCount }} 项，无需购买</span>
        <span class="tiny muted">采购条目：{{ list.length }} 项</span>
      </div>
      <p v-if="preciseCount" class="shopping-precise">📏 {{ preciseCount }} 道菜用量已精确到克/毫升（公共菜谱数据），「少许」不再是猜的。</p>
      <div v-if="list.length" class="shopping-list">
        <div class="shopping-list-summary">
          <div class="shopping-progress-copy">
            <span><b>{{ checkedCount }}</b> / {{ list.length }} 已勾选</span>
            <span class="tiny muted">{{ checkedPercent }}%</span>
          </div>
          <div class="shopping-progress" aria-hidden="true">
            <span :style="{ width: `${checkedPercent}%` }"></span>
          </div>
          <button class="btn btn-soft btn-sm" type="button" :aria-pressed="allChecked" @click="toggleAll">
            {{ allChecked ? '取消全选' : '全选全部' }}
          </button>
        </div>
        <div v-for="g in groups" :key="g.key" class="shopping-group">
          <template v-if="g.items.length">
            <div class="shopping-group-title">
              {{ g.icon }} {{ g.key }} <span class="tiny muted">{{ g.items.length }} 项</span>
              <button
                class="btn btn-ghost btn-sm shopping-select-all" type="button"
                :aria-label="`${groupAllChecked(g.items) ? '取消全选' : '全选'}${g.key}`"
                :aria-pressed="groupAllChecked(g.items)"
                @click="toggleGroup(g)"
              >{{ groupAllChecked(g.items) ? '取消全选' : '全选' }}</button>
            </div>
            <label
              v-for="item in g.items" :key="item.text" class="shopping-item"
              :class="{ done: checkedToday(item.text), 'was-bought': checkedBefore(item.text), 'has-pantry': item.inPantry }"
            >
              <input type="checkbox" :checked="checkedToday(item.text)" @change="toggle(item.text)" />
              <span class="shopping-name">{{ item.text }}</span>
              <span v-if="checkedBefore(item.text) && !checkedToday(item.text)" class="shopping-before" :title="`${markOf(item.text)?.date} 买过，今天点菜后又出现在清单`">昨天买过</span>
              <span v-if="item.inPantry" class="shopping-pantry" title="冰箱里已有库存">🧊 冰箱有{{ item.pantryStock?.quantity ? ` ${item.pantryStock.quantity}${item.pantryStock.unit}` : '' }}</span>
              <span v-if="item.pantryDeducted" class="shopping-deducted" title="采购量已扣除冰箱库存">已扣库存</span>
              <span v-if="item.amount" class="shopping-amount">{{ item.amount }}</span>
              <span v-if="item.count > 1" class="shopping-count">×{{ item.count }}</span>
              <button
                v-if="!checkedToday(item.text)" type="button"
                class="shopping-claim" :class="{ active: !!claimOf(item.text) }"
                @click.stop.prevent="claim(item.text)"
              >{{ claimOf(item.text) ? `${claimOf(item.text).userName} 正在买` : '我来买' }}</button>
              <span v-else class="shopping-buyer">{{ markOf(item.text)?.userName }} 已买</span>
              <span class="shopping-from tiny muted">{{ item.from.join('、') }}</span>
            </label>
          </template>
        </div>
      </div>
      <div v-else class="empty pad-18">
        <span class="empty-icon">🧺</span>{{ coveredCount ? '冰箱库存已经覆盖这份清单' : '还没点菜，或者这些菜还没填配料' }}
      </div>
      <div class="setting-actions">
        <button class="btn btn-soft btn-sm" :disabled="!list.length" @click="copyList">📋 复制清单</button>
        <button class="btn btn-soft btn-sm" :disabled="!list.length" @click="buildImage">🖼️ 生成清单图</button>
        <WeChatShareActions
          v-if="preview" :image-data="preview" :filename="`购物清单-${scopeDate}.png`"
          :title="scopeTitle" :text="shareText"
        />
        <button class="btn btn-ghost btn-sm" :disabled="!list.length" @click="clearChecked">清空这份清单</button>
        <span class="grow"></span>
        <button class="btn btn-primary btn-sm" @click="emit('close')">完成</button>
      </div>
      <div v-if="preview" class="shopping-preview">
        <img :src="preview" :alt="previewAlt" width="1440" :height="previewHeight" loading="lazy" decoding="async" />
        <div class="shopping-preview-hint">📱 可直接「分享到微信」；微信内不能弹出分享面板时，长按图片发送给家人</div>
      </div>
    </div>
  </div>
</template>
