// 把源站 52 道重庆菜（含图片）导入目标系统 10.10.10.251:8090。
// 需在「重庆菜分类已部署」之后运行（否则 cuisine=chongqing 会回退成家常菜）。
// 用法：node scripts/import_to_target.mjs
import { readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.BASE || 'http://10.10.10.251:8090'
const HERE = fileURLToPath(new URL('.', import.meta.url))
const SRC = join(HERE, '_src_dishes.json')
const IMG_DIR = join(HERE, '_src_images', '')

const dishes = JSON.parse(readFileSync(SRC, 'utf-8'))

// 简单辣度推断（源站没有该字段）
function guessSpicy(name) {
  if (/特辣|爆辣|麻辣/.test(name)) return 2
  if (/辣|泡椒|藤椒|香辣|干煸|水煮/.test(name)) return 1
  return 0
}

async function bootstrap() {
  const r = await fetch(`${BASE}/api/bootstrap`)
  const d = await r.json()
  return d
}

async function uploadImage(id) {
  const p = join(IMG_DIR, id + '.webp')
  if (!existsSync(p)) throw new Error('缺图 ' + id)
  const buf = readFileSync(p)
  const form = new FormData()
  form.append('file', new Blob([buf], { type: 'image/webp' }), id + '.webp')
  const r = await fetch(`${BASE}/api/upload`, { method: 'POST', body: form })
  const j = await r.json()
  if (!r.ok || !j.url) throw new Error('上传失败 ' + (j.error || r.status))
  return j.url
}

async function createDish(dish, imageUrl) {
  const body = {
    name: dish.name,
    cuisine: 'chongqing',
    desc: dish.description || '',
    tags: dish.tags || [],
    image: imageUrl,
    spicy: guessSpicy(dish.name),
    minutes: 20,
  }
  const r = await fetch(`${BASE}/api/dishes`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const j = await r.json()
  if (!r.ok) throw new Error('创建失败 ' + (j.error || r.status))
  return j
}

// 隐藏除 chongqing 以外的所有菜系（按用户「这些菜系都不要」的意愿；设置里可随时再打开）
async function hideUnwantedCuisines(cuisines) {
  const hidden = cuisines.filter((c) => c.key !== 'chongqing').map((c) => c.key)
  const r = await fetch(`${BASE}/api/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ familyName: '我们家', hiddenCuisines: hidden }),
  })
  const j = await r.json()
  if (!r.ok) throw new Error('隐藏菜系失败 ' + (j.error || r.status))
  return hidden
}

async function main() {
  const info = await bootstrap()
  const known = (info.cuisines || []).map((c) => c.key)
  if (!known.includes('chongqing')) {
    console.error('⚠️  目标系统还没有「重庆菜」分类，请先部署更新后再运行本脚本。')
    process.exit(2)
  }
  const existing = new Set((info.dishes || []).map((d) => d.name))
  let created = 0, skipped = 0, failed = []
  for (const d of dishes) {
    if (existing.has(d.name)) { skipped++; continue }
    try {
      const url = await uploadImage(d.id)
      await createDish(d, url)
      created++
    } catch (e) {
      failed.push([d.name, e.message])
    }
  }
  console.log(`创建: ${created}  跳过(已存在): ${skipped}  失败: ${failed.length}`)
  if (failed.length) console.log('失败明细:', failed)

  // 询问是否隐藏其它菜系：默认隐藏（符合用户意愿）。可用 HIDE=0 跳过。
  if (process.env.HIDE !== '0') {
    try {
      const hidden = await hideUnwantedCuisines(info.cuisines)
      console.log('已隐藏菜系:', hidden.join(', '))
    } catch (e) {
      console.log('隐藏菜系出错:', e.message)
    }
  }
}

main().catch((e) => { console.error(e); process.exit(1) })
