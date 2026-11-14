import { createApp } from 'vue'
import App from './App.vue'
import { installRipple } from './ripple'
import { installAudioUnlock } from './audio'
import { installTheme } from './theme'

// 样式按职责分层，后面的文件只做加法：先令牌、再基础、再组件、最后版面。
import './styles/tokens.css'
import './styles/base.css'
import './styles/components.css'
import './styles/screens.css'

installRipple()
installAudioUnlock() // 首次用户手势里解锁 AudioContext，音效才能自动播
installTheme()       // 在挂载前定好主题，避免首屏闪一下白
createApp(App).mount('#app')

// PWA：注册离线缓存；注册失败（非 HTTPS 局域网等）静默降级，不影响正常使用
// 检测到新版本时不再静默替换，而是广播 'sw-update' 事件，由页面弹「更新」提示，
// 用户点一下才真正切换，避免「改完得强刷才看到」的困扰。
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').then((reg) => {
      if (reg.waiting) window.dispatchEvent(new CustomEvent('sw-update'))
      reg.addEventListener('updatefound', () => {
        const installing = reg.installing
        if (installing) installing.addEventListener('statechange', () => {
          if (installing.state === 'installed' && navigator.serviceWorker.controller) {
            window.dispatchEvent(new CustomEvent('sw-update'))
          }
        })
      })
    }).catch(() => {})

    // 用户点「更新」：让等待中的新版本立即接管，随后 controllerchange 触发刷新
    window.addEventListener('sw-reload', () => {
      navigator.serviceWorker.getRegistration().then((reg) => {
        if (reg && reg.waiting) reg.waiting.postMessage({ type: 'SKIP_WAITING' })
      })
    })
    // 新的 service worker 接管后，刷新页面以加载新资源
    navigator.serviceWorker.addEventListener('controllerchange', () => location.reload())
  })
}
