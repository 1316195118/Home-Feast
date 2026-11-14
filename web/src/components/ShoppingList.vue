<script setup>
import { inject, computed, ref } from 'vue'
import { shoppingList, shoppingGroups, normalizeIngredient, shoppingKind } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const emit = defineEmits(['close'])
const { state, actions } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

// 内置菜的配料由 recipes.js 按「技法模板 + 主料表」动态生成，dish.ingredients 本身是空的；
// 用户在编辑器里自己填的配料优先（getRecipe 已处理），这里把它展开后再聚合。
const dishesWithIngredients = computed(() => state.dishes.map((d) => {
  const r = getRecipe(d)
  return { ...d, ingredients: r.ingredients.length ? r.ingredients : (d.ingredients || []), precise: !!r.precise }
}))
const list = computed(() => shoppingList(state.plan.orders || [], dishesWithIngredients.value))
const preciseCount = computed(() => {
  const byId = new Map(dishesWithIngredients.value.map((d) => [d.id, d]))
  return (state.plan.orders || []).filter((o) => byId.get(o.dishId)?.precise).length
})
// 冰箱里已有什么（仅主料，佐料不当作「已有」）：用来在清单上标注「🧊 冰箱有」
const pantrySet = computed(() => new Set(
  (state.pantry || [])
    .map((x) => normalizeIngredient(String(x)))
    .filter((x) => shoppingKind(x) !== '佐料'),
))
// 把「是否命中冰箱」挂到每个清单项上，命中即提示不用重复买
const enriched = computed(() => list.value.map((it) => ({
  ...it,
  inPantry: pantrySet.value.has(normalizeIngredient(it.text)),
})))
const groups = computed(() => shoppingGroups(enriched.value))
const STORE_KEY = `fm.shopping.${state.today}`

function readChecked() {
  try { return new Set(JSON.parse(localStorage.getItem(STORE_KEY) || '[]')) } catch { return new Set() }
}
const checked = ref(readChecked())
function persist() {
  try { localStorage.setItem(STORE_KEY, JSON.stringify([...checked.value])) } catch { /* 隐私模式忽略 */ }
}
function toggle(text) {
  const next = new Set(checked.value)
  if (next.has(text)) next.delete(text)
  else next.add(text)
  checked.value = next
  persist()
}
// 按「食材 / 佐料」一键全选；已全选时再点即整组取消
function toggleGroup(items) {
  const next = new Set(checked.value)
  const allChecked = items.every((x) => next.has(x.text))
  for (const x of items) {
    if (allChecked) next.delete(x.text)
    else next.add(x.text)
  }
  checked.value = next
  persist()
}
function groupAllChecked(items) {
  return items.length > 0 && items.every((x) => checked.value.has(x.text))
}
function clearChecked() {
  checked.value = new Set()
  persist()
}
async function copyList() {
  const lines = groups.value.flatMap((g) => g.items.length
    ? [`【${g.key}】`, ...g.items.map(
        (x) => `${checked.value.has(x.text) ? '☑' : '☐'} ${x.inPantry ? '🧊 ' : ''}${x.text}${x.amount ? ` · ${x.amount}` : ''}${x.count > 1 ? ` ×${x.count}` : ''}`,
      )]
    : [])
  const text = `家里吃点啥 · ${state.today} 购物清单\n${lines.join('\n')}`
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

// 生成清单图：纯 Canvas 自绘，不引入任何图片素材 / 第三方库，可直接保存或发给家人
const preview = ref('')
let imageCanvas = null
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
    .map((g) => ({ ...g, items: g.items.filter((x) => checked.value.has(x.text)) }))
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
  ctx.fillText(`${state.today} · 共 ${list.value.length} 项 · ${(state.plan.orders || []).length} 道菜`, PAD + 24, 106)

  let y = TOP
  for (const g of shown) {
    const accent = g.key === '食材' ? '#6d9177' : '#cf9a55'
    // 分组色条 + 标题
    roundRect(ctx, PAD, y + 2, 6, 24, 3)
    ctx.fillStyle = accent; ctx.fill()
    ctx.fillStyle = '#3c3229'
    ctx.font = `bold 22px ${FONT}`
    ctx.fillText(`${g.key}（${g.items.length}）`, PAD + 18, y + 22)
    y += GROUP_HEAD

    for (const item of g.items) {
      const done = checked.value.has(item.text)
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
  imageCanvas = canvas
  preview.value = canvas.toDataURL('image/png')
  actions.toast('清单图已生成，可保存到相册或直接分享', 'success')
}

// 保存到相册：优先调系统分享面板（里面有「存储图像 / 保存到相册」），
// 不支持时降级为下载，并提示长按预览图保存 —— 手机浏览器/微信里这是最可靠的方式。
async function saveToAlbum() {
  if (!imageCanvas || !preview.value) return
  const blob = await new Promise((res) => imageCanvas.toBlob(res, 'image/png'))
  if (blob) {
    const file = new File([blob], `购物清单-${state.today}.png`, { type: 'image/png' })
    if (navigator.canShare?.({ files: [file] })) {
      try {
        await navigator.share({ files: [file], title: `购物清单 ${state.today}` })
        return
      } catch (err) {
        if (err?.name === 'AbortError') return // 用户主动取消，不再降级
      }
    }
  }
  const link = document.createElement('a')
  link.href = preview.value
  link.download = `购物清单-${state.today}.png`
  link.click()
  actions.toast('已尝试保存；若没存上，长按上方图片选「保存图片」即可进相册', 'info')
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
        按今天已点菜品的配料自动归并，同名只列一条；勾选状态记在这台设备上（{{ state.today }}）。
      </p>
      <p v-if="preciseCount" class="shopping-precise">📏 {{ preciseCount }} 道菜用量已精确到克/毫升（公共菜谱数据），「少许」不再是猜的。</p>
      <div v-if="list.length" class="shopping-list">
        <div v-for="g in groups" :key="g.key" class="shopping-group">
          <template v-if="g.items.length">
            <div class="shopping-group-title">
              {{ g.icon }} {{ g.key }} <span class="tiny muted">{{ g.items.length }} 项</span>
              <button
                class="btn btn-ghost btn-sm shopping-select-all" type="button"
                :aria-label="`${groupAllChecked(g.items) ? '取消全选' : '全选'}${g.key}`"
                @click="toggleGroup(g.items)"
              >{{ groupAllChecked(g.items) ? '取消全选' : '全选' }}</button>
            </div>
            <label
              v-for="item in g.items" :key="item.text" class="shopping-item"
              :class="{ done: checked.has(item.text), 'has-pantry': item.inPantry }"
            >
              <input type="checkbox" :checked="checked.has(item.text)" @change="toggle(item.text)" />
              <span class="shopping-name">{{ item.text }}</span>
              <span v-if="item.inPantry" class="shopping-pantry" title="冰箱里已有，不用买">🧊 冰箱有</span>
              <span v-if="item.amount" class="shopping-amount">{{ item.amount }}</span>
              <span v-if="item.count > 1" class="shopping-count">×{{ item.count }}</span>
              <span class="shopping-from tiny muted">{{ item.from.join('、') }}</span>
            </label>
          </template>
        </div>
      </div>
      <div v-else class="empty pad-18">
        <span class="empty-icon">🧺</span>今天还没点菜，或者这些菜还没填配料
      </div>
      <div class="setting-actions">
        <button class="btn btn-soft btn-sm" :disabled="!list.length" @click="copyList">📋 复制清单</button>
        <button class="btn btn-soft btn-sm" :disabled="!list.length" @click="buildImage">🖼️ 生成清单图</button>
        <button v-if="preview" class="btn btn-soft btn-sm" @click="saveToAlbum">📱 保存到相册</button>
        <button class="btn btn-ghost btn-sm" :disabled="!checked.size" @click="clearChecked">清空勾选</button>
        <span class="grow"></span>
        <button class="btn btn-primary btn-sm" @click="emit('close')">完成</button>
      </div>
      <div v-if="preview" class="shopping-preview">
        <img :src="preview" alt="购物清单图" />
        <div class="shopping-preview-hint">📱 长按图片选「保存图片」即可存进相册；或点上面的「保存到相册」直接分享 / 存储</div>
      </div>
    </div>
  </div>
</template>
