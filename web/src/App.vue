<script setup>
import { reactive, provide, onMounted, onUnmounted, watch, ref, computed } from 'vue'
import { api, setAccessCode, getAccessCode } from './api'
import TodayView from './components/TodayView.vue'
import MenuView from './components/MenuView.vue'
import FamilyView from './components/FamilyView.vue'
import HistoryView from './components/HistoryView.vue'
import SettingsView from './components/SettingsView.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import { useModalBehavior } from './modal'
import { currentMeal } from './utils'
import { playAdd, playFav, playSave, playRemove, playError } from './audio'

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
  refresh().catch(() => {})
}


async function refresh() {
  const [info, plans] = await Promise.all([api.bootstrap(), api.plans()])
  state.version = info.version
  state.today = info.today
  state.meals = info.meals || []
  // 用户可能在设置里删掉了当前手动选中的那一餐，避免之后点菜提交不存在的 key
  if (state.mealPick && !state.meals.some((m) => m.key === state.mealPick)) state.mealPick = ''
  state.now = Date.now()
  state.meal = currentMeal(state.now, state.meals)
  state.settings = info.settings || { familyName: '我们家' }
  state.hiddenCuisines = info.settings?.hiddenCuisines || []
  state.cuisines = info.cuisines || []
  state.dishes = info.dishes || []
  state.members = info.members || []
  state.plan = info.plan || { date: info.today, orders: [] }
  state.plans = plans || []
  if (!state.members.some((m) => m.id === state.currentUserId)) {
    state.currentUserId = state.members[0]?.id || ''
  }
  state.ready = true
  state.locked = false
  state.bootError = ''
  // 只有「用户主动选过」才会写进 localStorage；这里的兜底不写，避免首次被静默记住
  if (!localStorage.getItem('fm.user') && state.members.length) needsUserPick.value = true
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
let lastLocalRefresh = 0 // 本机写操作后 guard 刷新过的时间，用来忽略 SSE 回声

async function guard(fn, message, key = '') {
  if (key && pendingKeys[key]) return null
  if (key) pendingKeys[key] = true
  state.busy++
  try {
    const result = await fn()
    await refresh()
    lastLocalRefresh = Date.now() // SSE 回声去重：本机刚刷新过就不再重复拉
    if (message) toast(message, 'success')
    return result === undefined ? true : result
  } catch (err) {
    toast(err.message || '操作失败，请稍后再试', 'error')
    return null
  } finally {
    state.busy--
    if (key) delete pendingKeys[key]
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

const actions = {
  toast,
  refresh: () => guard(() => Promise.resolve(), ''),
  addDish: (dish) => guard(() => api.addDish(dish), '菜品已加入菜谱', 'dish'),
  updateDish: (id, patch) => guard(() => api.updateDish(id, patch), '已保存', 'dish'),
  // 删除类操作都留一次「撤销」：菜品删错要重传配图，代价比弹确认框高得多
  // 「再来一桌」：把某天点过的菜原样加回今天。逐条加，跳过已删的菜。
  reorderDay: async (plan) => {
    if (state.busy > 0) return
    const items = (plan?.orders || []).filter((o) => o.dishId)
    if (!items.length) return
    const mine = state.currentUserId || items[0].userId
    state.busy++
    let added = 0
    let skipped = 0
    try {
      for (const o of items) {
        try {
          await api.addOrder(state.today, {
            dishId: o.dishId, userId: mine, note: o.note || '', meal: o.meal || '',
          })
          added++
        } catch {
          skipped++
        }
      }
      await refresh()
      if (!added) toast('这些菜都已经不在菜谱里了', 'error')
      else if (skipped) toast(`加回 ${added} 道，${skipped} 道已经没有了`, 'success')
      else toast(`已把 ${added} 道菜加回今天`, 'success')
    } finally {
      state.busy--
    }
  },

  // 做完之后的评价，只影响推荐权重
  setDishRating: (id, rating) => guard(() => api.updateDish(id, { rating }), '', 'rate'),
  deleteDish: (id) => {
    const dish = state.dishes.find((d) => d.id === id)
    return withSound(guard(() => api.deleteDish(id), '', 'dish'), playRemove).then((ok) => {
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
  deleteMember: (id) => withSound(guard(() => api.deleteMember(id), '已移除成员', 'member'), playRemove),

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
      state.plan = plan
      const i = state.plans.findIndex((p) => p.date === state.today)
      if (i >= 0) state.plans[i] = plan
      else state.plans = [...state.plans, plan]
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
  toggleOrder: (orderId) => guard(() => api.toggleOrder(state.today, orderId), '', `toggle:${orderId}`),
  // 做完饭一道道点上桌太麻烦：一次把今天还没上桌的全标记掉
  finishAllOrders: async () => {
    const todo = (state.plan.orders || []).filter((o) => !o.done)
    if (!todo.length) return
    state.busy++
    try {
      for (const o of todo) await api.toggleOrder(state.today, o.id)
      await refresh()
      toast(`${todo.length} 道菜全部上桌 🍽️`, 'success')
    } catch {
      toast('有几道没标上，请逐道再试一次', 'error')
    } finally {
      state.busy--
    }
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
  importBackup: (form) => guard(() => api.importBackup(form), '备份已恢复', 'import'),

  // 改口令不能走 guard：guard 成功后会立刻用「旧口令」刷新，会被 401 挡下。
  // 这里先把本机请求头换成新口令，再刷新。
  async setWebAccessCode(code) {
    state.busy++
    try {
      await api.saveSettings({ accessCode: code })
      setAccessCode(code)
      state.accessRequired = !!code
      await refresh()
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

watch(() => state.tab, (tab) => {
  window.history.replaceState(null, '', `#${tab}`)
}, { immediate: true })

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
  try {
    const cfg = await api.config()
    state.accessRequired = cfg.accessRequired
    state.accessEnvManaged = !!cfg.accessEnvManaged
    state.version = cfg.version || ''
    state.releaseNote = cfg.releaseNote || ''
    state.buildTime = cfg.buildTime || ''
    if (cfg.accessRequired && !getAccessCode()) {
      state.locked = true
      return
    }
    await refresh()
  } catch (err) {
    state.bootError = err.message || '连不上服务端'
  }
}

// 静默同步：后端会按饭点自动收起已过期的那一餐。
// 只更新订单列表与当前餐别，不动备注/分工，避免打断正在输入的内容。
async function syncNow() {
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
  try {
    const plans = await api.plans()
    // 历史记录页也一起更新，过饭点清理后立刻反映，不用等手动刷新。
    state.plans = plans || []
    const today = (plans || []).find((p) => p.date === state.today)
    const next = today?.orders || []
    const cur = state.plan.orders || []
    if (next.length !== cur.length || next.some((o, i) => o.id !== cur[i]?.id)) {
      state.plan.orders = next
    }
    // 分工也要跟上：SSE 断开时只能靠这里，否则别人认领了本机一直显示旧的。
    // 正在提交认领时跳过，避免请求返回前被旧值覆盖。
    if (!pending('duty')) {
      const nextDuty = today?.mealDuties || {}
      const curDuty = state.plan.mealDuties || {}
      if (JSON.stringify(nextDuty) !== JSON.stringify(curDuty)) {
        state.plan.mealDuties = nextDuty
      }
    }
  } catch {}
}

// 从后台切回来时立刻同步一次，不必等下一个 60s 周期。
function onVisible() {
  if (document.visibilityState === 'visible') syncNow()
}

// ---------- 多人实时同步（SSE）----------
// 服务端只推「数据变了」的版本号；这里做防抖与去重，避免打断正在输入的内容、
// 正在播放的动画，或把自己刚提交的写操作重复刷新一遍。
let source = null
let sseTimer = null
let ssePending = false
function isTyping() {
  const el = document.activeElement
  return !!el && /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName) && el.type !== 'checkbox'
}
function doRemoteRefresh() {
  if (document.hidden || isTyping() || state.busy > 0) { ssePending = true; return }
  if (Date.now() - lastLocalRefresh < 1200) return
  refresh().catch(() => {})
}
function scheduleRemoteRefresh() {
  if (!state.ready || state.locked) return
  if (document.hidden || isTyping() || state.busy > 0) { ssePending = true; return }
  clearTimeout(sseTimer)
  sseTimer = setTimeout(doRemoteRefresh, 350)
}
function flushRemoteRefresh() {
  if (!ssePending) return
  ssePending = false
  clearTimeout(sseTimer)
  sseTimer = setTimeout(doRemoteRefresh, 250)
}
function onFocusOut() { setTimeout(flushRemoteRefresh, 120) }
function onVisibilityRealtime() {
  if (document.visibilityState === 'visible') flushRemoteRefresh()
}
function installRealtime() {
  if (typeof EventSource === 'undefined') { state.realtime = 'unsupported'; return }
  try {
    source = new EventSource('/api/events')
    source.onopen = () => { state.realtime = 'online' }
    source.onmessage = () => scheduleRemoteRefresh()
    source.onerror = () => { state.realtime = 'reconnecting' }
    state.realtime = 'connecting'
  } catch {
    state.realtime = 'unsupported'
  }
  document.addEventListener('focusout', onFocusOut)
  document.addEventListener('visibilitychange', onVisibilityRealtime)
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
  syncTimer = setInterval(syncNow, 60000)
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
  if (source) source.close()
  clearTimeout(sseTimer)
  document.removeEventListener('focusout', onFocusOut)
  document.removeEventListener('visibilitychange', onVisibilityRealtime)
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
        <component :is="currentView" :key="state.tab" />
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
