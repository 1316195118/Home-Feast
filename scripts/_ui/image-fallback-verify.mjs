// 配图容错回归：主动拦截全部 /uploads/ 图片，确认会顺次回退到菜系或家常封面。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:18090'
const PORT = 9741
const processHandle = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=C:/Users/Administrator/AppData/Local/Temp/fm-img-fallback-${Date.now()}`,
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
const cdp = (method, params = {}) => new Promise((resolve) => {
  const id = ++sequence
  const listener = (event) => {
    const payload = JSON.parse(event.data)
    if (payload.id !== id) return
    socket.removeEventListener('message', listener)
    resolve(payload.result)
  }
  socket.addEventListener('message', listener)
  socket.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression) =>
  (await cdp('Runtime.evaluate', { expression, returnByValue: true }))?.result?.value

await cdp('Page.enable')
await cdp('Runtime.enable')
await cdp('Network.enable')
// 同时拦掉批量缩略图、专属图与上传图，确保至少触发一次组件内的真实回退；
// 菜系封面仍可访问，用来确认最终不是破图。
await cdp('Network.setBlockedURLs', { urls: ['*/thumb-bundle*', '*/uploads/*', '*/dish-photo/*'] })
await cdp('Emulation.setDeviceMetricsOverride', {
  width: 390, height: 844, deviceScaleFactor: 1, mobile: true, screenWidth: 390, screenHeight: 844,
})
await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#menu` })
await sleep(3200)
await evaluate(`document.querySelector('.user-pick-item')?.click()`)
// 失败图会先原位重试一次，再逐级降级；给两级回退留足时间，避免把
// 正在重试的图片误报成破图。
await sleep(1800)

const result = JSON.parse(await evaluate(`JSON.stringify((()=>{
  const images=[...document.querySelectorAll('.dish-card img')];
  return {
    total:images.length,
    broken:images.filter((image)=>image.complete&&image.naturalWidth===0).length,
    brokenSrc:images.filter((image)=>image.complete&&image.naturalWidth===0).map((image)=>image.currentSrc||image.src),
    uploads:images.filter((image)=>String(image.currentSrc||image.src).includes('/uploads/')&&image.naturalWidth>0).length,
    cuisineFallbacks:images.filter((image)=>String(image.currentSrc||image.src).includes('/dishes/')&&image.naturalWidth>0).length,
    fallbackLevels:images.map((image)=>Number(image.dataset.fallbackLevel||0)),
    overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth,
  };
})())`))
const pass = result.total > 0 && result.broken === 0 && result.cuisineFallbacks > 0 && !result.overflow
console.log(JSON.stringify({ pass, ...result }))
socket.close()
processHandle.kill()
if (!pass) process.exit(1)
