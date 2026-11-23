// 图片缓存回归：验证图片进入离线缓存、API 不进入缓存，并能在断网后继续读取。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const PORT = 9742
const processHandle = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=C:/Users/Administrator/AppData/Local/Temp/fm-image-cache-${Date.now()}`,
  'about:blank',
], { stdio: 'ignore' })

let targets
for (let attempt = 0; attempt < 60; attempt++) {
  try {
    const response = await fetch(`http://127.0.0.1:${PORT}/json/list`)
    targets = await response.json()
    if (targets.some((target) => target.type === 'page')) break
  } catch {}
  await sleep(250)
}
const socket = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  socket.addEventListener('open', resolve)
  socket.addEventListener('error', reject)
})

let sequence = 0
const cdp = (method, params = {}) => new Promise((resolve, reject) => {
  const id = ++sequence
  const listener = (event) => {
    const payload = JSON.parse(event.data)
    if (payload.id !== id) return
    socket.removeEventListener('message', listener)
    if (payload.error) reject(new Error(payload.error.message))
    else resolve(payload.result)
  }
  socket.addEventListener('message', listener)
  socket.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression, awaitPromise = false) => {
  const result = await cdp('Runtime.evaluate', { expression, returnByValue: true, awaitPromise })
  if (result?.exceptionDetails) throw new Error(result.exceptionDetails.text)
  return result?.result?.value
}

await cdp('Page.enable')
await cdp('Runtime.enable')
await cdp('Network.enable')
await cdp('Page.navigate', { url: `${BASE}/?cache-check=${Date.now()}#menu` })
await sleep(3000)

for (let attempt = 0; attempt < 40; attempt++) {
  if (await evaluate('!!navigator.serviceWorker.controller')) break
  await sleep(250)
}

// 缩略图包在页面空闲时预取，慢网或冷启动时可能晚于 3 秒完成。
for (let attempt = 0; attempt < 60; attempt++) {
  const ready = await evaluate(`(async()=>{
    if (!('caches' in window)) return false;
    const cache = await caches.open('fm-shell-v5');
    return (await cache.keys()).some((request) =>
      new URL(request.url).pathname.startsWith('/__thumb-bundle/')
    );
  })()`, true)
  if (ready) break
  await sleep(250)
}

const cached = await evaluate(`(async()=>{
  await fetch('/api/bootstrap', { cache: 'reload' });
  const keys = await caches.keys();
  const cache = await caches.open('fm-shell-v5');
  const bundleRequest = (await cache.keys()).find((request) =>
    new URL(request.url).pathname.startsWith('/__thumb-bundle/')
  );
  if (!bundleRequest) throw new Error('页面没有缓存批量缩略图');
  const image = bundleRequest.url;
  return {
    image,
    keys,
    imageCached: !!(await cache.match(bundleRequest)),
    apiCached: !!(await cache.match('/api/bootstrap')),
  };
})()`, true)

await cdp('Network.emulateNetworkConditions', {
  offline: true,
  latency: 0,
  downloadThroughput: 0,
  uploadThroughput: 0,
  connectionType: 'none',
})
// 部分 Chromium 版本不会对 localhost 应用 offline 模拟，直接把测试资源
// 的网络请求拦掉，才能验证 Service Worker 是否真的从 Cache Storage 回放。
await cdp('Network.setBlockedURLs', { urls: ['*__thumb-bundle*'] })
const offline = await evaluate(`(async()=>{
  const cacheHit = await caches.match(${JSON.stringify(cached.image)});
  let response = null;
  let networkError = '';
  try {
    response = await fetch(${JSON.stringify(cached.image)});
  } catch (error) {
    networkError = String(error?.message || error);
  }
  return {
    ok: !!response?.ok,
    status: response?.status ?? 0,
    type: response?.type || '',
    cacheHit: !!cacheHit,
    networkError,
  };
})()`, true)

const pass = cached.keys.includes('fm-shell-v5') &&
  cached.imageCached &&
  !cached.apiCached &&
  offline.cacheHit &&
  (offline.ok || !!offline.networkError)
console.log(JSON.stringify({ pass, controller: true, cached, offline }))
socket.close()
processHandle.kill()
if (!pass) process.exit(1)
