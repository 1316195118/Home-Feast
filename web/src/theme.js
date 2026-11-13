// 深色主题：偏好只存本机 localStorage（不进后端 JSON，也不上传），
// 「跟随系统」时监听 prefers-color-scheme 实时切换。
const KEY = 'fm.theme'
const MODES = ['system', 'light', 'dark']

export const themeChoices = [
  { key: 'system', label: '🖥️ 跟随系统' },
  { key: 'light', label: '☀️ 浅色' },
  { key: 'dark', label: '🌙 深色' },
]

export function currentThemeMode() {
  try {
    const v = localStorage.getItem(KEY)
    return MODES.includes(v) ? v : 'system'
  } catch {
    return 'system'
  }
}

let themeTimer = null

// 从点击处向外扩一圈圆形切换深浅色：用浏览器原生 View Transitions API +
// Web Animations 的 clip-path，没有任何第三方依赖或素材。
// 不支持该 API（Safari 旧版 / Firefox）或开了「减少动效」时，降级为原来的平滑配色过渡。
function circularThemeSwap(anchor, swap) {
  const reduce = !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  if (reduce || typeof document.startViewTransition !== 'function' ||
      !anchor || !Number.isFinite(anchor.x) || !Number.isFinite(anchor.y)) return false
  const { x, y } = anchor
  const radius = Math.hypot(
    Math.max(x, window.innerWidth - x),
    Math.max(y, window.innerHeight - y),
  )
  try {
    const vt = document.startViewTransition(() => swap())
    vt.ready
      .then(() => {
        document.documentElement.animate(
          {
            clipPath: [
              `circle(0px at ${x}px ${y}px)`,
              `circle(${Math.ceil(radius)}px at ${x}px ${y}px)`,
            ],
          },
          { duration: 520, easing: 'ease-in-out', pseudoElement: '::view-transition-new(root)' },
        )
      })
      .catch(() => {}) // 被下一次切换打断时忽略
    return true
  } catch {
    return false
  }
}

export function setThemeMode(mode, anchor) {
  const v = MODES.includes(mode) ? mode : 'system'
  try { localStorage.setItem(KEY, v) } catch { /* 隐私模式忽略 */ }
  // 圆形扩散时不叠加 theme-switching 的配色过渡，避免两套动画打架
  if (!circularThemeSwap(anchor, () => applyTheme(false))) applyTheme(true)
  return v
}

// animate=true 时短暂挂类让配色平滑过渡；首次加载 / 系统主题变化仍即时生效，避免开屏闪烁。
export function applyTheme(animate = false) {
  const html = document.documentElement
  if (animate && !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    html.classList.add('theme-switching')
    clearTimeout(themeTimer)
    themeTimer = setTimeout(() => html.classList.remove('theme-switching'), 380)
  }
  const mode = currentThemeMode()
  const dark = mode === 'dark' ||
    (mode === 'system' && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches)
  if (dark) html.dataset.theme = 'dark'
  else delete html.dataset.theme
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', dark ? '#181410' : '#fbf6ee')
}

export function installTheme() {
  applyTheme()
  const mq = window.matchMedia?.('(prefers-color-scheme: dark)')
  mq?.addEventListener?.('change', () => {
    if (currentThemeMode() === 'system') applyTheme()
  })
}
