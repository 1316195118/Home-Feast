<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'

// 3D 冰箱：勾选食材时实时放进冰箱格子，匹配到的菜数即时跳动。
// 纯 CSS 3D，无 WebGL；prefers-reduced-motion 下退化为平面。
const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
})
const emit = defineEmits(['toggle-door'])

const open = ref(true)
const landed = ref('')
const MAX = 12
const shown = computed(() => props.items.slice(0, MAX))
const overflow = computed(() => Math.max(0, props.items.length - MAX))
const cells = computed(() => {
  const filled = shown.value.map((name) => ({ name, ...props.visualOf(name) }))
  const pad = Math.max(0, 6 - filled.length)
  return [...filled, ...Array.from({ length: pad }, (_, i) => ({ name: '', ghost: i }))]
})

let timer = null
watch(() => props.items, (next, prev) => {
  const added = next.find((x) => !(prev || []).includes(x))
  if (added) {
    landed.value = added
    clearTimeout(timer)
    timer = setTimeout(() => { landed.value = '' }, 700)
    if (!open.value) open.value = true // 放进新食材时自动开门
  }
})
onUnmounted(() => clearTimeout(timer))

function toggleDoor() {
  open.value = !open.value
  emit('toggle-door', open.value)
}
</script>

<template>
  <div class="fridge-stage">
    <div class="fridge-scene">
      <div class="fridge" :class="{ 'is-open': open }">
        <div class="fridge-cavity">
          <div class="fridge-light"></div>
          <div class="fridge-grid">
            <div
              v-for="(c, i) in cells" :key="c.name || `ghost-${i}`"
              class="fridge-cell" :class="{ empty: !c.name, landed: c.name && landed === c.name }"
              :style="c.name && c.tint ? { background: c.tint } : undefined"
            >
              <template v-if="c.name">
                <span class="fridge-emoji">{{ c.icon }}</span>
                <span class="fridge-label">{{ c.name }}</span>
              </template>
            </div>
          </div>
          <div v-if="overflow" class="fridge-more">还有 +{{ overflow }} 样</div>
          <div v-if="!items.length" class="fridge-empty">冰箱空空的<br /><small>在下面点选食材放进来</small></div>
        </div>
        <button
          class="fridge-door" type="button"
          :aria-label="open ? '关上冰箱门' : '打开冰箱门'"
          :aria-pressed="open" @click="toggleDoor"
        >
          <span class="fridge-door-panel">
            <span class="fridge-brand">FRESH</span>
            <span class="fridge-handle"></span>
            <span class="fridge-door-shelf"></span>
          </span>
        </button>
      </div>
    </div>
    <div class="fridge-meta" aria-live="polite">
      <span>🧊 冰箱里 <b :key="`n-${items.length}`" class="fridge-num">{{ items.length }}</b> 样食材</span>
      <span class="fridge-sep">·</span>
      <span>菜谱里 <b :key="`h-${hitCount}`" class="fridge-num">{{ hitCount }}</b> 道用得上</span>
      <span class="fridge-tip">点冰箱门开 / 关</span>
    </div>
  </div>
</template>
