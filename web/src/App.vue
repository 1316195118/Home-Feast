<script setup>
import { reactive, provide, onMounted, onUnmounted, watch, ref, computed, defineAsyncComponent } from 'vue'
import { api, setAccessCode, getClientId } from './api'
import TodayView from './components/TodayView.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ViewLoading from './components/ViewLoading.vue'
import ViewLoadError from './components/ViewLoadError.vue'
import { useModalBehavior } from './modal'
import { currentMeal, normalizeMeals } from './utils'
import { playAdd, playFav, playSave, playRemove, playError } from './audio'

// 首屏只需要「今天」。其余页签按需下载，避免第一次打开时把设置、
// 历史和编辑器等暂时用不到的代码一起解析。200ms 内加载完成就不闪加载态。
const viewLoaders = {
  menu: () => import('./components/MenuView.vue'),
  family: () => import('./components/FamilyView.vue'),
  history: () => import('./components/HistoryView.vue'),
  settings: () => import('./components/SettingsView.vue'),
}
const lazyView = (key) => defineAsyncComponent({
  loader: viewLoaders[key],
  delay: 200,
  loadingComponent: ViewLoading,
  errorComponent: ViewLoadError,
  timeout: 15000,
  onError(_error, retry, fail, attempts) {
    if (attempts <= 1) setTimeout(retry, 300)
    else fail()
  },
})
const MenuView = lazyView('menu')
const FamilyView = lazyView('family')
const HistoryView = lazyView('history')
const SettingsView = lazyView('settings')
// 鼠标移到页签或键盘聚焦时就预取，真正点击时通常已经准备好；
// 不抢占首屏，因为菜单、家人等模块仍要等用户表现出访问意图才下载。
function prefetchView(key) {
  viewLoaders[key]?.().catch(() => {})
}
let idlePrefetchDone = false
function scheduleIdlePrefetch() {
  if (idlePrefetchDone) return
  idlePrefetchDone = true
  // 手机首屏稳定后再悄悄取菜谱页，第一次切换通常能直接出现。
  // 省流量模式和 2G/慢速网络不预取，避免为了「可能访问」抢带宽。
  const conn = navigator.connection
  if (navigator.onLine === false || conn?.saveData || /^(slow-2g|2g)$/.test(conn?.effectiveType || '')) return
  const run = () => prefetchView('menu')
  if ('requestIdleCallback' in window) {
    window.requestIdleCallback(run, { timeout: 2200 })
  } else {
    setTimeout(run, 1200)
  }
}

const tabs = [
  { key: 'today', label: '今天', icon: '🍚' },
  { key: 'menu', label: '菜谱', icon: '📖' },
  { key: 'family', label: '家人', icon: '👨‍👩‍👧' },
  { key: 'history', label: '记录', icon: '🗓️' },
  { key: 'settings', label: '设置', icon: '⚙️' },
]

// 本机还没记住身份时，首次打开主动让用户选一次
const needsUserPick = ref(false)

// PWA 新版本待更新：main.js 检测到 sw 处于 waiting 时广播 'sw-update'，这里弹提示条
const updateReady = ref(false)
function onSwUpdate() { updateReady.value = true }
function applyUpdate() { window.dispatchEvent(new CustomEvent('sw-reload')) }

// 用本地时区拼日期，不能用 toISOString()（UTC 会差一天导致跨天误判）
function localDate(d = new Date()) {
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

const state = reactive({
  ready: false,
  locked: false,
  accessRequired: false,
  accessEnvManaged: false, // 口令由环境变量控制时，网页里不能改
  bootError: '',
  toast: null,       // { id, text, type }
  busy: 0,           // 进行中的写操作数量，用于顶部进度条与防连点
  tab: 'today',
  version: '',
  revision: 0,
  releaseNote: '',   // 本次构建的更新说明，随二进制一起编译进来
  buildTime: '',     // 构建时间，用来判断线上跑的是哪一版
  today: '',
  meal: currentMeal(),   // 当前所处的餐别 key，随时间自动跟随（见 syncNow）
  mealPick: '',          // 用户手动指定的餐别；空串表示跟随当前时段（全站点菜入口共用）
  now: Date.now(),       // 响应式时间刻度，驱动「已过饭点」标记自动刷新
  meals: [],             // 由 bootstrap 下发的用餐时段配置（名字/时间可自定义）
  settings: { familyName: '我们家' },
  hiddenCuisines: [],
  cuisines: [],
  dishes: [],
  members: [],
  plan: { orders: [] },
  plans: [],
  realtime: 'off',     // SSE 连接状态：off / connecting / online / reconnecting / unsupported
  currentUserId: localStorage.getItem('fm.user') || '',
})

// 用动态组件 + key 让页签切换能走统一的进出场过渡。
const viewMap = { today: TodayView, menu: MenuView, family: FamilyView, history: HistoryView, settings: SettingsView }
const currentView = computed(() => viewMap[state.tab] || TodayView)

// 页签按左右方向切换：往右滑入、往左滑出，方向感让层级更好理解。
const pageDirection = ref('page')
watch(() => state.tab, (next, prev) => {
  const a = tabs.findIndex((t) => t.key === prev)
  const b = tabs.findIndex((t) => t.key === next)
  pageDirection.value = b >= a ? 'page' : 'page-back'
})

// 支持 #today / #menu 之类的直达链接，刷新后也停在原来那一页。
const hashTab = window.location.hash.replace('#', '')
if (tabs.some((t) => t.key === hashTab)) state.tab = hashTab

const codeInput = ref('')
const lockRoot = ref(null)
const userPickRoot = ref(null)
// 口令锁与首次选人都不允许 Esc 跳过，但仍锁背景滚动、接管焦点
useModalBehavior(() => {}, lockRoot, computed(() => state.locked), false)
useModalBehavior(
  () => {}, userPickRoot,
  computed(() => needsUserPick.value && state.ready && !state.locked), false,
)

// 成员密码：切到设了密码的身份之前先校验，避免家人随手用别人名义点菜。
// 密码哈希只在服务端 pins.json，前端只拿到 hasPin 布尔，不接触任何密码材料。
const pinPrompt = ref(null) // { id, name, avatar, purpose: 'switch' | 'pick' }
const pinInput = ref('')
const pinError = ref('')
const pinBusy = ref(false)
const pinRoot = ref(null)
useModalBehavior(
  () => { pinPrompt.value = null },
  pinRoot,
  computed(() => !!pinPrompt.value),
  true, // 可以取消，取消就保持原身份
)
function openPinPrompt(member, purpose) {
  pinPrompt.value = { id: member.id, name: member.name, avatar: member.avatar, purpose }
  pinInput.value = ''
  pinError.value = ''
}
async function submitPin() {
  const p = pinPrompt.value
  if (!p || pinBusy.value) return
  if (!pinInput.value) { pinError.value = '先输入密码'; return }
  pinBusy.value = true
  pinError.value = ''
  try {
    const res = await api.verifyMemberPin(p.id, pinInput.value)
    if (!res.ok) {
      pinError.value = '密码不对，再试一次'
      pinInput.value = ''
      toast('密码不对，身份没切换', 'error')
      return
    }
    pinPrompt.value = null
    pinInput.value = ''
    if (p.purpose === 'pick') doPick(p.id)
    else doSwitch(p.id)
  } catch (err) {
    pinError.value = err.message || '校验失败'
  } finally {
    pinBusy.value = false
  }
}

let toastTimer = null
let syncTimer = null
let toastSeq = 0

// type: success / error / warn / info。带 id 便于连续弹同类提示时重播动画。
// action：{ label, run } —— 用于「撤销」这类可回退操作，停留时间也放宽
function toast(message, type = 'info', action = null) {
  state.toast = { id: ++toastSeq, text: message, type, action }
  if (type === 'error') playError() // 失败提示配合抖动动画与低沉两音
  clearTimeout(toastTimer)
  const hold = action ? 5200 : (type === 'error' ? 3400 : 2600)
  toastTimer = setTimeout(() => { state.toast = null }, hold)
}
function runToastAction() {
  const act = state.toast?.action
  state.toast = null
  clearTimeout(toastTimer)
  act?.run?.()
}

// 网络与实时同步状态：断线时页面还能开（PWA 缓存），但改动不会同步，必须明确告知
const offline = ref(typeof navigator !== 'undefined' && navigator.onLine === false)
function onOffline() {
  offline.value = true
  toast('网络已断开，改动不会同步', 'warn')
}
function onOnline() {
  offline.value = false
  toast('网络已恢复', 'success')
  requestRefresh(0)
}


let refreshInFlight = null
let refreshAgain = false
let refreshTimer = null
let mealBoundaryTimer = null
let scrollRestoreTimer = null
let dataGeneration = 0
let localMutationDepth = 0
let remoteDuringMutation = false
const pageScrollPositions = new Map()

function restorePageScroll(tab) {
  clearTimeout(scrollRestoreTimer)
  const top = pageScrollPositions.get(tab) || 0
  // out-in 会先播放离场再插入新页；等新页结束后再恢复，避免被短暂的空内容钳到 0。
  scrollRestoreTimer = setTimeout(() => {
    if (state.tab === tab) window.scrollTo(0, top)
  }, 340)
}

function requestRefresh(delay = 220) {
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => {
    refreshTimer = null
    refresh().catch(() => {})
  }, Math.max(0, delay))
}

function sameValue(a, b) {
  if (a === b) return true
  try { return JSON.stringify(a) === JSON.stringify(b) } catch { return false }
}

// bootstrap 每次都会传回完整 JSON。若直接整体替换，237 道菜会全部变成新对象，
// DishCard 重渲染、getRecipe 的 WeakMap 缓存也会全丢。这里只替换真正变化的条目，
// 未变化时连数组引用都保留，让 Vue 能跳过整块更新。
function mergeByKey(previous, next, keyOf) {
  if (!Array.isArray(previous) || !Array.isArray(next)) return next
  const oldByKey = new Map(previous.map((item) => [keyOf(item), item]))
  let changed = previous.length !== next.length
  const merged = new Array(next.length)
  for (let i = 0; i < next.length; i++) {
    const item = next[i]
    const old = oldByKey.get(keyOf(item))
    // 顺序本身就是数据：用户调整菜系顺序时，条目对象可能一个都没变，
    // 只是换了位置。若这里仍返回旧数组，界面会一直显示调整前的排序。
    if (old && previous[i] === old && sameValue(old, item)) {
      merged[i] = old
    } else {
      merged[i] = item
      changed = true
    }
  }
  return changed ? merged : previous
}

function applyBootstrap(info) {
  if (
    state.ready
    && state.today === info.today
    && state.version === info.version
    && state.revision === Number(info.revision || 0)
  ) {
    state.now = Date.now()
    state.meal = currentMeal(state.now, state.meals)
    scheduleMealBoundary()
    return
  }
  state.version = info.version
  state.releaseNote = info.releaseNote || state.releaseNote
  state.buildTime = info.buildTime || state.buildTime
  state.accessRequired = !!info.accessRequired
  state.accessEnvManaged = !!info.accessEnvManaged
  state.revision = Number(info.revision || 0)
  state.today = info.today
  const nextMeals = normalizeMeals(info.meals)
  state.meals = sameValue(state.meals, nextMeals) ? state.meals : nextMeals
  // 用户可能在设置里删掉了当前手动选中的那一餐，避免之后点菜提交不存在的 key
  if (state.mealPick && !state.meals.some((m) => m.key === state.mealPick)) state.mealPick = ''
  state.now = Date.now()
  state.meal = currentMeal(state.now, state.meals)
  const nextSettings = info.settings || { familyName: '我们家' }
  state.settings = sameValue(state.settings, nextSettings) ? state.settings : nextSettings
  const nextHidden = nextSettings.hiddenCuisines || []
  state.hiddenCuisines = sameValue(state.hiddenCuisines, nextHidden) ? state.hiddenCuisines : nextHidden
  state.cuisines = mergeByKey(state.cuisines, info.cuisines || [], (c) => c.key)
  state.dishes = mergeByKey(state.dishes, info.dishes || [], (d) => d.id)
  state.members = mergeByKey(state.members, info.members || [], (m) => m.id)
  const nextPlans = mergeByKey(state.plans, info.plans || [], (p) => p.date)
  state.plans = nextPlans
  const planFromList = nextPlans.find((p) => p.date === info.today)
  const nextPlan = info.plan || { date: info.today, orders: [] }
  state.plan = planFromList
    || (state.plan?.date === info.today && sameValue(state.plan, nextPlan) ? state.plan : nextPlan)
  if (!state.members.some((m) => m.id === state.currentUserId)) {
    state.currentUserId = state.members[0]?.id || ''
  }
  state.ready = true
  state.locked = false
  state.bootError = ''
  scheduleIdlePrefetch()
  // 只有「用户主动选过」才会写进 localStorage；这里的兜底不写，避免首次被静默记住
  if (!localStorage.getItem('fm.user') && state.members.length) needsUserPick.value = true
  scheduleMealBoundary()
}

async function refresh() {
  // SSE、回到前台和兜底轮询可能同时发生。只保留一个在途请求；
  // 期间又收到更新就排队再拉一次，避免旧响应覆盖刚提交的新状态。
  if (refreshInFlight) {
    refreshAgain = true
    return refreshInFlight
  }
  refreshInFlight = (async () => {
    do {
      refreshAgain = false
      const generation = dataGeneration
      const info = await api.bootstrap()
      if (generation !== dataGeneration) {
        refreshAgain = true
        continue
      }
      applyBootstrap(info)
    } while (refreshAgain)
  })()
  try {
    return await refreshInFlight
  } finally {
    refreshInFlight = null
    if (refreshAgain) requestRefresh(0)
  }
}

function replacePlanInState(plan) {
  if (!plan?.date || !Array.isArray(plan.orders)) return false
  const index = state.plans.findIndex((p) => p.date === plan.date)
  if (index >= 0) {
    const next = state.plans.slice()
    next[index] = plan
    state.plans = next
  } else if (plan.date === state.today) {
    state.plans = [plan, ...state.plans]
  }
  if (plan.date === state.today) state.plan = plan
  return true
}

function applyDishResult(dish) {
  if (!dish?.id || !dish.cuisine) return false
  const index = state.dishes.findIndex((d) => d.id === dish.id)
  if (index >= 0) {
    const next = state.dishes.slice()
    next[index] = dish
    state.dishes = next
  } else {
    state.dishes = [...state.dishes, dish]
  }
  return true
}

function applyMemberResult(member) {
  if (!member?.id || !member.avatar) return false
  const index = state.members.findIndex((m) => m.id === member.id)
  if (index >= 0) {
    const current = state.members[index]
    const next = state.members.slice()
    next[index] = { ...member, hasPin: member.hasPin ?? current.hasPin }
    state.members = next
  } else {
    state.members = [...state.members, member]
  }
  return true
}

function applySettingsResult(settings) {
  if (!settings || typeof settings !== 'object' || !('familyName' in settings)) return false
  state.settings = settings
  state.hiddenCuisines = settings.hiddenCuisines || []
  state.meals = normalizeMeals(settings.meals)
  state.now = Date.now()
  state.meal = currentMeal(state.now, state.meals)
  if (Array.isArray(settings.cuisineOrder) && settings.cuisineOrder.length && state.cuisines.length) {
    const byKey = new Map(state.cuisines.map((c) => [c.key, c]))
    const seen = new Set()
    const ordered = []
    for (const key of settings.cuisineOrder) {
      const cuisine = byKey.get(key)
      if (cuisine && !seen.has(key)) {
        ordered.push(cuisine)
        seen.add(key)
      }
    }
    for (const cuisine of state.cuisines) {
      if (!seen.has(cuisine.key)) ordered.push(cuisine)
    }
    state.cuisines = ordered
  }
  scheduleMealBoundary()
  return true
}

function applyServerResult(result) {
  return replacePlanInState(result?.plan || result)
    || applyDishResult(result)
    || applyMemberResult(result)
    || applySettingsResult(result)
}

// 显式选择身份：只有用户主动切换/首次选择才持久化
function selectUser(id) {
  state.currentUserId = id
  if (id) localStorage.setItem('fm.user', id)
  else localStorage.removeItem('fm.user')
}

function doPick(id) {
  selectUser(id)
  needsUserPick.value = false
  const m = state.members.find((x) => x.id === id)
  if (m) toast(`${m.avatar} 欢迎回家，${m.name}`, 'success')
}
function pickUser(id) {
  const m = state.members.find((x) => x.id === id)
  if (m?.hasPin) { openPinPrompt(m, 'pick'); return }
  doPick(id)
}

function doSwitch(id) {
  selectUser(id)
  const m = state.members.find((x) => x.id === id)
  if (m) toast(`${m.avatar} 现在以 ${m.name} 的身份点菜`, 'info')
}
function onSwitchUser(e) {
  const id = e.target.value
  const m = state.members.find((x) => x.id === id)
  if (m?.hasPin) {
    // select 的 DOM 值已经变了，先复原；密码通过后再真正切身份
    e.target.value = state.currentUserId
    openPinPrompt(m, 'switch')
    return
  }
  doSwitch(id)
}

// 统一的「确认」弹窗：替换浏览器原生 confirm，保持界面风格一致，也带入场动画。
const confirmState = reactive({
  open: false, title: '', message: '', confirmText: '确定', cancelText: '取消',
  extraText: '', mode: 'confirm', danger: false,
})
let confirmResolve = null

function openConfirm(opts = {}) {
  confirmState.title = opts.title || '请确认'
  confirmState.message = opts.message || ''
  confirmState.confirmText = opts.confirmText || '确定'
  confirmState.cancelText = opts.cancelText || '取消'
  confirmState.extraText = opts.extraText || ''
  confirmState.mode = opts.mode || 'confirm'
  confirmState.danger = !!opts.danger
  confirmState.open = true
  return new Promise((res) => { confirmResolve = res })
}

// 二选一：resolve(true / false)
function confirmDialog(opts = {}) {
  return openConfirm({ ...opts, mode: 'confirm' })
}

// 三选一：resolve('ok' | 'extra' | 'cancel')，用于「同时认领 / 只换 / 取消」
function choiceDialog(opts = {}) {
  return openConfirm({ ...opts, mode: 'choice' })
}

function finishConfirm(result) {
  const mode = confirmState.mode
  const resolve = confirmResolve
  confirmState.open = false
  confirmResolve = null
  if (!resolve) return
  resolve(mode === 'choice' ? result : result === 'ok')
}
function onConfirmOk() { finishConfirm('ok') }
function onConfirmExtra() { finishConfirm('extra') }
function onConfirmCancel() { finishConfirm('cancel') }

// key 相同的写操作会互相防连点；组件可用 pending(key) 把按钮置为 loading/禁用。
const pendingKeys = reactive({})
function pending(key) { return !!pendingKeys[key] }

async function guard(fn, message, key = '', onSuccess = null) {
  if (key && pendingKeys[key]) return null
  if (key) pendingKeys[key] = true
  state.busy++
  localMutationDepth++
  try {
    const result = await fn()
    // 写接口返回的是完整结果。先原地更新，让弹窗/按钮立即收起，再让页面继续响应；
    // 不再为了本机一次写入阻塞式重拉整个首屏。
    dataGeneration++
    applyServerResult(result)
    onSuccess?.(result)
    if (message) toast(message, 'success')
    return result === undefined ? true : result
  } catch (err) {
    toast(err.message || '操作失败，请稍后再试', 'error')
    return null
  } finally {
    localMutationDepth--
    state.busy--
    if (key) delete pendingKeys[key]
    if (remoteDuringMutation && localMutationDepth === 0) {
      remoteDuringMutation = false
      // 这次写操作期间收到的远端变更，由下面这一次刷新统一吸收；
      // 同时关闭 SSE 待处理标记，避免回到前台时再刷新第二遍。
      ssePending = false
      clearTimeout(sseTimer)
      requestRefresh(500)
    }
  }
}

// 分工反馈写具体名字，不再用「TA」这种指代
function dutyName(id) {
  return state.members.find((m) => m.id === id)?.name || '家里人'
}

// 业务动作的音效：只在写操作真的成功后播一次，失败由 toast('error') 播 playError。
// 统一用 async 包一层 guard，避免把音效塞进每个组件的模板里。
function withSound(guardPromise, sound) {
  return guardPromise.then((ok) => { if (ok) sound(); return ok })
}

// 服务端单次最多 80 道。超过时按 80 一批顺序提交，仍远少于逐道请求；
// 每次返回的计划逐步合并，最后一次就是完整结果。
async function addOrdersInBatches(items) {
  let added = 0
  let skipped = 0
  let plan = null
  for (let i = 0; i < items.length; i += 80) {
    const result = await api.addOrders(state.today, {
      userId: state.currentUserId,
      items: items.slice(i, i + 80),
    })
    added += Number(result.added) || 0
    skipped += Number(result.skipped) || 0
    plan = result.plan || plan
  }
  return { plan, added, skipped }
}

const actions = {
  toast,
  refresh: () => refresh(),
  applyPlan: (plan) => {
    dataGeneration++
    return replacePlanInState(plan)
  },
  addDish: (dish) => guard(() => api.addDish(dish), '菜品已加入菜谱', 'dish'),
  updateDish: (id, patch) => guard(() => api.updateDish(id, patch), '已保存', 'dish'),
  // 删除类操作都留一次「撤销」：菜品删错要重传配图，代价比弹确认框高得多
  // 「再来一桌」：一次事务把某天点过的菜加回今天，跳过已删/已存在的菜。
  reorderDay: async (plan) => {
    const items = (plan?.orders || []).filter((o) => o.dishId)
    if (!items.length) return
    const mine = state.currentUserId || items[0].userId
    const mealKeys = new Set((state.meals || []).map((m) => m.key))
    const fallbackMeal = state.mealPick || state.meal
    state.currentUserId = mine
    const result = await withSound(guard(() => addOrdersInBatches(
      items.map((o) => ({
        dishId: o.dishId,
        note: o.note || '',
        meal: mealKeys.has(o.meal) ? o.meal : fallbackMeal,
      })),
    ), '', 'orders:batch'), playAdd)
    if (!result) return null
    const added = Number(result.added) || 0
    const skipped = Number(result.skipped) || 0
    if (!added) toast('这些菜都已经在今日菜单里，或已从菜谱移除', 'warn')
    else if (skipped) toast(`加回 ${added} 道，${skipped} 道已存在或已移除`, 'success')
    else toast(`已把 ${added} 道菜一次性加回今天`, 'success')
    return result
  },

  // 做完之后的评价，只影响推荐权重
  setDishRating: (id, rating) => guard(() => api.updateDish(id, { rating }), '', 'rate'),
  deleteDish: (id) => {
    const dish = state.dishes.find((d) => d.id === id)
    return withSound(guard(
      () => api.deleteDish(id),
      '',
      'dish',
      () => { state.dishes = state.dishes.filter((d) => d.id !== id) },
    ), playRemove).then((ok) => {
      if (!ok || !dish) return ok
      toast('菜品已移除', 'success', {
        label: '撤销',
        run: () => guard(() => api.addDish({
          name: dish.name, cuisine: dish.cuisine, desc: dish.desc, tags: dish.tags,
          image: dish.image, spicy: dish.spicy, minutes: dish.minutes,
          ingredients: dish.ingredients || [], steps: dish.steps || [],
        }), '菜品已恢复'),
      })
      return ok
    })
  },
  toggleFavorite: (dish) => withSound(guard(
    () => api.updateDish(dish.id, { favorite: !dish.favorite }),
    dish.favorite ? '已取消收藏' : '⭐ 收藏好啦', `fav:${dish.id}`,
  ), playFav),

  addMember: (member) => withSound(guard(() => api.addMember(member), '家里多了一位成员', 'member'), playAdd),
  updateMember: (id, patch) => withSound(guard(() => api.updateMember(id, patch), '已保存', 'member'), playSave),
  deleteMember: (id) => withSound(guard(
    () => api.deleteMember(id),
    '已移除成员',
    'member',
    () => {
      state.members = state.members.filter((m) => m.id !== id)
      if (!state.members.some((m) => m.id === state.currentUserId)) {
        state.currentUserId = state.members[0]?.id || ''
      }
    },
  ), playRemove),

  // 分工支持多人，且按餐次独立：默认只改「当前这一餐」
  setDutyList: (role, ids, meal = state.meal) =>
    guard(() => api.setPlan(state.today, {
      dutyMeal: meal,
      ...(role === 'chef' ? { chefIds: ids } : { dishwasherIds: ids }),
    }), '', 'duty'),
  // 兼容旧调用（骰子 PK / 排班套用）：单人等价于单元素数组
  setChef: (chefId) => guard(() => api.setPlan(state.today, { chefIds: chefId ? [chefId] : [] }), `已设为 ${dutyName(chefId)} 掌勺`, 'duty'),
  clearChef: () => guard(() => api.setPlan(state.today, { chefIds: [] }), '', 'duty'),
  setDishwasher: (id) => guard(() => api.setPlan(state.today, { dishwasherIds: id ? [id] : [] }), `已设为 ${dutyName(id)} 洗碗`, 'duty'),
  clearDishwasher: () => guard(() => api.setPlan(state.today, { dishwasherIds: [] }), '', 'duty'),
  // 骰子 PK / 排班套用：只作用于当前这一餐，其他餐次不受影响
  setDuties: (chefIds, dishwasherIds, meal = state.meal) =>
    guard(() => api.setPlan(state.today, {
      dutyMeal: meal,
      chefIds: Array.isArray(chefIds) ? chefIds : (chefIds ? [chefIds] : []),
      dishwasherIds: Array.isArray(dishwasherIds) ? dishwasherIds : (dishwasherIds ? [dishwasherIds] : []),
    }), '', 'duty'),
  // 骰子 PK 的点数 / 阶段 / 已看动画名单单独存：不走 guard，避免每次摇都整页 refresh 打断动画
  saveDice: async (patch) => {
    try {
      const plan = await api.setPlan(state.today, patch)
      dataGeneration++
      replacePlanInState(plan)
      return plan
    } catch (err) {
      toast(err.message || '骰子结果保存失败', 'error')
      return null
    }
  },
  setNote: (note) => guard(() => api.setPlan(state.today, { note }), '', 'note'),
  // 手动选的餐别放在全局 state：今天页、菜谱卡片、菜品详情弹窗三个入口归到同一餐。
  setMealPick: (key) => { state.mealPick = key || '' },
  order: (dishId, note = '', meal = '') => {
    // 同一道菜今天已经在菜单里就不再重复添加；返回 true 让详情弹窗也能正常关闭
    if ((state.plan.orders || []).some((o) => o.dishId === dishId)) {
      toast('这道菜已经在今日菜单里啦', 'warn')
      return Promise.resolve(true)
    }
    return withSound(guard(
      () => api.addOrder(state.today, {
        dishId, userId: state.currentUserId, note,
        meal: meal || state.mealPick || state.meal,
      }),
      '点好啦，等开饭 🍚', `order:${dishId}`,
    ), playAdd)
  },
  orderMany: (items) => {
    if (!Array.isArray(items) || !items.length) return Promise.resolve(null)
    return withSound(guard(
      () => addOrdersInBatches(items),
      '',
      'orders:batch',
    ), playAdd)
  },
  toggleOrder: (orderId) => guard(() => api.toggleOrder(state.today, orderId), '', `toggle:${orderId}`),
  // 做完饭一道道点上桌太麻烦：一次事务把今天还没上桌的全标记掉。
  finishAllOrders: async () => {
    const todo = (state.plan.orders || []).filter((o) => !o.done)
    if (!todo.length) return
    const result = await guard(() => api.setOrdersDone(state.today, {
      orderIds: todo.map((o) => o.id),
      done: true,
    }), '', 'orders:batch')
    if (result) toast(`${Number(result.updated) || todo.length} 道菜全部上桌 🍽️`, 'success')
  },
  removeOrder: (orderId) => {
    const order = (state.plan.orders || []).find((o) => o.id === orderId)
    return withSound(guard(() => api.removeOrder(state.today, orderId), '', `remove:${orderId}`), playRemove).then((ok) => {
      if (!ok || !order) return ok
      toast('已撤掉这道菜', 'success', {
        label: '撤销',
        run: () => guard(() => api.addOrder(state.today, {
          dishId: order.dishId, userId: order.userId, note: order.note, meal: order.meal,
        }), '已恢复这道菜'),
      })
      return ok
    })
  },

  saveSettings: (patch) => withSound(guard(() => api.saveSettings(patch), '设置已保存', 'settings'), playSave),
  setHiddenCuisines: (hidden) => guard(
    () => api.saveSettings({ familyName: state.settings.familyName, hiddenCuisines: hidden }),
    '', 'cuisines',
  ),
  importBackup: (form) => guard(
    () => api.importBackup(form),
    '备份已恢复',
    'import',
    () => requestRefresh(0),
  ),

  // 改口令不能走 guard：guard 成功后会立刻用「旧口令」刷新，会被 401 挡下。
  // 这里先把本机请求头换成新口令，再刷新。
  async setWebAccessCode(code) {
    state.busy++
    try {
      const settings = await api.saveSettings({ accessCode: code })
      setAccessCode(code)
      state.accessRequired = !!code
      dataGeneration++
      applySettingsResult(settings)
      toast(code ? '访问口令已开启，其它设备下次打开需要输入' : '访问口令已关闭', 'success')
      return true
    } catch (err) {
      toast(err.message || '保存访问口令失败', 'error')
      return false
    } finally {
      state.busy--
    }
  },

  async unlock(code) {
    setAccessCode(code)
    try {
      await refresh()
      toast('欢迎回家', 'success')
    } catch (err) {
      state.locked = true
      toast(err.message || '口令不正确', 'error')
    }
  },
}

provide('app', { state, actions, tabs, selectUser, confirmDialog, choiceDialog, pending })

watch(() => state.tab, (tab, previous) => {
  if (previous && previous !== tab) pageScrollPositions.set(previous, window.scrollY)
  window.history.replaceState(null, '', `#${tab}`)
  if (previous) restorePageScroll(tab)
}, { immediate: true, flush: 'sync' })

function onLocked() {
  state.locked = true
  state.ready = false
}

// 浏览器前进/后退或直接改地址栏 hash 时也要切页（replaceState 不会触发本事件，不会成环）
function onHashChange() {
  const key = window.location.hash.replace('#', '')
  if (tabs.some((t) => t.key === key) && state.tab !== key) state.tab = key
}

async function boot() {
  state.bootError = ''
  try {
    await refresh()
  } catch (err) {
    // 401 会由 request() 广播 fm:locked，直接显示口令框，不把「需要口令」误报成断网。
    if (!state.locked) state.bootError = err.message || '连不上服务端'
  }
}

// 静默同步：后端会按饭点自动收起已过期的那一餐。
// 只更新订单列表与当前餐别，不动备注/分工，避免打断正在输入的内容。
function scheduleMealBoundary() {
  clearTimeout(mealBoundaryTimer)
  const now = new Date()
  const current = now.getHours() * 3600 + now.getMinutes() * 60 + now.getSeconds()
  let waitSeconds = 24 * 3600 - current
  for (const meal of state.meals) {
    const [hours, minutes] = String(meal.end || '').split(':').map(Number)
    if (!Number.isFinite(hours) || !Number.isFinite(minutes)) continue
    const end = hours * 3600 + minutes * 60
    if (end > current) waitSeconds = Math.min(waitSeconds, end - current)
  }
  // 饭点结束时立刻向后端确认一次，让该餐已过期但未上桌的菜及时收起。
  mealBoundaryTimer = setTimeout(
    () => syncNow('meal-end'),
    Math.max(300, waitSeconds * 1000 + 120),
  )
}

async function syncNow(reason = 'poll') {
  if (!state.ready || state.locked) return
  const stamp = Date.now()
  state.now = stamp
  state.meal = currentMeal(stamp, state.meals)
  // 页面开着跨过午夜：日期变了必须重新拉首屏，否则点菜会落到昨天的计划里。
  if (localDate(new Date(stamp)) !== state.today) {
    state.mealPick = ''
    try { await refresh() } catch {}
    return
  }
  if (document.hidden) return
  // SSE 在线时的普通轮询没有意义；饭点、回到前台和断线兜底才需要主动拉取。
  if (reason === 'poll' && state.realtime === 'online') return
  if (state.busy > 0 || localMutationDepth > 0 || refreshInFlight) {
    requestRefresh(600)
    return
  }
  try { await refresh() } catch {}
}

// 从后台切回来时立刻同步一次，不必等下一个 60s 周期。
function onVisible() {
  if (document.visibilityState === 'visible') {
    if (ssePending) flushRemoteRefresh()
    else syncNow('visible')
  }
}

// ---------- 多人实时同步（SSE）----------
// 服务端只推「数据变了」的版本号；这里做防抖与去重，避免打断正在输入的内容、
// 正在播放的动画，或把自己刚提交的写操作重复刷新一遍。
let source = null
let sseTimer = null
let ssePending = false
let lastSseVersion = 0
function isTyping() {
  const el = document.activeElement
  return !!el && /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName) && el.type !== 'checkbox'
}
function doRemoteRefresh() {
  if (document.hidden || isTyping() || state.busy > 0 || localMutationDepth > 0) {
    if (localMutationDepth > 0) remoteDuringMutation = true
    ssePending = true
    return
  }
  requestRefresh(0)
}
function scheduleRemoteRefresh() {
  if (!state.ready || state.locked) return
  if (document.hidden || isTyping() || state.busy > 0 || localMutationDepth > 0) {
    if (localMutationDepth > 0) remoteDuringMutation = true
    ssePending = true
    return
  }
  clearTimeout(sseTimer)
  sseTimer = setTimeout(doRemoteRefresh, 350)
}
function flushRemoteRefresh() {
  if (!ssePending) return
  ssePending = false
  clearTimeout(sseTimer)
  sseTimer = setTimeout(doRemoteRefresh, 120)
}
function onFocusOut() { setTimeout(flushRemoteRefresh, 120) }
function installRealtime() {
  if (typeof EventSource === 'undefined') { state.realtime = 'unsupported'; return }
  try {
    const clientId = encodeURIComponent(getClientId())
    source = new EventSource(`/api/events?clientId=${clientId}`)
    source.onopen = () => { state.realtime = 'online' }
    source.onmessage = (event) => {
      let version = 0
      try { version = Number(JSON.parse(event.data)?.v) || 0 } catch {}
      if (version && version <= lastSseVersion) return
      if (version) lastSseVersion = version
      scheduleRemoteRefresh()
    }
    source.onerror = () => { state.realtime = 'reconnecting' }
    state.realtime = 'connecting'
  } catch {
    state.realtime = 'unsupported'
  }
  document.addEventListener('focusout', onFocusOut)
  window.addEventListener('focus', flushRemoteRefresh)
}

onMounted(() => {
  window.addEventListener('fm:locked', onLocked)
  window.addEventListener('hashchange', onHashChange)
  document.addEventListener('visibilitychange', onVisible)
  window.addEventListener('sw-update', onSwUpdate)
  window.addEventListener('offline', onOffline)
  window.addEventListener('online', onOnline)
  boot()
  installRealtime()
  // SSE 在线时完全靠事件同步；这里只是断线时的低频兜底，不再每分钟和页面操作抢刷新。
  syncTimer = setInterval(() => syncNow('poll'), 5 * 60 * 1000)
})
onUnmounted(() => {
  window.removeEventListener('fm:locked', onLocked)
  window.removeEventListener('hashchange', onHashChange)
  window.removeEventListener('offline', onOffline)
  window.removeEventListener('online', onOnline)
  document.removeEventListener('visibilitychange', onVisible)
  window.removeEventListener('sw-update', onSwUpdate)
  clearTimeout(toastTimer)
  if (syncTimer) clearInterval(syncTimer)
  clearTimeout(refreshTimer)
  clearTimeout(mealBoundaryTimer)
  clearTimeout(scrollRestoreTimer)
  if (source) source.close()
  clearTimeout(sseTimer)
  document.removeEventListener('focusout', onFocusOut)
  window.removeEventListener('focus', flushRemoteRefresh)
})
</script>

<template>
  <div class="app-shell">
    <!-- 写操作进行中：顶栏下沿一条细进度条，给「点了没反应」一个明确交代 -->
    <Transition name="progress">
      <div v-if="state.busy > 0" class="busy-bar" role="progressbar" aria-label="正在保存" />
    </Transition>

    <!-- PWA 新版本就绪：点一下才刷新到新版本，不再需要手动强刷 -->
    <Transition name="update">
      <div v-if="updateReady" class="update-banner" role="status">
        <span class="update-dot" aria-hidden="true">🎉</span>
        <span class="update-text">有新版本啦，点此更新到最新功能</span>
        <button class="update-btn" type="button" @click="applyUpdate">更新</button>
        <button class="update-close" type="button" aria-label="暂不更新" @click="updateReady = false">稍后</button>
      </div>
    </Transition>

    <header class="topbar">
      <div class="topbar-inner">
        <div class="brand">
          <span class="brand-mark">🍚</span>
          <div>
            <div class="brand-name">家里吃点啥</div>
            <div class="brand-sub">{{ state.settings.familyName }} · 今天吃什么，大家一起定</div>
          </div>
        </div>
        <div class="top-actions">
          <div class="user-switch" v-if="state.ready">
            <span class="user-switch-label">我是</span>
            <select
              :value="state.currentUserId" aria-label="选择当前家庭成员"
              @change="onSwitchUser"
            >
              <option v-for="m in state.members" :key="m.id" :value="m.id">{{ m.avatar }} {{ m.name }}</option>
            </select>
          </div>
        </div>
      </div>
      <nav class="tabs" v-if="state.ready">
        <button
          v-for="t in tabs" :key="t.key"
          class="tab" :class="{ active: state.tab === t.key }"
          @mouseenter="prefetchView(t.key)" @focus="prefetchView(t.key)"
          @touchstart.passive="prefetchView(t.key)"
          @click="state.tab = t.key"
        >
          <span class="tab-icon">{{ t.icon }}</span><span>{{ t.label }}</span>
        </button>
      </nav>
    </header>

    <!-- 网络 / 实时同步异常时才出现：平时不占版面，出问题必须让用户看见 -->
    <Transition name="fade">
      <div v-if="offline || state.realtime === 'reconnecting'" class="net-banner" role="status">
        <template v-if="offline">📵 已离线，改动不会同步到其他设备</template>
        <template v-else>🔄 与服务器断开，正在重连…</template>
      </div>
    </Transition>

    <main class="main" v-if="state.ready">
      <Transition :name="pageDirection" mode="out-in">
        <KeepAlive>
          <component :is="currentView" :key="state.tab" />
        </KeepAlive>
      </Transition>
    </main>

    <div v-else-if="!state.locked" class="loading">
      <div v-if="state.bootError">
        <p>😵 {{ state.bootError }}</p>
        <button class="btn btn-primary mt-12" @click="boot">再试一次</button>
      </div>
      <div v-else class="boot-skeleton" role="status" aria-label="正在摆碗筷">
        <div></div><div></div><div></div>
        <span class="visually-hidden">正在摆碗筷…</span>
      </div>
    </div>

    <footer class="footer" v-if="state.ready">
      家里吃点啥 · 数据只保存在你自己的设备 / NAS
    </footer>

    <Transition name="toast">
      <div
        v-if="state.toast" :key="state.toast.id"
        class="toast" :class="[`toast-${state.toast.type}`, { 'has-action': state.toast.action }]"
        role="status" aria-live="polite"
        @click="!state.toast.action && (state.toast = null)"
      >
        <span class="toast-icon" aria-hidden="true">{{ {
          success: '✓', error: '!', warn: '!', info: '·',
        }[state.toast.type] || '·' }}</span>
        <span class="toast-text">{{ state.toast.text }}</span>
        <button v-if="state.toast.action" class="toast-action" type="button" @click.stop="runToastAction">
          {{ state.toast.action.label }}
        </button>
      </div>
    </Transition>

    <Transition name="modal">
      <div v-if="state.locked" ref="lockRoot" class="modal-backdrop">
        <div class="modal">
          <div class="modal-head"><h2>🔒 需要访问口令</h2></div>
          <p class="muted fs-88">
            这台设备设置了访问口令。输入后会记在本机浏览器里，之后不用重复输入。
          </p>
          <input
            v-model="codeInput" class="input mt-14" type="password" placeholder="访问口令"
            @keyup.enter="actions.unlock(codeInput)"
          />
          <div class="modal-foot">
            <button class="btn btn-primary btn-lg" @click="actions.unlock(codeInput)">进入</button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 首次打开：主动选一次「我是谁」，之后点菜/掌勺/洗碗都记在这个名字下 -->
    <Transition name="modal">
      <div v-if="needsUserPick && state.ready && !state.locked" class="modal-backdrop user-pick-backdrop">
      <div ref="userPickRoot" class="modal user-pick-modal" role="dialog" aria-modal="true" aria-label="先选一下你是谁">
        <div class="user-pick-head">
          <span class="user-pick-emoji">👋</span>
          <h2>先选一下你是谁</h2>
          <p class="muted">之后点菜、掌勺、洗碗都会记在这个名字下，右上角随时能换。</p>
        </div>
        <div v-if="!state.members.length" class="user-pick-empty">
          <p class="muted">家里还没有成员，先关掉去添加一位。</p>
          <button class="btn btn-soft btn-sm" @click="needsUserPick = false">先看看</button>
        </div>
        <div v-else class="user-pick-list">
          <button v-for="m in state.members" :key="m.id" class="user-pick-item" @click="pickUser(m.id)">
            <span class="avatar avatar-lg" :style="{ background: m.color }">{{ m.avatar }}</span>
            <span class="user-pick-name">{{ m.name }}</span>
            <span class="user-pick-arrow">›</span>
          </button>
        </div>
      </div>
    </div>
    </Transition>

    <!-- 成员密码：切到设了密码的身份前校验；叠在首次选人弹窗之上 -->
    <Transition name="modal">
      <div v-if="pinPrompt" class="modal-backdrop pin-backdrop" @click.self="pinPrompt = null">
        <div ref="pinRoot" class="modal pin-modal" role="dialog" aria-modal="true" aria-label="输入成员密码">
          <div class="modal-head">
            <h2>🔒 {{ pinPrompt.avatar }} {{ pinPrompt.name }} 的密码</h2>
            <button class="icon-btn" aria-label="取消" @click="pinPrompt = null">✕</button>
          </div>
          <p class="setting-desc">这位家人设了密码，切换身份前先验证一下。</p>
          <input
            v-model="pinInput" class="input pin-input" type="password"
            inputmode="numeric" maxlength="16" placeholder="4–16 位密码"
            aria-label="成员密码" @keyup.enter="submitPin"
          />
          <p v-if="pinError" class="pin-error" role="alert">{{ pinError }}</p>
          <div class="modal-foot">
            <button class="btn btn-soft" @click="pinPrompt = null">取消</button>
            <button class="btn btn-primary" :disabled="pinBusy" @click="submitPin">
              {{ pinBusy ? '校验中…' : '确认切换' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 统一确认弹窗：替换浏览器原生 confirm，与所有窗口保持同款动画 -->
    <Transition name="modal">
      <ConfirmDialog
        v-if="confirmState.open"
        :title="confirmState.title" :message="confirmState.message"
        :confirm-text="confirmState.confirmText" :cancel-text="confirmState.cancelText"
        :extra-text="confirmState.extraText" :danger="confirmState.danger"
        @confirm="onConfirmOk" @extra="onConfirmExtra" @cancel="onConfirmCancel"
      />
    </Transition>
  </div>
</template>
