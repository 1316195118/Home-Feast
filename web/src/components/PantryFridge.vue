<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import Food3D from './Food3D.vue'
import { pantryGroups, foodShape } from '../utils'

// 写实风格 3D 冰箱：三层冷藏 + 双开门，纯 CSS 几何体做食材立体造型。
const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
})

// 精确分层：直接按 presets 的分组归属，避免「豆腐被正则判到下层」这类错位
const GROUP_LAYER = { meat: 'upper', seafood: 'upper', veg: 'middle', mushroom: 'middle', staple: 'lower', fruit: 'lower', dairy: 'lower' }
const LAYER_OF_NAME = (() => {
  const map = {}
  for (const g of pantryGroups || []) {
    for (const n of g.items) map[n] = GROUP_LAYER[g.key] || 'middle'
  }
  return map
})()
// 造型统一由 utils.foodShape 提供，搜索卡片与冰箱用同一套
const shapeOf = (name) => foodShape(name)
const layerOf = (name) => LAYER_OF_NAME[name] || 'middle'
// 真机结构：上半部双开门（两层），下部整宽大抽屉（一层）
const doorLayers = computed(() => [
  { key: 'upper', label: '冷藏 · 上层', foods: props.items.filter((n) => layerOf(n) === 'upper') },
  { key: 'middle', label: '保鲜 · 中层', foods: props.items.filter((n) => layerOf(n) === 'middle') },
])
const drawerItems = computed(() => props.items.filter((n) => layerOf(n) === 'lower'))
const drawerOpen = ref(true)

// 每层显示上限跟着屏宽走：手机一行放不下 10 个，会挤成细条
const perLayer = ref(10)
function syncPerLayer() {
  const w = window.innerWidth || 1024
  perLayer.value = w <= 380 ? 5 : w <= 560 ? 6 : w <= 760 ? 8 : 10
}
syncPerLayer()
if (typeof window !== 'undefined') window.addEventListener('resize', syncPerLayer, { passive: true })
onUnmounted(() => window.removeEventListener('resize', syncPerLayer))

const leftOpen = ref(true), rightOpen = ref(true), landed = ref('')
let timer = null
watch(() => props.items, (next, prev) => {
  const added = next.find((x) => !(prev || []).includes(x))
  if (added) {
    landed.value = added
    clearTimeout(timer)
    timer = setTimeout(() => { landed.value = '' }, 850)
    leftOpen.value = true; rightOpen.value = true; drawerOpen.value = true
  }
})
onUnmounted(() => clearTimeout(timer))
const openAll = () => { leftOpen.value = true; rightOpen.value = true; drawerOpen.value = true }
</script>

<template>
  <div class="fridge-stage">
    <div class="fridge-scene">
      <div class="fridge">
        <!-- 上半：双开门冷藏室 -->
        <div class="fridge-upper">
          <div class="fridge-body">
            <div class="fridge-top">❄ 冷藏室 <span class="fridge-temp">4°C</span></div>
            <div class="fridge-cavity">
              <div class="fridge-glow"></div>
              <div v-for="layer in doorLayers" :key="layer.key" class="fridge-layer">
                <div class="fridge-layer-label">{{ layer.label }}</div>
                <div class="fridge-food-row">
                  <div
                    v-for="(food, fi) in layer.foods.slice(0, perLayer)" :key="food"
                    class="fridge-food" :class="{ landed: landed === food }"
                    :style="{ animationDelay: landed === food ? '0ms' : `${fi * 40}ms` }"
                  >
                    <Food3D :name="food" v-bind="shapeOf(food)" />
                    <span class="fridge-food-name">{{ food }}</span>
                  </div>
                  <span v-if="!layer.foods.length" class="fridge-layer-empty">空</span>
                  <span v-if="layer.foods.length > perLayer" class="fridge-food-more">+{{ layer.foods.length - perLayer }}</span>
                </div>
                <div class="fridge-shelf"></div>
              </div>
            </div>
          </div>
          <button class="fridge-door fridge-door-l" :class="{ open: leftOpen }" type="button" aria-label="开关左门" @click="leftOpen = !leftOpen">
            <span class="door-face"><span class="door-handle l"></span><span class="door-bin"></span><span class="door-bin b2"></span></span>
          </button>
          <button class="fridge-door fridge-door-r" :class="{ open: rightOpen }" type="button" aria-label="开关右门" @click="rightOpen = !rightOpen">
            <span class="door-face"><span class="door-handle r"></span><span class="door-bin"></span><span class="door-bin b2"></span></span>
          </button>
        </div>

        <!-- 下半：整宽冷冻抽屉 -->
        <div class="fridge-lower">
          <div class="fridge-body drawer-body">
            <div class="fridge-layer drawer-layer">
              <div class="fridge-layer-label">❄ 冷冻抽屉</div>
              <div class="fridge-food-row">
                <div
                  v-for="(food, fi) in drawerItems.slice(0, perLayer)" :key="food"
                  class="fridge-food" :class="{ landed: landed === food }"
                  :style="{ animationDelay: landed === food ? '0ms' : `${fi * 40}ms` }"
                >
                  <Food3D :name="food" v-bind="shapeOf(food)" />
                  <span class="fridge-food-name">{{ food }}</span>
                </div>
                <span v-if="!drawerItems.length" class="fridge-layer-empty">空</span>
                <span v-if="drawerItems.length > perLayer" class="fridge-food-more">+{{ drawerItems.length - perLayer }}</span>
              </div>
            </div>
          </div>
          <button class="fridge-drawer" :class="{ open: drawerOpen }" type="button" aria-label="拉开冷冻抽屉" @click="drawerOpen = !drawerOpen">
            <span class="drawer-face"><span class="drawer-handle"></span></span>
          </button>
        </div>
      </div>
    </div>
    <div class="fridge-meta" aria-live="polite">
      <span>🧊 <b :key="`n-${items.length}`" class="fridge-num">{{ items.length }}</b> 样食材</span>
      <span class="fridge-sep">·</span>
      <span>菜谱 <b :key="`h-${hitCount}`" class="fridge-num">{{ hitCount }}</b> 道用得上</span>
      <button v-if="!leftOpen || !rightOpen || !drawerOpen" class="fridge-openall" type="button" @click="openAll">全部打开</button>
      <span v-else class="fridge-tip">点门 / 抽屉开关</span>
    </div>
  </div>
</template>
