<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import { dishImage } from '../utils'
import { cachedThumb } from '../thumb-cache'

const props = defineProps({
  item: { type: Object, required: true },
  cuisines: { type: Array, default: () => [] },
  dishes: { type: Array, default: null },
  size: { type: String, default: '' },
  alt: { type: String, default: '' },
})

// 先试当前清晰度；缩略图缺失时退回原图；原图损坏再退回菜系封面，
// 最后固定回退家常封面。这样任何一张上传图异常都不会留下破图。
const candidates = computed(() => {
  const preferred = dishImage(props.item, props.cuisines, props.dishes, props.size)
  const urls = [
    cachedThumb(preferred) || preferred,
    dishImage(props.item, props.cuisines, props.dishes, ''),
  ]
  const key = props.item?.cuisine || props.item?.dishCuisine
  urls.push((props.cuisines || []).find((cuisine) => cuisine.key === key)?.image || '')
  urls.push('/dishes/homestyle.webp')
  return urls.filter((url, index) => url && urls.indexOf(url) === index)
})

const attempt = ref(0)
const loaded = ref(false)
const retry = ref(0)
const renderKey = ref(0)
let retryTimer = 0

function resetRetry() {
  clearTimeout(retryTimer)
  retryTimer = 0
  retry.value = 0
}

watch(candidates, () => {
  attempt.value = 0
  loaded.value = false
  resetRetry()
})
const source = computed(() => candidates.value[Math.min(attempt.value, candidates.value.length - 1)] || '')

function fallback() {
  loaded.value = false
  // 局域网偶发丢包或设备刚从休眠恢复时，先对同一地址静默重试一次；
  // 成功就原位淡入，仍失败才降级，避免一张图短暂超时就把整道菜换成菜系封面。
  if (retry.value === 0 && source.value) {
    retryTimer = setTimeout(() => {
      retry.value = 1
      renderKey.value += 1
    }, 450)
    return
  }
  resetRetry()
  if (attempt.value < candidates.value.length - 1) {
    attempt.value += 1
    renderKey.value += 1
  }
}
function onLoad() {
  clearTimeout(retryTimer)
  retryTimer = 0
  loaded.value = true
}
onUnmounted(() => clearTimeout(retryTimer))
</script>

<template>
  <img
    :key="renderKey"
    :src="source"
    :alt="alt"
    :class="{ 'is-loaded': loaded }"
    :data-fallback-level="attempt"
    :data-retry-count="retry"
    draggable="false"
    @load="onLoad"
    @error="fallback"
  />
</template>

<style scoped>
img {
  opacity: 0;
  transition: opacity 0.24s var(--ease);
}
img.is-loaded {
  opacity: 1;
}
@media (prefers-reduced-motion: reduce) {
  img { transition: none; }
}
</style>
