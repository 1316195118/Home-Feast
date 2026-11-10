// 成员密码验收：哈希存储 / 不进 bootstrap 与备份 / 修改需旧密码 / 校验链路 / 双视口 UI。
// 会临时给第一位成员设密码，结束时一定清除（见 main 的 finally）。
import { spawn } from 'node:child_process'
import { setTimeout as sleep } from 'node:timers/promises'

const BASE = process.env.BASE || 'http://127.0.0.1:8090'

async function api(method, path, body) {
  const r = await fetch(BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  })
  const ct = r.headers.get('content-type') || ''
  const data = ct.includes('json') ? await r.json() : await r.arrayBuffer()
  return { status: r.status, data }
}

const boot = (await api('GET', '/api/bootstrap')).data
const [a, b] = boot.members
console.log(`成员: ${boot.members.map((m) => m.name).join('、')} | 测试对象 ${a.name}`)

async function clearPin() {
  try { await api('POST', `/api/members/${a.id}/pin`, { pin: '', oldPin: '1234' }) } catch {}
  try { await api('POST', `/api/members/${a.id}/pin`, { pin: '', oldPin: '5678' }) } catch {}
}

try {
  await clearPin()
  // 1) 设置密码
  const set = await api('POST', `/api/members/${a.id}/pin`, { pin: '1234', oldPin: '' })
  console.log('设置密码:', set.status, JSON.stringify(set.data))
  const b1 = (await api('GET', '/api/bootstrap')).data
  const raw = JSON.stringify(b1)
  console.log('  bootstrap hasPin:', b1.members.find((m) => m.id === a.id)?.hasPin,
    '| 不含 hash/salt:', !raw.includes('"hash"') && !raw.includes('"salt"'),
    '| 不含明文:', !raw.includes('1234'))

  // 2) 备份 zip 不含密码文件
  const zip = Buffer.from((await api('GET', '/api/backup')).data)
  console.log('  备份 zip 不含 pins.json:', !zip.includes(Buffer.from('pins.json')),
    '| 不含 access.json:', !zip.includes(Buffer.from('access.json')))

  // 3) 校验接口：错误/正确；失败必须是 200 + ok:false（401 会误触发全局口令锁）
  const bad = await api('POST', `/api/members/${a.id}/verify`, { pin: '0000' })
  const good = await api('POST', `/api/members/${a.id}/verify`, { pin: '1234' })
  console.log('  校验 错/对:', bad.status, bad.data.ok, '/', good.status, good.data.ok)

  // 4) 修改必须带对旧密码
  const wrongOld = await api('POST', `/api/members/${a.id}/pin`, { pin: '5678', oldPin: '9999' })
  const rightOld = await api('POST', `/api/members/${a.id}/pin`, { pin: '5678', oldPin: '1234' })
  console.log('  错误旧密码被拒:', wrongOld.status === 400, '| 正确旧密码可改:', rightOld.status === 200)
  // 改回 1234 供 UI 测试
  await api('POST', `/api/members/${a.id}/pin`, { pin: '1234', oldPin: '5678' })

  async function run(width, height, tag, mobile) {
    const PORT = 9800 + (mobile ? 50 : 0) + Math.floor(Math.random() * 40)
    const PROFILE = `C:/Users/Administrator/AppData/Local/Temp/fm-pin-${tag}-${Date.now()}`
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
    const errors = []
    ws.addEventListener('message', (e) => {
      const j = JSON.parse(e.data)
      if (j.method === 'Runtime.exceptionThrown') errors.push('exception')
      if (j.method === 'Runtime.consoleAPICalled' && j.params?.type === 'error') errors.push('console.error')
    })
    await cdp('Runtime.enable')
    await cdp('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile, screenWidth: width, screenHeight: height })
    await cdp('Page.navigate', { url: `${BASE}/?v=${Date.now()}#today` })
    await sleep(3000)

    console.log(`\n=== ${tag} ${width}x${height} ===`)
    // 首次选人：点有密码的成员 -> 弹密码框
    await ev(`[...document.querySelectorAll('.user-pick-item')].find(x=>x.textContent.includes(${JSON.stringify(a.name)}))?.click()`)
    await sleep(350)
    let s = JSON.parse(await ev(`JSON.stringify((()=>({
      pinModal: !!document.querySelector('.pin-modal'),
      pickStillOpen: !!document.querySelector('.user-pick-backdrop'),
    }))())`))
    console.log('首次选人 -> 弹密码框:', s.pinModal, '| 选人框仍在:', s.pickStillOpen)

    // 错误密码：不切换
    await ev(`(()=>{const i=document.querySelector('.pin-input');i.value='0000';i.dispatchEvent(new Event('input',{bubbles:true}));})()`)
    await sleep(120)
    await ev(`[...document.querySelectorAll('.pin-modal .btn')].find(b=>b.textContent.includes('确认'))?.click()`)
    await sleep(450)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      err: document.querySelector('.pin-error')?.textContent.trim() || '',
      pickStillOpen: !!document.querySelector('.user-pick-backdrop'),
    }))())`))
    console.log('错误密码 -> 提示:', s.err, '| 身份未切换:', s.pickStillOpen)

    // 正确密码：切换成功
    await ev(`(()=>{const i=document.querySelector('.pin-input');i.value='1234';i.dispatchEvent(new Event('input',{bubbles:true}));})()`)
    await sleep(120)
    await ev(`[...document.querySelectorAll('.pin-modal .btn')].find(b=>b.textContent.includes('确认'))?.click()`)
    await sleep(600)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      pinModal: !!document.querySelector('.pin-modal'),
      pickStillOpen: !!document.querySelector('.user-pick-backdrop'),
      who: document.querySelector('.user-switch select')?.value || '',
      stored: localStorage.getItem('fm.user') || '',
    }))())`))
    console.log('正确密码 -> 弹窗关闭:', !s.pinModal && !s.pickStillOpen, '| 当前身份是 A:', s.who === a.id, '| 已记住:', s.stored === a.id)

    // 顶部切到无密码成员 B，再切回 A 应弹密码框；取消后身份不变
    await ev(`(()=>{const s=document.querySelector('.user-switch select');s.value=${JSON.stringify(b.id)};s.dispatchEvent(new Event('change',{bubbles:true}));})()`)
    await sleep(350)
    const switchedB = await ev(`document.querySelector('.user-switch select')?.value`)
    await ev(`(()=>{const s=document.querySelector('.user-switch select');s.value=${JSON.stringify(a.id)};s.dispatchEvent(new Event('change',{bubbles:true}));})()`)
    await sleep(350)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      pinModal: !!document.querySelector('.pin-modal'),
      who: document.querySelector('.user-switch select')?.value || '',
    }))())`))
    console.log('切到 B 成功:', switchedB === b.id, '| 再切回 A 弹密码框:', s.pinModal, '| 取消前身份仍是 B:', s.who === b.id)
    await ev(`[...document.querySelectorAll('.pin-modal .btn')].find(b=>b.textContent.includes('取消'))?.click()`)
    await sleep(300)

    // 家人页：密码标识与设置入口
    await ev(`window.location.hash='#family'`)
    await sleep(700)
    s = JSON.parse(await ev(`JSON.stringify((()=>({
      cards: document.querySelectorAll('.member-card').length,
      pinTag: [...document.querySelectorAll('.member-card')].some(c=>c.textContent.includes('切换身份需要密码')),
      changeBtn: [...document.querySelectorAll('.member-card .btn')].some(b=>b.textContent.includes('改密码')),
      setBtn: [...document.querySelectorAll('.member-card .btn')].some(b=>b.textContent.includes('设密码')),
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      smallBtns: [...document.querySelectorAll('.btn,.chip,.tab')].filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&(r.width<40||r.height<36)}).length,
    }))())`))
    console.log('家人页 卡片', s.cards, '| 已设密码标识', s.pinTag, '| 改密码按钮', s.changeBtn, '| 设密码按钮', s.setBtn)
    console.log('  横向溢出', s.overflow, '| 过小控件', s.smallBtns, '| JS 异常', errors.length)
    ws.close(); proc.kill()
  }
  await run(1280, 900, 'desktop', false)
  await run(390, 844, 'phone', true)
} finally {
  await clearPin()
  const final = (await api('GET', '/api/bootstrap')).data
  console.log('\n清理后 hasPin:', final.members.map((m) => `${m.name}:${!!m.hasPin}`).join(' '))
}
console.log('DONE')
