// 弹窗的共用行为：Esc 关闭、打开时锁背景滚动、焦点陷阱、记住并归还焦点。
// 用「栈」保证嵌套弹窗（例如菜品详情上再叠确认框）时只有最上层响应 Esc / Tab。
// 两种用法：
//   1) 独立组件被 v-if 挂载/卸载 —— useModalBehavior(close, rootRef)
//   2) 父组件里的条件弹窗 —— useModalBehavior(close, rootRef, openRef)，随开关入栈/出栈
import { onMounted, onUnmounted, watch, ref, onBeforeUnmount } from 'vue'

const stack = []
let listening = false

// 可聚焦元素：排除 disabled / 隐藏 / tabindex=-1，避免把焦点送进看不见的控件
function focusableIn(root) {
  if (!root) return []
  const sel = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  return [...root.querySelectorAll(sel)].filter((el) => {
    if (el.hasAttribute('disabled') || el.getAttribute('aria-hidden') === 'true') return false
    return el.offsetWidth > 0 || el.offsetHeight > 0 || el === document.activeElement
  })
}

function onKey(e) {
  if (!stack.length) return
  const top = stack[stack.length - 1]
  if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    top.close()
    return
  }
  if (e.key !== 'Tab') return
  // 焦点陷阱：Tab / Shift+Tab 在最上层弹窗内循环，不再跑到背景页面
  const root = top.rootRef?.value
  if (!root) return
  const items = focusableIn(root)
  if (!items.length) {
    e.preventDefault()
    try { root.focus() } catch { /* 忽略 */ }
    return
  }
  const first = items[0]
  const last = items[items.length - 1]
  const active = document.activeElement
  if (e.shiftKey) {
    // 焦点在容器本身（刚打开时）或首个元素：Shift+Tab 回到最后一个
    if (active === first || active === root || !root.contains(active)) {
      e.preventDefault()
      last.focus()
    }
  } else if (active === last || !root.contains(active)) {
    e.preventDefault()
    first.focus()
  }
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
  stack.push({ close, rootRef })
  ensureListener()
  syncScrollLock()
  requestAnimationFrame(() => focusInto(rootRef))
}
function leave(close, restoreTo) {
  const i = stack.findIndex((s) => s.close === close)
  if (i >= 0) stack.splice(i, 1)
  syncScrollLock()
  const el = restoreTo.value
  if (el && document.contains(el)) {
    try { el.focus({ preventScroll: true }) } catch { /* 忽略 */ }
  }
}
function focusInto(rootRef) {
  const root = rootRef?.value
  if (!root) return
  const target = root.querySelector('[autofocus]') || root
  if (!root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1')
  // 不自动聚焦输入框，避免手机端一开弹窗就弹起键盘；Tab 会从容器进入首个控件
  try { target.focus({ preventScroll: true }) } catch { /* 忽略 */ }
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
