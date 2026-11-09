// 真实视口截图：桌面 1280 与手机 390，逐页签截图，用于诊断布局。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import fs from 'node:fs'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = 'http://127.0.0.1:8090'
const OUT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_ui/shots'
fs.mkdirSync(OUT, { recursive: true })

const TABS = ['today', 'menu', 'family', 'history', 'settings']

async function run(width, height, tag, mobile) {
  const PORT = 9300 + (mobile ? 10 : 0) + Math.floor(Math.random() * 50)
  const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-ui-${tag}-${Date.now()}`
  const proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`, 'about:blank',
  ], { stdio: 'ignore' })

  let targets
  for (let i = 0; i < 60; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${PORT}/json/list`)
      targets = await r.json()
      if (targets.some((t) => t.type === 'page')) break
    } catch {}
    await sleep(250)
  }
  const ws = new WebSocket(targets.find((t) => t.type === 'page').webSocketDebuggerUrl)
  await new Promise((res, rej) => { ws.addEventListener('open', res); ws.addEventListener('error', rej) })
  let seq = 0
  const cdp = (m, p = {}) => new Promise((r) => {
    const id = ++seq
    const f = (e) => { const j = JSON.parse(e.data); if (j.id !== id) return; ws.removeEventListener('message', f); r(j.result) }
    ws.addEventListener('message', f)
    ws.send(JSON.stringify({ id, method: m, params: p }))
  })
  const ev = async (x) => (await cdp('Runtime.evaluate', { expression: x, returnByValue: true }))?.result?.value

  await cdp('Page.enable')
  // 关闭 reduced-motion，让截图反映真实静态布局（动画不影响布局，但保险）
  await cdp('Emulation.setEmulatedMedia', { media: 'screen', features: [{ name: 'prefers-reduced-motion', value: 'no-preference' }] })
  await cdp('Emulation.setDeviceMetricsOverride', {
    width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height,
  })
  await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#${TABS[0]}` })
  await sleep(3200)
  await ev(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(400)

  for (const tab of TABS) {
    await ev(`window.location.hash = '#${tab}'`)
    await sleep(900)
    const info = await ev(`(()=>{
      const de=document.documentElement;
      return JSON.stringify({
        scrollW: de.scrollWidth, clientW: de.clientWidth,
        bodyScrollH: document.body.scrollHeight,
        cards: document.querySelectorAll('.dish-card,.setting-card,.member-card,.history-card').length,
      })
    })()`)
    const shot = await cdp('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false })
    fs.writeFileSync(`${OUT}/${tag}-${tab}.png`, Buffer.from(shot.data, 'base64'))
    console.log(`${tag}/${tab}`, info)
  }
  ws.close(); proc.kill()
}

await run(1280, 900, 'desktop', false)
await run(390, 844, 'phone', true)
console.log('DONE')
