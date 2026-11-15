<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useModalBehavior } from '../modal'

// 烹饪模式：做饭时手上是油，只能扫一眼屏幕。
// 一步一屏、大字号、大按钮，能左右滑动；进入时申请屏幕常亮，退出立刻释放。
const props = defineProps({
  dish: { type: Object, required: true },
  recipe: { type: Object, required: true },
})
const emit = defineEmits(['close', 'done'])

const steps = computed(() => props.recipe.steps || [])
const idx = ref(0)
const doneSet = ref(new Set())
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const total = computed(() => steps.value.length)
const isLast = computed(() => idx.value >= total.value - 1)

function go(delta) {
  const next = idx.value + delta
  if (next < 0 || next >= total.value) return
  idx.value = next
}
function toggleDone(i) {
  const s = new Set(doneSet.value)
  if (s.has(i)) s.delete(i)
  else s.add(i)
  doneSet.value = s
}

// 左右滑动翻页；垂直方向不拦，避免挡住滚动
let startX = 0
let startY = 0
let tracking = false
function onTouchStart(e) {
  const t = e.changedTouches[0]
  startX = t.clientX
  startY = t.clientY
  tracking = true
}
function onTouchEnd(e) {
  if (!tracking) return
  tracking = false
  const t = e.changedTouches[0]
  const dx = t.clientX - startX
  const dy = t.clientY - startY
  if (Math.abs(dx) < 48 || Math.abs(dx) < Math.abs(dy)) return
  go(dx < 0 ? 1 : -1)
}

// 屏幕常亮：做饭时不会做到一半黑屏。不支持就静默跳过。
let lock = null
const lockOk = ref(false)
async function acquireLock() {
  try {
    if ('wakeLock' in navigator) {
      lock = await navigator.wakeLock.request('screen')
      lockOk.value = true
      lock.addEventListener?.('release', () => { lockOk.value = false })
    }
  } catch { /* 电量低或用户拒绝时忽略 */ }
}
function releaseLock() {
  try { lock?.release?.() } catch { /* 忽略 */ }
  lock = null
  lockOk.value = false
}
onMounted(acquireLock)
onUnmounted(releaseLock)

function finish() {
  releaseLock()
  emit('done')
}
</script>

<template>
  <div ref="root" class="cook-mask" role="dialog" aria-modal="true" aria-label="烹饪模式" @click.self="emit('close')">
    <div class="cook-sheet" @touchstart.passive="onTouchStart" @touchend="onTouchEnd">
      <header class="cook-head">
        <div class="cook-title">
          <b>{{ dish.name }}</b>
          <span class="cook-progress">{{ Math.min(idx + 1, total) }} / {{ total }}</span>
          <span v-if="lockOk" class="cook-lock">🔆 屏幕常亮</span>
        </div>
        <button class="icon-btn" type="button" aria-label="退出做菜" @click="emit('close')">✕</button>
      </header>

      <div class="cook-progress-bar" aria-hidden="true">
        <span :style="{ width: `${((idx + 1) / total) * 100}%` }"></span>
      </div>

      <main class="cook-body">
        <Transition name="cook-step" mode="out-in">
          <div :key="idx" class="cook-step">
            <div class="cook-num">第 {{ idx + 1 }} 步</div>
            <p class="cook-text" :class="{ 'is-done': doneSet.has(idx) }">{{ steps[idx] }}</p>
            <button class="cook-check" type="button" :class="{ 'is-done': doneSet.has(idx) }" @click="toggleDone(idx)">
              {{ doneSet.has(idx) ? '✓ 这步做完了' : '标记这步做完' }}
            </button>
          </div>
        </Transition>

        <details class="cook-ingredients">
          <summary>🧺 配料（{{ recipe.ingredients.length }} 样）</summary>
          <ul>
            <li v-for="(ing, i) in recipe.ingredients" :key="i">{{ ing }}</li>
          </ul>
        </details>
      </main>

      <footer class="cook-foot">
        <button class="btn btn-soft cook-btn" type="button" :disabled="idx === 0" @click="go(-1)">上一步</button>
        <button v-if="!isLast" class="btn btn-primary cook-btn" type="button" @click="go(1)">下一步</button>
        <button v-else class="btn btn-primary cook-btn" type="button" @click="finish">做好了 🍽️</button>
      </footer>
    </div>
  </div>
</template>
