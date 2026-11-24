// 数据整形：只处理每道菜的 ingredients，把多配料合并行拆成独立条目，
// 并修正数量与名称粘连的记录。做法文本里的顿号不会碰。
// 脚本可重复执行；已经拆分过的行不会再次变化。
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const FILES = [
  path.join(ROOT, 'data', 'dishes.json'),
  path.join(ROOT, 'web', 'src', 'howto-amounts.json'),
]

const replacements = new Map([
  ['花椒 30 颗20g', ['花椒 10g']],
  ['花生 10 颗30g', ['花生 30g']],
  ['小葱 2 颗50g', ['葱 50g']],
  ['姜 1 小块20g', ['姜 20g']],
  ['鸡 半只500g', ['鸡 半只（约 500g）']],
  ['蒜 2 个 10g', ['蒜 10g']],
  ['大葱 30g约半颗', ['大葱 30g（约半颗）']],
  ['野生大脚菌（干）50g', ['野生大脚菌（干） 50g']],
  ['新鲜土笋（沙虫）500g', ['新鲜土笋（沙虫） 500g']],
  ['蓬灰水（或碱水）10ml', ['蓬灰水（或碱水） 10ml']],
  ['石鸡（山蛙）600g', ['石鸡（山蛙） 600g']],
  ['问政山笋（鲜笋）500g', ['问政山笋（鲜笋） 500g']],
  ['青红椒 各 2 个', ['青椒 2 个', '红椒 2 个']],
])

function normalizeItems(items) {
  return (items || []).flatMap((raw) => {
    const text = String(raw).trim()
    return replacements.get(text)
      || (text.includes('、') ? text.split('、').map((part) => part.trim()).filter(Boolean) : [text])
  })
}

const dishesPath = FILES[0]
const dishes = JSON.parse(fs.readFileSync(dishesPath, 'utf8'))
let dishChanged = 0
for (const dish of dishes) {
  if (!Array.isArray(dish.ingredients)) continue
  const next = normalizeItems(dish.ingredients)
  if (JSON.stringify(next) !== JSON.stringify(dish.ingredients)) {
    dish.ingredients = next
    dishChanged++
  }
}
if (dishChanged) {
  const backupDir = path.join(ROOT, 'data', 'backups')
  fs.mkdirSync(backupDir, { recursive: true })
  const stamp = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14)
  fs.copyFileSync(dishesPath, path.join(backupDir, `dishes.pre-recipe-normalize-${stamp}.json`))
  fs.writeFileSync(dishesPath, `${JSON.stringify(dishes, null, 2)}\n`)
}
console.log(`${path.relative(ROOT, dishesPath)}: 修正 ${dishChanged} 道菜`)

const amountPath = FILES[1]
const amounts = JSON.parse(fs.readFileSync(amountPath, 'utf8'))
let amountChanged = 0
for (const [name, items] of Object.entries(amounts)) {
  const next = normalizeItems(items)
  if (JSON.stringify(next) !== JSON.stringify(items)) {
    amounts[name] = next
    amountChanged++
  }
}
if (amountChanged) {
  fs.writeFileSync(amountPath, `${JSON.stringify(amounts, null, 1)}\n`)
}
console.log(`${path.relative(ROOT, amountPath)}: 修正 ${amountChanged} 道菜`)
