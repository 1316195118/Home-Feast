import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'
const EDGE='C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
const PORT=9260, BASE='http://10.10.10.251:8090'
const PROFILE=`C:/Users/Administrator/AppData/Local/Temp/fm-cdp-cq-${Date.now()}`
const proc=spawn(EDGE,['--headless=new','--disable-gpu','--no-sandbox',`--remote-debugging-port=${PORT}`,`--user-data-dir=${PROFILE}`,'about:blank'],{stdio:'ignore'})
let targets
for(let i=0;i<50;i++){try{const r=await fetch(`http://127.0.0.1:${PORT}/json/list`);targets=await r.json();if(targets.some(t=>t.type==='page'))break}catch{}await sleep(300)}
const ws=new WebSocket(targets.find(t=>t.type==='page').webSocketDebuggerUrl)
await new Promise((res,rej)=>{ws.addEventListener('open',res);ws.addEventListener('error',rej)})
let seq=0
const cdp=(m,p={})=>new Promise(r=>{const id=++seq;const f=e=>{const m=JSON.parse(e.data);if(m.id!==id)return;ws.removeEventListener('message',f);r(m.result)};ws.addEventListener('message',f);ws.send(JSON.stringify({id,method:m,params:p}))})
const ev=async(x,a=false)=>{const r=await cdp('Runtime.evaluate',{expression:x,returnByValue:true,awaitPromise:a});return r?.result?.value}
await cdp('Page.enable')
await cdp('Emulation.setDeviceMetricsOverride',{width:1280,height:900,deviceScaleFactor:1,mobile:false})
await cdp('Page.navigate',{url:`${BASE}/?v=${Date.now()}#menu`})
await sleep(3500)
await ev(`document.querySelector('.user-pick-item')?.click()`)
await sleep(600)
const r=await ev(`(()=>{
  const imgs=[...document.querySelectorAll('.dish-photo img')];
  const newImgs=imgs.filter(i=>i.src.includes('/uploads/img'));
  const withFilter=imgs.filter(i=>i.style.filter&&i.style.filter.includes('hue-rotate'));
  const broken=imgs.filter(i=>i.complete&&i.naturalWidth===0);
  const ratio=(()=>{const el=document.querySelector('.dish-photo');if(!el)return null;const b=el.getBoundingClientRect();return (b.width/b.height).toFixed(3)})();
  return JSON.stringify({total:imgs.length,newUploads:newImgs.length,withHueFilter:withFilter.length,broken:broken.length,cardRatio:ratio,sampleSrc:newImgs[0]?.src||null});
})()`)
console.log('菜谱页:',r)
// 截图
const shot=await cdp('Page.captureScreenshot',{format:'png',captureBeyondViewport:false})
const fs=await import('node:fs');fs.writeFileSync('scripts/_cq/menu_shot.png',Buffer.from(shot.data,'base64'))
console.log('screenshot saved')
ws.close();proc.kill();console.log('DONE')
