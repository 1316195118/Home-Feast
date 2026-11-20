<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

// 字段级说明的问号：桌面悬停即显，触摸端点一下展开，点外面收起。
// 只用于「这项填什么/为什么这么设」这类辅助解释；页面级引导仍保留明文。
const props = defineProps({
  text: { type: String, required: true },
  label: { type: String, default: '说明' },
})
const open = ref(false)
const root = ref(null)
let timer = null
// 触屏设备上 tap 会先合成 mouseenter 再触发 click，
// 若两者都绑事件，show() 后紧跟 toggle() 会把浮层关掉——只在支持悬停的设备启用 hover。
const canHover = typeof window !== 'undefined' && window.matchMedia('(hover: hover)').matches

function show() {
  clearTimeout(timer)
  open.value = true
}
function hideSoon() {
  clearTimeout(timer)
  timer = setTimeout(() => { open.value = false }, 120)
}
function toggle(e) {
  e.stopPropagation()
  open.value ? (open.value = false) : show()
}
function onDocClick(e) {
  if (open.value && root.value && !root.value.contains(e.target)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => {
  clearTimeout(timer)
  document.removeEventListener('click', onDocClick)
})
</script>

<template>
  <span ref="root" class="help-wrap" @mouseenter="canHover && show()" @mouseleave="canHover && hideSoon()">
    <button
      class="help-btn" type="button" :aria-label="label" :aria-expanded="open"
      @click="toggle" @focus="canHover && show()" @blur="canHover && hideSoon()"
    >?</button>
    <Transition name="help-fade">
      <span v-if="open" class="help-pop" role="tooltip">{{ text }}</span>
    </Transition>
  </span>
</template>
