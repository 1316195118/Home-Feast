import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const API_TARGET = 'http://127.0.0.1:8090'
const apiProxy = {
  target: API_TARGET,
  changeOrigin: true,
  // 浏览器写请求会带 Origin。开发代理改变 Host 后，后端仍应看到同一来源，
  // 否则同源保护会把正常的本地保存/更新请求当成跨站请求拒绝。
  configure(proxy) {
    proxy.on('proxyReq', (proxyReq) => proxyReq.setHeader('Origin', API_TARGET))
  },
}

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsDir: 'assets',
  },
  server: {
    port: 5173,
    proxy: {
      '/api': apiProxy,
      '/uploads': {
        target: API_TARGET,
        changeOrigin: true,
      },
    },
  },
})
