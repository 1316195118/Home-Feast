// 轻量即时反馈工具：校验抖动 / 成功弹跳 / 点击飘字。
// 纯 class + CSS 变量 + 现有 CSS 动画，不引入依赖；遵循 prefers-reduced-motion 自动降级。
// 飘字复用 index.html 里预置的 #fx-layer 节点池：运行时**绝不增删 DOM 节点**，
// 只改坐标、CSS 变量和 class，避免与 Vue 渲染产生 insertBefore 竞态。

const reduceMotion = () => !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

function resolve(target) {
  return typeof target === 'string' ? document.querySelector(target) : target
}

// 先移除再强制重排，保证连续点击也能每次重播动画
function replay(el, cls, ms, removeAfter = true) {
  if (!el) return
  el.classList.remove(cls)
  void el.offsetWidth
  el.classList.add(cls)
  if (removeAfter) setTimeout(() => el.classList.remove(cls), ms)
}

/** 校验失败：抖一下 + 红色边框（.is-invalid） */
export function shake(target, ms = 460) {
  if (reduceMotion()) return
  replay(resolve(target), 'is-invalid', ms)
}

/** 保存成功：轻轻弹一下（.feedback-success） */
export function bounce(target, ms = 560) {
  if (reduceMotion()) return
  replay(resolve(target), 'feedback-success', ms)
}

/** 从某个坐标飘出几个小图标（复用预置节点池，不插入/删除 DOM） */
export function sparkle(x, y, emoji = '✨', count = 3) {
  const layer = document.getElementById('fx-layer')
  if (!layer || reduceMotion()) return
  const pool = layer.querySelectorAll('.sparkle')
  for (let i = 0; i < count && i < pool.length; i++) {
    const el = pool[i]
    el.style.left = `${x + (i - (count - 1) / 2) * 20}px`
    el.style.top = `${y}px`
    el.style.setProperty('--fx-emoji', JSON.stringify(emoji))
    el.style.animationDelay = `${i * 70}ms`
    replay(el, 'is-on', 0, false)
  }
}

/** 从点击的元素中心飘出 */
export function sparkleFromEvent(event, emoji) {
  const rect = event?.currentTarget?.getBoundingClientRect?.()
  if (rect) sparkle(rect.left + rect.width / 2, rect.top + rect.height / 2, emoji)
}
