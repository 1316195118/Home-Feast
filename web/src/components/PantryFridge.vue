<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import PantryImage from './PantryImage.vue'
import { pantryGroups } from '../utils'

const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
  daysOf: { type: Function, default: () => 0 },
  dishCountOf: { type: Function, default: () => 0 },
  drop: { type: Object, default: null },
})
const emit = defineEmits(['sticker', 'food-click'])

const GROUP_ZONE = {
  dairy: 'dairy',
  mushroom: 'chilled',
  meat: 'meat',
  seafood: 'seafood',
  veg: 'veg',
  fruit: 'fruit',
  staple: 'door',
}
const GROUP_OF_NAME = (() => {
  const map = {}
  for (const group of pantryGroups || []) {
    for (const name of group.items) map[name] = group.key
  }
  return map
})()
const FROZEN_NAME = /冷冻|速冻|冰淇淋|冰棒|雪糕|水饺|汤圆|冻肉|冻虾/
function zoneOf(name) {
  if (FROZEN_NAME.test(name)) return 'freeze'
  return GROUP_ZONE[GROUP_OF_NAME[name]] || 'chilled'
}
const inZone = (zone) => props.items.filter((name) => zoneOf(name) === zone)
const dairyItems = computed(() => inZone('dairy'))
const chilledItems = computed(() => inZone('chilled'))
const meatItems = computed(() => inZone('meat'))
const seafoodItems = computed(() => inZone('seafood'))
const vegItems = computed(() => inZone('veg'))
const fruitItems = computed(() => inZone('fruit'))
const doorItems = computed(() => inZone('door'))
const freezeItems = computed(() => inZone('freeze'))
const coldCount = computed(() => props.items.length - freezeItems.value.length)

const view = ref('all')
const viewLabel = computed(() => ({ all: '全景', cold: '冷藏', freeze: '冷冻' })[view.value])
const coldDim = computed(() => view.value === 'freeze')
const freezeDim = computed(() => view.value === 'cold')
function setView(next) {
  view.value = view.value === next ? 'all' : next
}

const leftOpen = ref(true)
const rightOpen = ref(true)
const crisperOpen = ref(true)
const freezerOpen = ref(true)
const mistOn = ref(false)
const landed = ref('')
const fullPulse = ref(false)
let mistTimer = null
let landTimer = null
let fullTimer = null

function puffMist(delay = 0) {
  clearTimeout(mistTimer)
  mistTimer = setTimeout(() => {
    mistOn.value = true
    mistTimer = setTimeout(() => { mistOn.value = false }, 1250)
  }, delay)
}

watch(() => props.items.length, (next, previous) => {
  if (next >= 12 && next > (previous || 0)) {
    fullPulse.value = true
    clearTimeout(fullTimer)
    fullTimer = setTimeout(() => { fullPulse.value = false }, 900)
  }
})
watch(() => props.items, (next, previous) => {
  const added = next.find((name) => !(previous || []).includes(name))
  if (!added) return
  landed.value = added
  clearTimeout(landTimer)
  landTimer = setTimeout(() => { landed.value = '' }, 900)
  if (!leftOpen.value && !rightOpen.value) {
    leftOpen.value = true
    rightOpen.value = true
    puffMist()
  }
})

function freshness(name) {
  const days = props.daysOf(name) || 0
  if (days >= 14) return 'is-stale'
  if (days >= 7) return 'is-aging'
  return ''
}
function hashOf(name) {
  let hash = 2166136261
  for (const char of String(name)) hash = Math.imul(hash ^ char.codePointAt(0), 16777619)
  return hash >>> 0
}
function photoStyle(name, index = 0) {
  const hash = hashOf(name)
  return {
    '--photo-turn': `${((hash % 900) / 100 - 4.5).toFixed(2)}deg`,
    '--photo-scale': `${(0.92 + (hash % 17) / 100).toFixed(2)}`,
    '--photo-lift': `${(hash % 4)}px`,
    '--photo-delay': `${Math.min(index * 34, 280)}ms`,
    zIndex: 2 + (hash % 4),
  }
}
function emptyText(zone) {
  return ({
    dairy: '蛋奶层还空着',
    chilled: '菌菇豆制品层还空着',
    meat: '生肉托盘还空着',
    seafood: '水产托盘还空着',
    veg: '蔬菜保鲜抽屉还空着',
    fruit: '水果保鲜抽屉还空着',
    door: '门架还空着',
    freeze: '冷冻区还空着',
  })[zone]
}

const fridgeEl = ref(null)
const flies = ref([])
let flyId = 0
const FLY_Y = { dairy: .25, chilled: .39, meat: .55, seafood: .55, veg: .74, fruit: .74, door: .44, freeze: .91 }
watch(() => props.drop, (drop) => {
  if (!drop || !fridgeEl.value) return
  const box = fridgeEl.value.getBoundingClientRect()
  const zone = zoneOf(drop.name)
  const hash = hashOf(drop.name)
  const targetX = box.left + box.width * (.32 + (hash % 37) / 100)
  const targetY = box.top + box.height * (FLY_Y[zone] || .55)
  const id = ++flyId
  flies.value = [...flies.value, {
    id,
    name: drop.name,
    path: `M ${drop.x}px ${drop.y}px Q ${(drop.x + targetX) / 2}px ${Math.min(drop.y, targetY) - 105}px ${targetX}px ${targetY}px`,
  }]
  setTimeout(() => { flies.value = flies.value.filter((fly) => fly.id !== id) }, 850)
})
const flyStyle = (fly) => ({ offsetPath: `path('${fly.path}')` })

function toggleDoor(side) {
  if (side === 'left') leftOpen.value = !leftOpen.value
  else rightOpen.value = !rightOpen.value
  if (leftOpen.value || rightOpen.value) {
    puffMist(80)
    emit('sticker', 'ding')
  }
}
function openEverything() {
  leftOpen.value = true
  rightOpen.value = true
  crisperOpen.value = true
  freezerOpen.value = true
  puffMist(50)
  emit('sticker', 'ding')
}
const allOpen = computed(() => leftOpen.value && rightOpen.value && crisperOpen.value && freezerOpen.value)

const glare = ref({ x: 50, y: 30 })
function onMove(event) {
  if (!fridgeEl.value) return
  const box = fridgeEl.value.getBoundingClientRect()
  glare.value = {
    x: Math.max(0, Math.min(100, ((event.clientX - box.left) / box.width) * 100)),
    y: Math.max(0, Math.min(100, ((event.clientY - box.top) / box.height) * 100)),
  }
}

onUnmounted(() => {
  clearTimeout(mistTimer)
  clearTimeout(landTimer)
  clearTimeout(fullTimer)
})
</script>

<template>
  <div class="real-fridge-stage">
    <div class="fly-host" aria-hidden="true">
      <span v-for="fly in flies" :key="fly.id" class="flying-food" :style="flyStyle(fly)">
        <PantryImage :name="fly.name" eager />
      </span>
    </div>

    <div class="real-fridge-scene" @mousemove="onMove" @mouseleave="glare = { x: 50, y: 30 }">
      <div
        ref="fridgeEl"
        class="real-fridge"
        :class="{ 'is-full': fullPulse, 'doors-closed': !leftOpen && !rightOpen }"
        :style="{ '--rf-glare-x': `${glare.x}%`, '--rf-glare-y': `${glare.y}%` }"
      >
        <div class="rf-shadow" aria-hidden="true"></div>
        <div class="rf-shell">
          <div class="rf-brand-row">
            <span class="rf-brand-mark">家</span>
            <span class="rf-brand-name">HOME FRESH</span>
            <span class="rf-model">No. 8090</span>
          </div>

          <div class="rf-cold" :class="{ 'is-dim': coldDim }">
            <div class="rf-cold-head">
              <div>
                <span class="rf-eyebrow">COLD ZONE</span>
                <strong>冷藏室</strong>
              </div>
              <div class="rf-head-actions">
                <button
                  class="rf-view-button"
                  type="button"
                  :title="`当前：${viewLabel}，点击切换`"
                  @click.stop="view = view === 'all' ? 'cold' : 'all'"
                >{{ viewLabel }}</button>
                <span class="rf-temp"><i></i>4°C</span>
              </div>
            </div>

            <div class="rf-cavity">
              <div class="rf-light" aria-hidden="true"></div>
              <div class="rf-glass-glare" aria-hidden="true"></div>
              <div class="rf-snow-dust" aria-hidden="true"></div>

              <section class="rf-zone rf-zone-dairy">
                <header>
                  <span class="rf-zone-icon">🥚</span>
                  <span><b>蛋奶 · 即食</b><small>上层</small></span>
                  <em>{{ dairyItems.length }}</em>
                </header>
                <div class="rf-goods-scroll">
                  <div class="rf-goods">
                    <button
                      v-for="(name, index) in dairyItems" :key="name"
                      class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                      :style="photoStyle(name, index)"
                      :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                      type="button" @click="emit('food-click', name)"
                    >
                      <PantryImage :name="name" :eager="index < 6" />
                      <span>{{ name }}</span>
                    </button>
                    <span v-if="!dairyItems.length" class="rf-empty">{{ emptyText('dairy') }}</span>
                  </div>
                </div>
                <div class="rf-shelf-edge" aria-hidden="true"><i></i><i></i><i></i></div>
              </section>

              <section class="rf-zone rf-zone-chilled">
                <header>
                  <span class="rf-zone-icon">🍄</span>
                  <span><b>菌菇 · 豆制品</b><small>密封保鲜</small></span>
                  <em>{{ chilledItems.length }}</em>
                </header>
                <div class="rf-goods-scroll">
                  <div class="rf-goods">
                    <button
                      v-for="(name, index) in chilledItems" :key="name"
                      class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                      :style="photoStyle(name, index)"
                      :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                      type="button" @click="emit('food-click', name)"
                    >
                      <PantryImage :name="name" :eager="index < 6" />
                      <span>{{ name }}</span>
                    </button>
                    <span v-if="!chilledItems.length" class="rf-empty">{{ emptyText('chilled') }}</span>
                  </div>
                </div>
                <div class="rf-shelf-edge" aria-hidden="true"><i></i><i></i><i></i></div>
              </section>

              <div class="rf-protein-row">
                <section class="rf-zone rf-protein-tray meat">
                  <header>
                    <span class="rf-zone-icon">🥩</span>
                    <span><b>生肉托盘</b><small>防串味</small></span>
                    <em>{{ meatItems.length }}</em>
                  </header>
                  <div class="rf-goods-scroll">
                    <div class="rf-goods">
                      <button
                        v-for="(name, index) in meatItems" :key="name"
                        class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                        :style="photoStyle(name, index)"
                        :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                        type="button" @click="emit('food-click', name)"
                      >
                        <PantryImage :name="name" :eager="index < 4" />
                        <span>{{ name }}</span>
                      </button>
                      <span v-if="!meatItems.length" class="rf-empty">{{ emptyText('meat') }}</span>
                    </div>
                  </div>
                </section>

                <section class="rf-zone rf-protein-tray seafood">
                  <header>
                    <span class="rf-zone-icon">🐟</span>
                    <span><b>水产托盘</b><small>独立分区</small></span>
                    <em>{{ seafoodItems.length }}</em>
                  </header>
                  <div class="rf-goods-scroll">
                    <div class="rf-goods">
                      <button
                        v-for="(name, index) in seafoodItems" :key="name"
                        class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                        :style="photoStyle(name, index)"
                        :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                        type="button" @click="emit('food-click', name)"
                      >
                        <PantryImage :name="name" :eager="index < 4" />
                        <span>{{ name }}</span>
                      </button>
                      <span v-if="!seafoodItems.length" class="rf-empty">{{ emptyText('seafood') }}</span>
                    </div>
                  </div>
                </section>
              </div>

              <div class="rf-crispers" :class="{ open: crisperOpen }">
                <section class="rf-crisper veg">
                  <div class="rf-crisper-goods">
                    <button
                      v-for="(name, index) in vegItems" :key="name"
                      class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                      :style="photoStyle(name, index)"
                      :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                      type="button" @click="emit('food-click', name)"
                    >
                      <PantryImage :name="name" />
                      <span>{{ name }}</span>
                    </button>
                    <span v-if="!vegItems.length" class="rf-empty">{{ emptyText('veg') }}</span>
                  </div>
                  <button class="rf-crisper-face" type="button" @click="crisperOpen = !crisperOpen">
                    <span class="rf-drawer-handle"></span>
                    <b>高湿蔬菜抽屉</b>
                    <small>{{ vegItems.length }} 样 · 点击{{ crisperOpen ? '关上' : '拉开' }}</small>
                  </button>
                </section>
                <section class="rf-crisper fruit">
                  <div class="rf-crisper-goods">
                    <button
                      v-for="(name, index) in fruitItems" :key="name"
                      class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                      :style="photoStyle(name, index)"
                      :title="dishCountOf(name) ? `${name}：${dishCountOf(name)} 道菜用得上` : name"
                      type="button" @click="emit('food-click', name)"
                    >
                      <PantryImage :name="name" />
                      <span>{{ name }}</span>
                    </button>
                    <span v-if="!fruitItems.length" class="rf-empty">{{ emptyText('fruit') }}</span>
                  </div>
                  <button class="rf-crisper-face" type="button" @click="crisperOpen = !crisperOpen">
                    <span class="rf-drawer-handle"></span>
                    <b>水果保鲜抽屉</b>
                    <small>{{ fruitItems.length }} 样 · 点击{{ crisperOpen ? '关上' : '拉开' }}</small>
                  </button>
                </section>
              </div>
            </div>
          </div>

          <section class="rf-freezer" :class="{ 'is-dim': freezeDim, open: freezerOpen }">
            <div class="rf-freezer-inner">
              <header>
                <span class="rf-zone-icon">❄</span>
                <span><b>冷冻室</b><small>-18°C</small></span>
                <em>{{ freezeItems.length }}</em>
              </header>
              <div class="rf-goods-scroll">
                <div class="rf-goods">
                  <button
                    v-for="(name, index) in freezeItems" :key="name"
                    class="rf-food" :class="[freshness(name), { landed: landed === name }]"
                    :style="photoStyle(name, index)" type="button" @click="emit('food-click', name)"
                  >
                    <PantryImage :name="name" />
                    <span>{{ name }}</span>
                  </button>
                  <span v-if="!freezeItems.length" class="rf-empty">{{ emptyText('freeze') }}</span>
                </div>
              </div>
            </div>
            <button class="rf-freezer-face" type="button" @click="freezerOpen = !freezerOpen">
              <span class="rf-freezer-ice" aria-hidden="true"></span>
              <span class="rf-drawer-handle"></span>
              <span><b>冷冻抽屉</b><small>{{ freezeItems.length }} 样 · 点击{{ freezerOpen ? '关上' : '拉开' }}</small></span>
            </button>
          </section>

          <div
            class="rf-door rf-door-left" :class="{ open: leftOpen }"
            role="button" tabindex="0" :aria-label="leftOpen ? '关左门' : '开左门'"
            @click="toggleDoor('left')" @keydown.enter="toggleDoor('left')"
          >
            <div class="rf-door-face">
              <span class="rf-handle left"></span>
              <div class="rf-door-bin">
                <small>门架 · 主食干货</small>
                <div>
                  <span v-for="name in doorItems.slice(0, 5)" :key="name" class="rf-door-mini" :title="name">
                    <PantryImage :name="name" />
                    <b>{{ name }}</b>
                  </span>
                  <span v-if="!doorItems.length" class="rf-door-empty">空</span>
                </div>
              </div>
              <div class="rf-magnets">
                <button class="rf-magnet" type="button" title="去菜谱点菜" @click.stop="emit('sticker', 'menu')">📖</button>
                <button class="rf-magnet" type="button" title="看购物清单" @click.stop="emit('sticker', 'shopping')">🛒</button>
              </div>
            </div>
          </div>
          <div
            class="rf-door rf-door-right" :class="{ open: rightOpen }"
            role="button" tabindex="0" :aria-label="rightOpen ? '关右门' : '开右门'"
            @click="toggleDoor('right')" @keydown.enter="toggleDoor('right')"
          >
            <div class="rf-door-face">
              <span class="rf-handle right"></span>
              <div class="rf-door-badge"><b>今日</b><span>好好吃饭</span></div>
              <button class="rf-magnet week" type="button" title="生成一周菜单" @click.stop="emit('sticker', 'week')">🗓️</button>
            </div>
          </div>

          <div v-if="mistOn" class="rf-mist" aria-hidden="true">
            <span v-for="i in 7" :key="i" :style="{ animationDelay: `${i * 70}ms` }"></span>
          </div>
          <span class="rf-foot-light"></span>
        </div>
      </div>
    </div>

    <div class="real-fridge-meta" aria-live="polite">
      <span class="rf-total">🧊 <b :key="`count-${items.length}`">{{ items.length }}</b> 样</span>
      <button class="rf-chip" :class="{ active: view === 'cold' }" type="button" @click="setView('cold')">🥬 冷藏 {{ coldCount }}</button>
      <button class="rf-chip" :class="{ active: view === 'freeze' }" type="button" @click="setView('freeze')">❄ 冷冻 {{ freezeItems.length }}</button>
      <span class="rf-sep">·</span>
      <span>菜谱 <b :key="`hit-${hitCount}`">{{ hitCount }}</b> 道用得上</span>
      <button v-if="!allOpen" class="rf-open-all" type="button" @click="openEverything">全部打开</button>
    </div>
  </div>
</template>

<style scoped>
.real-fridge-stage {
  --rf-ice: #eaf1ee;
  --rf-ice-deep: #c8d7d2;
  --rf-chrome: #aebdb8;
  --rf-ink: #3e5652;
  position: relative;
  margin: 6px 0 16px;
}
.real-fridge-scene {
  display: flex;
  justify-content: center;
  padding: 12px 10px 68px;
  perspective: 1450px;
  perspective-origin: 50% 38%;
  overflow: visible;
}
.real-fridge {
  position: relative;
  width: min(100%, 458px);
  transform: rotateX(1.6deg);
  transform-style: preserve-3d;
}
.rf-shadow {
  position: absolute;
  left: 8%; right: 8%; bottom: -34px; height: 42px;
  border-radius: 50%;
  background: radial-gradient(ellipse, rgba(63,78,72,.28), transparent 70%);
  filter: blur(8px);
}
.rf-shell {
  position: relative;
  padding: 13px 13px 14px;
  border: 1px solid rgba(132,153,147,.55);
  border-radius: 30px;
  background:
    linear-gradient(112deg, rgba(255,255,255,.86), transparent 24%),
    linear-gradient(150deg, #f8f4ea 0%, #e7e5dc 44%, #d5dcd8 100%);
  box-shadow:
    inset 1px 1px 0 rgba(255,255,255,.95),
    inset -3px -5px 12px rgba(75,89,84,.12),
    0 18px 34px rgba(63,73,69,.24);
}
.rf-brand-row {
  height: 30px; display: flex; align-items: center; gap: 8px;
  padding: 0 8px 7px; color: #687975;
}
.rf-brand-mark {
  width: 20px; height: 20px; display: grid; place-items: center;
  border-radius: 50%; color: #fff9ec; background: var(--brand, #c45a3c);
  font-size: .64rem; font-weight: 900;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.35);
}
.rf-brand-name { font-size: .55rem; font-weight: 900; letter-spacing: 2px; }
.rf-model { margin-left: auto; font-size: .48rem; letter-spacing: .8px; opacity: .65; }
.rf-cold, .rf-freezer { position: relative; transition: filter .45s var(--ease), opacity .45s var(--ease); }
.rf-cold.is-dim, .rf-freezer.is-dim { filter: saturate(.34) brightness(.88); opacity: .72; }
.rf-cold {
  height: 430px; border: 2px solid #b9c7c2; border-radius: 22px;
  background: linear-gradient(160deg, #f2f7f4, #dce6e1);
  box-shadow: inset 0 3px 10px rgba(255,255,255,.95), inset 0 -7px 15px rgba(56,74,68,.13);
}
.rf-cold-head {
  height: 43px; display: flex; align-items: center; justify-content: space-between;
  padding: 5px 11px 4px; border-bottom: 1px solid rgba(135,159,153,.34);
  background: linear-gradient(180deg, rgba(255,255,255,.86), rgba(232,240,237,.52));
  border-radius: 20px 20px 0 0;
}
.rf-cold-head > div:first-child { display: flex; align-items: baseline; gap: 7px; }
.rf-eyebrow { font-size: .45rem; font-weight: 900; letter-spacing: 1px; color: #8ba09b; }
.rf-cold-head strong { font-size: .75rem; color: #506863; }
.rf-head-actions { display: flex; align-items: center; gap: 6px; }
.rf-view-button, .rf-temp {
  min-height: 24px; display: inline-flex; align-items: center;
  border: 1px solid rgba(127,153,147,.42); border-radius: 999px;
  color: #5d746f; background: rgba(255,255,255,.7);
  font-size: .52rem; font-weight: 800;
}
.rf-view-button { padding: 0 9px; cursor: pointer; }
.rf-temp { gap: 4px; padding: 0 8px; color: #347f8b; }
.rf-temp i { width: 6px; height: 6px; border-radius: 50%; background: #65c4d0; box-shadow: 0 0 7px #65c4d0; }
.rf-cavity {
  position: absolute; inset: 43px 7px 7px; overflow: hidden;
  border-radius: 7px 7px 16px 16px;
  background: linear-gradient(180deg, #f9fbf9 0%, #edf4f1 37%, #e2ece8 100%);
  box-shadow: inset 0 5px 13px rgba(48,76,68,.12);
  display: grid; grid-template-rows: 78px 90px 116px 108px; gap: 3px; padding: 4px 6px 5px;
}
.rf-light {
  position: absolute; inset: 0; z-index: 1; pointer-events: none;
  background:
    radial-gradient(ellipse at 25% 1%, rgba(255,255,255,1), transparent 37%),
    radial-gradient(ellipse at 78% 4%, rgba(210,239,244,.66), transparent 34%);
  animation: rf-breathe 5s ease-in-out infinite;
}
.rf-glass-glare {
  position: absolute; inset: 0; z-index: 8; pointer-events: none; mix-blend-mode: screen;
  background: radial-gradient(190px circle at var(--rf-glare-x,50%) var(--rf-glare-y,30%), rgba(255,255,255,.4), transparent 68%);
}
.rf-snow-dust {
  position: absolute; inset: 0; z-index: 2; pointer-events: none; opacity: .27;
  background-image:
    radial-gradient(circle at 12% 18%, white 0 1px, transparent 1.5px),
    radial-gradient(circle at 76% 12%, white 0 1px, transparent 1.5px),
    radial-gradient(circle at 45% 61%, white 0 1px, transparent 1.5px);
  background-size: 73px 59px, 91px 67px, 110px 83px;
}
.rf-zone {
  position: relative; z-index: 3; min-width: 0;
  padding: 5px 7px 8px; border-radius: 8px 8px 5px 5px;
}
.rf-zone > header, .rf-freezer-inner > header {
  height: 23px; display: flex; align-items: center; gap: 5px; color: #55706a;
}
.rf-zone > header > span:nth-child(2), .rf-freezer-inner > header > span:nth-child(2) {
  display: flex; align-items: baseline; gap: 5px; min-width: 0;
}
.rf-zone header b, .rf-freezer-inner header b { font-size: .56rem; white-space: nowrap; }
.rf-zone header small, .rf-freezer-inner header small { font-size: .43rem; color: #94aaa5; white-space: nowrap; }
.rf-zone header em, .rf-freezer-inner header em {
  min-width: 16px; height: 16px; display: inline-grid; place-items: center;
  margin-left: auto; padding: 0 4px; border-radius: 999px; color: #fff;
  background: #91aaa4; font-size: .44rem; font-style: normal; font-weight: 900;
}
.rf-zone-icon { font-size: .68rem; line-height: 1; filter: saturate(.82); }
.rf-goods-scroll {
  overflow-x: auto; overflow-y: hidden; padding: 1px 1px 3px; scrollbar-width: none;
  mask-image: linear-gradient(90deg, #000 0, #000 calc(100% - 15px), transparent);
}
.rf-goods-scroll::-webkit-scrollbar { display: none; }
.rf-goods { display: flex; align-items: flex-end; gap: 2px; min-width: max-content; min-height: 45px; }
.rf-food {
  position: relative; flex: none; width: 42px; height: 47px;
  display: flex; flex-direction: column; align-items: center; padding: 0;
  border: 0; color: #50645f; background: transparent; cursor: pointer;
  transform: rotate(var(--photo-turn)) scale(var(--photo-scale)) translateY(var(--photo-lift));
  transform-origin: 50% 90%;
  transition: transform .2s var(--ease), filter .3s var(--ease);
  animation: rf-food-in .52s cubic-bezier(.18,1.35,.36,1) var(--photo-delay) backwards;
}
.rf-food :deep(.pantry-photo) {
  width: 40px; height: 35px;
  filter: drop-shadow(0 2px 2px rgba(45,62,56,.14));
}
.rf-food > span:last-child {
  max-width: 46px; margin-top: -1px; overflow: hidden;
  font-size: .41rem; font-weight: 800; line-height: 1.3;
  text-overflow: ellipsis; white-space: nowrap;
}
.rf-food:hover, .rf-food:focus-visible {
  z-index: 12; transform: rotate(0deg) scale(1.16) translateY(-3px);
}
.rf-food:active { transform: scale(.94); }
.rf-food.landed { animation: rf-food-landed .82s cubic-bezier(.18,1.38,.34,1); }
.rf-food.is-aging :deep(.pantry-photo) { filter: saturate(.64) sepia(.12) drop-shadow(0 2px 2px rgba(45,62,56,.12)); }
.rf-food.is-stale :deep(.pantry-photo) { filter: saturate(.3) brightness(.92) blur(.25px); transform: rotate(-4deg) translateY(2px); }
.rf-food.is-stale > span:last-child { color: #9a685c; text-decoration: line-through; }
.rf-shelf-edge {
  position: absolute; left: 2px; right: 2px; bottom: 1px; height: 6px;
  border: 1px solid rgba(133,158,151,.48); border-radius: 50%;
  background: linear-gradient(180deg, rgba(255,255,255,.98), rgba(191,211,205,.75));
  box-shadow: 0 3px 4px rgba(57,78,71,.15);
}
.rf-shelf-edge i {
  position: absolute; top: 1px; bottom: 1px; width: 1px;
  background: rgba(144,170,163,.35);
}
.rf-shelf-edge i:nth-child(1) { left: 25%; }
.rf-shelf-edge i:nth-child(2) { left: 52%; }
.rf-shelf-edge i:nth-child(3) { left: 78%; }
.rf-protein-row {
  position: relative; z-index: 3; min-width: 0;
  display: grid; grid-template-columns: 1fr 1fr; gap: 4px;
}
.rf-protein-tray {
  min-width: 0; border: 1px solid rgba(172,160,144,.48);
  background: linear-gradient(160deg, rgba(255,253,248,.76), rgba(241,232,218,.62));
  box-shadow: inset 0 2px 5px rgba(255,255,255,.76), 0 2px 4px rgba(79,67,53,.1);
}
.rf-protein-tray.seafood { background: linear-gradient(160deg, rgba(247,252,252,.78), rgba(223,235,235,.65)); }
.rf-protein-tray::before {
  content: ''; position: absolute; inset: 24px 5px 5px; border-radius: 5px;
  border: 1px dashed rgba(159,149,134,.25); pointer-events: none;
}
.rf-protein-tray .rf-food { width: 39px; height: 52px; }
.rf-protein-tray .rf-food :deep(.pantry-photo) { width: 37px; height: 38px; }
.rf-crispers {
  position: relative; z-index: 5; min-width: 0;
  display: grid; grid-template-columns: 1fr 1fr; gap: 5px;
}
.rf-crisper {
  position: relative; min-width: 0; overflow: hidden;
  border: 1px solid rgba(134,158,145,.54); border-radius: 12px 12px 9px 9px;
  background:
    linear-gradient(115deg, rgba(255,255,255,.54), transparent 30%),
    linear-gradient(180deg, rgba(219,236,220,.82), rgba(193,216,197,.9));
}
.rf-crisper.fruit {
  background:
    linear-gradient(115deg, rgba(255,255,255,.54), transparent 30%),
    linear-gradient(180deg, rgba(243,224,218,.82), rgba(226,203,196,.86));
}
.rf-crisper-goods {
  position: absolute; inset: 4px 4px 31px; display: flex; align-items: flex-end; gap: 1px;
  overflow-x: auto; scrollbar-width: none; transform-origin: 50% 100%;
  transition: transform .5s cubic-bezier(.24,.85,.3,1), opacity .35s;
}
.rf-crisper-goods::-webkit-scrollbar { display: none; }
.rf-crisper-goods .rf-food { width: 39px; height: 45px; }
.rf-crisper-goods .rf-food :deep(.pantry-photo) { width: 37px; height: 34px; }
.rf-crispers:not(.open) .rf-crisper-goods { opacity: .12; }
.rf-crispers.open .rf-crisper-goods { transform: translateY(-4px); }
.rf-crisper-face, .rf-freezer-face {
  position: absolute; left: 3px; right: 3px; bottom: 2px; height: 31px;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 1px; border: 1px solid rgba(132,157,146,.45); border-radius: 8px;
  color: #536d64; background: linear-gradient(180deg, rgba(255,255,255,.82), rgba(205,224,211,.86));
  box-shadow: inset 0 1px 0 rgba(255,255,255,.94), 0 2px 4px rgba(46,67,57,.12);
  cursor: pointer; transition: transform .45s cubic-bezier(.25,.85,.28,1);
}
.rf-crisper.fruit .rf-crisper-face { background: linear-gradient(180deg, rgba(255,249,245,.85), rgba(230,205,198,.88)); }
.rf-crispers.open .rf-crisper-face { transform: translateY(13px) rotateX(7deg); }
.rf-crisper-face b, .rf-freezer-face b { font-size: .5rem; line-height: 1; }
.rf-crisper-face small, .rf-freezer-face small { font-size: .39rem; opacity: .72; line-height: 1; }
.rf-drawer-handle {
  position: absolute; top: 4px; left: 50%; width: 58px; height: 3px;
  border-radius: 999px; background: linear-gradient(180deg, #f7fbf9, #9db3ac);
  box-shadow: 0 1px 2px rgba(39,57,50,.24); transform: translateX(-50%);
}
.rf-empty {
  align-self: center; padding: 4px 9px; border-radius: 999px;
  color: #9fb1ac; background: rgba(255,255,255,.48);
  font-size: .45rem; white-space: nowrap;
}
.rf-freezer {
  height: 90px; margin-top: 7px; border: 2px solid #b8c8c2; border-radius: 16px 16px 21px 21px;
  background: linear-gradient(180deg, #e4eeea, #cfded9);
  box-shadow: inset 0 3px 8px rgba(255,255,255,.95), 0 8px 14px rgba(41,59,53,.14);
}
.rf-freezer-inner {
  position: absolute; inset: 4px 6px 6px; padding: 3px 6px; overflow: hidden;
  border-radius: 11px; background: linear-gradient(180deg, #eef7f9, #dcebed);
}
.rf-freezer-inner .rf-food { width: 39px; height: 49px; }
.rf-freezer-inner .rf-food :deep(.pantry-photo) { width: 37px; height: 36px; }
.rf-freezer-face {
  inset: 2px; height: auto; z-index: 6; gap: 3px; border-radius: 14px 14px 18px 18px;
  background:
    linear-gradient(115deg, rgba(255,255,255,.7), transparent 31%),
    linear-gradient(165deg, #eef3ef, #c6d6d1 72%, #b8cac4);
  box-shadow: inset 0 1px 0 rgba(255,255,255,.95), 0 5px 10px rgba(43,65,57,.18);
}
.rf-freezer.open .rf-freezer-face { transform: translateY(46px) rotateX(8deg); }
.rf-freezer-ice {
  position: absolute; inset: 0; border-radius: inherit; opacity: .32;
  background:
    radial-gradient(circle at 15% 18%, #fff 0 2px, transparent 3px),
    radial-gradient(circle at 75% 70%, #fff 0 1.5px, transparent 2.5px),
    radial-gradient(circle at 42% 30%, #fff 0 1px, transparent 2px);
  background-size: 56px 43px, 81px 62px, 47px 39px;
}
.rf-door {
  position: absolute; z-index: 20; top: 43px; width: calc(50% + 1px); height: 430px;
  cursor: pointer; transform-style: preserve-3d;
  transition: transform .7s cubic-bezier(.3,.76,.3,1); outline: none;
}
.rf-door-left {
  left: 0; transform-origin: left center;
  transform: perspective(1000px) rotateY(-82deg);
}
.rf-door-right {
  right: 0; transform-origin: right center;
  transform: perspective(1000px) rotateY(82deg);
}
.rf-door:not(.open).rf-door-left { transform: perspective(1000px) rotateY(0deg); }
.rf-door:not(.open).rf-door-right { transform: perspective(1000px) rotateY(0deg); }
.rf-door:focus-visible .rf-door-face { outline: 2px solid var(--brand); outline-offset: 3px; }
.rf-door-face {
  position: absolute; inset: 0; overflow: hidden;
  border: 2px solid rgba(157,175,168,.68);
  background:
    linear-gradient(118deg, rgba(255,255,255,.86), transparent 26%),
    linear-gradient(155deg, #fbf7ee, #e5e2d9 63%, #d1d8d4);
  box-shadow: inset 0 1px 8px rgba(255,255,255,.9), 5px 8px 15px rgba(46,61,56,.19);
}
.rf-door-left .rf-door-face { border-radius: 20px 8px 8px 20px; }
.rf-door-right .rf-door-face { border-radius: 8px 20px 20px 8px; }
.rf-door:not(.open) .rf-door-face { background: linear-gradient(150deg, #fbf8ef, #dddcd4 60%, #c7d0cc); }
.rf-handle {
  position: absolute; top: 22%; width: 10px; height: 126px;
  border: 1px solid rgba(111,133,126,.38); border-radius: 999px;
  background: linear-gradient(90deg, #95aaa4, #f6fbf8 46%, #8fa59f);
  box-shadow: 2px 4px 7px rgba(38,53,47,.2), inset 0 1px 0 rgba(255,255,255,.8);
}
.rf-handle.left { right: 10px; }
.rf-handle.right { left: 10px; }
.rf-door-bin {
  position: absolute; left: 12px; right: 18px; bottom: 21px; min-height: 76px;
  padding: 8px 6px 6px; border: 1px solid rgba(147,169,162,.54); border-radius: 10px;
  background: rgba(255,255,255,.48);
  box-shadow: inset 0 1px 0 rgba(255,255,255,.85);
}
.rf-door-bin > small {
  display: block; margin-bottom: 4px; color: #728680;
  font-size: .43rem; font-weight: 800; text-align: center;
}
.rf-door-bin > div { display: flex; justify-content: center; gap: 3px; }
.rf-door-mini { width: 37px; display: flex; flex-direction: column; align-items: center; gap: 1px; }
.rf-door-mini :deep(.pantry-photo) { width: 30px; height: 30px; }
.rf-door-mini b {
  max-width: 36px; overflow: hidden; color: #657871;
  font-size: .36rem; text-overflow: ellipsis; white-space: nowrap;
}
.rf-door-empty { color: #9eb0aa; font-size: .55rem; }
.rf-door-badge {
  position: absolute; left: 18px; right: 22px; top: 91px; padding: 11px 8px;
  border: 1px dashed rgba(173,146,111,.55); border-radius: 9px;
  color: #8b7258; transform: rotate(-2deg); text-align: center;
}
.rf-door-badge b { display: block; font-family: serif; font-size: 1.1rem; }
.rf-door-badge span { font-size: .43rem; font-weight: 700; letter-spacing: .4px; }
.rf-magnets {
  position: absolute; left: 18px; top: 15px;
  display: flex; flex-direction: column; gap: 5px;
}
.rf-magnet {
  width: 23px; height: 23px; display: grid; place-items: center; padding: 0;
  border: 1px solid rgba(181,149,100,.48); border-radius: 6px;
  background: linear-gradient(150deg, #fff8e6, #efd8b4);
  box-shadow: 0 2px 4px rgba(107,77,47,.14);
  cursor: pointer; font-size: .69rem; transition: transform .18s var(--ease);
}
.rf-magnet:hover { transform: translateY(-2px) rotate(-5deg) scale(1.08); }
.rf-magnet.week { position: absolute; right: 18px; top: 15px; }
.rf-mist { position: absolute; inset: 50px 0 auto; z-index: 19; height: 180px; pointer-events: none; }
.rf-mist span {
  position: absolute; bottom: 0; width: 52px; height: 52px; border-radius: 50%; opacity: 0;
  background: radial-gradient(circle, rgba(255,255,255,.9), rgba(255,255,255,0) 71%);
  filter: blur(3px); animation: rf-mist-rise 1.2s ease-out forwards;
}
.rf-mist span:nth-child(1) { left: 4%; }
.rf-mist span:nth-child(2) { left: 19%; }
.rf-mist span:nth-child(3) { left: 35%; }
.rf-mist span:nth-child(4) { left: 50%; }
.rf-mist span:nth-child(5) { left: 65%; }
.rf-mist span:nth-child(6) { left: 79%; }
.rf-mist span:nth-child(7) { left: 91%; }
.rf-foot-light {
  position: absolute; left: 15%; right: 15%; bottom: -9px; height: 10px;
  border-radius: 50%; background: radial-gradient(ellipse, rgba(255,55,30,.2), transparent 68%);
  filter: blur(2px);
}
.real-fridge.is-full .rf-shell { animation: rf-strain .9s var(--ease); }
.real-fridge-meta {
  display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: 6px;
  margin-top: 5px; color: var(--ink-soft); font-size: .73rem; font-weight: 650;
}
.real-fridge-meta b { color: var(--brand); }
.rf-total b, .real-fridge-meta > span > b { display: inline-block; animation: rf-count .42s var(--ease-out); }
.rf-chip {
  min-height: 27px; padding: 2px 9px; border: 1px solid var(--line);
  border-radius: 999px; color: var(--ink-soft); background: var(--paper-deep);
  cursor: pointer; font-size: .66rem; font-weight: 750;
}
.rf-chip.active { border-color: var(--brand); color: var(--brand-dark); background: var(--brand-soft); }
.rf-sep { color: var(--line-strong); }
.rf-open-all {
  min-height: 27px; padding: 2px 9px; border: 1px solid var(--brand);
  border-radius: 999px; color: var(--brand-dark); background: var(--brand-soft);
  cursor: pointer; font-size: .65rem; font-weight: 800;
}
.fly-host { position: fixed; inset: 0; z-index: 9998; pointer-events: none; }
.flying-food {
  position: absolute; left: 0; top: 0; width: 54px; height: 54px;
  offset-distance: 0%; offset-rotate: 0deg;
  animation: rf-fly-in .82s cubic-bezier(.35,.7,.3,1) forwards;
  filter: drop-shadow(0 6px 8px rgba(57,45,38,.25));
}
@keyframes rf-food-in {
  from { opacity: 0; transform: translateY(-18px) scale(.45) rotate(-12deg); }
  to { opacity: 1; transform: rotate(var(--photo-turn)) scale(var(--photo-scale)) translateY(var(--photo-lift)); }
}
@keyframes rf-food-landed {
  0% { opacity: 0; transform: translateY(-30px) scale(.4) rotate(-15deg); }
  65% { opacity: 1; transform: translateY(3px) scale(1.18) rotate(3deg); }
  100% { transform: rotate(var(--photo-turn)) scale(var(--photo-scale)) translateY(var(--photo-lift)); }
}
@keyframes rf-breathe { 0%,100% { opacity: .82; } 50% { opacity: 1; } }
@keyframes rf-mist-rise {
  0% { opacity: 0; transform: translateY(12px) scale(.5); }
  34% { opacity: .8; }
  100% { opacity: 0; transform: translateY(-58px) scale(1.55); }
}
@keyframes rf-strain {
  0%,100% { transform: none; }
  32% { transform: translateY(1px) scaleX(.997); }
  66% { transform: translateY(-1px) scaleX(1.002); }
}
@keyframes rf-count { 0% { transform: scale(1.65); opacity: .35; } 100% { transform: scale(1); opacity: 1; } }
@keyframes rf-fly-in {
  0% { offset-distance: 0%; transform: scale(1); opacity: 1; }
  72% { opacity: 1; }
  100% { offset-distance: 100%; transform: scale(.55); opacity: 0; }
}
@media (max-width: 560px) {
  .real-fridge-scene { padding: 7px 0 63px; perspective: 900px; }
  .rf-shell { padding: 10px 10px 12px; border-radius: 25px; }
  .rf-brand-row { height: 27px; padding-bottom: 5px; }
  .rf-cold { height: 405px; border-radius: 19px; }
  .rf-cavity { inset: 41px 5px 6px; grid-template-rows: 74px 85px 110px 105px; padding-inline: 4px; }
  .rf-cold-head { height: 41px; padding-inline: 8px; }
  .rf-door { top: 41px; height: 405px; }
  .rf-door-left { transform: perspective(780px) rotateY(-79deg); }
  .rf-door-right { transform: perspective(780px) rotateY(79deg); }
  .rf-door:not(.open).rf-door-left { transform: perspective(780px) rotateY(0); }
  .rf-door:not(.open).rf-door-right { transform: perspective(780px) rotateY(0); }
  .rf-food { width: 37px; height: 44px; }
  .rf-food :deep(.pantry-photo) { width: 35px; height: 32px; }
  .rf-protein-tray .rf-food { width: 35px; }
  .rf-protein-tray .rf-food :deep(.pantry-photo) { width: 33px; height: 34px; }
  .rf-crisper-goods .rf-food { width: 35px; }
  .rf-crisper-goods .rf-food :deep(.pantry-photo) { width: 33px; height: 31px; }
  .rf-freezer { height: 86px; }
  .rf-door-bin { left: 8px; right: 14px; }
  .rf-door-bin > div { gap: 1px; }
  .real-fridge-meta { margin-top: 7px; gap: 4px; font-size: .69rem; }
  .rf-chip, .rf-open-all { min-height: 28px; padding-inline: 8px; font-size: .62rem; }
}
@media (max-width: 360px) {
  .rf-shell { padding-inline: 8px; }
  .rf-cold { height: 390px; }
  .rf-cavity { grid-template-rows: 70px 82px 104px 102px; }
  .rf-door { height: 390px; }
  .rf-zone { padding-inline: 5px; }
  .rf-zone header small { display: none; }
  .rf-food { width: 34px; }
  .rf-food :deep(.pantry-photo) { width: 33px; }
  .rf-cold-head .rf-eyebrow, .rf-model { display: none; }
}
@media (hover: none) {
  .rf-food:hover { transform: rotate(var(--photo-turn)) scale(var(--photo-scale)) translateY(var(--photo-lift)); }
  .rf-magnet:hover { transform: none; }
}
@media (prefers-reduced-motion: reduce) {
  .rf-door, .rf-freezer-face, .rf-crisper-face, .rf-food, .rf-light,
  .rf-mist span, .flying-food, .real-fridge.is-full .rf-shell {
    animation: none !important; transition: none !important;
  }
  .rf-door-left, .rf-door-right { transform: scaleX(.05); }
}
</style>
