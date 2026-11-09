// 把因同名被跳过的 4 道菜（家常豆腐/口水鸡/麻婆豆腐/毛血旺）改挂到「重庆菜」，
// 并补上 gancook 源站图片，使其与已导入的 48 道 重庆菜 一致。
// 不改动原有 desc/tags，仅设置 cuisine + image（+ spicy 按菜名猜）。
import { readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.BASE || 'http://10.10.10.251:8090'
const HERE = fileURLToPath(new URL('.', import.meta.url))
const SRC = join(HERE, '_src_dishes.json')
const IMG_DIR = join(HERE, '_src_images', '')

const NAMES = ['家常豆腐', '口水鸡', '麻婆豆腐', '毛血旺']

function guessSpicy(name) {
  if (/特辣|爆辣|麻辣/.test(name)) return 2
  if (/辣|泡椒|藤椒|香辣|干煸|水煮/.test(name)) return 1
  return 0
}

async function getJSON(url) {
  const r = await fetch(url)
  return r.json()
}

async function uploadImage(uuid) {
  const p = join(IMG_DIR, uuid + '.webp')
  if (!existsSync(p)) throw new Error('缺图 ' + uuid)
  const buf = readFileSync(p)
  const form = new FormData()
  form.append('file', new Blob([buf], { type: 'image/webp' }), uuid + '.webp')
  const r = await fetch(`${BASE}/api/upload`, { method: 'POST', body: form })
  const j = await r.json()
  if (!r.ok || !j.url) throw new Error('上传失败 ' + (j.error || r.status))
  return j.url
}

async function patchDish(id, patch) {
  const r = await fetch(`${BASE}/api/dishes/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  const j = await r.json()
  if (!r.ok) throw new Error('更新失败 ' + (j.error || r.status))
  return j
}

async function main() {
  const live = await getJSON(`${BASE}/api/dishes`)
  const dishes = live.dishes || live
  const byName = Object.fromEntries(dishes.map((d) => [d.name, d]))

  const src = JSON.parse(readFileSync(SRC, 'utf-8'))
  const srcByName = Object.fromEntries(src.map((d) => [d.name, d]))

  let ok = 0, fail = []
  for (const name of NAMES) {
    const tgt = byName[name]
    const s = srcByName[name]
    if (!tgt) { fail.push([name, '目标系统找不到该菜']); continue }
    if (!s) { fail.push([name, '源站找不到该菜']); continue }
    try {
      const url = await uploadImage(s.id)
      const updated = await patchDish(tgt.id, {
        cuisine: 'chongqing',
        image: url,
        spicy: guessSpicy(name),
      })
      console.log(`✔ ${name} -> 重庆菜  (image=${updated.image})`)
      ok++
    } catch (e) {
      fail.push([name, e.message])
    }
  }
  console.log(`\n成功: ${ok}  失败: ${fail.length}`)
  if (fail.length) console.log('失败明细:', fail)
}

main().catch((e) => { console.error(e); process.exit(1) })
