// 移动网络首屏回归：4 Mbps + 80ms 延迟下检查缩略图、首屏耗时和白屏/破图。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const PORT = 9743
const processHandle = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=C:/Users/Administrator/AppData/Local/Temp/fm-load-perf-${Date.now()}`,
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
await cdp('Page.addScriptToEvaluateOnNewDocument', {
  source: `
    window.__fmLcp = 0;
    try {
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) window.__fmLcp = entry.startTime;
      }).observe({ type: 'largest-contentful-paint', buffered: true });
    } catch {}
  `,
})
await cdp('Emulation.setDeviceMetricsOverride', {
  width: 390, height: 844, deviceScaleFactor: 2, mobile: true, screenWidth: 390, screenHeight: 844,
})
await cdp('Network.emulateNetworkConditions', {
  offline: false,
  latency: 80,
  downloadThroughput: 500_000,
  uploadThroughput: 1_000_000,
  connectionType: 'cellular4g',
})

const started = Date.now()
await cdp('Page.navigate', { url: `${BASE}/?perf=${started}#menu` })
for (let attempt = 0; attempt < 80; attempt++) {
  if (await evaluate(`document.querySelectorAll('.dish-card').length > 0`)) break
  await sleep(100)
}
await sleep(2500)

const metrics = await evaluate(`(()=>{
  const nav = performance.getEntriesByType('navigation')[0] || {};
  const paints = Object.fromEntries(performance.getEntriesByType('paint').map((entry) => [entry.name, Math.round(entry.startTime)]));
  const resources = performance.getEntriesByType('resource');
  const images = [...document.querySelectorAll('.dish-card img')];
  const imageResources = resources.filter((entry) => entry.initiatorType === 'img');
  // 列表图片本来就按视口懒加载，不能要求首屏两秒内把屏外 48 张全部下载完。
  // 这里检查「首屏附近应有图」是否都已显示，同时保留懒加载数量，防止优化后反而改成全量抢带宽。
  const nearViewport = images.filter((image) => {
    const rect = image.getBoundingClientRect();
    return rect.bottom > -500 && rect.top < innerHeight + 500;
  });
  const loadedNearViewport = nearViewport.filter((image) => image.complete && image.naturalWidth > 0);
  return {
    readyMs: ${Date.now()} - ${started},
    domMs: Math.round(nav.domContentLoadedEventEnd || 0),
    loadMs: Math.round(nav.loadEventEnd || 0),
    lcpMs: Math.round(window.__fmLcp || 0),
    paints,
    cards: document.querySelectorAll('.dish-card').length,
    images: images.length,
    loaded: images.filter((image) => image.complete && image.naturalWidth > 0).length,
    broken: images.filter((image) => image.complete && image.naturalWidth === 0).length,
    nearViewport: nearViewport.length,
    loadedNearViewport: loadedNearViewport.length,
    lazy: images.filter((image) => image.loading === 'lazy').length,
    optimized: images.filter((image) => /\\/(?:thumb\\/|dish-photo\\/)/.test(String(image.currentSrc || image.src))).length,
    imageKb: Math.round(imageResources.reduce((sum, entry) => sum + (entry.transferSize || 0), 0) / 1024),
    resourceKb: Math.round(resources.reduce((sum, entry) => sum + (entry.transferSize || 0), 0) / 1024),
    overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  };
})()`)

const pass = metrics.cards > 0 &&
  metrics.broken === 0 &&
  metrics.nearViewport > 0 &&
  metrics.loadedNearViewport === metrics.nearViewport &&
  metrics.lazy > 0 &&
  metrics.lcpMs > 0 &&
  metrics.lcpMs < 4000 &&
  !metrics.overflow
console.log(JSON.stringify({ pass, ...metrics }))
socket.close()
processHandle.kill()
if (!pass) process.exit(1)
