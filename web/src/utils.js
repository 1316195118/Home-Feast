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

// 购物清单：把今天已点菜品的配料按「第一段食材名」聚合，同名合并并记下来源菜。
// 纯前端计算，不新增后端字段；「盐 少许」「盐 适量」会归成一条「盐」。
export function shoppingList(orders, dishes) {
  const map = new Map()
  const byId = new Map((dishes || []).map((d) => [d.id, d]))
  for (const o of orders || []) {
    const d = byId.get(o.dishId)
    if (!d) continue
    for (const raw of d.ingredients || []) {
      const text = String(raw).trim()
      if (!text) continue
      const key = text.split(/\s+/)[0].replace(/[0-9０-９]/g, '').trim() || text
      if (!map.has(key)) map.set(key, { text: key, count: 0, from: [] })
      const item = map.get(key)
      item.count++
      if (!item.from.includes(d.name)) item.from.push(d.name)
    }
  }
  return [...map.values()].sort((a, b) => a.text.localeCompare(b.text, 'zh'))
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
