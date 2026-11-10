// 界面共用的小工具，纯函数，方便单独测试。

// item 可能是菜品 Dish（字段 image/cuisine），也可能是点菜记录 Order（字段 dishImage/dishCuisine）。
// 传 dishes 时，订单行优先取菜谱里这道菜「当前」的图片，这样改图后今日菜单会同步；
// 菜品被删了就退回点菜时存下的快照，最后才用菜系插画。
export function dishImage(item, cuisines, dishes) {
  if (item?.dishId && Array.isArray(dishes)) {
    const live = dishes.find((d) => d.id === item.dishId)
    if (live?.image) return live.image
  }
  const snapshot = item?.image || item?.dishImage
  if (snapshot) return snapshot
  const key = item?.cuisine || item?.dishCuisine
  const found = (cuisines || []).find((c) => c.key === key)
  return found ? found.image : '/dishes/homestyle.jpg'
}

export function cuisineName(key, cuisines) {
  const found = cuisines.find((c) => c.key === key)
  return found ? found.name : '家常菜'
}

export function cuisineIcon(key, cuisines) {
  const found = cuisines.find((c) => c.key === key)
  return found ? found.icon : '🍳'
}

// 某个菜系是否被用户隐藏（在「设置 → 菜系显示」里关掉）。
export function isCuisineHidden(key, hidden) {
  return Array.isArray(hidden) && hidden.includes(key)
}

// 同一个菜系共用一张插画时，用稳定的哈希给每道菜一点不同的取景与色调，
// 避免一屏看起来像复制粘贴。取值刻意做小，不改变插画本身的观感。
// 但菜品若有自己的专属配图（逐道写实图），则不做任何扰动——否则每张图的
// 木纹背景会被 hue-rotate/saturate 染成不同色调，整套看起来反而不统一。
export function artStyle(item, dishes) {
  const hasOwnImage = !!(
    item?.image || item?.dishImage ||
    (item?.dishId && Array.isArray(dishes) && dishes.find((d) => d.id === item.dishId)?.image)
  )
  if (hasOwnImage) return {}
  // 订单行优先用菜品 id，保证同一道菜在菜谱和今日菜单里的取景/色调完全一致
  const key = String(item?.dishId || item?.id || item?.dishName || item?.name || '')
  let hash = 0
  for (let i = 0; i < key.length; i++) hash = (hash * 31 + key.charCodeAt(i)) >>> 0
  const positions = ['center 42%', 'center 30%', 'center 56%', 'center 36%', 'center 50%']
  const hue = (hash % 9) - 4
  const saturation = (0.94 + (hash % 5) * 0.03).toFixed(2)
  return {
    objectPosition: positions[hash % positions.length],
    filter: `hue-rotate(${hue}deg) saturate(${saturation})`,
  }
}

export function spicyText(level) {
  return ['不辣', '微辣', '中辣', '特辣'][Math.max(0, Math.min(3, level || 0))]
}

export function pepperText(level) {
  return '🌶️'.repeat(Math.max(0, Math.min(3, level || 0))) || '—'
}

export function friendlyDate(value) {
  const d = new Date(`${value}T00:00:00`)
  if (Number.isNaN(d.getTime())) return value
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
  const month = d.getMonth() + 1
  const date = d.getDate()
  const today = new Date()
  const todayStr = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
  return `${month}月${date}日 ${week}${value === todayStr ? ' · 今天' : ''}`
}

export function memberOf(id, members) {
  return members.find((m) => m.id === id) || null
}

export function pickRandom(list, count) {
  const pool = [...list]
  const out = []
  while (pool.length && out.length < count) {
    out.push(pool.splice(Math.floor(Math.random() * pool.length), 1)[0])
  }
  return out
}

// 头像按分组展示（人物 / 动物 / 食物 / 兴趣），扁平数组留给随机选头像等旧逻辑复用。
export const avatarGroups = [
  { label: '人物', items: ['👨', '👩', '🧑', '👧', '👦', '👶', '👵', '👴', '🧒', '🧔', '🙂', '😎', '🥸', '🤓', '👩‍🦰', '👨‍🦱'] },
  { label: '动物', items: ['🐱', '🐶', '🦊', '🐻', '🐼', '🐰', '🐷', '🐸', '🐵', '🦁', '🐯', '🐮', '🐔', '🐧', '🦄', '🐙'] },
  { label: '食物', items: ['🍚', '🍜', '🍲', '🍳', '🥟', '🍎', '🍉', '🍰', '🍪', '☕', '🧋', '🍡'] },
  { label: '兴趣', items: ['⚽', '🎮', '🎨', '🎵', '📚', '🚴', '🌻', '✈️', '🧩', '🎯', '🏀', '🎸'] },
]
export const avatarChoices = avatarGroups.flatMap((g) => g.items)
export const colorChoices = ['#d9a066', '#cf8b9b', '#7fa98d', '#8d9bc4', '#c59a6d', '#a98bb5']

// 家庭口味避雷：预设一批常见忌口，成员可多选也可自定义
export const avoidChoices = [
  '香菜', '葱', '姜', '蒜', '辣', '花生',
  '海鲜', '牛奶', '鸡蛋', '牛肉', '猪肉', '羊肉',
  '菌菇', '内脏', '芹菜', '胡萝卜',
]

// 一道菜命中了某位成员的哪些避雷项：菜名 / 描述 / 标签 / 配料里出现即算，
// 「辣」额外看辣度字段。返回命中项数组，空数组表示安全。
export function avoidConflicts(dish, member) {
  const avoid = Array.isArray(member?.avoid) ? member.avoid : []
  if (!avoid.length || !dish) return []
  const hay = [dish.name, dish.desc, ...(dish.tags || []), ...(dish.ingredients || [])]
    .filter(Boolean).join(' ')
  return avoid.filter((a) => (a === '辣' ? (dish.spicy || 0) > 0 : hay.includes(a)))
}

// 购物清单分类：佐料（调味料 / 炝锅辅料）与食材（主料）分开列。
// 用具体词而不是单字「油」「粉」，避免把「油炸花生米」「中筋面粉」误判成佐料。
const SEASONING_WORDS = [
  '盐', '糖', '冰糖', '白糖', '蜂蜜', '醋', '香醋', '生抽', '老抽', '酱油', '豉油', '蚝油',
  '料酒', '黄酒', '食用油', '香油', '辣椒油', '花椒油', '黄油', '番茄酱',
  '蒜', '姜', '葱', '花椒', '胡椒', '五香粉', '椒盐', '干辣椒', '泡椒',
  '豆瓣酱', '豆豉', '火锅底料', '淀粉', '红薯淀粉', '地瓜粉', '味精', '鸡精',
  '芝麻', '熟芝麻', '清水', '温水', '高汤', '开水',
  // HowToCook 精确用量里出现的调味料，确保购物清单分组与「佐料不参与推荐」一致
  '藤椒油', '菜籽油', '植物油', '葱油酱汁', '郫县豆瓣酱', '红油豆瓣酱',
  '白胡椒粉', '花椒粉', '辣椒粉', '鱼露', '芥末', '白醋',
]
// 这些词里虽然带「蒜 / 葱」，但本身是蔬菜主料，优先判为食材
const INGREDIENT_EXCEPTIONS = ['蒜苗', '蒜苔', '蒜薹', '青蒜', '洋葱', '姜母', '姜撞奶']

export function shoppingKind(name) {
  const n = String(name || '')
  if (INGREDIENT_EXCEPTIONS.some((k) => n.includes(k))) return '食材'
  return SEASONING_WORDS.some((k) => n.includes(k)) ? '佐料' : '食材'
}

// 购物清单：把今天已点菜品的配料按「第一段食材名」聚合，同名合并并记下来源菜。
// 纯前端计算，不新增后端字段；「盐 少许」「盐 适量」会归成一条「盐」。
// 用量文本形如「鸡腿肉 200g」「番茄 2 个」「盐 少许」，拆成食材名 + 用量。
// 同名食材合并时：能解析成同一单位的数值就累加出总重量/总个数，否则把原文并列。
const AMOUNT_UNITS = 'g|克|kg|千克|ml|毫升|个|只|根|片|块|勺|瓣|段|把|斤|两|罐|支|茶匙|汤匙|小块'
function parseAmount(raw) {
  const s = String(raw || '').replace(/\s+/g, '')
  if (!s) return null
  const m = s.match(new RegExp(`^([0-9.]+)(${AMOUNT_UNITS})$`))
  return m ? { n: Number(m[1]), unit: m[2] } : null
}
function formatAmounts(list) {
  const uniq = [...new Set((list || []).map((x) => String(x).trim()).filter(Boolean))]
  if (!uniq.length) return ''
  // 只有一个来源就照抄原用量（「200g」「2 个」「少许」），不加多余的「约」
  if (uniq.length === 1) return uniq[0]
  const parsed = uniq.map(parseAmount)
  if (parsed.every(Boolean) && new Set(parsed.map((p) => p.unit)).size === 1) {
    const sum = parsed.reduce((s, p) => s + p.n, 0)
    const n = Number.isInteger(sum) ? sum : Number(sum.toFixed(1))
    return `合计 ${n}${parsed[0].unit}`
  }
  return uniq.slice(0, 2).join('、') + (uniq.length > 2 ? '…' : '')
}

export function shoppingList(orders, dishes) {
  const map = new Map()
  const byId = new Map((dishes || []).map((d) => [d.id, d]))
  for (const o of orders || []) {
    const d = byId.get(o.dishId)
    if (!d) continue
    for (const raw of d.ingredients || []) {
      const text = String(raw).trim()
      if (!text) continue
      const bits = text.split(/\s+/)
      const key = bits[0].replace(/[0-9０-９]/g, '').trim() || text
      if (!map.has(key)) map.set(key, { text: key, kind: shoppingKind(key), count: 0, from: [], amounts: [] })
      const item = map.get(key)
      item.count++
      const amount = bits.slice(1).join(' ').trim()
      if (amount && !item.amounts.includes(amount)) item.amounts.push(amount)
      if (!item.from.includes(d.name)) item.from.push(d.name)
    }
  }
  return [...map.values()].map((x) => ({ ...x, amount: formatAmounts(x.amounts) })).sort((a, b) =>
    a.kind === b.kind ? a.text.localeCompare(b.text, 'zh') : (a.kind === '食材' ? -1 : 1))
}

// 按「食材 / 佐料」两组输出，供清单面板分节显示
export function shoppingGroups(list) {
  return [
    { key: '食材', icon: '🥬', items: (list || []).filter((x) => x.kind === '食材') },
    { key: '佐料', icon: '🧂', items: (list || []).filter((x) => x.kind === '佐料') },
  ]
}

// 用餐时段预设图标：点一下就选，仍可在面板底部的输入框里自定义
export const mealIconChoices = [
  '🌅', '☀️', '🌙', '🍽️', '🥐', '🍚',
  '🍜', '🍲', '🥣', '🍰', '☕', '🍵',
  '🌆', '🌃', '⏰', '🕐', '🥗', '🍱',
  '🍳', '🥪', '🍡', '🧋', '🌄', '🍎',
]

// 默认三餐，与后端 store.DefaultMeals() 一致，仅作兜底。
// 运行时一律以 bootstrap 下发的 state.meals 为准（用户可在设置里自定义名字和时间）。
export const DEFAULT_MEALS = [
  { key: 'breakfast', label: '早餐', icon: '🌅', end: '10:00' },
  { key: 'lunch', label: '午餐', icon: '☀️', end: '15:00' },
  { key: 'dinner', label: '晚餐', icon: '🌙', end: '23:00' },
]

// "HH:MM" -> 从 0 点起的分钟数（0..1440），非法返回 -1
export function mealEndMinutes(end) {
  const p = String(end || '').split(':')
  if (p.length !== 2 || p[0] === '' || p[1] === '') return -1
  const h = Number(p[0]); const m = Number(p[1])
  if (!Number.isInteger(h) || !Number.isInteger(m) || h < 0 || h > 24 || m < 0 || m > 59) return -1
  if (h === 24 && m !== 0) return -1
  return h * 60 + m
}

// 统一拿一份可用配置：空/非法回退默认，并按结束时刻升序排列
export function normalizeMeals(meals) {
  const list = (Array.isArray(meals) ? meals : []).filter(
    (m) => m && m.key && mealEndMinutes(m.end) >= 0,
  )
  if (!list.length) return DEFAULT_MEALS
  return list.slice().sort((a, b) => mealEndMinutes(a.end) - mealEndMinutes(b.end))
}

// now 可传 Date、时间戳或省略；传 state.now 让模板随时间 tick 重算
function toDate(now) {
  if (now instanceof Date) return now
  if (typeof now === 'number') return new Date(now)
  return new Date()
}

export function currentMeal(now = new Date(), meals) {
  const list = normalizeMeals(meals)
  const d = toDate(now)
  const cur = d.getHours() * 60 + d.getMinutes()
  for (const m of list) {
    if (cur < mealEndMinutes(m.end)) return m.key
  }
  return list[list.length - 1]?.key || ''
}

export function mealOf(key, meals) {
  return normalizeMeals(meals).find((x) => x.key === key) || null
}

// 该餐是否已过饭点（到了自定义结束时刻就视为可自动收起）
export function isMealExpired(meal, now = new Date(), meals) {
  const m = mealOf(meal, meals)
  if (!m) return false
  const d = toDate(now)
  return d.getHours() * 60 + d.getMinutes() >= mealEndMinutes(m.end)
}

// ---------- 第二批：每周排班 / 智能推荐 ----------
export const WEEKDAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
// 冰箱食材预设：每项都有一张自绘「效果图」（大 emoji + 渐变底色），
// 风格与站内自绘素材一致，不引入第三方图片；自定义食材由 pantryVisual 兜底配色。
export const pantryGroups = [
  {
    key: 'meat', label: '🥩 肉禽', tint: '#f6ded8',
    items: ['猪肉', '五花肉', '排骨', '牛肉', '牛腩', '鸡胸肉', '鸡腿肉', '鸡翅', '鸡蛋', '培根', '香肠', '羊肉'],
  },
  {
    key: 'seafood', label: '🦐 水产', tint: '#dcebf0',
    items: ['虾', '虾仁', '鲈鱼', '鲤鱼', '带鱼', '三文鱼', '鱿鱼', '花甲', '生蚝', '海带'],
  },
  {
    key: 'veg', label: '🥬 蔬菜', tint: '#dcecd3',
    items: ['番茄', '土豆', '青椒', '黄瓜', '白菜', '菠菜', '茄子', '西兰花', '胡萝卜', '洋葱', '芹菜', '生菜', '上海青', '豆角', '莴笋', '冬瓜', '南瓜', '玉米', '青蒜'],
  },
  {
    key: 'mushroom', label: '🍄 菌菇豆制品', tint: '#e8e1d1',
    items: ['香菇', '金针菇', '平菇', '木耳', '豆腐', '豆干', '腐竹', '黄豆', '毛豆'],
  },
  {
    key: 'staple', label: '🍚 主食', tint: '#f3e7cc',
    items: ['米饭', '面条', '挂面', '馒头', '饺子皮', '面粉', '年糕', '粉丝', '面包'],
  },
  {
    key: 'fruit', label: '🍎 水果', tint: '#f7dfd8',
    items: ['苹果', '香蕉', '柠檬', '菠萝', '梨', '橙子'],
  },
  {
    key: 'dairy', label: '🥛 蛋奶', tint: '#f5ecd9',
    items: ['牛奶', '酸奶', '黄油', '奶酪', '咸鸭蛋'],
  },
]

// 展开成扁平预设（每项带 group），兼容 pantryVisual / pantryPresets 的既有调用。
const ICONS = {
  猪肉: '🥩', 五花肉: '🥓', 排骨: '🍖', 牛肉: '🥩', 牛腩: '🥩', 鸡胸肉: '🍗', 鸡腿肉: '🍗',
  鸡翅: '🍗', 鸡蛋: '🥚', 培根: '🥓', 香肠: '🌭', 羊肉: '🍖',
  虾: '🦐', 虾仁: '🦐', 鲈鱼: '🐟', 鲤鱼: '🐟', 带鱼: '🐟', 三文鱼: '🐟', 鱿鱼: '🦑',
  花甲: '🐚', 生蚝: '🦪', 海带: '🌿',
  番茄: '🍅', 土豆: '🥔', 青椒: '🫑', 黄瓜: '🥒', 白菜: '🥬', 菠菜: '🥬', 茄子: '🍆',
  西兰花: '🥦', 胡萝卜: '🥕', 洋葱: '🧅', 芹菜: '🥬', 生菜: '🥬', 上海青: '🥬',
  豆角: '🫛', 莴笋: '🥬', 冬瓜: '🥒', 南瓜: '🎃', 玉米: '🌽', 青蒜: '🌿',
  香菇: '🍄', 金针菇: '🍄', 平菇: '🍄', 木耳: '🍄', 豆腐: '🧈', 豆干: '🧈', 腐竹: '🧈',
  黄豆: '🫘', 毛豆: '🫛',
  米饭: '🍚', 面条: '🍜', 挂面: '🍜', 馒头: '🥟', 饺子皮: '🥟', 面粉: '🌾', 年糕: '🍡',
  粉丝: '🍜', 面包: '🍞',
  苹果: '🍎', 香蕉: '🍌', 柠檬: '🍋', 菠萝: '🍍', 梨: '🍐', 橙子: '🍊',
  牛奶: '🥛', 酸奶: '🥛', 黄油: '🧈', 奶酪: '🧀', 咸鸭蛋: '🥚',
}
export const pantryPresets = pantryGroups.flatMap((g) =>
  [...new Set(g.items)].map((name) => ({ name, icon: ICONS[name] || '🥗', tint: g.tint, group: g.key })),
)
const PANTRY_TINTS = ['#f3e2c9', '#dfe9d2', '#f5d9d0', '#e5e0ef', '#f7e8c6', '#d9e7e6']
export function pantryVisual(name) {
  const hit = pantryPresets.find((p) => p.name === name)
  if (hit) return hit
  let h = 0
  for (const ch of String(name || '')) h = (h * 31 + ch.codePointAt(0)) >>> 0
  return { name, icon: '🥗', tint: PANTRY_TINTS[h % PANTRY_TINTS.length] }
}

// 取某天的排班；date 是 "YYYY-MM-DD"，用中午构造避免本地时区把日期挪到前一天。
export function rosterForDate(roster, date) {
  if (!roster?.enabled || !date) return null
  const d = new Date(`${date}T12:00:00`)
  if (Number.isNaN(d.getTime())) return null
  const day = roster.days?.[String(d.getDay())]
  if (!day || (!day.chefId && !day.washId)) return null
  return day
}

function monthOf(dateLike) {
  if (!dateLike) return new Date().getMonth() + 1
  if (typeof dateLike === 'string') {
    const m = Number(dateLike.slice(5, 7))
    if (m >= 1 && m <= 12) return m
  }
  const d = dateLike instanceof Date ? dateLike : new Date(dateLike)
  return Number.isNaN(d.getTime()) ? new Date().getMonth() + 1 : d.getMonth() + 1
}

// 季节偏好：只影响推荐排序，不硬性屏蔽任何菜（家里想吃什么还是家里说了算）。
export function seasonOf(dateLike) {
  const m = monthOf(dateLike)
  if (m >= 3 && m <= 5) return { key: 'spring', label: '春', hint: '清爽时令', likes: ['cold', 'zhejiang', 'jiangsu', 'cantonese'], dislikes: ['sichuan', 'hunan'] }
  if (m >= 6 && m <= 8) return { key: 'summer', label: '夏', hint: '清爽开胃', likes: ['cold', 'soup', 'zhejiang', 'cantonese'], dislikes: ['anhui', 'hunan'] }
  if (m >= 9 && m <= 11) return { key: 'autumn', label: '秋', hint: '温补滋润', likes: ['soup', 'fujian', 'staple', 'homestyle'], dislikes: ['cold'] }
  return { key: 'winter', label: '冬', hint: '热乎下饭', likes: ['soup', 'sichuan', 'hunan', 'homestyle', 'staple'], dislikes: ['cold', 'dessert'] }
}

// 冰箱食材命中：只统计主料（食材），佐料（盐 / 油 / 葱蒜 / 酱油…）一律不参与，
// 否则「有盐有葱」会把几乎每道菜都算成「冰箱里有」，推荐就失去意义。
// 菜谱筛选与今日页智能推荐共用这一套规则，勾选变化后两处一起实时重排。
export function pantryMatches(dish, pantry, ingredientsOf = null) {
  const pan = (Array.isArray(pantry) ? pantry : [])
    .map((x) => String(x).trim()).filter(Boolean)
    .filter((p) => shoppingKind(p) !== '佐料')
  if (!pan.length || !dish) return { hits: [], count: 0 }
  const ings = typeof ingredientsOf === 'function' ? ingredientsOf(dish) : (dish.ingredients || [])
  const mainOnly = (Array.isArray(ings) ? ings : [])
    .filter(Boolean)
    .filter((i) => shoppingKind(String(i).split(/\s+/)[0]) !== '佐料')
    .join(' ')
  const hits = pan.filter((p) => mainOnly.includes(p))
  return { hits, count: hits.length }
}

function tinyHash(s) {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return (h >>> 0) / 4294967295
}

// 本地规则推荐，不接外部 LLM：季节 + 冰箱食材 + 口味避雷 + 耗时 + 收藏。
// salt 变化可以「换一批」，但同一盐值下结果稳定，刷新页面不会乱跳。
export function smartSuggest(dishes, opts = {}) {
  const {
    today = '', pantry = [], member = null, hiddenCuisines = [],
    orders = [], excludeIds = [], limit = 3, salt = 0,
    ingredientsOf = null,
  } = opts
  const season = seasonOf(today)
  const picked = new Set([...(orders || []).map((o) => o.dishId), ...(excludeIds || [])])
  const scored = []

  for (const d of dishes || []) {
    if (!d || picked.has(d.id) || isCuisineHidden(d.cuisine, hiddenCuisines)) continue
    const ings = typeof ingredientsOf === 'function' ? ingredientsOf(d) : (d.ingredients || [])
    // 内置菜的配料由 recipes.js 动态生成，这里用展开后的配料做避雷判断才准
    if (avoidConflicts({ ...d, ingredients: Array.isArray(ings) ? ings : [] }, member).length) continue

    let score = 1.5 + tinyHash(`${d.id}:${salt}`) * 1.6
    const reasons = []
    if (season.likes.includes(d.cuisine)) score += 2.2
    if (season.dislikes.includes(d.cuisine)) score -= 1.2
    const hits = pantryMatches(d, pantry, ingredientsOf).hits
    if (hits.length) {
      score += Math.min(hits.length, 4) * 2.6
      reasons.push(`用得上冰箱里的${hits.slice(0, 2).join('、')}`)
    } else if (season.likes.includes(d.cuisine)) {
      reasons.push(`${season.label}天吃${season.hint}`)
    }
    if (d.minutes > 0 && d.minutes <= 25) {
      score += 1.1
      if (reasons.length < 2) reasons.push(`${d.minutes} 分钟快手`)
    }
    if (d.minutes >= 60) score -= 0.8
    if (d.favorite) score += 0.9
    if (!reasons.length) reasons.push(`${season.label}天也合适`)
    scored.push({ dish: d, score, reasons: reasons.slice(0, 2) })
  }
  scored.sort((a, b) => b.score - a.score || String(a.dish.id).localeCompare(String(b.dish.id)))
  return scored.slice(0, limit)
}
