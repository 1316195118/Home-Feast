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

export function setThemeMode(mode) {
  const v = MODES.includes(mode) ? mode : 'system'
  try { localStorage.setItem(KEY, v) } catch { /* 隐私模式忽略 */ }
  applyTheme()
  return v
}

export function applyTheme() {
  const mode = currentThemeMode()
  const dark = mode === 'dark' ||
    (mode === 'system' && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches)
  const root = document.documentElement
  if (dark) root.dataset.theme = 'dark'
  else delete root.dataset.theme
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
