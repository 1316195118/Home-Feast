import fs from 'node:fs/promises'
import path from 'node:path'
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = (process.env.BASE || 'http://127.0.0.1:18092').replace(/\/+$/, '')
const OUT = process.env.OUT || 'C:/Users/Administrator/AppData/Local/Temp/family-menu-visual'
const FILTER = String(process.env.FILTER || '').trim()

async function capture(width, height, tag, mobile) {
  const port = 9700 + Math.floor(Math.random() * 100)
  const profile = path.join(process.env.TEMP || '.', `fm-shot-${tag}-${Date.now()}`)
  const proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })
  let targets
  for (let i = 0; i < 80; i++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch {}
    await sleep(200)
  }
  if (!targets) throw new Error('Edge debug port unavailable')
  const ws = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve)
    ws.addEventListener('error', reject)
  })
  let seq = 0
  const pending = new Map()
  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data)
    if (message.id && pending.has(message.id)) {
      pending.get(message.id)(message.result)
      pending.delete(message.id)
    }
  })
  const cdp = (method, params = {}) => new Promise((resolve) => {
    const id = ++seq
    pending.set(id, resolve)
    ws.send(JSON.stringify({ id, method, params }))
  })
  const evaluate = async (expression) => {
    const value = await cdp('Runtime.evaluate', { expression, returnByValue: true })
    return value?.result?.value
  }
  await cdp('Page.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width, height, deviceScaleFactor: mobile ? 2 : 1, mobile,
    screenWidth: width, screenHeight: height,
  })
  await cdp('Page.navigate', { url: `${BASE}/?shot=${Date.now()}#today` })
  await sleep(3200)
  await evaluate(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(3000)
  for (const tab of ['today', 'menu']) {
    await evaluate(`window.location.hash='#${tab}'`)
    await sleep(tab === 'menu' ? 1300 : 500)
    await evaluate('window.scrollTo(0,0)')
    await sleep(250)
    const screenshot = await cdp('Page.captureScreenshot', {
      format: 'png', fromSurface: true, captureBeyondViewport: false,
    })
    const file = path.join(OUT, `${tag}-${tab}.png`)
    await fs.mkdir(OUT, { recursive: true })
    await fs.writeFile(file, Buffer.from(screenshot.data, 'base64'))
    console.log(file)
    if (tab === 'menu') {
      console.log(await evaluate(`JSON.stringify([...document.querySelectorAll('.dish-photo img')].slice(0,12).map((img)=>({alt:img.alt,src:img.getAttribute('src'),complete:img.complete,w:img.naturalWidth,h:img.naturalHeight})))`))
      if (FILTER) {
        await evaluate(`(() => {
          const input = document.querySelector('.search-box input')
          if (!input) return false
          const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set
          setter.call(input, ${JSON.stringify(FILTER)})
          input.dispatchEvent(new Event('input', { bubbles: true }))
          input.focus()
          return true
        })()`)
        await sleep(1400)
        const filtered = await cdp('Page.captureScreenshot', {
          format: 'png', fromSurface: true, captureBeyondViewport: false,
        })
        const filteredFile = path.join(OUT, `${tag}-menu-filtered.png`)
        await fs.writeFile(filteredFile, Buffer.from(filtered.data, 'base64'))
        console.log(filteredFile)
        console.log(await evaluate(`JSON.stringify([...document.querySelectorAll('.dish-photo img')].slice(0,12).map((img)=>({alt:img.alt,src:img.getAttribute('src'),complete:img.complete,w:img.naturalWidth,h:img.naturalHeight})))`))
      }
    }
  }
  ws.close()
  killProcessTree(proc)
}

await capture(390, 844, 'phone', true)
await capture(1440, 900, 'desktop', false)
