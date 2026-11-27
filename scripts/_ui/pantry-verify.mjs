// 冰箱食材专项回归：真实图片、分层数量、开门/抽屉动画、5 档视口和横向溢出。
// 用法：先启动本地服务端，再 node scripts/_ui/pantry-verify.mjs
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'
import { check, finishAssertions } from './_assert.mjs'

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
    const right=document.querySelector('.rf-door-right');
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
      doorWidths: left&&right ? [
        Math.round(left.getBoundingClientRect().width),
        Math.round(right.getBoundingClientRect().width),
      ] : null,
    };
  })())`))

  await browser.evaluate(`document.querySelector('.rf-door-left .rf-door-toggle')?.click()`)
  await sleep(760)
  const doorAfter = await browser.evaluate(`getComputedStyle(document.querySelector('.rf-door-left')).transform`)
  const doorKeyboard = await browser.evaluate(`(()=>{
    const button=document.querySelector('.rf-door-left .rf-door-toggle');
    button?.focus();
    return { focusable:document.activeElement===button, label:button?.getAttribute('aria-label')||'' };
  })()`)
  await browser.evaluate(`document.querySelector('.rf-door-left .rf-door-toggle')?.click()`)
  await sleep(760)

  const rightBefore = await browser.evaluate(`getComputedStyle(document.querySelector('.rf-door-right')).transform`)
  await browser.evaluate(`document.querySelector('.rf-door-right .rf-door-toggle')?.click()`)
  await sleep(760)
  const rightAfter = await browser.evaluate(`getComputedStyle(document.querySelector('.rf-door-right')).transform`)
  await browser.evaluate(`document.querySelector('.rf-door-right .rf-door-toggle')?.click()`)
  await sleep(760)

  await browser.evaluate(`document.querySelector('.rf-freezer-face')?.click()`)
  await sleep(560)
  const freezerOpen = await browser.evaluate(`document.querySelector('.rf-freezer')?.classList.contains('open')`)

  const screenshot = await browser.cdp('Page.captureScreenshot', {
    format: 'png', captureBeyondViewport: false,
  })
  const shotPath = path.join(OUT, `fridge-${tag}.png`)
  fs.writeFileSync(shotPath, Buffer.from(screenshot.data, 'base64'))

  const magnetResults = []
  for (const [selector, expectedHash] of [
    ['.rf-door-left .rf-magnets button[title="去菜谱点菜"]', '#menu'],
    ['.rf-door-left .rf-magnets button[title="看购物清单"]', '#today'],
    ['.rf-door-right button[title="生成一周菜单"]', '#today'],
  ]) {
    await browser.evaluate(`location.hash='#menu'`)
    await sleep(650)
    await browser.evaluate(`[...document.querySelectorAll('.btn')].find((button)=>button.textContent.includes('冰箱食材'))?.click()`)
    await sleep(650)
    await browser.evaluate(`document.querySelector(${JSON.stringify(selector)})?.click()`)
    await sleep(500)
    magnetResults.push(await browser.evaluate(`({
      closed:!document.querySelector('.pantry-modal'),
      hash:location.hash,
    })`))
  }

  const threshold = size.width <= 360 ? 280 : size.width <= 600 ? 340 : 420
  const pass = !!before.fridge
    && before.foods > 0
    && before.broken === 0
    && !before.overflow
    && before.fridge.w >= threshold
    && doorAfter !== before.doorTransform
    && doorKeyboard.focusable
    && /开左门|关左门/.test(doorKeyboard.label)
    && rightAfter !== rightBefore
    && freezerOpen === false
    && magnetResults[0]?.closed && magnetResults[0]?.hash === '#menu'
    && magnetResults[1]?.closed && magnetResults[1]?.hash === '#today'
    && magnetResults[2]?.closed && magnetResults[2]?.hash === '#today'
    && browser.errors.length === 0

  // 已有完整 pass 判定，但只打印不退出；用 check 让它真正影响退出码。
  check(`${tag} 冰箱专项全部检查`, pass === true)
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
    doorWidths: before.doorWidths,
    doorChanged: doorAfter !== before.doorTransform,
    rightDoorChanged: rightAfter !== rightBefore,
    doorKeyboard,
    freezerClosed: freezerOpen === false,
    magnets: magnetResults,
    jsErrors: browser.errors.length,
    screenshot: shotPath.replaceAll('\\', '/'),
  }))

  browser.socket.close()
  killProcessTree(browser.process)
  await sleep(250)
}
