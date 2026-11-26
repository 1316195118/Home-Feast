// 菜谱卡片文字重叠回归：检查标题、菜系、时间、辣度、标签和按钮的实际边界。
// 用法：启动服务端后 node scripts/_ui/overlap-audit.mjs
import { spawn } from 'node:child_process'
import path from 'node:path'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const SIZES = (process.env.SIZES || '320x568,375x812,390x844,768x1024,1280x900')
  .split(',')
  .map((entry) => {
    const [width, height] = entry.trim().split('x').map(Number)
    return { width, height, mobile: width <= 600 }
  })

const SCAN = (rootSelector) => `JSON.stringify((()=>{
  const root=document.querySelector(${JSON.stringify(rootSelector)})||document.body;
  const round=(n)=>Math.round(n*10)/10;
  const rect=(el)=>{const r=el.getBoundingClientRect();return {l:r.left,t:r.top,r:r.right,b:r.bottom,w:r.width,h:r.height}};
  const visible=(r)=>r.w>0&&r.h>0;
  const intersects=(a,b)=>Math.min(a.r,b.r)-Math.max(a.l,b.l)>1&&Math.min(a.b,b.b)-Math.max(a.t,b.t)>1;
  const label=(el)=>(el.textContent||'').replace(/\\s+/g,' ').trim().slice(0,30);
  const failures=[];
  for(const [cardIndex,card] of [...root.querySelectorAll('.dish-card,.detail-info')].entries()){
    const title=card.querySelector('.dish-title');
    const titleName=card.querySelector('.dish-name-link');
    const cuisine=title?.querySelector('.tiny');
    const desc=card.querySelector('.dish-desc');
    const meta=[...card.querySelectorAll('.dish-meta > span')];
    const tags=[...card.querySelectorAll('.dish-tags > .tag')];
    const foot=[...card.querySelectorAll('.dish-foot > button')];
    const sectionRoots = card.matches('.detail-info')
      ? ['h2','.detail-desc','.detail-meta','.dish-tags','.detail-actions','.detail-rating']
      : ['.dish-title','.dish-desc','.dish-tags','.dish-meta','.dish-foot'];
    const groups=[
      ['title', [titleName,cuisine].filter(Boolean)],
      ['meta', meta],
      ['tags', tags],
      ['foot', foot],
      ['sections', sectionRoots.map((selector)=>card.querySelector(selector)).filter(Boolean)],
    ];
    for(const [kind,els] of groups){
      for(let i=0;i<els.length;i++)for(let j=i+1;j<els.length;j++){
        const a=rect(els[i]),b=rect(els[j]);
        if(!visible(a)||!visible(b)) continue;
        if(intersects(a,b)) failures.push({
          cardIndex,kind,
          a:label(els[i]),b:label(els[j]),
          ar:[round(a.l),round(a.t),round(a.r),round(a.b)],
          br:[round(b.l),round(b.t),round(b.r),round(b.b)],
          card:label(card.querySelector('.dish-name-link')),
        });
      }
    }
    const cardRect=rect(card);
    for(const el of [titleName,cuisine,desc,...meta,...tags,...foot].filter(Boolean)){
      const r=rect(el);
      if(visible(r)&&(r.l<cardRect.l-1||r.r>cardRect.r+1)) failures.push({
        cardIndex,kind:'horizontal-overflow',a:label(el),b:'card',
        ar:[round(r.l),round(r.t),round(r.r),round(r.b)],
        br:[round(cardRect.l),round(cardRect.t),round(cardRect.r),round(cardRect.b)],
        card:label(card.querySelector('.dish-name-link')),
      });
    }
  }
  const textRects=[];
  const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);
  while(walker.nextNode()){
    const node=walker.currentNode;
    const text=(node.textContent||'').replace(/\\s+/g,' ').trim();
    if(!text) continue;
    const parent=node.parentElement;
    if(!parent||parent.closest('script,style,svg')) continue;
    const style=getComputedStyle(parent);
    if(style.display==='none'||style.visibility==='hidden'||Number(style.opacity)===0) continue;
    const range=document.createRange();
    range.selectNodeContents(node);
    for(const r of range.getClientRects()){
      const box={l:r.left,t:r.top,r:r.right,b:r.bottom,w:r.width,h:r.height};
      if(visible(box)) textRects.push({...box,text:text.slice(0,36),el:parent});
    }
  }
  for(let i=0;i<textRects.length;i++)for(let j=i+1;j<textRects.length;j++){
    const a=textRects[i],b=textRects[j];
    if(a.el===b.el||a.el.contains(b.el)||b.el.contains(a.el)) continue;
    if(intersects(a,b)) failures.push({
      cardIndex:-1,kind:'text-overlap',a:a.text,b:b.text,
      ar:[round(a.l),round(a.t),round(a.r),round(a.b)],
      br:[round(b.l),round(b.t),round(b.r),round(b.b)],
      card:'text',
    });
  }
  return {
    cards:root.querySelectorAll('.dish-card').length,
    failures:failures.slice(0,40),
    filterRects:[...root.querySelectorAll('.menu-filter-rail > *, .menu-filter-rail .quick-chip')]
      .map((el)=>({text:label(el),...rect(el),display:getComputedStyle(el).display,gap:getComputedStyle(el).gap})),
    titleWidths:root.querySelector('.dish-title')?{
      client:root.querySelector('.dish-title').clientWidth,
      scroll:root.querySelector('.dish-title').scrollWidth,
    }:null,
  };
})())`

async function audit({ width, height, mobile }, index) {
  const port = 9900 + index
  const profile = path.join(process.env.TEMP || '.', `fm-overlap-${Date.now()}-${index}`)
  const proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })

  let targets
  for (let i = 0; i < 70; i++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch {}
    await sleep(200)
  }
  if (!targets) throw new Error('Edge 调试端口未就绪')

  const ws = new WebSocket(targets.find((target) => target.type === 'page').webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve)
    ws.addEventListener('error', reject)
  })
  let seq = 0
  const pending = new Map()
  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data)
    if (!message.id || !pending.has(message.id)) return
    const { resolve, reject } = pending.get(message.id)
    pending.delete(message.id)
    if (message.error) reject(new Error(message.error.message))
    else resolve(message.result)
  })
  const cdp = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++seq
    pending.set(id, { resolve, reject })
    ws.send(JSON.stringify({ id, method, params }))
  })
  const evaluate = async (expression) => {
    const result = await cdp('Runtime.evaluate', { expression, returnByValue: true })
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.text || '页面脚本执行失败')
    return result.result?.value
  }

  try {
    await cdp('Page.enable')
    await cdp('Emulation.setDeviceMetricsOverride', {
      width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height,
    })
    await cdp('Page.navigate', { url: `${BASE}/?overlap=${Date.now()}#menu` })
    await sleep(2800)
    await evaluate(`document.querySelector('.user-pick-item')?.click()`)
    for (let i = 0; i < 40; i++) {
      if (await evaluate(`!!document.querySelector('.dish-card')`)) break
      await sleep(100)
    }
    await sleep(450)
    const result = JSON.parse(await evaluate(SCAN('main')))
    console.log(JSON.stringify({ viewport: `${width}x${height}`, page: 'menu', ...result }))
    await evaluate(`document.querySelector('.dish-photo-open')?.click()`)
    await sleep(450)
    const detail = JSON.parse(await evaluate(SCAN('.dish-detail')))
    console.log(JSON.stringify({ viewport: `${width}x${height}`, page: 'detail', ...detail }))
    return result.failures.length === 0 && detail.failures.length === 0
  } finally {
    ws.close()
    killProcessTree(proc)
  }
}

let pass = true
for (const [index, size] of SIZES.entries()) {
  pass = (await audit(size, index)) && pass
}
if (!pass) process.exitCode = 1
