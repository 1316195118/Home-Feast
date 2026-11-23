<script setup>
import { inject, ref, computed, watch, onMounted, onUnmounted, onActivated, onDeactivated, nextTick } from 'vue'
import DishCard from './DishCard.vue'
import DishEditor from './DishEditor.vue'
import PantryFridge from './PantryFridge.vue'
import OrderPanel from './OrderPanel.vue'
import PantryImage from './PantryImage.vue'
import DishDetail from './DishDetail.vue'
import { normalizeMeals, isCuisineHidden, pantryGroups, pantryVisual, pantryMatches, normalizeIngredient, hasDishImage, dishImage, friendlyDate, pantryExpiry } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'
import { cacheThumb, thumbCacheEpoch } from '../thumb-cache'

const { state, actions, pending } = inject('app')
const orderContext = computed(() => {
  const date = state.selectedDate || state.today
  const base = new Date(`${state.today}T12:00:00`)
  const target = new Date(`${date}T12:00:00`)
  const offset = Math.round((target - base) / 86400000)
  const relative = offset === 0 ? '今天' : offset === 1 ? '明天' : offset === 2 ? '后天' : ''
  const meal = normalizeMeals(state.meals).find((item) => item.key === state.mealPick)
  return `${relative ? `${relative} · ` : ''}${friendlyDate(date)}${meal ? ` · ${meal.label}` : ''}`
})
const activeCuisine = ref('all')
const keyword = ref('')
const onlyFavorite = ref(false)
const quickFilter = ref('') // '' | quick | mild | image
function toggleQuick(k) { quickFilter.value = quickFilter.value === k ? '' : k }
const editorOpen = ref(false)
const editorDish = ref(null)
// 详情按 id 引用，编辑保存后能自动拿到最新数据
const detailId = ref(null)
const detailDish = computed(() => state.dishes.find((d) => d.id === detailId.value) || null)

// 只展示没有被隐藏的菜系；snack 在下面用手动 chip 单独渲染，这里排除避免重复
const visibleCuisines = computed(() =>
  (state.cuisines || []).filter((c) => !isCuisineHidden(c.key, state.hiddenCuisines) && c.key !== 'snack')
)

// 「小吃」独立成栏：按标签横切聚合。剥离后小吃只出现在「全部」和「小吃」，
// 不再掺在地域菜系里，地域菜系的计数也同步扣掉，避免「川菜 10 道点进去只有 8 道」。
const SNACK_TAGS = ['小吃', '点心']
// 数据层已把小吃菜 cuisine 迁为 snack；标签兜底保证老数据/新加的菜也能进这一栏。
// 用「包含」而不是「等于」：像「川渝小吃」「广式点心」这类细分标签也要能命中。
const snackCache = new WeakMap()
function isSnack(d) {
  if (snackCache.has(d)) return snackCache.get(d)
  const value = d.cuisine === 'snack'
    || (d.tags || []).some((t) => SNACK_TAGS.some((k) => String(t).includes(k)))
  snackCache.set(d, value)
  return value
}

// 分类计数、可见数量和「小吃」数量共享一次遍历；这些数据与搜索词无关，
// 不会因为输入每一个字而重复扫 237 道菜。
const dishStats = computed(() => {
  const map = {}
  let visible = 0
  let snacks = 0
  for (const d of state.dishes) {
    if (isCuisineHidden(d.cuisine, state.hiddenCuisines)) continue
    visible++
    const snack = isSnack(d)
    if (snack) snacks++
    if (snack) continue // 小吃单独成栏，不重复计入地域菜系
    map[d.cuisine] = (map[d.cuisine] || 0) + 1
  }
  return { counts: map, visible, snacks }
})
const snackCount = computed(() => dishStats.value.snacks)
const counts = computed(() => dishStats.value.counts)
const visibleDishCount = computed(() => dishStats.value.visible)

const searchTextCache = new WeakMap()
function dishSearchText(d) {
  const cached = searchTextCache.get(d)
  if (cached !== undefined) return cached
  const ings = (getRecipe(d)?.ingredients || d.ingredients || []).join(' ')
  const text = [d.name, d.desc, ...(d.tags || []), ings].join(' ').toLowerCase()
  searchTextCache.set(d, text)
  return text
}

// 「优先推荐」：按冰箱已选主料的命中数实时重排，佐料不参与；
// 命中数相同保持菜谱原顺序，勾选食材后 computed 自动重算，无需刷新。
const priorityPantry = ref(localStorage.getItem('fm.priorityPantry') === '1')
function togglePriority() {
  priorityPantry.value = !priorityPantry.value
  try { localStorage.setItem('fm.priorityPantry', priorityPantry.value ? '1' : '0') } catch { /* 隐私模式忽略 */ }
}
const recipeIngredients = (d) => getRecipe(d).ingredients || []

// 有筛选时才显示结果数，避免和菜系条「全部 N」重复
const hasFilter = computed(() =>
  !!keyword.value.trim() || activeCuisine.value !== 'all' || onlyFavorite.value || !!quickFilter.value || priorityPantry.value,
)
const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  const base = state.dishes.filter((d) => {
    if (isCuisineHidden(d.cuisine, state.hiddenCuisines)) return false
    if (activeCuisine.value === 'snack') {
      if (!isSnack(d)) return false
    } else if (activeCuisine.value !== 'all') {
      if (d.cuisine !== activeCuisine.value) return false
      if (isSnack(d)) return false // 小吃已独立成栏，地域菜系里不再重复出现
    }
    if (onlyFavorite.value && !d.favorite) return false
    if (quickFilter.value === 'quick' && !((d.minutes || 20) <= 30)) return false
    if (quickFilter.value === 'mild' && (d.spicy || 0) > 0) return false
    if (quickFilter.value === 'image' && !hasDishImage(d, state.dishes)) return false
    if (!q) return true
    // 配料也纳入搜索：搜「土豆」能找出所有用到土豆的菜，和冰箱食材推荐口径一致
    return dishSearchText(d).includes(q)
  })
  if (!priorityPantry.value) return base
  const pantry = state.settings.pantry || []
  return base
    .map((d, i) => ({ d, i, hits: pantryMatches(d, pantry, recipeIngredients).count }))
    .sort((a, b) => b.hits - a.hits || a.i - b.i)
    .map(({ d, hits }) => ({ ...d, _pantryHits: hits }))
})

// 分批渲染：首批 48 张，滚到接近底部再补 48 张；筛选条件一变就回到首批。
const PAGE = 48
const visibleCount = ref(PAGE)

// 卡片仍使用懒加载，避免同时解码 237 张大图；但缩略图在后台批量预热。
// 浏览器原生懒加载只会追赶视口，连续下拉时新卡片总要等一轮网络和解码。
// 一页缩略图（实测约 5.5 MB）合并成一个请求并保存为会话内 Blob；
// 批量接口不可用时才退回 6 路单图预取，单张失败仍由卡片自己的回退兜底。
const THUMB_PREFETCH_WORKERS = 6
const THUMB_BUNDLE_SIZE = PAGE
const prefetchedThumbs = new Set()
let prefetchTimer = 0
let prefetchIdle = 0
let prefetchRun = 0
let prefetchAbort = null

function thumbBundleKey(urls) {
  let first = 2166136261
  let second = 2246822507 ^ urls.length
  for (const url of urls) {
    for (let index = 0; index < url.length; index++) {
      const code = url.charCodeAt(index)
      first = Math.imul(first ^ code, 16777619)
      second = Math.imul(second ^ code, 3266489909)
    }
    first = Math.imul(first ^ 0xff, 16777619)
    second = Math.imul(second ^ 0xff, 3266489909)
  }
  return `/__thumb-bundle/${urls.length}-${(first >>> 0).toString(36)}${(second >>> 0).toString(36)}`
}

async function cacheThumbBundle(urls, run, signal) {
  for (let start = 0; start < urls.length; start += THUMB_BUNDLE_SIZE) {
    if (run !== prefetchRun) return
    const chunk = urls.slice(start, start + THUMB_BUNDLE_SIZE)
    const cacheKey = thumbBundleKey(chunk)
    let data = null
    let cache = null
    try {
      cache = 'caches' in window ? await caches.open('fm-shell-v5') : null
      const hit = await cache?.match(cacheKey)
      if (hit) data = await hit.arrayBuffer()
    } catch {
      // Cache Storage 被禁用时直接走网络。
    }
    if (!data) {
      const response = await fetch('/thumb-bundle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        signal,
        body: JSON.stringify({ urls: chunk }),
      })
      if (!response.ok) throw new Error(`缩略图包请求失败：${response.status}`)
      data = await response.arrayBuffer()
      if (cache) {
        try {
          await cache.put(cacheKey, new Response(data.slice(0), {
            headers: {
              'Content-Type': 'application/octet-stream',
              'Cache-Control': 'public, no-cache',
            },
          }))
        } catch {
          // 空间不足时仍使用本次已取回的图片。
        }
      }
    }
    if (data.byteLength < 4) throw new Error('缩略图包为空')
    const view = new DataView(data)
    const headerLength = view.getUint32(0, false)
    if (headerLength < 2 || 4 + headerLength > data.byteLength) throw new Error('缩略图索引损坏')
    const entries = JSON.parse(new TextDecoder().decode(new Uint8Array(data, 4, headerLength)))
    const bodyOffset = 4 + headerLength
    for (const entry of entries) {
      if (run !== prefetchRun) return
      if (!entry?.url || !entry.size) continue
      const bytes = new Uint8Array(data, bodyOffset + entry.offset, entry.size)
      cacheThumb(entry.url, new Blob([bytes], { type: entry.type || 'image/jpeg' }))
      prefetchedThumbs.add(entry.url)
    }
  }
}

async function prefetchIndividually(urls, run, signal) {
  let cursor = 0
  const worker = async () => {
    while (run === prefetchRun) {
      const url = urls[cursor++]
      if (!url) return
      if (prefetchedThumbs.has(url)) continue
      try {
        const response = await fetch(url, {
          cache: 'force-cache',
          credentials: 'same-origin',
          priority: 'low',
          signal,
        })
        if (!response.ok) continue
        const blob = await response.blob()
        if (!blob.type.startsWith('image/')) continue
        cacheThumb(url, blob)
        prefetchedThumbs.add(url)
      } catch {
        // 单张图片失败不影响列表；卡片自己的重试与封面回退仍会兜底。
      }
    }
  }
  await Promise.all(Array.from({ length: THUMB_PREFETCH_WORKERS }, worker))
}

async function prefetchThumbnails(dishes, run, signal) {
  const urls = [...new Set(dishes.map((dish) =>
    dishImage(dish, state.cuisines, state.dishes, 'thumb')
  ).filter(Boolean))]
  const pending = urls.filter((url) => !prefetchedThumbs.has(url))
  if (!pending.length) return
  try {
    await cacheThumbBundle(pending, run, signal)
  } catch (error) {
    if (error?.name === 'AbortError' || run !== prefetchRun) return
    const remaining = pending.filter((url) => !prefetchedThumbs.has(url))
    await prefetchIndividually(remaining, run, signal)
  }
}

function cancelThumbPrefetch() {
  clearTimeout(prefetchTimer)
  prefetchTimer = 0
  if (prefetchIdle && 'cancelIdleCallback' in window) {
    window.cancelIdleCallback(prefetchIdle)
  }
  prefetchIdle = 0
  prefetchAbort?.abort()
  prefetchAbort = null
}

function scheduleThumbPrefetch() {
  const run = ++prefetchRun
  cancelThumbPrefetch()
  const connection = navigator.connection
  const downlink = Number(connection?.downlink)
  const constrained = connection?.saveData ||
    /(^|-)2g$|(^|-)3g$/.test(connection?.effectiveType || '') ||
    (Number.isFinite(downlink) && downlink > 0 && downlink < 1)
  if (constrained) return
  // 只预热当前已渲染批次。首屏先让卡片自己的懒加载抢连接，剩余批次
  // 等滚动扩展 visibleCount 后再预取，避免 237 张图在慢网下阻塞 LCP。
  const snapshot = filtered.value.slice(0, visibleCount.value)
  const controller = new AbortController()
  prefetchAbort = controller
  const start = () => {
    prefetchIdle = 0
    if (run === prefetchRun) void prefetchThumbnails(snapshot, run, controller.signal)
  }
  prefetchTimer = setTimeout(() => {
    if (run !== prefetchRun) return
    if ('requestIdleCallback' in window) {
      prefetchIdle = window.requestIdleCallback(start, { timeout: 1800 })
    } else {
      start()
    }
  }, 500)
}
watch(
  [filtered, visibleCount, () => state.settings.imageMode],
  scheduleThumbPrefetch,
  { immediate: true },
)
watch(thumbCacheEpoch, () => {
  // 恢复备份会同时清空 Blob 与 Cache Storage；这里的 URL 集合也必须
  // 失效，否则会被误判为“已经预取”，导致新图片一直不补。
  prefetchedThumbs.clear()
  scheduleThumbPrefetch()
})
onUnmounted(() => {
  prefetchRun++
  cancelThumbPrefetch()
})

// 切换分类时让网格整体淡入一次（只按分类触发，搜索输入时列表保持不重建）
const gridKey = computed(() => activeCuisine.value)

const visibleDishes = computed(() => filtered.value.slice(0, visibleCount.value))
const hasMore = computed(() => visibleCount.value < filtered.value.length)
const loadMoreEl = ref(null)

function maybeLoadMore() {
  if (!hasMore.value) return
  const el = loadMoreEl.value
  if (el && el.getBoundingClientRect().top < window.innerHeight + 600) {
    visibleCount.value = Math.min(visibleCount.value + PAGE, filtered.value.length)
  }
}

watch(
  () => [activeCuisine.value, keyword.value, onlyFavorite.value, quickFilter.value, priorityPantry.value, state.hiddenCuisines],
  () => { visibleCount.value = PAGE }
)
watch([filtered, visibleCount], () => nextTick(maybeLoadMore))

// 选择困难时「随便来一道」：在当前筛选结果里随机挑一道，直接打开详情
function randomDish() {
  const pool = filtered.value.length ? filtered.value : state.dishes
  if (!pool.length) return
  detailId.value = pool[Math.floor(Math.random() * pool.length)].id
}

// 菜谱很长时提供「回到顶部」：滚动超过 420px 才出现，避免干扰首屏
const showTop = ref(false)
function onScroll() {
  showTop.value = (window.scrollY || 0) > 420
  maybeLoadMore()
}
function scrollTop() { window.scrollTo({ top: 0, behavior: 'smooth' }) }
onMounted(() => {
  // 从首页推荐面板跳过来时，自动打开冰箱食材设置
  try {
    if (sessionStorage.getItem('fm:open-pantry') === '1') {
      sessionStorage.removeItem('fm:open-pantry')
      nextTick(openPantry)
    }
  } catch { /* 隐私模式忽略 */ }
})
function attachScrollListener() {
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
  scheduleThumbPrefetch()
}
function detachScrollListener() { window.removeEventListener('scroll', onScroll) }
onActivated(attachScrollListener)
onDeactivated(() => {
  detachScrollListener()
  prefetchRun++
  cancelThumbPrefetch()
})
onUnmounted(detachScrollListener)

// 当前选中的分类若是被隐藏了，自动退回「全部」
watch(
  () => [state.hiddenCuisines, activeCuisine.value],
  () => {
    if (activeCuisine.value !== 'all' && activeCuisine.value !== 'snack' && isCuisineHidden(activeCuisine.value, state.hiddenCuisines)) {
      activeCuisine.value = 'all'
    }
  }
)

function openAdd() {
  editorDish.value = null
  editorOpen.value = true
}
function openEdit(dish) {
  editorDish.value = dish
  editorOpen.value = true
}
function openDetail(dish) {
  detailId.value = dish.id
}
function editFromDetail(dish) {
  detailId.value = null
  openEdit(dish)
}

// ---------- 冰箱食材（入口放在菜谱标题后，和菜谱一样用带效果图的卡片挑选）----------
const pantryOpen = ref(false)
const pantryRoot = ref(null)
// 已下单悬浮：组件内部处理餐次分组与订单操作
const orderPanelOpen = ref(false)
const todayOrders = computed(() => state.plan?.orders || [])
function openOrderPanel() { orderPanelOpen.value = true }

// 批量点菜：点「多选」进入选择模式，勾几道一起加入今日菜单
const multiMode = ref(false)
const multiSet = ref(new Set())
function toggleMulti() {
  multiMode.value = !multiMode.value
  multiSet.value = new Set()
}
function togglePick(id) {
  const next = new Set(multiSet.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  multiSet.value = next
}
async function addAllPicked() {
  const ids = [...multiSet.value]
  if (!ids.length) return
  try {
    const result = await actions.orderMany(ids.map((dishId) => ({ dishId, meal: state.meal })))
    if (!result) return
    const added = Number(result.added) || 0
    if (added) actions.toast(`已把 ${added} 道菜一次性加进今日菜单 🍽️`, 'success')
    else actions.toast('这些菜已经在今日菜单里了', 'warn')
  } finally {
    multiSet.value = new Set()
    multiMode.value = false
  }
}

const pantryDraft = ref(new Set(state.settings.pantry || []))
// 食材 → 加入日期（YYYY-MM-DD），用于「放了几天」提醒
const pantryAdded = ref({ ...(state.settings.pantryAdded || {}) })
const pantryStockDraft = ref(JSON.parse(JSON.stringify(state.settings.pantryStock || {})))
const todayStr = () => state.today || new Date().toISOString().slice(0, 10)
function daysInFridge(name) {
  const d = pantryAdded.value[name]
  if (!d) return 0
  const t = new Date(`${todayStr()}T12:00:00`).getTime()
  const s0 = new Date(`${d}T12:00:00`).getTime()
  if (!Number.isFinite(t) || !Number.isFinite(s0)) return 0
  return Math.max(0, Math.round((t - s0) / 86400000))
}
const UNIT_OPTIONS = ['g', 'kg', 'ml', 'l', '个', '根', '袋', '盒', '瓶', '把', '块', '片']
function stockOf(name) {
  if (!pantryStockDraft.value[name]) pantryStockDraft.value[name] = { quantity: 0, unit: 'g', added: pantryAdded.value[name] || todayStr(), expires: '' }
  return pantryStockDraft.value[name]
}
function suggestedExpiry(name) {
  const group = pantryGroups.find((item) => item.items.includes(name))?.key || ''
  const days = { meat: 3, seafood: 2, dairy: 7, veg: 7, fruit: 10, mushroom: 5, staple: 30 }[group] || 7
  const d = new Date(`${todayStr()}T12:00:00`)
  d.setDate(d.getDate() + days)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
function fillSuggestedExpiry(name) {
  const stock = stockOf(name)
  stock.expires = suggestedExpiry(name)
  scheduleSavePantry()
}
function expiryOf(name) {
  return pantryExpiry(stockOf(name), todayStr())
}
function expiryLabel(name) {
  const stateValue = expiryOf(name)
  if (!stateValue) return ''
  if (stateValue.days < 0) return `已过期 ${Math.abs(stateValue.days)} 天`
  if (stateValue.days === 0) return '今天到期'
  return `${stateValue.days} 天后到期`
}
// 拖动排序：拖动已选卡片调整顺序，常用的放前面
const dragName = ref('')
function onDragStart(name) { dragName.value = name }
function onDragOver(name, e) {
  e.preventDefault()
  const from = dragName.value
  if (!from || from === name) return
  const list = [...pantryDraft.value]
  const fi = list.indexOf(from), ti = list.indexOf(name)
  if (fi < 0 || ti < 0) return
  list.splice(ti, 0, list.splice(fi, 1)[0])
  pantryDraft.value = new Set(list)
  dragName.value = name
}
function onDragEnd() { dragName.value = ''; scheduleSavePantry() }

// 触屏拖动排序：HTML5 drag 在 iOS/安卓 WebView 不触发，用 pointer 事件补一套。
// 长按 300ms 激活 → 跟手移动 → 手指下的卡片即目标位 → 松手保存。
const touchDrag = ref(null) // { name, startX, startY, dx, dy }
let pressTimer = null
let downInfo = null
function tDown(name, e) {
  if (e.pointerType === 'mouse') return
  downInfo = { name, x: e.clientX, y: e.clientY }
  clearTimeout(pressTimer)
  pressTimer = setTimeout(() => {
    touchDrag.value = { name, startX: e.clientX, startY: e.clientY, dx: 0, dy: 0 }
    navigator.vibrate?.(14)
  }, 300)
}
function tMove(e) {
  if (!downInfo) return
  if (!touchDrag.value) {
    if (Math.hypot(e.clientX - downInfo.x, e.clientY - downInfo.y) > 8) clearTimeout(pressTimer)
    return
  }
  if (e.cancelable) e.preventDefault()
  touchDrag.value = { ...touchDrag.value, dx: e.clientX - touchDrag.value.startX, dy: e.clientY - touchDrag.value.startY }
  const hit = document.elementFromPoint(e.clientX, e.clientY)?.closest?.('[data-pantry-name]')
  const target = hit?.getAttribute('data-pantry-name')
  if (target && target !== touchDrag.value.name) {
    const list = [...pantryDraft.value]
    const fi = list.indexOf(touchDrag.value.name)
    const ti = list.indexOf(target)
    if (fi >= 0 && ti >= 0 && fi !== ti) {
      list.splice(ti, 0, list.splice(fi, 1)[0])
      pantryDraft.value = new Set(list)
    }
  }
}
function tUp() {
  clearTimeout(pressTimer)
  if (touchDrag.value) scheduleSavePantry()
  touchDrag.value = null
  downInfo = null
}
const touchStyle = (name) => (touchDrag.value?.name === name
  ? { transform: `translate(${touchDrag.value.dx}px, ${touchDrag.value.dy}px) scale(1.045)`, zIndex: 6 }
  : null)
const pantryCustom = ref('')
const pantryQuery = ref('')
useModalBehavior(() => { pantryOpen.value = false }, pantryRoot, pantryOpen)

// 106 种常见生鲜/储存食材分 7 类；图片、搜索和冰箱分层共用同一份预设。
// 搜索驱动：默认只显示「已选食材」，输入关键词才展示匹配结果，不把整个目录铺满。
const pantrySearching = computed(() => !!pantryQuery.value.trim())
const shownPantryGroups = computed(() => {
  const q = pantryQuery.value.trim().toLowerCase()
  if (!q) return []
  return pantryGroups
    .map((g) => ({ ...g, items: g.items.filter((n) => n.toLowerCase().includes(q)) }))
    .filter((g) => g.items.length)
})
const selectedPantryCards = computed(() => [...pantryDraft.value].map((n) => {
  const g = pantryGroups.find((x) => x.items.includes(n))
  const stock = pantryStockDraft.value[n] || null
  return {
    name: n,
    tint: g?.tint || pantryVisual(n).tint,
    group: g?.label || '',
    stock,
    expiry: stock ? pantryExpiry(stock, todayStr()) : null,
    ...pantryVisual(n),
  }
}))
function jumpToGroup(key) {
  const g = pantryGroups.find((x) => x.key === key)
  pantryQuery.value = g ? (g.label.replace(/^\S+\s*/, '').slice(0, 2) || '') : ''
}
function clearPantry() {
  pantryDraft.value = new Set()
  pantryAdded.value = {}
  pantryStockDraft.value = {}
  scheduleSavePantry()
}

// 预设食材按分类展示（见 pantryGroups）；这里只算出「自定义」的那些，单独一组。
const pantryCustomCards = computed(() => {
  const q = pantryQuery.value.trim().toLowerCase()
  return [...pantryDraft.value]
    .filter((n) => !pantryGroups.some((g) => g.items.includes(n)))
    .filter((n) => !q || n.toLowerCase().includes(q))
    .map((n) => pantryVisual(n))
})
function openPantry() {
  pantryDraft.value = new Set(state.settings.pantry || [])
  pantryAdded.value = { ...(state.settings.pantryAdded || {}) }
  pantryStockDraft.value = JSON.parse(JSON.stringify(state.settings.pantryStock || {}))
  for (const name of pantryDraft.value) {
    if (!pantryStockDraft.value[name]) {
      pantryStockDraft.value[name] = {
        quantity: 0, unit: 'g', added: pantryAdded.value[name] || todayStr(), expires: suggestedExpiry(name),
      }
    }
  }
  pantryCustom.value = ''
  pantryQuery.value = ''
  orderPanelOpen.value = false
  pantryOpen.value = true
}
// 实时保存：勾选后 500ms 自动写盘，无需点「保存食材」。
// 首页推荐 watch 的是 state.settings.pantry，保存后立刻重排。
let pantryTimer = null
function scheduleSavePantry() {
  clearTimeout(pantryTimer)
  pantryTimer = setTimeout(() => {
    actions.saveSettings({
      pantry: [...pantryDraft.value],
      pantryAdded: pantryAdded.value,
      pantryStock: pantryStockDraft.value,
    })
  }, 500)
}
onUnmounted(() => clearTimeout(pantryTimer))

function togglePantry(name) {
  const next = new Set(pantryDraft.value)
  const added = { ...pantryAdded.value }
  if (next.has(name)) {
    next.delete(name)
    delete added[name]
    delete pantryStockDraft.value[name]
  } else {
    next.add(name)
    added[name] = todayStr() // 新放的记今天，用于保质期提醒
    pantryStockDraft.value[name] = {
      quantity: 0,
      unit: 'g',
      added: added[name],
      expires: suggestedExpiry(name),
    }
  }
  pantryDraft.value = next
  pantryAdded.value = added
  scheduleSavePantry()
}
// 单样食材能被多少道菜用到：点冰箱里的食材时给出「N 道菜用得上」的反馈
const dishCountCache = computed(() => {
  const map = {}
  for (const d of state.dishes) {
    const ings = recipeIngredients(d)
    const seen = new Set()
    for (const raw of ings) {
      const n = normalizeIngredient(raw)
      if (n && !seen.has(n)) { seen.add(n); map[n] = (map[n] || 0) + 1 }
    }
  }
  return map
})
const dishCountOf = (name) => dishCountCache.value[normalizeIngredient(name)] || 0

// 投递给冰箱做「飞入」动画
const dropPayload = ref(null)
function sendDrop(name, event) {
  const r = event?.currentTarget?.getBoundingClientRect?.()
  if (!r) return
  dropPayload.value = { name, x: r.left + r.width / 2, y: r.top + r.height / 2, t: Date.now() }
}

// 冰箱贴 / 开门音效 / 食材点击联动
function onFridgeSticker(key) {
  if (key === 'ding') { playFridgeDing(); return }
  if (key === 'menu') { pantryOpen.value = false; state.tab = 'menu' }
  if (key === 'shopping') { pantryOpen.value = false; state.tab = 'today' }
  if (key === 'week') { pantryOpen.value = false; state.tab = 'today' }
}
function onFoodClick(name) {
  const n = dishCountOf(name)
  actions.toast(n ? `${name}：${n} 道菜用得上，搜一下看看` : `${name}：暂时没有菜用到它`, n ? 'success' : 'info')
  pantryQuery.value = name
}

// 极轻的开箱「叮」：Web Audio 合成，不引素材；跟随全局静音设置
let audioCtx = null
function playFridgeDing() {
  try {
    if (state.settings?.soundEnabled === false) return
    if (typeof window === 'undefined') return
    audioCtx = audioCtx || new (window.AudioContext || window.webkitAudioContext)()
    if (audioCtx.state === 'suspended') audioCtx.resume()
    const now = audioCtx.currentTime
    const osc = audioCtx.createOscillator()
    const gain = audioCtx.createGain()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(1180, now)
    osc.frequency.exponentialRampToValueAtTime(760, now + 0.09)
    gain.gain.setValueAtTime(0.0001, now)
    gain.gain.exponentialRampToValueAtTime(0.05, now + 0.012)
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.16)
    osc.connect(gain).connect(audioCtx.destination)
    osc.start(now); osc.stop(now + 0.18)
  } catch { /* 浏览器不支持音频时静默 */ }
}

// 实时反馈：当前勾选的食材能匹配到多少道菜，让用户知道选了有用
const pantryHitCount = computed(() => {
  if (!pantryDraft.value.size) return 0
  return state.dishes.filter((d) =>
    pantryMatches(d, [...pantryDraft.value], recipeIngredients).count > 0,
  ).length
})
// 语音输入：一句话说出冰箱里的东西（Chrome/Edge 支持，Safari/iOS 部分支持）
const listening = ref(false)
const speechSupported = typeof window !== 'undefined' && ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window)
let recognizer = null
function startVoice() {
  if (!speechSupported) {
    actions.toast('这个浏览器不支持语音，可以手打「土豆、番茄、鸡蛋」批量添加', 'warn')
    return
  }
  if (listening.value) { recognizer?.stop(); return }
  const Ctor = window.SpeechRecognition || window.webkitSpeechRecognition
  recognizer = new Ctor()
  recognizer.lang = 'zh-CN'
  recognizer.interimResults = false
  recognizer.maxAlternatives = 1
  listening.value = true
  recognizer.onresult = (e) => {
    const text = e.results?.[0]?.[0]?.transcript || ''
    if (text) addBatchPantry(text)
  }
  recognizer.onerror = () => { listening.value = false; actions.toast('没听清，再说一次或直接手打', 'error') }
  recognizer.onend = () => { listening.value = false }
  try { recognizer.start() } catch { listening.value = false }
}
onUnmounted(() => { try { recognizer?.abort() } catch { /* 忽略 */ } })

// 批量添加：支持「土豆、番茄、鸡蛋」「土豆 番茄 鸡蛋」「土豆和番茄」等
function addBatchPantry(text) {
  const words = String(text || '')
    .split(/[、,，;；\s]+|和|还有|以及/)
    .map((x) => x.trim().replace(/^(冰箱里|有|还?有)/, '').trim())
    .filter((x) => x && x.length <= 12)
  if (!words.length) return
  const next = new Set(pantryDraft.value)
  const added = { ...pantryAdded.value }
  for (const w of words) {
    // 去掉「两斤」「一把」这类数量词，只留食材名
    const name = w.replace(/[0-9一二三四五六七八九十半两]+(斤|个|把|根|颗|头|片|块|袋|盒|瓶)$/g, '').trim() || w
    next.add(name)
    if (!added[name]) added[name] = todayStr()
    if (!pantryStockDraft.value[name]) {
      pantryStockDraft.value[name] = { quantity: 0, unit: 'g', added: added[name], expires: suggestedExpiry(name) }
    }
  }
  pantryDraft.value = next
  pantryAdded.value = added
  scheduleSavePantry()
  actions.toast(`已放进冰箱：${words.join('、')}`, 'success')
}

function addCustomPantry() {
  const words = pantryCustom.value.split(/[、,，\s]+/).map((s) => s.trim()).filter(Boolean)
  const next = new Set(pantryDraft.value)
  const added = { ...pantryAdded.value }
  for (const w of words) {
    const name = w.slice(0, 12)
    if (!name) continue
    next.add(name)
    if (!added[name]) added[name] = todayStr()
    if (!pantryStockDraft.value[name]) {
      pantryStockDraft.value[name] = {
        quantity: 0,
        unit: 'g',
        added: added[name],
        expires: suggestedExpiry(name),
      }
    }
  }
  pantryDraft.value = next
  pantryAdded.value = added
  pantryCustom.value = ''
  scheduleSavePantry()
}
async function savePantry() {
  const ok = await actions.saveSettings({
    pantry: [...pantryDraft.value],
    pantryAdded: pantryAdded.value,
    pantryStock: pantryStockDraft.value,
  })
  if (ok) pantryOpen.value = false
}
</script>

<template>
  <section>
    <div class="section-head menu-head">
      <div>
        <h2 class="section-title">📖 家里的菜谱</h2>
        <div class="section-sub">{{ state.dishes.length }} 道菜 · {{ state.cuisines.length }} 个菜系，菜品、用量和配图都能自己改</div>
        <div class="menu-order-context"><span aria-hidden="true">🍚</span> 正在为 <b>{{ orderContext }}</b> 点菜</div>
      </div>
      <div class="menu-head-actions">
        <button class="btn btn-soft btn-sm" @click="openPantry">
          🧊 冰箱食材 <span class="tiny">{{ (state.settings.pantry || []).length }}</span>
        </button>
        <button class="btn btn-soft btn-sm" :class="{ 'btn-primary': multiMode }" :aria-pressed="multiMode" @click="toggleMulti">
          {{ multiMode ? '退出多选' : '☑ 多选' }}
        </button>
        <button class="btn btn-primary" @click="openAdd">＋ 添加菜品</button>
      </div>
    </div>

    <div class="cuisine-strip">
      <button class="chip cuisine-chip" :class="{ active: activeCuisine === 'all' }" @click="activeCuisine = 'all'">
        🍽️ 全部 <span class="tiny">{{ visibleDishCount }}</span>
      </button>
      <button
        class="chip cuisine-chip" :class="{ active: activeCuisine === 'snack', 'is-empty': !snackCount }"
        title="干炸响铃、虾饺、小笼包这类解馋小食"
        @click="activeCuisine = 'snack'"
      >
        🥟 小吃 <span class="tiny">{{ snackCount }}</span>
      </button>
      <button
        v-for="c in visibleCuisines" :key="c.key"
        class="chip cuisine-chip" :class="{ active: activeCuisine === c.key, 'is-empty': !counts[c.key] }"
        @click="activeCuisine = c.key"
        :title="c.desc"
      >
        {{ c.icon }} {{ c.name }} <span class="tiny">{{ counts[c.key] || 0 }}</span>
      </button>
    </div>

    <div class="toolbar">
      <div class="quick-filters">
      <button
        v-for="f in [
          { key: 'quick', label: '⏱ 30分钟内' },
          { key: 'mild', label: '🌶️ 不辣' },
          { key: 'image', label: '🖼️ 有图' },
        ]" :key="f.key" class="chip quick-chip" :class="{ active: quickFilter === f.key }"
        :aria-pressed="quickFilter === f.key" @click="toggleQuick(f.key)"
      >{{ f.label }}</button>
    </div>

    <div class="search-box">
        <input v-model="keyword" class="input" placeholder="搜菜名、口味、配料…" aria-label="搜索菜品" />
        <button v-if="keyword" class="search-clear" type="button" aria-label="清空搜索" @click="keyword = ''">✕</button>
      </div>
      <button class="chip" :class="{ active: onlyFavorite }" :aria-pressed="onlyFavorite" @click="onlyFavorite = !onlyFavorite">
        ⭐ 只看收藏
      </button>
      <button
        class="chip" :class="{ active: priorityPantry }" :aria-pressed="priorityPantry"
        title="按冰箱已选的食材把能做的菜排到前面；盐、油、葱蒜等佐料不参与"
        @click="togglePriority"
      >🥬 优先推荐</button>
      <button class="chip" type="button" title="在当前筛选结果里随机挑一道" @click="randomDish">🎲 随便来一道</button>
      <!-- 状态行常驻，避免筛选后才新增一行而把下方网格整体推下去。 -->
      <span class="tiny muted menu-filter-status">
        <template v-if="priorityPantry && !(state.settings.pantry || []).length">先点「🧊 冰箱食材」选几样</template>
        <template v-else>
          {{ hasFilter ? '找到' : '共' }}
          <span class="value-pop" :key="filtered.length">{{ filtered.length }}</span> 道
        </template>
      </span>
    </div>

    <!-- 不用 TransitionGroup：切换菜系时整片卡片进出场 + move 会让画面明显晃一下。
         增删菜品是低频操作，稳定的静态网格比 0.3s 动画更重要。 -->
    <div v-if="filtered.length" :key="gridKey" class="dish-grid grid-in">
      <DishCard
        v-for="(d, i) in visibleDishes" :key="d.id" :dish="d" :highlight="keyword"
        :multi="multiMode" :multi-picked="multiSet.has(d.id)"
        :priority="i < 4"
        @multi-toggle="togglePick(d.id)" @edit="openEdit" @open="openDetail"
      />
    </div>
    <!-- 分批渲染的哨兵：滚动到附近就自动补下一批，不打断浏览 -->
    <div v-if="filtered.length" ref="loadMoreEl" class="grid-more" :class="{ 'is-done': !hasMore }">
      <template v-if="hasMore">正在加载更多…（{{ visibleCount }} / {{ filtered.length }}）</template>
      <template v-else>共 {{ filtered.length }} 道，已经到底啦</template>
    </div>
    <Transition name="pop" mode="out-in">
      <div v-if="!filtered.length" key="empty" class="empty">
        <span class="empty-icon">{{ onlyFavorite && !keyword.trim() ? '⭐' : '🥕' }}</span>
        <template v-if="onlyFavorite && !keyword.trim()">还没有收藏的菜，点菜品卡片右上角的 ☆ 试试</template>
        <template v-else>没找到合适的菜，换个词或配料试试，也可以自己加一道</template>
      </div>
    </Transition>

    <Transition name="modal">
      <DishDetail
        v-if="detailDish" :dish="detailDish"
        @close="detailId = null" @edit="editFromDetail"
      />
    </Transition>
    <Transition name="modal">
      <DishEditor v-if="editorOpen" :dish="editorDish" @close="editorOpen = false" />
    </Transition>

    <!-- 已下单悬浮入口：必须 Teleport 到 body，
         页面切换动画的 .page-enter-active 带 transform 会把 fixed 捕获成相对定位 -->
    <Teleport to="body">
    <button
      v-if="state.tab === 'menu' && !pantryOpen" class="order-fab" type="button"
      :aria-label="`查看已下单的 ${todayOrders.length} 道菜`"
      @click="openOrderPanel"
    >
      🧾 已下单
      <span :key="`fab-${todayOrders.length}`" class="order-fab-count">{{ todayOrders.length }}</span>
    </button>
    </Teleport>

    <OrderPanel v-if="orderPanelOpen" @close="orderPanelOpen = false" />

    <Transition name="pop">
      <button v-if="showTop" class="back-top" type="button" aria-label="回到顶部" @click="scrollTop">↑</button>
    </Transition>

    <!-- 批量点菜浮条 -->
    <Transition name="sheet-up">
      <div v-if="multiMode" class="multi-bar" role="toolbar" aria-label="批量点菜">
        <span class="multi-count">已选 <b>{{ multiSet.size }}</b> 道</span>
        <span class="grow"></span>
        <button class="btn btn-sm" @click="toggleMulti">取消</button>
        <button class="btn btn-primary btn-sm" :disabled="!multiSet.size || state.busy > 0" @click="addAllPicked">
          {{ state.busy > 0 ? '点菜中…' : '＋ 加入今日菜单' }}
        </button>
      </div>
    </Transition>

    <Transition name="modal">
      <div v-if="pantryOpen" class="modal-backdrop" @click.self="pantryOpen = false">
        <div ref="pantryRoot" class="modal modal-wide pantry-modal" role="dialog" aria-modal="true" aria-label="冰箱食材">
          <div class="modal-head">
            <h2>🧊 冰箱里有什么</h2>
            <button class="icon-btn" aria-label="关闭" @click="pantryOpen = false">✕</button>
          </div>
          <PantryFridge
            :items="[...pantryDraft]" :visual-of="pantryVisual" :hit-count="pantryHitCount"
            :days-of="daysInFridge" :dish-count-of="dishCountOf" :drop="dropPayload"
            @sticker="onFridgeSticker" @food-click="onFoodClick"
          />
          <p class="setting-desc">
            点选家里现有的食材，会按冷藏、生鲜托盘和保鲜抽屉真实摆放；「今天吃啥」和菜谱「优先推荐」也会立刻按它们重排。佐料不参与推荐。
          </p>
          <div class="pantry-toolbar">
            <input
              v-model="pantryQuery" class="input" placeholder="搜食材，例如：排骨、番茄…"
              aria-label="搜索食材" maxlength="20"
            />
            <button class="btn btn-soft btn-sm" :disabled="!pantryDraft.size" @click="clearPantry">清空已选</button>
          </div>
          <div class="pantry-jumps">
            <span class="tiny muted">试试：</span>
            <button v-for="q in ['番茄', '排骨', '鸡蛋', '青菜', '牛奶']" :key="q" type="button" class="chip" @click="pantryQuery = q">{{ q }}</button>
            <button v-if="pantryQuery" class="chip" type="button" @click="pantryQuery = ''">✕ 清除</button>
          </div>
          <div class="pantry-feedback" aria-live="polite">
            <Transition name="duty-pop">
              <span :key="pantryDraft.size + '-' + pantryHitCount">
                🥬 已选 <b>{{ pantryDraft.size }}</b> 样食材，菜谱里有 <b>{{ pantryHitCount }}</b> 道菜用得上
              </span>
            </Transition>
          </div>
          <div v-if="!pantrySearching && !selectedPantryCards.length" class="pantry-no-result tiny muted">冰箱里还没食材。搜一个，或在下面自定义添加。</div>
          <div v-if="pantrySearching && !shownPantryGroups.length && !pantryCustomCards.length" class="pantry-no-result tiny muted">没找到「{{ pantryQuery }}」，可在下面「添加自定义食材」里加一个。</div>
          <div v-if="!pantrySearching && selectedPantryCards.length" class="pantry-section">
            <div class="pantry-group-head">🧊 冰箱里已选（{{ selectedPantryCards.length }}）</div>
            <div class="pantry-grid">
              <button
                v-for="p in selectedPantryCards" :key="p.name" type="button"
                class="pantry-card active"
                :class="{ 'is-dragging': dragName === p.name || touchDrag?.name === p.name }"
                draggable="true"
                :data-pantry-name="p.name"
                :style="[{ '--group-tint': p.tint }, touchStyle(p.name)]"
                :aria-label="`从冰箱移除 ${p.name}，可拖动排序`"
                @click="touchDrag?.name === p.name ? null : togglePantry(p.name)"
                @dragstart="onDragStart(p.name)" @dragover="onDragOver(p.name, $event)"
                @dragend="onDragEnd" @drop.prevent="onDragEnd"
                @pointerdown="tDown(p.name, $event)" @pointermove="tMove($event)"
                @pointerup="tUp" @pointercancel="tUp"
              >
                <span class="pantry-art" :style="{ background: p.tint }"><PantryImage :name="p.name" eager /></span>
                <span class="pantry-card-name">{{ p.name }}</span>
                <!-- 放了几天：7 天转黄、14 天转红 -->
                <span
                  v-if="pantryAdded[p.name]"
                  class="pantry-age" :class="{ warn: daysInFridge(p.name) >= 7, old: daysInFridge(p.name) >= 14 }"
                  :title="daysInFridge(p.name) > 0 ? `放进冰箱第 ${daysInFridge(p.name)} 天` : '今天放进冰箱的'"
                >{{ daysInFridge(p.name) > 0 ? `${daysInFridge(p.name)}天` : '今天' }}</span>
                <span
                  v-if="p.stock?.expires" class="pantry-expiry"
                  :class="p.expiry?.status"
                  :title="expiryLabel(p.name)"
                >{{ p.expiry?.days < 0 ? '已过期' : p.expiry?.days === 0 ? '今天到期' : `${p.expiry?.days}天` }}</span>
                <span class="pantry-check">✓</span>
              </button>
            </div>
          </div>
          <details v-if="selectedPantryCards.length" class="pantry-stock-editor">
            <summary>⚖️ 库存数量与保质期 <span class="tiny muted">可选，用于整周采购自动扣减和临期提醒</span></summary>
            <div class="pantry-stock-list">
              <div v-for="p in selectedPantryCards" :key="`stock-${p.name}`" class="pantry-stock-row">
                <span class="pantry-stock-name">{{ p.name }}</span>
                <input
                  v-model.number="p.stock.quantity" class="input pantry-stock-qty"
                  type="number" min="0" max="100000" step="1"
                  :aria-label="`${p.name}库存数量`" @change="scheduleSavePantry"
                />
                <select v-model="p.stock.unit" class="select pantry-stock-unit" :aria-label="`${p.name}库存单位`" @change="scheduleSavePantry">
                  <option v-for="unit in UNIT_OPTIONS" :key="unit" :value="unit">{{ unit }}</option>
                </select>
                <input
                  v-model="p.stock.expires" class="input pantry-stock-expiry"
                  type="date" :aria-label="`${p.name}保质期`" @change="scheduleSavePantry"
                />
                <button class="btn btn-ghost btn-sm" type="button" @click="fillSuggestedExpiry(p.name)">建议日期</button>
                <span class="pantry-stock-state" :class="p.expiry?.status">{{ expiryLabel(p.name) }}</span>
              </div>
            </div>
          </details>
          <div v-for="g in shownPantryGroups" :key="g.key" :id="'pantry-group-' + g.key" class="pantry-section">
            <div class="pantry-group-head" :style="{ borderColor: g.tint }">{{ g.label }}</div>
            <div class="pantry-grid">
              <button
                v-for="p in g.items" :key="p" type="button"
                class="pantry-card" :class="{ active: pantryDraft.has(p) }"
                :style="{ '--group-tint': g.tint }"
                :aria-pressed="pantryDraft.has(p)" @click="togglePantry(p); !pantryDraft.has(p) && sendDrop(p, $event)"
              >
                <span class="pantry-art" :style="{ background: pantryVisual(p).tint }">
                  <PantryImage :name="p" />
                </span>
                <span class="pantry-card-name">{{ p }}</span>
                <span v-if="pantryDraft.has(p)" class="pantry-check">✓</span>
              </button>
            </div>
          </div>
          <div v-if="pantryCustomCards.length" class="pantry-section">
            <div class="pantry-group-head">✏️ 自定义食材</div>
            <div class="pantry-grid">
              <button
                v-for="p in pantryCustomCards" :key="p.name" type="button"
                class="pantry-card" :class="{ active: pantryDraft.has(p.name) }"
                :style="{ '--group-tint': p.tint }"
                :aria-pressed="pantryDraft.has(p.name)" @click="togglePantry(p.name)"
              >
                <span class="pantry-art" :style="{ background: p.tint }">
                  <PantryImage :name="p.name" />
                </span>
                <span class="pantry-card-name">{{ p.name }}</span>
                <span v-if="pantryDraft.has(p.name)" class="pantry-check">✓</span>
              </button>
            </div>
          </div>
          <div class="pantry-add-row">
            <input
              v-model="pantryCustom" class="input" maxlength="80"
              placeholder="一句话批量添加：土豆、番茄、鸡蛋…"
              aria-label="批量添加食材"
              @keyup.enter="addBatchPantry(pantryCustom); pantryCustom = ''"
            />
            <button
              class="btn btn-sm mic-btn" :class="{ 'is-listening': listening }"
              :disabled="!speechSupported"
              :title="speechSupported ? '说出冰箱里的食材，一句多个' : '当前浏览器不支持语音'"
              @click="startVoice"
            >{{ listening ? '🎙️' : '🎤' }}</button>
            <span v-if="listening" class="voice-wave" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
            <button class="btn btn-soft btn-sm" @click="addBatchPantry(pantryCustom); pantryCustom = ''">添加</button>
          </div>
          <div class="modal-foot">
            <span class="tiny muted">✓ 勾选后自动保存，首页推荐即时更新</span>
            <button class="btn btn-primary" @click="pantryOpen = false">完成</button>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>
