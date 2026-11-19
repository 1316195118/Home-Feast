<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import Food3D from './Food3D.vue'
import { pantryGroups, foodShape } from '../utils'

// 写实风格 3D 冰箱：上半双开门 + 下半整宽冷冻抽屉，纯 CSS 几何体。
const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
  daysOf: { type: Function, default: () => 0 },      // 放进冰箱多少天，用于新鲜度衰减
  dishCountOf: { type: Function, default: () => 0 }, // 每样食材能被多少道菜用到
  drop: { type: Object, default: null },             // 外部投递 { name, x, y, t }
})
const emit = defineEmits(['sticker', 'food-click'])

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

// 新鲜度：7 天起发蔫、14 天更明显
function freshness(name) {
  const d = props.daysOf(name) || 0
  if (d >= 14) return 'stale'
  if (d >= 7) return 'aging'
  return ''
}

// 视角切换
const VIEWS = ['all', 'cold', 'freeze']
const view = ref('all')
const viewLabel = computed(() => ({ all: '全部', cold: '冷藏', freeze: '冷冻' })[view.value])
function cycleView() { view.value = VIEWS[(VIEWS.indexOf(view.value) + 1) % VIEWS.length] }
const dimUpper = computed(() => view.value === 'freeze')
const dimLower = computed(() => view.value === 'cold')

const leftOpen = ref(true), rightOpen = ref(true), drawerOpen = ref(true)
const landed = ref('')
const mistOn = ref(false)
const fullPulse = ref(false)
let timer = null, mistTimer = null, fullTimer = null

function puffMist() {
  mistOn.value = true
  clearTimeout(mistTimer)
  mistTimer = setTimeout(() => { mistOn.value = false }, 1200)
}
// 放满 12 样以上时轻微"吃力"闪一下
watch(() => props.items.length, (n, p) => {
  if (n >= 12 && n > (p || 0)) {
    fullPulse.value = true
    clearTimeout(fullTimer)
    fullTimer = setTimeout(() => { fullPulse.value = false }, 900)
  }
})
watch(() => props.items, (next, prev) => {
  const added = next.find((x) => !(prev || []).includes(x))
  if (added) {
    landed.value = added
    clearTimeout(timer)
    timer = setTimeout(() => { landed.value = '' }, 850)
    if (!leftOpen.value && !rightOpen.value) { leftOpen.value = true; rightOpen.value = true; puffMist() }
  }
})

// 飞入轨迹：从点击的卡片沿弧线飞到冰箱对应层
const fridgeEl = ref(null)
const flies = ref([])
let flyId = 0
watch(() => props.drop, (d) => {
  if (!d || !fridgeEl.value) return
  const box = fridgeEl.value.getBoundingClientRect()
  const target = layerOf(d.name)
  const ty = target === 'upper' ? box.top + box.height * 0.26
    : target === 'middle' ? box.top + box.height * 0.48
      : box.top + box.height * 0.84
  const tx = box.left + box.width * 0.5
  const id = ++flyId
  flies.value = [...flies.value, {
    id, name: d.name,
    path: `M ${d.x}px ${d.y}px Q ${(d.x + tx) / 2}px ${Math.min(d.y, ty) - 90}px ${tx}px ${ty}px`,
  }]
  setTimeout(() => { flies.value = flies.value.filter((f) => f.id !== id) }, 820)
})
const flyStyle = (f) => ({ offsetPath: `path('${f.path}')` })

onUnmounted(() => { clearTimeout(timer); clearTimeout(mistTimer); clearTimeout(fullTimer) })

// 层架反光跟随鼠标
const glare = ref({ x: 50, y: 30 })
function onMove(e) {
  if (!fridgeEl.value) return
  const b = fridgeEl.value.getBoundingClientRect()
  glare.value = {
    x: Math.max(0, Math.min(100, ((e.clientX - b.left) / b.width) * 100)),
    y: Math.max(0, Math.min(100, ((e.clientY - b.top) / b.height) * 100)),
  }
}

function toggleDoor(side) {
  if (side === 'left') leftOpen.value = !leftOpen.value
  else rightOpen.value = !rightOpen.value
  if (leftOpen.value || rightOpen.value) { puffMist(); emit('sticker', 'ding') }
}
function toggleDrawer() {
  drawerOpen.value = !drawerOpen.value
  if (drawerOpen.value) { puffMist(); emit('sticker', 'ding') }
}
const openAll = () => {
  leftOpen.value = true; rightOpen.value = true; drawerOpen.value = true
  puffMist(); emit('sticker', 'ding')
}
</script>

<template>
  <div class="fridge-stage">
    <!-- 飞行中的食材：从卡片弧线飞进冰箱 -->
    <div class="fly-host" aria-hidden="true">
      <span v-for="f in flies" :key="f.id" class="flying-food" :style="flyStyle(f)">
        <Food3D :name="f.name" v-bind="shapeOf(f.name)" />
      </span>
    </div>

    <div class="fridge-scene" @mousemove="onMove" @mouseleave="glare = { x: 50, y: 30 }">
      <div
        ref="fridgeEl" class="fridge"
        :class="{ 'view-cold': view === 'cold', 'view-freeze': view === 'freeze', 'is-full': fullPulse }"
        :style="{ '--glare-x': glare.x + '%', '--glare-y': glare.y + '%' }"
      >
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
              <div class="fridge-glare"></div>
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
                <div class="fridge-food-scroll">
                  <div class="fridge-food-row">
                    <div
                      v-for="(food, fi) in layer.foods" :key="food"
                      class="fridge-food" :class="[freshness(food), { landed: landed === food }]"
                      :style="{ animationDelay: landed === food ? '0ms' : `${fi * 38}ms` }"
                      :title="dishCountOf(food) ? `${food}：${dishCountOf(food)} 道菜用得上` : food"
                      @click="emit('food-click', food)"
                    >
                      <Food3D :name="food" v-bind="shapeOf(food)" />
                      <span class="fridge-food-name">{{ food }}</span>
                    </div>
                    <span v-if="!layer.foods.length" class="fridge-layer-empty">
                      <span class="empty-doodle" aria-hidden="true">
                        <span class="doodle-shelf"></span>
                        <span class="doodle-dot a"></span><span class="doodle-dot b"></span>
                      </span>
                      <span class="empty-text">{{ layer.key === 'upper' ? '上层还空着' : '这里也空着' }}</span>
                    </span>
                  </div>
                </div>
                <div class="fridge-shelf"></div>
              </div>
            </div>
          </div>
          <button class="fridge-door fridge-door-l" :class="{ open: leftOpen }" type="button" aria-label="开关左门" @click="toggleDoor('left')">
            <span class="door-face">
              <span class="door-handle l"></span>
              <span class="door-bin"></span><span class="door-bin b2"></span>
              <span class="fridge-magnets">
                <button class="magnet" type="button" title="去菜谱点菜" @click.stop="emit('sticker', 'menu')">📖</button>
                <button class="magnet" type="button" title="看购物清单" @click.stop="emit('sticker', 'shopping')">🛒</button>
              </span>
            </span>
          </button>
          <button class="fridge-door fridge-door-r" :class="{ open: rightOpen }" type="button" aria-label="开关右门" @click="toggleDoor('right')">
            <span class="door-face">
              <span class="door-handle r"></span>
              <span class="door-bin"></span><span class="door-bin b2"></span>
              <span class="fridge-magnets right">
                <button class="magnet" type="button" title="生成一周菜单" @click.stop="emit('sticker', 'week')">🗓️</button>
              </span>
            </span>
          </button>
          <div v-if="mistOn" class="fridge-mist" aria-hidden="true">
            <span v-for="i in 5" :key="i" :style="{ animationDelay: `${i * 90}ms` }"></span>
          </div>
        </div>

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
                    class="fridge-food" :class="[freshness(food), { landed: landed === food }]"
                    :style="{ animationDelay: landed === food ? '0ms' : `${fi * 38}ms` }"
                    :title="dishCountOf(food) ? `${food}：${dishCountOf(food)} 道菜用得上` : food"
                    @click="emit('food-click', food)"
                  >
                    <Food3D :name="food" v-bind="shapeOf(food)" />
                    <span class="fridge-food-name">{{ food }}</span>
                  </div>
                  <span v-if="!lowerItems.length" class="fridge-layer-empty">
                    <span class="empty-doodle big" aria-hidden="true">
                      <span class="doodle-shelf"></span>
                      <span class="doodle-dot a"></span><span class="doodle-dot b"></span>
                    </span>
                    <span class="empty-text">抽屉空空的</span>
                  </span>
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
