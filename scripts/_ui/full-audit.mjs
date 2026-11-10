// 全站 UI 审计：尺寸 / 排序 / 动画 / 反馈 / 输入框 / 文案 / 重复入口，双视口。
// 输出摘要到 stdout，完整数据写 scripts/_ui/audit-report.json 供逐项核对。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import fs from 'node:fs'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'
const OUT = 'C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_ui/audit-report.json'
const TABS = ['today', 'menu', 'family', 'history', 'settings']

const COLLECT = `JSON.stringify((()=>{
  const q=(s)=>[...document.querySelectorAll(s)];
  const txt=(e)=>(e.textContent||'').replace(/\\s+/g,' ').trim();
  const r=(e)=>e.getBoundingClientRect();
  const buttons=q('.btn,.chip,.icon-btn,.tab,.chef-choice,.wash-choice,.pantry-card').map(e=>({
    t:txt(e).slice(0,26), w:Math.round(r(e).width), h:Math.round(r(e).height),
    cls:(e.className||'').toString().slice(0,40),
  })).filter(x=>x.w>0);
  const inputs=q('input,select,textarea').filter(e=>r(e).width>0).map(e=>({
    tag:e.tagName.toLowerCase(), ph:e.getAttribute('placeholder')||'',
    h:Math.round(r(e).height), fs:getComputedStyle(e).fontSize,
    pad:getComputedStyle(e).padding, type:e.getAttribute('type')||'',
  }));
  const titles=q('[title]').map(e=>e.getAttribute('title').trim()).filter(Boolean);
  const placeholders=q('[placeholder]').map(e=>e.getAttribute('placeholder').trim()).filter(Boolean);
  const anim=q('*').filter(e=>{const s=getComputedStyle(e);return s.transitionDuration!=='0s'||s.animationName!=='none'});
  const durations=[...new Set(anim.map(e=>getComputedStyle(e).transitionDuration).filter(x=>x!=='0s'))];
  const cards=q('.card,.panel,.stat-item').map(e=>Math.round(r(e).width));
  const chipOrder=q('.cuisine-chip').map(e=>txt(e).slice(0,12));
  const toolOrder=q('.toolbar .chip').map(e=>txt(e));
  const tabs=q('.tab').map(e=>txt(e));
  const de=document.documentElement;
  return {
    buttons, inputs, titles, placeholders,
    animCount:anim.length, durations,
    cardWidths:[...new Set(cards)], chipOrder, toolOrder, tabs,
    smallTargets:buttons.filter(x=>x.w<44||x.h<36).map(x=>x.t+' '+x.w+'x'+x.h),
    overflow:de.scrollWidth>de.clientWidth,
    toast:txt(document.querySelector('.toast')||document.createElement('span')).slice(0,60),
  };
})())`

async function run(width, height, tag, mobile) {
  const PORT = 9100 + (mobile ? 40 : 0) + Math.floor(Math.random() * 30)
  const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-full-${tag}-${Date.now()}`
  const proc = spawn('C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${PROFILE}`, 'about:blank',
  ], { stdio: 'ignore' })
  let targets
  for (let i = 0; i < 60; i++) {
    try { const r = await fetch(`http://127.0.0.1:${PORT}/json/list`); targets = await r.json(); if (targets.some((t) => t.type === 'page')) break } catch {}
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
  await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height })
  await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
  await sleep(3200)
  await ev(`document.querySelector('.user-pick-item')?.click()`)
  await sleep(450)
  const report = { [tag]: {} }
  for (const tab of TABS) {
    await ev(`window.location.hash='#${tab}'`)
    await sleep(650)
    report[tag][tab] = JSON.parse(await ev(COLLECT))
    const d = report[tag][tab]
    console.log(`${tag}/${tab.padEnd(9)} 按钮${String(d.buttons.length).padStart(3)} 小目标${String(d.smallTargets.length).padStart(2)} 输入框${String(d.inputs.length).padStart(2)} 动画元素${String(d.animCount).padStart(3)} 时长${d.durations.length}种 卡宽${d.cardWidths.length}种 溢出${d.overflow}`)
  }
  ws.close(); proc.kill()
  return report[tag]
}

const all = { desktop: await run(1280, 900, 'desktop', false), phone: await run(390, 844, 'phone', true) }
// 汇总重复 / 过小 / 文案
const uniq = (a) => [...new Set(a)]
const allBtns = uniq(Object.values(all).flatMap((v) => Object.values(v).flatMap((d) => d.buttons.map((b) => b.t))))
const allSmall = uniq(Object.values(all).flatMap((v) => Object.values(v).flatMap((d) => d.smallTargets)))
const allPh = uniq(Object.values(all).flatMap((v) => Object.values(v).flatMap((d) => d.placeholders)))
const allDurations = uniq(Object.values(all).flatMap((v) => Object.values(v).flatMap((d) => d.durations)))
const inputSizes = uniq(Object.values(all).flatMap((v) => Object.values(v).flatMap((d) => d.inputs.map((i) => `${i.tag}:${i.h}px/${i.fs}`))))
console.log('\n=== 汇总 ===')
console.log('按钮文案:', JSON.stringify(allBtns))
console.log('过小目标:', JSON.stringify(allSmall))
console.log('placeholder:', JSON.stringify(allPh))
console.log('输入框尺寸/字号:', JSON.stringify(inputSizes))
console.log('动画时长种类:', JSON.stringify(allDurations))
console.log('菜谱工具条顺序:', JSON.stringify(all.desktop.menu.toolOrder))
console.log('菜系顺序:', JSON.stringify(all.desktop.menu.chipOrder))
fs.writeFileSync(OUT, JSON.stringify(all, null, 1))
console.log('完整报告:', OUT)
