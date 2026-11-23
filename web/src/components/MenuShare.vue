<script setup>
import { computed, ref, onMounted } from 'vue'
import { useModalBehavior } from '../modal'
import WeChatShareActions from './WeChatShareActions.vue'

// 今日菜单分享图：把今天点的菜、点菜人、当前餐的分工画成一张暖米纸风格的图。
// 纯 canvas 绘制，不依赖任何库；生成后可直接分享或长按存相册。
const props = defineProps({
  orders: { type: Array, default: () => [] },
  grouped: { type: Object, default: () => ({}) },
  meals: { type: Array, default: () => [] },
  today: { type: String, default: '' },
  dayLabel: { type: String, default: '菜单' },
  chefs: { type: Array, default: () => [] },
  washers: { type: Array, default: () => [] },
})
const emit = defineEmits(['close'])
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const preview = ref('')
const previewHeight = ref(0)
const FONT = '"PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif'

function build() {
  const W = 720
  const PAD = 44
  const rows = []
  for (const m of props.meals) {
    const list = props.grouped?.[m.key] || []
    if (!list.length) continue
    rows.push({
      label: `${m.icon || '🍽️'} ${m.label}`,
      items: list.map((o) => `${o.dishName}  · ${o.userName || ''}`),
    })
  }
  const H = 170 + rows.reduce((n, r) => n + 42 + r.items.length * 34 + 14, 0) + 92

  const canvas = document.createElement('canvas')
  canvas.width = W
  canvas.height = H
  const ctx = canvas.getContext('2d')

  ctx.fillStyle = '#fbf6ee'
  ctx.fillRect(0, 0, W, H)
  // 细点纹，跟购物清单/周菜单一套纸感
  ctx.fillStyle = 'rgba(160,130,100,0.055)'
  for (let x = 12; x < W; x += 22) {
    for (let y = 12; y < H; y += 22) ctx.fillRect(x, y, 1.5, 1.5)
  }
  // 顶部色带
  ctx.fillStyle = '#b4473c'
  ctx.fillRect(0, 0, W, 8)

  ctx.fillStyle = '#3c3229'
  ctx.font = `bold 38px ${FONT}`
  ctx.fillText('🍚 今天吃什么', PAD, 76)
  ctx.font = `20px ${FONT}`
  ctx.fillStyle = '#8a7a68'
  ctx.fillText(props.today || '', PAD, 112)

  let y = 162
  for (const row of rows) {
    ctx.fillStyle = '#b4473c'
    ctx.font = `bold 23px ${FONT}`
    ctx.fillText(row.label, PAD, y)
    y += 38
    ctx.font = `21px ${FONT}`
    for (const item of row.items) {
      ctx.fillStyle = '#3c3229'
      ctx.fillText('• ' + item, PAD + 8, y)
      y += 34
    }
    y += 14
  }

  // 底部分工
  ctx.fillStyle = 'rgba(180,71,60,0.10)'
  ctx.fillRect(PAD, y, W - PAD * 2, 62)
  ctx.font = `bold 20px ${FONT}`
  ctx.fillStyle = '#b4473c'
  const chef = props.chefs.map((m) => m.name).join('、') || '未定'
  const wash = props.washers.map((m) => m.name).join('、') || '未定'
  ctx.fillText(`👨‍🍳 ${chef}      🧽 ${wash}`, PAD + 16, y + 40)
  ctx.font = `15px ${FONT}`
  ctx.fillStyle = '#9b8b7b'
  ctx.fillText('家里吃点啥 · 大家一起来定', PAD, H - 18)

  previewHeight.value = H
  preview.value = canvas.toDataURL('image/png')
}

onMounted(build)

const shareText = computed(() => {
  const lines = props.meals.flatMap((meal) => {
    const orders = props.grouped?.[meal.key] || []
    if (!orders.length) return []
    return [`${meal.icon || '🍽️'} ${meal.label}`, ...orders.map((o) => `· ${o.dishName}（${o.userName || '家人'}点的）`)]
  })
  const chef = props.chefs.map((m) => m.name).join('、') || '还没定'
  const washer = props.washers.map((m) => m.name).join('、') || '还没定'
  return `家里吃点啥 · ${props.today}\n${lines.join('\n')}\n掌勺：${chef}  洗碗：${washer}\n大家一起来定～`
})
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="分享今日菜单">
      <div class="modal-head">
        <h2>📤 {{ dayLabel }}菜单</h2>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <div class="shopping-preview">
        <img
          v-if="preview" :src="preview"
          :alt="`${today || '今天'}的${dayLabel}菜单分享图，共 ${orders.length} 道菜`"
          width="720" :height="previewHeight" loading="lazy" decoding="async"
        />
        <div class="shopping-preview-hint">📱 点「分享到微信」直接发给家人；微信内受限时可长按图片发送</div>
      </div>
      <div class="modal-foot">
        <WeChatShareActions
          :image-data="preview" :filename="`今日菜单-${today}.png`"
          :title="`今日菜单 ${today}`" :text="shareText" save-label="保存到相册"
        />
      </div>
    </div>
  </div>
</template>
