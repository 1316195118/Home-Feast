<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import Food3D from './Food3D.vue'
import { pantryGroups, foodShape } from '../utils'

// 写实风格 3D 冰箱：上半双开门 + 下半整宽冷冻抽屉，纯 CSS 几何体。
const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
})

// 精确分层：按预设分组归属，不靠名字正则
const GROUP_LAYER = { meat: 'upper', seafood: 'upper', veg: 'middle', mushroom: 'middle', staple: 'lower', fruit: 'lower', dairy: 'lower' }
const LAYER_OF_NAME = (() => {
  const map = {}
  for (const g of pantryGroups || []) for (const n of g.items) map[n] = GROUP_LAYER[g.key] || 'middle'
  return map
})()
const shapeOf = (name) => foodShape(name)
const layerOf = (name) => LAYER_OF_NAME[name] || 'middle'

const upperItems = computed(() => props.items.filter((n) => layerOf(n) === 'upper'))
const middleItems = computed(() => props.items.filter((n) => layerOf(n) === 'middle'))
const lowerItems = computed(() => props.items.filter((n) => layerOf(n) === 'lower'))

// 视角：all 全看 / cold 只高亮冷藏 / freeze 只高亮冷冻。旋钮点击循环。
const VIEWS = ['all', 'cold', 'freeze']
const view = ref('all')
const viewLabel = computed(() => ({ all: '全部', cold: '冷藏', freeze: '冷冻' })[view.value])
function cycleView() {
  view.value = VIEWS[(VIEWS.indexOf(view.value) + 1) % VIEWS.length]
}
const dimUpper = computed(() => view.value === 'freeze')
const dimLower = computed(() => view.value === 'cold')

const leftOpen = ref(true), rightOpen = ref(true), drawerOpen = ref(true)
const landed = ref('')
const mistOn = ref(false)
let timer = null, mistTimer = null

function puffMist() {
  mistOn.value = true
  clearTimeout(mistTimer)
  mistTimer = setTimeout(() => { mistOn.value = false }, 1200)
}
watch(() => props.items, (next, prev) => {
  const added = next.find((x) => !(prev || []).includes(x))
  if (added) {
    landed.value = added
    clearTimeout(timer)
    timer = setTimeout(() => { landed.value = '' }, 850)
    if (!leftOpen.value && !rightOpen.value) { leftOpen.value = true; rightOpen.value = true; puffMist() }
  }
})
onUnmounted(() => { clearTimeout(timer); clearTimeout(mistTimer) })

function toggleDoor(side) {
  if (side === 'left') leftOpen.value = !leftOpen.value
  else rightOpen.value = !rightOpen.value
  if (leftOpen.value || rightOpen.value) puffMist()
}
function toggleDrawer() {
  drawerOpen.value = !drawerOpen.value
  if (drawerOpen.value) puffMist()
}
const openAll = () => { leftOpen.value = true; rightOpen.value = true; drawerOpen.value = true; puffMist() }
</script>

<template>
  <div class="fridge-stage">
    <div class="fridge-scene">
      <div class="fridge" :class="{ 'view-cold': view === 'cold', 'view-freeze': view === 'freeze' }">
        <!-- 上半：双开门冷藏室 -->
        <div class="fridge-upper" :class="{ 'is-dim': dimUpper }">
          <div class="fridge-body">
            <div class="fridge-top">
              <span class="fridge-top-label">❄ 冷藏室</span>
              <button class="fridge-knob" type="button" :title="`当前视角：${viewLabel}，点击切换`" @click.stop="cycleView">
                <span class="knob-dial" :class="`knob-${view}`"></span>
                <span class="knob-text">{{ viewLabel }}</span>
              </button>
              <span class="fridge-temp">4°C</span>
            </div>
            <div class="fridge-cavity">
              <div class="fridge-glow"></div>
              <div
                v-for="layer in [
                  { key: 'upper', label: '冷藏 · 上层', foods: upperItems },
                  { key: 'middle', label: '保鲜 · 中层', foods: middleItems },
                ]" :key="layer.key" class="fridge-layer"
              >
                <div class="fridge-layer-label">
                  {{ layer.label }}
                  <span v-if="layer.foods.length" class="layer-count">{{ layer.foods.length }}</span>
                </div>
                <!-- 横向滚动：食材再多也能全部看全，不再被 +N 截断 -->
                <div class="fridge-food-scroll">
                  <div class="fridge-food-row">
                    <div
                      v-for="(food, fi) in layer.foods" :key="food"
                      class="fridge-food" :class="{ landed: landed === food }"
                      :style="{ animationDelay: landed === food ? '0ms' : `${fi * 38}ms` }"
                    >
                      <Food3D :name="food" v-bind="shapeOf(food)" />
                      <span class="fridge-food-name">{{ food }}</span>
                    </div>
                    <span v-if="!layer.foods.length" class="fridge-layer-empty">空</span>
                  </div>
                </div>
                <div class="fridge-shelf"></div>
              </div>
            </div>
          </div>
          <button class="fridge-door fridge-door-l" :class="{ open: leftOpen }" type="button" aria-label="开关左门" @click="toggleDoor('left')">
            <span class="door-face"><span class="door-handle l"></span><span class="door-bin"></span><span class="door-bin b2"></span></span>
          </button>
          <button class="fridge-door fridge-door-r" :class="{ open: rightOpen }" type="button" aria-label="开关右门" @click="toggleDoor('right')">
            <span class="door-face"><span class="door-handle r"></span><span class="door-bin"></span><span class="door-bin b2"></span></span>
          </button>
          <!-- 开门冷气 -->
          <div v-if="mistOn" class="fridge-mist" aria-hidden="true">
            <span v-for="i in 5" :key="i" :style="{ animationDelay: `${i * 90}ms` }"></span>
          </div>
        </div>

        <!-- 下半：整宽冷冻抽屉 -->
        <div class="fridge-lower" :class="{ 'is-dim': dimLower }">
          <div class="fridge-body drawer-body">
            <div class="fridge-layer drawer-layer">
              <div class="fridge-layer-label">
                ❄ 冷冻抽屉
                <span v-if="lowerItems.length" class="layer-count">{{ lowerItems.length }}</span>
              </div>
              <div class="fridge-food-scroll">
                <div class="fridge-food-row">
                  <div
                    v-for="(food, fi) in lowerItems" :key="food"
                    class="fridge-food" :class="{ landed: landed === food }"
                    :style="{ animationDelay: landed === food ? '0ms' : `${fi * 38}ms` }"
                  >
                    <Food3D :name="food" v-bind="shapeOf(food)" />
                    <span class="fridge-food-name">{{ food }}</span>
                  </div>
                  <span v-if="!lowerItems.length" class="fridge-layer-empty">空</span>
                </div>
              </div>
            </div>
          </div>
          <button class="fridge-drawer" :class="{ open: drawerOpen }" type="button" aria-label="拉开冷冻抽屉" @click="toggleDrawer">
            <span class="drawer-face"><span class="drawer-handle"></span></span>
          </button>
        </div>
        <div class="fridge-brand">HOME · FRESH</div>
      </div>
    </div>

    <div class="fridge-meta" aria-live="polite">
      <span class="fridge-total">
        🧊 <b :key="`n-${items.length}`" class="fridge-num">{{ items.length }}</b> 样
      </span>
      <!-- 分层统计：一眼看出每层放了几样 -->
      <span class="fridge-chips">
        <span class="fridge-chip" :class="{ active: view === 'cold' }" @click="view = view === 'cold' ? 'all' : 'cold'">🥩 冷藏 {{ upperItems.length + middleItems.length }}</span>
        <span class="fridge-chip" :class="{ active: view === 'freeze' }" @click="view = view === 'freeze' ? 'all' : 'freeze'">❄ 冷冻 {{ lowerItems.length }}</span>
      </span>
      <span class="fridge-sep">·</span>
      <span>菜谱 <b :key="`h-${hitCount}`" class="fridge-num">{{ hitCount }}</b> 道用得上</span>
      <button v-if="!leftOpen || !rightOpen || !drawerOpen" class="fridge-openall" type="button" @click="openAll">全部打开</button>
    </div>
  </div>
</template>
