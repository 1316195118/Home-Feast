// 冰箱食材专项回归：真实图片、分层数量、开门/抽屉动画、5 档视口和横向溢出。
// 用法：先启动本地服务端，再 node scripts/_ui/pantry-verify.mjs
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:18090'
const OUT = path.resolve('scripts/_ui/shots/pantry')
const SIZES = (process.env.SIZES || '320x568,360x740,390x844,768x1024,1440x900')
  .split(',')
  .map((entry) => {
    const [width, height] = entry.trim().split('x').map(Number)
    if (!width || !height) throw new Error(`无效尺寸：${entry}`)
    return { width, height, mobile: width <= 600 }
  })
fs.mkdirSync(OUT, { recursive: true })

async function createPage(width, height, mobile, index) {
  const port = 9620 + index
  const profile = `C:/Users/Administrator/AppData/Local/Temp/fm-pantry-${Date.now()}-${index}`
  const process = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })
  let targets
  for (let attempt = 0; attempt < 60; attempt++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch {}
    await sleep(250)
  }
  if (!targets?.some((target) => target.type === 'page')) throw new Error('Edge DevTools 未就绪')

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

  const errors = []
  socket.addEventListener('message', (event) => {
    const payload = JSON.parse(event.data)
    if (payload.method === 'Runtime.exceptionThrown') errors.push('exception')
    if (payload.method === 'Runtime.consoleAPICalled' && payload.params?.type === 'error') errors.push('console')
  })
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height,
  })
  return { process, socket, cdp, evaluate, errors }
}

for (const [index, size] of SIZES.entries()) {
  const browser = await createPage(size.width, size.height, size.mobile, index)
  const tag = `${size.width}x${size.height}`
  await browser.cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#menu` })
  await sleep(3000)
  await browser.evaluate(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(450)
  await browser.evaluate(`[...document.querySelectorAll('.btn')].find((button)=>button.textContent.includes('冰箱食材'))?.click()`)
  await sleep(850)
  // 使用临时实例时冰箱可能为空；放入一组已生成照片和回退食材，
  // 同时覆盖生肉托盘、水产托盘、蛋奶层和蔬菜抽屉。
  const samples = ['猪肉', '排骨', '鸡蛋', '虾', '鲈鱼', '番茄']
  const searchAdd = (name) => `(()=>{
    const input=document.querySelector('.pantry-toolbar .input');
    if(!input)return false;
    const setter=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;
    setter.call(input,${JSON.stringify(name)});
    input.dispatchEvent(new Event('input',{bubbles:true}));
    return true;
  })()`
  for (const name of samples) {
    await browser.evaluate(searchAdd(name))
    await sleep(180)
    await browser.evaluate(`(()=>{
      const card=[...document.querySelectorAll('.pantry-card')].find((item)=>item.querySelector('.pantry-card-name')?.textContent.trim()===${JSON.stringify(name)}&&!item.classList.contains('active'));
      card?.click();
    })()`)
    await sleep(150)
  }
  await browser.evaluate(`(()=>{
    const input=document.querySelector('.pantry-toolbar .input');
    if(!input)return;
    const setter=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;
    setter.call(input,''); input.dispatchEvent(new Event('input',{bubbles:true}));
  })()`)
  await sleep(900)

  const before = JSON.parse(await browser.evaluate(`JSON.stringify((()=>{
    const modal=document.querySelector('.pantry-modal');
    const fridge=document.querySelector('.real-fridge');
    const images=[...document.querySelectorAll('.real-fridge img')];
    const left=document.querySelector('.rf-door-left');
    return {
      modal: modal?{w:Math.round(modal.getBoundingClientRect().width),h:Math.round(modal.getBoundingClientRect().height)}:null,
      fridge: fridge?{w:Math.round(fridge.getBoundingClientRect().width),h:Math.round(fridge.getBoundingClientRect().height)}:null,
      zones:document.querySelectorAll('.rf-zone').length,
      foods:document.querySelectorAll('.real-fridge .rf-food').length,
      photos:images.length,
      loaded:images.filter((image)=>image.complete&&image.naturalWidth>0).length,
      broken:images.filter((image)=>image.complete&&image.naturalWidth===0).length,
      fallbacks:document.querySelectorAll('.real-fridge .food3d').length,
      overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth,
      doorTransform:left?getComputedStyle(left).transform:'',
    };
  })())`))

  await browser.evaluate(`document.querySelector('.rf-door-left')?.click()`)
  await sleep(760)
  const doorAfter = await browser.evaluate(`getComputedStyle(document.querySelector('.rf-door-left')).transform`)
  await browser.evaluate(`document.querySelector('.rf-freezer-face')?.click()`)
  await sleep(560)
  const freezerOpen = await browser.evaluate(`document.querySelector('.rf-freezer')?.classList.contains('open')`)

  const screenshot = await browser.cdp('Page.captureScreenshot', {
    format: 'png', captureBeyondViewport: false,
  })
  const shotPath = path.join(OUT, `fridge-${tag}.png`)
  fs.writeFileSync(shotPath, Buffer.from(screenshot.data, 'base64'))

  const threshold = size.width <= 360 ? 280 : size.width <= 600 ? 340 : 420
  const pass = !!before.fridge
    && before.foods > 0
    && before.broken === 0
    && !before.overflow
    && before.fridge.w >= threshold
    && doorAfter !== before.doorTransform
    && freezerOpen === false
    && browser.errors.length === 0

  console.log(JSON.stringify({
    tag,
    pass,
    modal: before.modal,
    fridge: before.fridge,
    foods: before.foods,
    photos: before.photos,
    loaded: before.loaded,
    fallbacks: before.fallbacks,
    broken: before.broken,
    overflow: before.overflow,
    doorChanged: doorAfter !== before.doorTransform,
    freezerClosed: freezerOpen === false,
    jsErrors: browser.errors.length,
    screenshot: shotPath.replaceAll('\\', '/'),
  }))

  browser.socket.close()
  browser.process.kill()
  await sleep(250)
}
