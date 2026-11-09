import { createApp } from 'vue'
import App from './App.vue'
import { installRipple } from './ripple'
import { installAudioUnlock } from './audio'

// 样式按职责分层，后面的文件只做加法：先令牌、再基础、再组件、最后版面。
import './styles/tokens.css'
import './styles/base.css'
import './styles/components.css'
import './styles/screens.css'

installRipple()
installAudioUnlock() // 首次用户手势里解锁 AudioContext，音效才能自动播
createApp(App).mount('#app')
