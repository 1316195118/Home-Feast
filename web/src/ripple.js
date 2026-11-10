// 全局点击反馈：用事件委托挂在 document 上，不需要给每个按钮加指令。
// 一次点击同时给四件事：轻点击音、涟漪、点击点扩散的「指向环」、目标短暂高亮，
// 让用户清楚知道自己点到了哪里。只对浅色可点击控件生效；用独立覆盖层裁剪，
// 避免给宿主加 overflow:hidden 而裁掉 .tab::before 这类移到边界外的装饰。
import { playTap } from './audio'
const SELECTOR = [
  '.btn', '.chip', '.icon-btn', '.tab', '.duty-claim',
  '.user-pick-item', '.dish-recipe', '.dish-fav', '.suggest-recipe',
  '.avatar-picker button', '.swatch', '.cuisine-toggle',
].join(',')

function hostFor(el) {
  if (!(el instanceof Element)) return null
  const host = el.closest(SELECTOR)
  if (!host || host.disabled || host.getAttribute('aria-disabled') === 'true') return null
  return host
}

function onPointerDown(e) {
  if (e.button != null && e.button !== 0) return
  const host = hostFor(e.target)
  if (!host) return
  playTap() // 极轻的一声；静音开关 / 未解锁时自动静默
  const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  if (reduce) return // 减少动效偏好下只保留声音，不做视觉扩散

  const rect = host.getBoundingClientRect()
  if (!rect.width || !rect.height) return
  const cs = getComputedStyle(host)
  if (cs.position === 'static') host.style.position = 'relative'

  const layer = document.createElement('span')
  layer.className = 'ripple-layer'
  const ink = document.createElement('span')
  ink.className = 'ripple-ink'
  const size = Math.max(rect.width, rect.height) * 1.8
  ink.style.width = ink.style.height = `${size}px`
  ink.style.left = `${e.clientX - rect.left - size / 2}px`
  ink.style.top = `${e.clientY - rect.top - size / 2}px`
  // 指向环：从点击的确切位置向外扩一圈，像给目标「画了个圈」
  const pointer = document.createElement('span')
  pointer.className = 'ripple-pointer'
  pointer.style.left = `${e.clientX - rect.left}px`
  pointer.style.top = `${e.clientY - rect.top}px`
  layer.appendChild(ink)
  layer.appendChild(pointer)
  host.appendChild(layer)
  host.classList.add('is-tapped')
  setTimeout(() => host.classList.remove('is-tapped'), 260)
  const kill = () => layer.remove()
  ink.addEventListener('animationend', kill, { once: true })
  setTimeout(kill, 700) // 动画被中断时的兜底
}

export function installRipple() {
  document.addEventListener('pointerdown', onPointerDown, true)
  return () => document.removeEventListener('pointerdown', onPointerDown, true)
}
