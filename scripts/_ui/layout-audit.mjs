// 双视口布局审计：横向溢出 / 触控目标尺寸 / 文字截断 / 菜系切换位移。
// 用法：先启动服务端，再 node scripts/_ui/layout-audit.mjs
// 可设 BASE=http://10.10.10.251:8090 直接审计线上。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const TABS = ['today', 'menu', 'family', 'history', 'settings']

const AUDIT = `(()=>{
  const de=document.documentElement, vw=de.clientWidth;
  const inScroller=(el)=>{let p=el.parentElement;while(p&&p!==document.body){const s=getComputedStyle(p);if(s.overflowX==='auto'||s.overflowX==='scroll')return true;p=p.parentElement}return false};
  const all=[...document.querySelectorAll('body *')].filter(e=>{const s=getComputedStyle(e);return s.display!=='none'&&s.visibility!=='hidden'&&s.position!=='fixed'});
  const overflow=all.filter(e=>!inScroller(e)&&e.getBoundingClientRect().right>vw+1).slice(0,6).map(e=>e.className||e.tagName);
  const clipped=all.filter(e=>{const s=getComputedStyle(e);return (s.overflow==='hidden'||s.textOverflow==='ellipsis')&&e.scrollWidth>e.clientWidth+2&&e.clientWidth>0}).slice(0,6).map(e=>e.className||e.tagName);
  const small=[...document.querySelectorAll('.btn,.chip,.tab,.icon-btn')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.width<40||r.height<36)}).slice(0,8).map(e=>(e.className||'').trim()+':'+Math.round(e.getBoundingClientRect().width)+'x'+Math.round(e.getBoundingClientRect().height));
  const toast=document.querySelector('.toast');
  return JSON.stringify({
    vw, scrollW:de.scrollWidth, gutter:getComputedStyle(de).scrollbarGutter,
    overflow, clipped, small,
    toast: toast?{w:Math.round(toast.getBoundingClientRect().width),h:Math.round(toast.getBoundingClientRect().height),lines:Math.round(toast.getBoundingClientRect().height/22)}:null,
  })
})()`

async function run(width, height, tag, mobile) {
  const PORT = 9500 + (mobile ? 60 : 0) + Math.floor(Math.random() * 40)
  const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-audit-${tag}-${Date.now()}`
  const proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`, 'about:blank',
  ], { stdio: 'ignore' })
  let targets
  for (let i = 0; i < 60; i++) {
    try { const r = await fetch(`http://127.0.0.1:${PORT}/json/list`); targets = await r.json(); if (targets.some(t => t.type === 'page')) break } catch {}
    await sleep(250)
  }
  const ws = new WebSocket(targets.find(t => t.type === 'page').webSocketDebuggerUrl)
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
  await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height })
  await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
  await sleep(3000)
  await ev(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(450)

  console.log(`\n=== ${tag} ${width}x${height} ===`)
  for (const tab of TABS) {
    await ev(`window.location.hash='#${tab}'`)
    await sleep(650)
    const a = JSON.parse(await ev(AUDIT))
    console.log(`${tab.padEnd(9)} 溢出=${a.overflow.length} 截断=${a.clipped.length} 小按钮=${a.small.length} scrollW=${a.scrollW}/${a.vw} gutter=${a.gutter} toast=${a.toast ? a.toast.w + 'x' + a.toast.h : '无'}`)
    if (a.overflow.length) console.log('   横向溢出:', a.overflow.join(' | '))
    if (a.clipped.length) console.log('   文字截断:', a.clipped.join(' | '))
    if (a.small.length) console.log('   过小控件:', a.small.join(' | '))
  }

  // 菜系切换抖动量化：记录切换前后视口宽度、网格起点、动画类数量与卡片位移
  await ev(`window.location.hash='#menu'`)
  // 等页签进出场动画彻底结束并归零滚动，再取基线，否则会把「切页」的位移误算到「切菜系」头上
  await sleep(1100)
  await ev(`window.scrollTo(0,0)`)
  await sleep(250)
  const before = JSON.parse(await ev(`(()=>{const g=document.querySelector('.dish-grid');return JSON.stringify({w:document.documentElement.clientWidth,top:g?Math.round(g.getBoundingClientRect().top):null,anim:document.querySelectorAll('[class*="grid-enter"],[class*="grid-leave"],[class*="grid-move"]').length})})()`))
  await ev(`document.querySelectorAll('.cuisine-chip')[1]?.click()`)
  await sleep(60)
  const mid = JSON.parse(await ev(`(()=>{const g=document.querySelector('.dish-grid');const c=document.querySelector('.dish-card');return JSON.stringify({w:document.documentElement.clientWidth,top:g?Math.round(g.getBoundingClientRect().top):null,cardTop:c?Math.round(c.getBoundingClientRect().top):null,anim:document.querySelectorAll('[class*="grid-enter"],[class*="grid-leave"],[class*="grid-move"]').length})})()`))
  await sleep(420)
  const after = JSON.parse(await ev(`(()=>{const g=document.querySelector('.dish-grid');const c=document.querySelector('.dish-card');return JSON.stringify({w:document.documentElement.clientWidth,top:g?Math.round(g.getBoundingClientRect().top):null,cardTop:c?Math.round(c.getBoundingClientRect().top):null,anim:document.querySelectorAll('[class*="grid-enter"],[class*="grid-leave"],[class*="grid-move"]').length})})()`))
  console.log(`菜系切换 前=${JSON.stringify(before)} 切换后60ms=${JSON.stringify(mid)} 稳定后=${JSON.stringify(after)}`)
  console.log(`  => 视口宽度变化 ${after.w - before.w}px | 网格起点变化 ${after.top - before.top}px | 卡片位移 ${(mid.cardTop != null && after.cardTop != null) ? after.cardTop - mid.cardTop : 'n/a'}px | 残留动画类 ${after.anim}`)

  ws.close(); proc.kill()
}

await run(1280, 900, 'desktop', false)
await run(390, 844, 'phone', true)
console.log('\nDONE')
