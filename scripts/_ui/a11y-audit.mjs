// 真实页面无障碍回归：标题层级、交互语义、可访问名称、触控尺寸与文字对比度。
// 用法：先启动服务端，再 node scripts/_ui/a11y-audit.mjs
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
import { killProcessTree } from './browser-cleanup.mjs'

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const BASE = (process.env.BASE || 'http://127.0.0.1:8090').replace(/\/+$/, '')
const TABS = ['today', 'menu', 'family', 'history', 'settings']
const THEME = process.env.THEME || ''

const AUDIT = `JSON.stringify((()=>{
  const visible=(el)=>{
    const s=getComputedStyle(el), r=el.getBoundingClientRect();
    return s.display!=='none'&&s.visibility!=='hidden'&&Number(s.opacity)>0&&r.width>0&&r.height>0
  };
  const text=(el)=>(el.textContent||'').replace(/\\s+/g,' ').trim();
  const hasReadableText=(el)=>text(el)
    .replace(/\\p{Extended_Pictographic}|\\p{Emoji_Component}|[\\uFE0F\\u200D]/gu,'')
    .trim().length>0;
  const name=(el)=>(
    el.getAttribute('aria-label')||el.getAttribute('title')||
    el.getAttribute('alt')||text(el)
  ).trim();
  const parse=(value)=>{
    const m=String(value).match(/rgba?\\(([^)]+)\\)/);
    if(!m)return null;
    const p=m[1].split(/[ ,/]+/).filter(Boolean).map(Number);
    return {r:p[0],g:p[1],b:p[2],a:p.length>3?p[3]:1};
  };
  const mix=(fg,bg)=>{
    const a=fg.a+bg.a*(1-fg.a);
    if(!a)return {r:0,g:0,b:0,a:0};
    return {
      r:(fg.r*fg.a+bg.r*bg.a*(1-fg.a))/a,
      g:(fg.g*fg.a+bg.g*bg.a*(1-fg.a))/a,
      b:(fg.b*fg.a+bg.b*bg.a*(1-fg.a))/a,a,
    };
  };
  const lum=(c)=>{
    const f=(v)=>{v/=255;return v<=.03928?v/12.92:Math.pow((v+.055)/1.055,2.4)};
    return .2126*f(c.r)+.7152*f(c.g)+.0722*f(c.b);
  };
  const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05)};
  const surface=(el)=>{
    let current=el;
    while(current&&current!==document.documentElement){
      const s=getComputedStyle(current);
      if(s.backgroundImage!=='none')return null;
      const c=parse(s.backgroundColor);
      if(c&&c.a>.92)return c;
      current=current.parentElement;
    }
    return parse(getComputedStyle(document.body).backgroundColor)||{r:251,g:246,b:238,a:1};
  };
  const controls=[...document.querySelectorAll('button,a[href],input,select,textarea,[role="button"]')]
    .filter(visible).filter((el)=>!el.matches('input[type="hidden"]'));
  const unnamed=controls.filter((el)=>!name(el)).slice(0,12)
    .map((el)=>el.tagName.toLowerCase()+'.'+String(el.className||'').slice(0,30));
  const nested=controls.filter((el)=>el.querySelector('button,a[href],input,select,textarea,[role="button"]'))
    .slice(0,12).map((el)=>el.tagName.toLowerCase()+'.'+String(el.className||'').slice(0,40));
  const small=innerWidth<=768?controls.filter((el)=>{
    const r=el.getBoundingClientRect(), s=getComputedStyle(el);
    return !el.disabled&&s.display!=='none'&&(r.width<43.5||r.height<43.5);
  }).slice(0,12).map((el)=>{
    const r=el.getBoundingClientRect();
    return (name(el)||el.tagName)+' '+Math.round(r.width)+'x'+Math.round(r.height);
  }):[];
  const missingAlt=[...document.querySelectorAll('img')].filter(visible)
    .filter((img)=>!img.hasAttribute('alt')).slice(0,12)
    .map((img)=>img.getAttribute('src')||'(no src)');
  const lowContrast=[];
  for(const el of document.querySelectorAll('body *')){
    if(!visible(el)||['SCRIPT','STYLE','SVG','PATH'].includes(el.tagName))continue;
    const direct=[...el.childNodes].some((n)=>n.nodeType===Node.TEXT_NODE&&n.textContent.trim());
    if(!direct||!hasReadableText(el))continue;
    const bg=surface(el);
    if(!bg)continue;
    const fg=parse(getComputedStyle(el).color);
    if(!fg)continue;
    const actual=mix(fg,bg);
    const size=parseFloat(getComputedStyle(el).fontSize)||16;
    const weight=Number(getComputedStyle(el).fontWeight)||400;
    const large=size>=24||(size>=18.66&&weight>=700);
    const need=large?3:4.5;
    const value=ratio(actual,bg);
    if(value+0.03<need)lowContrast.push({
      value:Number(value.toFixed(2)),need,
      text:text(el).slice(0,48),
      cls:String(el.className||el.tagName).slice(0,45),
    });
  }
  const placeholders=[...document.querySelectorAll('input[placeholder],textarea[placeholder]')]
    .filter(visible).map((el)=>{
      const bg=surface(el);
      const fg=parse(getComputedStyle(el,'::placeholder').color);
      return fg&&bg?{value:Number(ratio(fg,bg).toFixed(2)),text:el.getAttribute('placeholder')}:null;
    }).filter(Boolean).filter((item)=>item.value<4.47);
  return {
    h1:[...document.querySelectorAll('h1')].filter(visible).map(text),
    unnamed,nested,small,missingAlt,
    lowContrast:lowContrast.slice(0,12),
    placeholders,
    overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth,
  };
})())`

async function auditViewport(width, height, label, mobile) {
  const port = 9700 + (mobile ? 50 : 0) + Math.floor(Math.random() * 40)
  const profile = `${process.env.TEMP || '.'}/fm-a11y-${label}-${Date.now()}`
  const proc = spawn(EDGE, [
    '--headless=new', '--disable-gpu', '--no-sandbox',
    `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: 'ignore' })

  let targets
  for (let i = 0; i < 60; i++) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/list`)
      targets = await response.json()
      if (targets.some((target) => target.type === 'page')) break
    } catch {}
    await sleep(250)
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
    await cdp('Page.navigate', { url: `${BASE}/?a11y=${Date.now()}#today` })
    await sleep(3000)
    if (THEME) {
      await evaluate(`document.documentElement.dataset.theme=${JSON.stringify(THEME)}`)
      // 等待主题色过渡完成，避免在过渡中间取样造成伪低对比。
      await sleep(520)
    }
    await evaluate(`document.querySelector('.user-pick-item')?.click()`)
    for (let i = 0; i < 35; i++) {
      if (await evaluate(`!!document.querySelector('main h1')`)) break
      await sleep(100)
    }

    console.log(`\n=== ${label} ${width}x${height}${THEME ? ` ${THEME}` : ''} ===`)
    for (const tab of TABS) {
      await evaluate(`window.location.hash='#${tab}'`)
      for (let i = 0; i < 35; i++) {
        if (await evaluate(`!document.querySelector('.page-enter-active,.page-leave-active') && !!document.querySelector('main h1')`)) break
        await sleep(100)
      }
      const report = JSON.parse(await evaluate(AUDIT))
      console.log(
        `${tab.padEnd(9)} h1=${report.h1.length ? '✓' : '✗'} 无名控件=${report.unnamed.length}`
        + ` 嵌套交互=${report.nested.length} 小目标=${report.small.length}`
        + ` 低对比=${report.lowContrast.length} 缺alt=${report.missingAlt.length}`
        + ` 占位对比=${report.placeholders.length} 溢出=${report.overflow}`,
      )
      for (const [kind, list] of Object.entries(report)) {
        if (['h1', 'overflow'].includes(kind) || !Array.isArray(list) || !list.length) continue
        console.log(`  ${kind}:`, JSON.stringify(list))
      }
      if (report.h1.length !== 1) console.log('  h1:', JSON.stringify(report.h1))
    }
  } finally {
    ws.close()
    killProcessTree(proc)
  }
}

await auditViewport(390, 844, 'phone', true)
await auditViewport(1280, 900, 'desktop', false)
