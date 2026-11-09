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
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {})
  })
}
