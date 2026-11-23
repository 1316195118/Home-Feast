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
import './styles/legal.css'

installRipple()
installAudioUnlock() // 首次用户手势里解锁 AudioContext，音效才能自动播
installTheme()       // 在挂载前定好主题，避免首屏闪一下白
createApp(App).mount('#app')

// PWA：正式构建注册离线缓存；注册失败（非 HTTPS 局域网等）静默降级，不影响正常使用。
// 开发模式主动注销旧 SW，否则它会缓存未带 hash 的源码模块，导致 HMR 混用两套 Vue。
// 检测到新版本时不再静默替换，而是广播 'sw-update' 事件，由页面弹「更新」提示，
// 用户点一下才真正切换，避免「改完得强刷才看到」的困扰。
if ('serviceWorker' in navigator) {
  if (!import.meta.env.PROD) {
    navigator.serviceWorker.getRegistrations()
      .then((regs) => Promise.all(regs.map((reg) => reg.unregister())))
      .catch(() => {})
  } else {
    window.addEventListener('load', () => {
      let reloadingForUpdate = false
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
        reloadingForUpdate = true
        navigator.serviceWorker.getRegistration().then((reg) => {
          if (reg && reg.waiting) reg.waiting.postMessage({ type: 'SKIP_WAITING' })
        })
      })
      // 第一次安装时 SW 的 clients.claim() 也会触发 controllerchange；那不代表有
      // 新资源，刷新只会把首屏 bootstrap / SSE 重建一次。只有用户确认升级后的
      // SKIP_WAITING 接管才刷新。
      navigator.serviceWorker.addEventListener('controllerchange', () => {
        if (reloadingForUpdate) location.reload()
      })
    })
  }
}
