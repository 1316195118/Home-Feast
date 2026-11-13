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

export function setThemeMode(mode) {
  const v = MODES.includes(mode) ? mode : 'system'
  try { localStorage.setItem(KEY, v) } catch { /* 隐私模式忽略 */ }
  applyTheme(true)
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
