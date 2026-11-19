<script setup>
import { ref, computed, watch, onUnmounted } from 'vue'
import Food3D from './Food3D.vue'

// 写实风格 3D 冰箱：三层冷藏 + 双开门，纯 CSS 几何体做食材立体造型。
const props = defineProps({
  items: { type: Array, default: () => [] },
  visualOf: { type: Function, required: true },
  hitCount: { type: Number, default: 0 },
})

// 食材 → 3D 造型 + 颜色 + 所在层
const SHAPES = {
  番茄: ['sphere', '#e04434'], 苹果: ['sphere', '#d23b32'], 橙子: ['sphere', '#ed9121'],
  土豆: ['sphere', '#c8a165'], 柠檬: ['sphere', '#e8cf3b'], 梨: ['sphere', '#c8cf5e'],
  胡萝卜: ['cone', '#e2761f'], 白萝卜: ['cone', '#e8e6d8'],
  黄瓜: ['long', '#4e9b47'], 茄子: ['long', '#6b3f86'], 玉米: ['long', '#e5bb34'], 莴笋: ['long', '#7fae52'], 冬瓜: ['long', '#b7cf9a'],
  白菜: ['leafy', '#8dc45a'], 菠菜: ['leafy', '#3f8f3a'], 生菜: ['leafy', '#82ba4c'], 上海青: ['leafy', '#5a9e42'],
  芹菜: ['leafy', '#6fae52'], 青蒜: ['leafy', '#6aa347'], 葱: ['leafy', '#79ad4a'],
  鸡蛋: ['egg', '#f0e2c4'], 咸鸭蛋: ['egg', '#e6d5b4'],
  鲈鱼: ['fish', '#7fa9bd'], 鲤鱼: ['fish', '#b98a5e'], 带鱼: ['fish', '#a9bcc4'], 三文鱼: ['fish', '#e88a63'], 鱿鱼: ['fish', '#d8b4a0'],
  虾: ['shrimp', '#f08a6a'], 虾仁: ['shrimp', '#f0a088'],
  猪肉: ['meat', '#d9665c'], 五花肉: ['meat', '#e08a7c'], 排骨: ['meat', '#c25a50'], 牛肉: ['meat', '#a63c35'],
  牛腩: ['meat', '#9c3a32'], 鸡胸肉: ['meat', '#e0b49c'], 鸡腿肉: ['meat', '#d59a78'], 鸡翅: ['meat', '#dc9d63'],
  羊肉: ['meat', '#b8534a'], 培根: ['meat', '#c85b4e'], 香肠: ['meat', '#c34a3f'],
  香菇: ['mushroom', '#9c7b58'], 金针菇: ['mushroom', '#dcc98c'], 平菇: ['mushroom', '#bba98c'], 木耳: ['mushroom', '#5c4a44'],
  牛奶: ['carton', '#cfe0e6'], 酸奶: ['carton', '#e8e0d0'], 黄油: ['carton', '#f0d878'], 奶酪: ['carton', '#eec84f'],
  米饭: ['bowl', '#f2ece0'], 面条: ['bowl', '#e8d9a8'], 挂面: ['bowl', '#e2d0a0'], 馒头: ['bowl', '#f4ece0'],
  饺子皮: ['bowl', '#efe6d4'], 面粉: ['bowl', '#f0eadc'], 年糕: ['bowl', '#f2ebe0'], 粉丝: ['bowl', '#e6dfcf'], 面包: ['bowl', '#d1a15e'],
  黄豆: ['beans', '#d8bc4e'], 毛豆: ['beans', '#8fbb52'], 豆角: ['beans', '#6aa347'],
}
const UPPER = ['meat', 'seafood'], MIDDLE = ['veg', 'mushroom'], LOWER = ['staple', 'fruit', 'dairy']
function shapeOf(name) {
  const hit = SHAPES[name]
  if (hit) return { kind: hit[0], color: hit[1] }
  const v = props.visualOf(name)
  return { kind: 'cube', color: v.tint || '#d8c8b0' }
}
const layerOf = (name) => {
  if (UPPER.some((k) => /猪|牛|羊|鸡|虾|鱼|肉|排|翅|培根|香肠|蛋/.test(name))) return 'upper'
  if (LOWER.some((k) => /米|面|馒头|粉|包|豆|奶|酪|油|苹果|香蕉|柠檬|菠萝|梨|橙/.test(name))) return 'lower'
  if (/香|菇|耳|腐|豆/.test(name)) return 'middle'
  return 'middle'
}
const layers = computed(() => [
  { key: 'upper', label: '冷冻上层', foods: props.items.filter((n) => layerOf(n) === 'upper') },
  { key: 'middle', label: '保鲜中层', foods: props.items.filter((n) => layerOf(n) === 'middle') },
  { key: 'lower', label: '冷藏下层', foods: props.items.filter((n) => layerOf(n) === 'lower') },
])

const leftOpen = ref(true), rightOpen = ref(true), landed = ref('')
let timer = null
watch(() => props.items, (next, prev) => {
  const added = next.find((x) => !(prev || []).includes(x))
  if (added) {
    landed.value = added
    clearTimeout(timer)
    timer = setTimeout(() => { landed.value = '' }, 850)
    leftOpen.value = true; rightOpen.value = true
  }
})
onUnmounted(() => clearTimeout(timer))
const openAll = () => { leftOpen.value = true; rightOpen.value = true }
</script>

<template>
  <div class="fridge-stage">
    <div class="fridge-scene">
      <div class="fridge">
        <div class="fridge-body">
          <div class="fridge-top">❄ 冷藏 <span class="fridge-temp">4°C</span></div>
          <div class="fridge-cavity">
            <div class="fridge-glow"></div>
            <div v-for="layer in layers" :key="layer.key" class="fridge-layer">
              <div class="fridge-layer-label">{{ layer.label }}</div>
              <div class="fridge-food-row">
                <div
                  v-for="food in layer.foods.slice(0, 9)" :key="food"
                  class="fridge-food" :class="{ landed: landed === food }"
                >
                  <Food3D :name="food" v-bind="shapeOf(food)" />
                  <span class="fridge-food-name">{{ food }}</span>
                </div>
                <span v-if="!layer.foods.length" class="fridge-layer-empty">空</span>
                <span v-if="layer.foods.length > 9" class="fridge-food-more">+{{ layer.foods.length - 9 }}</span>
              </div>
              <div class="fridge-shelf"></div>
            </div>
          </div>
          <div class="fridge-foot">HOME · FRESH</div>
        </div>
        <button class="fridge-door fridge-door-l" :class="{ open: leftOpen }" type="button" aria-label="开关左门" @click="leftOpen = !leftOpen">
          <span class="door-face"><span class="door-line"></span><span class="door-handle l"></span><span class="door-bin"></span></span>
        </button>
        <button class="fridge-door fridge-door-r" :class="{ open: rightOpen }" type="button" aria-label="开关右门" @click="rightOpen = !rightOpen">
          <span class="door-face"><span class="door-line"></span><span class="door-handle r"></span><span class="door-bin"></span></span>
        </button>
      </div>
    </div>
    <div class="fridge-meta" aria-live="polite">
      <span>🧊 <b :key="`n-${items.length}`" class="fridge-num">{{ items.length }}</b> 样食材</span>
      <span class="fridge-sep">·</span>
      <span>菜谱 <b :key="`h-${hitCount}`" class="fridge-num">{{ hitCount }}</b> 道用得上</span>
      <button v-if="!leftOpen || !rightOpen" class="fridge-openall" type="button" @click="openAll">全部打开</button>
      <span v-else class="fridge-tip">点门开关 · 食材自动分层</span>
    </div>
  </div>
</template>
