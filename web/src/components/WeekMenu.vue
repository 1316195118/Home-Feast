<script setup>
import { inject, ref, computed } from 'vue'
import { planWeekMenu, classifyDish } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const emit = defineEmits(['close'])
const { state, actions } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const people = ref(Math.max(2, Math.min(6, (state.members?.length || 0) || 3)))
const peopleOptions = [2, 3, 4, 5, 6]
const week = ref([])
const salt = ref(0)
const KIND_LABEL = { meat: '荤', veg: '素', soup: '汤', staple: '主食', dessert: '甜品' }

function generate() {
  week.value = planWeekMenu(state.dishes, {
    people: people.value,
    today: state.today,
    salt: salt.value,
    pantry: state.settings.pantry || [],
    member: state.members.find((m) => m.id === state.currentUserId) || null,
    hiddenCuisines: state.hiddenCuisines,
    recentIds: [],
    ingredientsOf: (d) => getRecipe(d).ingredients || [],
  })
  const total = week.value.reduce((n, d) => n + d.dishes.length, 0)
  actions.toast(`已生成 ${people.value} 人份 · ${week.value.length} 天 · ${total} 道`, 'success')
}
function shuffleWeek() { salt.value += 1; generate() }

const totalDishes = computed(() => week.value.reduce((n, d) => n + d.dishes.length, 0))

// 导出图片：纯 Canvas 自绘，暖米纸底 + 每天一行菜名（自动换行），保存/分享同购物清单
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
function wrapLines(ctx, text, maxW) {
  const lines = []
  let line = ''
  for (const ch of [...String(text)]) {
    if (ctx.measureText(line + ch).width > maxW) { lines.push(line); line = ch }
    else line += ch
  }
  if (line) lines.push(line)
  return lines
}

function buildImage() {
  if (!week.value.length) return
  const W = 720
  const PAD = 36
  const probe = document.createElement('canvas').getContext('2d')
  probe.font = `19px ${FONT}`
  const rows = week.value.map((day) => ({
    day,
    lines: wrapLines(probe, day.dishes.map((d) => d.name).join(' · '), W - PAD * 2 - 8),
  }))
  const HEAD = 150
  const GAP = 16
  const FOOT = 56
  let H = HEAD
  for (const r of rows) H += 30 + r.lines.length * 26 + GAP
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
    for (let y = 14; y < H; y += 18) { ctx.beginPath(); ctx.arc(x, y, 1, 0, Math.PI * 2); ctx.fill() }
  }

  // 标题卡
  roundRect(ctx, PAD, 28, W - PAD * 2, 92, 18)
  ctx.fillStyle = '#fffdf9'; ctx.fill()
  ctx.strokeStyle = '#e9ddca'; ctx.lineWidth = 1; ctx.stroke()
  ctx.fillStyle = '#c05a3c'
  ctx.font = `bold 28px ${FONT}`
  ctx.fillText('家里吃点啥 · 本周菜单', PAD + 22, 72)
  ctx.fillStyle = '#948576'
  ctx.font = `15px ${FONT}`
  const range = week.value.length ? `${week.value[0].label} – ${week.value[week.value.length - 1].label}` : ''
  ctx.fillText(`${people.value} 人份 · ${range} · 共 ${totalDishes.value} 道`, PAD + 22, 101)

  // 每天一块：日期行 + 菜名（可换行）+ 分隔线
  let y = HEAD
  for (const r of rows) {
    ctx.fillStyle = '#c05a3c'
    ctx.font = `bold 20px ${FONT}`
    ctx.fillText(`${r.day.label} ${r.day.weekday}`, PAD, y + 20)
    y += 30
    ctx.fillStyle = '#3c3229'
    ctx.font = `19px ${FONT}`
    for (const line of r.lines) { ctx.fillText(line, PAD + 4, y + 18); y += 26 }
    ctx.strokeStyle = '#eee3d2'; ctx.lineWidth = 1
    ctx.beginPath(); ctx.moveTo(PAD, y + 6); ctx.lineTo(W - PAD, y + 6); ctx.stroke()
    y += GAP
  }

  ctx.textAlign = 'center'
  ctx.fillStyle = '#948576'
  ctx.font = `15px ${FONT}`
  ctx.fillText('今天也要好好吃饭  ·  家里吃点啥', W / 2, H - 24)
  imageCanvas = canvas
  preview.value = canvas.toDataURL('image/png')
  actions.toast('菜单图已生成，可保存或分享', 'success')
}

async function saveToAlbum() {
  if (!imageCanvas || !preview.value) return
  const blob = await new Promise((res) => imageCanvas.toBlob(res, 'image/png'))
  if (blob) {
    const file = new File([blob], `本周菜单-${state.today}.png`, { type: 'image/png' })
    if (navigator.canShare?.({ files: [file] })) {
      try {
        await navigator.share({ files: [file], title: `本周菜单 ${state.today}` })
        return
      } catch (err) {
        if (err?.name === 'AbortError') return
      }
    }
  }
  const link = document.createElement('a')
  link.href = preview.value
  link.download = `本周菜单-${state.today}.png`
  link.click()
  actions.toast('已尝试保存；若没存上，长按上方图片选「保存图片」即可进相册', 'info')
}
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="本周菜单">
      <div class="modal-head">
        <h2>🗓️ 本周菜单</h2>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <p class="setting-desc">
        按人数配出「荤 + 素 + 汤（+ 主食）」，整周尽量不重复；只按季节、冰箱食材和口味打分，不发数据。
      </p>

      <div class="week-bar">
        <span class="tiny muted">人数</span>
        <button
          v-for="n in peopleOptions" :key="n" type="button"
          class="pill" :class="{ active: people === n }"
          @click="people = n"
        >{{ n }}人</button>
        <span class="grow"></span>
        <button class="btn btn-primary btn-sm" type="button" @click="generate">生成本周菜单</button>
        <button v-if="week.length" class="btn btn-ghost btn-sm" type="button" @click="shuffleWeek">换一版</button>
      </div>

      <div v-if="week.length" class="week-list">
        <div v-for="day in week" :key="day.date" class="week-day">
          <div class="week-day-head">
            <span class="week-date">{{ day.label }}</span>
            <span class="week-weekday">{{ day.weekday }}</span>
          </div>
          <div class="week-dishes">
            <span
              v-for="d in day.dishes" :key="d.id"
              class="week-dish" :class="`kind-${classifyDish(d)}`"
            >{{ d.name }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty pad-18">
        <span class="empty-icon">🗓️</span>选好人数，点「生成本周菜单」
      </div>

      <div class="setting-actions">
        <button class="btn btn-soft btn-sm" :disabled="!week.length" @click="buildImage">🖼️ 导出图片</button>
        <button v-if="preview" class="btn btn-soft btn-sm" @click="saveToAlbum">📱 保存到相册</button>
        <button class="btn btn-ghost btn-sm" :disabled="!week.length" @click="week = []">清空</button>
        <span class="grow"></span>
        <button class="btn btn-primary btn-sm" @click="emit('close')">完成</button>
      </div>

      <div v-if="preview" class="shopping-preview">
        <img :src="preview" alt="本周菜单图" />
        <div class="shopping-preview-hint">📱 长按图片选「保存图片」即可存进相册；或点上面「保存到相册」直接分享 / 存储</div>
      </div>
    </div>
  </div>
</template>
