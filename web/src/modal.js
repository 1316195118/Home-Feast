// 弹窗的共用行为：Esc 关闭、打开时锁背景滚动、记住并归还焦点。
// 用「栈」保证嵌套弹窗（例如菜品详情上再叠确认框）时只有最上层响应 Esc。
// 两种用法：
//   1) 独立组件被 v-if 挂载/卸载 —— useModalBehavior(close, rootRef)
//   2) 父组件里的条件弹窗 —— useModalBehavior(close, rootRef, openRef)，随开关入栈/出栈
import { onMounted, onUnmounted, watch, ref, onBeforeUnmount } from 'vue'

const stack = []
let listening = false

function onKey(e) {
  if (e.key !== 'Escape' || !stack.length) return
  e.preventDefault()
  e.stopPropagation()
  stack[stack.length - 1]()
}
function ensureListener() {
  if (listening) return
  window.addEventListener('keydown', onKey, true)
  listening = true
}
function syncScrollLock() {
  document.body.classList.toggle('has-modal', stack.length > 0)
}

function enter(close, rootRef, restoreTo) {
  restoreTo.value = document.activeElement
  stack.push(close)
  ensureListener()
  syncScrollLock()
  requestAnimationFrame(() => focusInto(rootRef))
}
function leave(close, restoreTo) {
  const i = stack.lastIndexOf(close)
  if (i >= 0) stack.splice(i, 1)
  syncScrollLock()
  const el = restoreTo.value
  if (el && document.contains(el)) {
    try { el.focus({ preventScroll: true }) } catch {}
  }
}
function focusInto(rootRef) {
  const root = rootRef?.value
  if (!root) return
  const target = root.querySelector('[autofocus]') || root
  if (!root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1')
  // 不自动聚焦输入框，避免手机端一开弹窗就弹起键盘
  try { target.focus({ preventScroll: true }) } catch {}
}

export function useModalBehavior(close, rootRef = ref(null), openRef = null, dismissible = true) {
  const restoreTo = ref(null)
  // 不可关闭的弹窗（口令锁、首次选人）仍要锁滚动，但不进 Esc 栈
  const stackClose = () => { if (dismissible) close() }
  let active = false

  const setActive = (on) => {
    if (on === active) return
    active = on
    if (on) enter(stackClose, rootRef, restoreTo)
    else leave(stackClose, restoreTo)
  }

  if (openRef) {
    watch(openRef, setActive, { immediate: true, flush: 'post' })
    onBeforeUnmount(() => setActive(false))
  } else {
    onMounted(() => setActive(true))
    onUnmounted(() => setActive(false))
  }
}
