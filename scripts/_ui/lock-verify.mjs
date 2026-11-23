// 访问口令启动回归：首屏 bootstrap 返回 401 时必须直接进入锁屏，而不是报「断网」。
// 用法：CODE=测试口令 BASE=http://127.0.0.1:18093 node scripts/_ui/lock-verify.mjs
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const CODE = process.env.CODE || ''
if (!CODE) throw new Error('请设置 CODE')
const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const PORT = 9740 + Math.floor(Math.random() * 80)
const proc = spawn(EDGE, [
  '--headless=new', '--disable-gpu', '--no-sandbox',
  `--remote-debugging-port=${PORT}`,
  `--user-data-dir=${process.env.TEMP || '.'}/fm-lock-${Date.now()}`, 'about:blank',
], { stdio: 'ignore' })

let targets
for (let i = 0; i < 80; i++) {
  try {
    targets = await fetch(`http://127.0.0.1:${PORT}/json/list`).then((response) => response.json())
    if (targets.some((target) => target.type === 'page')) break
  } catch {}
  await sleep(200)
}
const ws = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  ws.addEventListener('open', resolve)
  ws.addEventListener('error', reject)
})
let seq = 0
const pending = new Map()
const events = []
ws.addEventListener('message', (event) => {
  const message = JSON.parse(event.data)
  if (message.id && pending.has(message.id)) {
    pending.get(message.id)(message)
    pending.delete(message.id)
  } else if (message.method) {
    events.push(message)
  }
})
const cdp = (method, params = {}) => new Promise((resolve, reject) => {
  const id = ++seq
  pending.set(id, (message) => message.error
    ? reject(new Error(message.error.message))
    : resolve(message.result))
  ws.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression) => {
  const result = await cdp('Runtime.evaluate', { expression, returnByValue: true })
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text)
  return result.result?.value
}
const waitFor = async (expression, timeout = 8000) => {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    if (await evaluate(expression)) return true
    await sleep(100)
  }
  return false
}

try {
  await cdp('Page.enable')
  await cdp('Runtime.enable')
  await cdp('Network.enable')
  await cdp('Emulation.setDeviceMetricsOverride', {
    width: 390, height: 844, deviceScaleFactor: 2, mobile: true,
    screenWidth: 390, screenHeight: 844,
  })
  await cdp('Page.navigate', { url: `${BASE}/?lock=${Date.now()}#today` })
  const locked = await waitFor(`!!document.querySelector('.modal h2')?.textContent.includes('访问口令')`, 10000)
  console.log(`${locked ? '  ✓' : '  ✗'} 未带口令时直接显示访问口令锁屏`)
  console.log(`${await evaluate(`!document.body.textContent.includes('连不上服务端')`) ? '  ✓' : '  ✗'} 没有误报断网`)

  await evaluate(`(()=>{
    const input=document.querySelector('.modal input[type=password]');
    const setter=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;
    setter.call(input,${JSON.stringify(CODE)});
    input.dispatchEvent(new Event('input',{bubbles:true}));
    [...document.querySelectorAll('.modal button')].find((button)=>button.textContent.includes('进入')).click();
  })()`)
  const unlocked = await waitFor(`!!document.querySelector('.tabs') && ![...document.querySelectorAll('.modal h2')].some((node)=>node.textContent.includes('访问口令'))`, 10000)
  console.log(`${unlocked ? '  ✓' : '  ✗'} 正确口令可进入应用`)
  const errors = events.filter((event) =>
    event.method === 'Runtime.exceptionThrown'
    || (event.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(event.params.type)))
  console.log(`${errors.length === 0 ? '  ✓' : '  ✗'} 锁屏到解锁无浏览器 error / warn（${errors.length} 条）`)
  if (!locked || !unlocked || errors.length) process.exitCode = 1
} finally {
  ws.close()
  killProcessTree(proc)
}
