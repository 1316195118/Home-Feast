// 全量菜谱审计：内置菜最终展开后的配料、做法、重复项和非采购项。
// 用法：node scripts/audit_recipes.mjs [--json]
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const require = createRequire(import.meta.url)
const esbuild = require(path.join(ROOT, 'web', 'node_modules', 'esbuild'))

async function bundle(entry) {
  const result = await esbuild.build({
    entryPoints: [path.join(ROOT, 'web', 'src', entry)],
    bundle: true,
    write: false,
    format: 'esm',
    platform: 'neutral',
    logLevel: 'silent',
  })
  const encoded = Buffer.from(result.outputFiles[0].contents).toString('base64')
  return import(`data:text/javascript;base64,${encoded}`)
}

const { getRecipe } = await bundle('recipes.js')
const {
  normalizeIngredient, shoppingKind, shoppingList,
} = await bundle('utils.js')
const { ingredientBaseName, isNonPurchasableIngredient } = await bundle('ingredient-utils.js')
const dishes = JSON.parse(fs.readFileSync(path.join(ROOT, 'data', 'dishes.json'), 'utf8'))

const VAGUE = /适量|少许|若干|随便|按需|一点|少量/

const report = {
  total: dishes.length,
  missingIngredients: [],
  missingSteps: [],
  vagueAmounts: [],
  nonShoppingItems: [],
  duplicateNames: [],
  shortSteps: [],
  malformedAmounts: [],
  shoppingWaterItems: [],
  failedChecks: [],
}

for (const dish of dishes) {
  const recipe = getRecipe(dish)
  const ingredients = Array.isArray(recipe.ingredients) ? recipe.ingredients : []
  const steps = Array.isArray(recipe.steps) ? recipe.steps : []
  if (!ingredients.length) report.missingIngredients.push(dish.name)
  if (!steps.length) report.missingSteps.push(dish.name)

  for (const raw of ingredients) {
    const text = String(raw).trim()
    if (!text) continue
    const bits = text.split(/\s+/)
    const name = bits[0]
    const amount = bits.slice(1).join(' ').trim()
    if (isNonPurchasableIngredient(name)) report.nonShoppingItems.push({ dish: dish.name, ingredient: text })
    if (VAGUE.test(text)) report.vagueAmounts.push({ dish: dish.name, ingredient: text })
    if (!amount || !/[0-9一二两三四五六七八九十半]/.test(amount)) {
      report.malformedAmounts.push({ dish: dish.name, ingredient: text })
    }
  }

  const grouped = new Map()
  for (const raw of ingredients) {
    const text = String(raw).trim()
    const name = text.split(/\s+/)[0]
    if (!name || isNonPurchasableIngredient(name)) continue
    const key = normalizeIngredient(name)
    if (!grouped.has(key)) grouped.set(key, [])
    grouped.get(key).push(text)
  }
  for (const [key, items] of grouped) {
    if (items.length > 1) report.duplicateNames.push({ dish: dish.name, key, items })
  }

  const stepText = steps.join(' ').trim()
  if (steps.length < 2 || stepText.length < 30) {
    report.shortSteps.push({ dish: dish.name, count: steps.length, length: stepText.length })
  }
}

// 用真实购物清单入口跑全量菜谱，确认水类没有穿过 UI 过滤层。
const expandedDishes = dishes.map((dish) => ({
  ...dish,
  ingredients: getRecipe(dish).ingredients.slice(),
}))
const allOrders = expandedDishes.map((dish) => ({ dishId: dish.id, dishName: dish.name }))
const fullShoppingList = shoppingList(allOrders, expandedDishes)
report.shoppingWaterItems = fullShoppingList
  .filter((item) => isNonPurchasableIngredient(item.text))
  .map((item) => ({ item: item.text, from: item.from }))

// 模拟旧数据卷里尚未重新保存的合并行，确保只更新二进制也能正确展示与采购。
const legacyDish = {
  id: '__legacy_recipe_check__',
  name: '旧数据兼容检查',
  ingredients: ['姜 10g、蒜 10g', '清水 100ml'],
  steps: ['测试步骤'],
}
const legacyRecipe = getRecipe(legacyDish)
const legacyShopping = shoppingList(
  [{ dishId: legacyDish.id }],
  [{ ...legacyDish, ingredients: legacyRecipe.ingredients }],
)
if (
  legacyRecipe.ingredients.join('|') !== '姜 10g|蒜 10g|清水 100ml'
  || legacyShopping.map((item) => item.text).sort().join('|') !== '姜|蒜'
) {
  report.failedChecks.push('legacyIngredientCompatibility')
}

report.shoppingKinds = [...new Set(dishes.flatMap((dish) => getRecipe(dish).ingredients)
  .map((line) => String(line).split(/\s+/)[0])
  .filter(Boolean)
  .map((name) => ({ name, kind: shoppingKind(name) })))]
  .filter((item, index, all) => all.findIndex((other) => other.name === item.name) === index)

if (report.missingIngredients.length) report.failedChecks.push('missingIngredients')
if (report.missingSteps.length) report.failedChecks.push('missingSteps')
if (report.vagueAmounts.length) report.failedChecks.push('vagueAmounts')
if (report.duplicateNames.length) report.failedChecks.push('duplicateNames')
if (report.shortSteps.length) report.failedChecks.push('shortSteps')
if (report.malformedAmounts.length) report.failedChecks.push('malformedAmounts')
if (report.shoppingWaterItems.length) report.failedChecks.push('shoppingWaterItems')
if (normalizeIngredient('葱花') !== '葱' || normalizeIngredient('葱段') !== '葱') {
  report.failedChecks.push('scallionAlias')
}
if (ingredientBaseName('洋葱') === '葱' || ingredientBaseName('蒜苗') === '蒜') {
  report.failedChecks.push('ingredientFalseMerge')
}

if (process.argv.includes('--json')) {
  console.log(JSON.stringify(report, null, 2))
} else {
  console.log(`菜品总数: ${report.total}`)
  console.log(`缺配料: ${report.missingIngredients.length}`)
  if (report.missingIngredients.length) console.log(`  ${report.missingIngredients.join('、')}`)
  console.log(`缺做法: ${report.missingSteps.length}`)
  if (report.missingSteps.length) console.log(`  ${report.missingSteps.join('、')}`)
  console.log(`模糊用量: ${report.vagueAmounts.length}`)
  for (const item of report.vagueAmounts) console.log(`  ${item.dish}: ${item.ingredient}`)
  console.log(`菜谱含烹饪用水（不进入购物清单）: ${report.nonShoppingItems.length}`)
  for (const item of report.nonShoppingItems) console.log(`  ${item.dish}: ${item.ingredient}`)
  console.log(`同菜同义重复: ${report.duplicateNames.length}`)
  for (const item of report.duplicateNames) console.log(`  ${item.dish}: ${item.key} ← ${item.items.join(' / ')}`)
  console.log(`步骤过短: ${report.shortSteps.length}`)
  console.log(`配料格式异常: ${report.malformedAmounts.length}`)
  for (const item of report.malformedAmounts) console.log(`  ${item.dish}: ${item.ingredient}`)
  console.log(`购物清单水项: ${report.shoppingWaterItems.length}`)
  for (const item of report.shoppingWaterItems) console.log(`  ${item.item} ← ${item.from.join('、')}`)
  console.log(report.failedChecks.length ? `检查未通过: ${report.failedChecks.join('、')}` : '全量检查通过')
}

if (report.failedChecks.length) process.exitCode = 1
