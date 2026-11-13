// 轻量即时反馈工具：校验抖动 / 成功弹跳 / 飘字 / 彩带 / 震动。
// 全部是纯 class + CSS 变量 + 浏览器原生 API，不引入任何第三方依赖或素材。
// 特效复用 index.html 预置的 #fx-layer 节点池，运行时**绝不增删 DOM 节点**，
// 只改坐标、CSS 变量和 class，避免与 Vue 渲染产生 insertBefore 竞态。
// 遵循 prefers-reduced-motion 自动降级；震动同时尊重站内音效静音开关。

import { isMuted } from './audio'

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

// 彩带用站内已有的品牌色系，纯色小块 + CSS 图形，不涉及任何图片 / 图标库素材
const CONFETTI_COLORS = ['#c25a3a', '#e0a458', '#6d9b6d', '#7b8fc4', '#d47f9e', '#f2dfc4']

/** 成功时撒一小把彩带：12 个预置色块向外抛洒并旋转下落，1 秒后淡出 */
export function celebrate(x, y, count = 12) {
  const layer = document.getElementById('fx-layer')
  if (!layer || reduceMotion()) return
  const pool = [...layer.querySelectorAll('.confetti')].slice(0, count)
  // 批量移除 → 只强制重排一次 → 批量挂类，避免逐个节点触发 12 次同步布局
  pool.forEach((el) => el.classList.remove('is-on'))
  void layer.offsetWidth
  pool.forEach((el, i) => {
    const angle = (Math.PI * 2 * i) / pool.length + 0.35
    const dist = 72 + (i % 3) * 26
    el.style.left = `${x}px`
    el.style.top = `${y}px`
    el.style.setProperty('--cx', `${Math.round(Math.cos(angle) * dist)}px`)
    el.style.setProperty('--cy', `${Math.round(Math.sin(angle) * dist + 46)}px`)
    el.style.setProperty('--cr', `${(i % 2 ? 1 : -1) * (260 + i * 24)}deg`)
    el.style.setProperty('--cc', CONFETTI_COLORS[i % CONFETTI_COLORS.length])
    el.style.animationDelay = `${i * 20}ms`
    el.classList.add('is-on')
  })
}

/** 手机轻震动：浏览器原生 Vibration API，无依赖；静音或减少动效时不触发 */
export function vibrate(pattern = 15) {
  if (reduceMotion() || isMuted()) return
  try {
    if (typeof navigator.vibrate === 'function') navigator.vibrate(pattern)
  } catch { /* 不支持或被浏览器策略拦截时静默忽略 */ }
}
