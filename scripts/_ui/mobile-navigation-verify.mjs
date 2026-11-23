// 手机慢网下快速连续切页：检查过渡期间主内容是否始终有可见节点，
// 以及页面是否出现横向溢出或运行时报错。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const PORT = 9745
const processHandle = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=C:/Users/Administrator/AppData/Local/Temp/fm-mobile-nav-${Date.now()}`,
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
if (!targets?.some((target) => target.type === 'page')) {
  processHandle.kill()
  throw new Error('无法连接 Edge 调试端口')
}

const socket = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  socket.addEventListener('open', resolve)
  socket.addEventListener('error', reject)
})

let sequence = 0
const runtimeErrors = []
socket.addEventListener('message', (event) => {
  const message = JSON.parse(event.data)
  if (message.method === 'Runtime.exceptionThrown') {
    const details = message.params?.exceptionDetails || {}
    runtimeErrors.push({
      text: details.text || '运行时异常',
      description: details.exception?.description || '',
      url: details.url || '',
      line: details.lineNumber,
      column: details.columnNumber,
    })
  }
})
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

await cdp('Page.navigate', { url: `${BASE}/?mobile-nav=${Date.now()}#menu` })
for (let attempt = 0; attempt < 120; attempt++) {
  if (await evaluate(`!!document.querySelector('.user-pick-item') || !!document.querySelector('.dish-card')`)) break
  await sleep(100)
}
await evaluate(`document.querySelector('.user-pick-item')?.click()`)
for (let attempt = 0; attempt < 80; attempt++) {
  if (await evaluate(`document.querySelectorAll('.tab').length === 5`)) break
  await sleep(100)
}

const result = await evaluate(`(async () => {
  const samples = []
  const tabs = [...document.querySelectorAll('.tab')]
  for (let index = 0; index < 40; index++) {
    const tab = tabs[index % tabs.length]
    tab?.click()
    await new Promise((resolve) => setTimeout(resolve, 45))
    const main = document.querySelector('main')
    const visible = [...(main?.children || [])].some((element) => {
      const rect = element.getBoundingClientRect()
      return rect.width > 0 && rect.height > 0
    })
    samples.push({
      tab: tab?.innerText?.trim() || '',
      visible,
      mainHeight: Math.round(main?.getBoundingClientRect().height || 0),
      loading: !!document.querySelector('.view-loading'),
      error: !!document.querySelector('.view-load-error'),
    })
  }
  return {
    samples,
    overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  }
})()`, true)

const blankSamples = result.samples.filter((item) => !item.visible || item.mainHeight <= 0)
const errorSamples = result.samples.filter((item) => item.error)
const pass = blankSamples.length === 0 && errorSamples.length === 0 &&
  !result.overflow && runtimeErrors.length === 0
console.log(JSON.stringify({
  pass,
  samples: result.samples.length,
  blankSamples: blankSamples.length,
  loadingSamples: result.samples.filter((item) => item.loading).length,
  errorSamples: errorSamples.length,
  overflow: result.overflow,
  runtimeErrors: runtimeErrors.slice(0, 5),
}, null, 2))

socket.close()
processHandle.kill()
if (!pass) process.exit(1)
