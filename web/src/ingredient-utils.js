// 配料名称归一：只用于搜索、库存匹配和购物清单聚合，不改变菜谱里的展示文案。
// 这里刻意不把洋葱、蒜苗、蒜苔、青蒜、糖蒜等归成葱/蒜，它们是不同的食材。
const INGREDIENT_ALIAS = Object.freeze({
  西红柿: '番茄',
  洋柿子: '番茄',
  大葱葱白: '葱',
  大蒜: '蒜',
  蒜头: '蒜',
  老姜: '姜',
  子姜: '姜',
  生姜: '姜',
  洋葱碎: '洋葱',
  食用盐: '盐',
  白砂糖: '白糖',
  生抽酱油: '生抽',
  老抽酱油: '老抽',
  水淀粉: '淀粉',
  芝麻油: '香油',
  红糖水: '红糖',
  蒜水: '蒜',
  植物油: '食用油',
  油: '食用油',
  柿子椒: '青椒',
  菜椒: '青椒',
  葱头: '洋葱',
  玉葱: '洋葱',
  地瓜: '红薯',
  蕃薯: '红薯',
})

const HERB_FORMS = [
  { name: '葱', re: /^(?:小|大|香|青|鲜|胡)?葱(?:花|段|丝|末|白|绿|叶|结|碎|头)?$/ },
  { name: '姜', re: /^(?:生|老|子|鲜)?姜(?:片|丝|末|蓉|汁|块|米)?$/ },
  { name: '蒜', re: /^(?:大|鲜)?蒜(?:瓣|片|末|蓉|泥|头|米)?$/ },
]

// 水不是采购项，但仍然是菜谱做法和用量的一部分。
// 蓬灰水、碱水、水淀粉等是实际配料，不在此列。
const NON_PURCHASABLE = new Set([
  '水', '清水', '热水', '温水', '凉水', '冷水', '冰水', '开水',
  '凉开水', '凉白开', '沸水', '滚水', '山泉水', '盐水', '淡盐水',
  '葱姜水', '姜水', '花椒水', '蒜水', '清水或高汤', '高汤或清水', '卤水',
])

export function ingredientBaseName(name) {
  let value = String(name || '')
    .normalize('NFKC')
    .trim()
    .replace(/[0-9]/g, '')
    .replace(/\s+/g, '')

  if (!value) return ''
  if (INGREDIENT_ALIAS[value]) return INGREDIENT_ALIAS[value]
  const herb = HERB_FORMS.find((item) => item.re.test(value))
  return herb ? herb.name : value
}

export function isNonPurchasableIngredient(name) {
  return NON_PURCHASABLE.has(ingredientBaseName(name))
}

// 兼容历史卷数据里「姜 10g、蒜 10g」这类合并行。
// 展示和采购都先拆成独立配料，避免把后一半误当成前一味的用量。
export function splitIngredientItems(items) {
  return (items || [])
    .flatMap((item) => String(item || '').split(/[、，]/))
    .map((item) => item.trim())
    .filter(Boolean)
}

