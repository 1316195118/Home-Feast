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

let toastTimer = null
let syncTimer = null
let toastSeq = 0

// type: success / error / warn / info。带 id 便于连续弹同类提示时重播动画。
function toast(message, type = 'info') {
  state.toast = { id: ++toastSeq, text: message, type }
  if (type === 'error') playError() // 失败提示配合抖动动画与低沉两音
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { state.toast = null }, type === 'error' ? 3400 : 2600)
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

function pickUser(id) {
  selectUser(id)
  needsUserPick.value = false
  const m = state.members.find((x) => x.id === id)
  if (m) toast(`${m.avatar} 欢迎回家，${m.name}`, 'success')
}

function onSwitchUser(e) {
  const id = e.target.value
  selectUser(id)
  const m = state.members.find((x) => x.id === id)
  if (m) toast(`${m.avatar} 现在以 ${m.name} 的身份点菜`, 'info')
}

// 统一的「确认」弹窗：替换浏览器原生 confirm，保持界面风格一致，也带入场动画。
const confirmState = reactive({
  open: false, title: '', message: '', confirmText: '确定', cancelText: '取消', danger: false,
})
let confirmResolve = null
function confirmDialog(opts = {}) {
  confirmState.title = opts.title || '请确认'
  confirmState.message = opts.message || ''
  confirmState.confirmText = opts.confirmText || '确定'
  confirmState.cancelText = opts.cancelText || '取消'
  confirmState.danger = !!opts.danger
  confirmState.open = true
  return new Promise((res) => { confirmResolve = res })
}
function onConfirmOk() { confirmState.open = false; confirmResolve?.(true); confirmResolve = null }
function onConfirmCancel() { confirmState.open = false; confirmResolve?.(false); confirmResolve = null }

// key 相同的写操作会互相防连点；组件可用 pending(key) 把按钮置为 loading/禁用。
const pendingKeys = reactive({})
function pending(key) { return !!pendingKeys[key] }

async function guard(fn, message, key = '') {
  if (key && pendingKeys[key]) return null
  if (key) pendingKeys[key] = true
  state.busy++
  try {
    const result = await fn()
    await refresh()
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
  deleteDish: (id) => withSound(guard(() => api.deleteDish(id), '菜品已移除', 'dish'), playRemove),
  toggleFavorite: (dish) => withSound(guard(
    () => api.updateDish(dish.id, { favorite: !dish.favorite }),
    dish.favorite ? '已取消收藏' : '⭐ 收藏好啦', `fav:${dish.id}`,
  ), playFav),

  addMember: (member) => withSound(guard(() => api.addMember(member), '家里多了一位成员', 'member'), playAdd),
  updateMember: (id, patch) => withSound(guard(() => api.updateMember(id, patch), '已保存', 'member'), playSave),
  deleteMember: (id) => withSound(guard(() => api.deleteMember(id), '已移除成员', 'member'), playRemove),

  setChef: (chefId) => guard(() => api.setPlan(state.today, { chefId }), `今天由 ${dutyName(chefId)} 掌勺 👨‍🍳`, 'duty'),
  clearChef: () => guard(() => api.setPlan(state.today, { chefId: '' }), '', 'duty'),
  setDishwasher: (id) => guard(() => api.setPlan(state.today, { dishwasherId: id }), `今天由 ${dutyName(id)} 洗碗 🧽`, 'duty'),
  clearDishwasher: () => guard(() => api.setPlan(state.today, { dishwasherId: '' }), '', 'duty'),
  // 骰子 PK 的结果一次提交，避免两次请求两次刷新
  setDuties: (chefId, dishwasherId) =>
    guard(() => api.setPlan(state.today, { chefId, dishwasherId }), '分工定好啦 🎲', 'duty'),
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
  order: (dishId, note = '', meal = '') => withSound(guard(
    () => api.addOrder(state.today, {
      dishId, userId: state.currentUserId, note,
      meal: meal || state.mealPick || state.meal,
    }),
    '点好啦，等开饭 🍚', `order:${dishId}`,
  ), playAdd),
  toggleOrder: (orderId) => guard(() => api.toggleOrder(state.today, orderId), '', `toggle:${orderId}`),
  removeOrder: (orderId) => withSound(guard(() => api.removeOrder(state.today, orderId), '已撤掉这道菜', `remove:${orderId}`), playRemove),

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

provide('app', { state, actions, tabs, selectUser, confirmDialog, pending })

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
  } catch {}
}

// 从后台切回来时立刻同步一次，不必等下一个 60s 周期。
function onVisible() {
  if (document.visibilityState === 'visible') syncNow()
}

onMounted(() => {
  window.addEventListener('fm:locked', onLocked)
  window.addEventListener('hashchange', onHashChange)
  document.addEventListener('visibilitychange', onVisible)
  boot()
  syncTimer = setInterval(syncNow, 60000)
})
onUnmounted(() => {
  window.removeEventListener('fm:locked', onLocked)
  window.removeEventListener('hashchange', onHashChange)
  document.removeEventListener('visibilitychange', onVisible)
  clearTimeout(toastTimer)
  if (syncTimer) clearInterval(syncTimer)
})
</script>

<template>
  <div class="app-shell">
    <!-- 写操作进行中：顶栏下沿一条细进度条，给「点了没反应」一个明确交代 -->
    <Transition name="progress">
      <div v-if="state.busy > 0" class="busy-bar" role="progressbar" aria-label="正在保存" />
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

    <main class="main" v-if="state.ready">
      <Transition :name="pageDirection" mode="out-in">
        <component :is="currentView" :key="state.tab" />
      </Transition>
    </main>

    <div v-else-if="!state.locked" class="loading">
      <div v-if="state.bootError">
        <p>😵 {{ state.bootError }}</p>
        <button class="btn btn-primary" style="margin-top: 12px" @click="boot">再试一次</button>
      </div>
      <div v-else class="boot-skeleton" role="status" aria-label="正在摆碗筷">
        <div></div><div></div><div></div>
        <span class="visually-hidden">正在摆碗筷…</span>
      </div>
    </div>

    <footer class="footer" v-if="state.ready">
      数据只保存在你自己的设备 / NAS · 家里吃点啥 v{{ state.version }}
    </footer>

    <Transition name="toast">
      <div
        v-if="state.toast" :key="state.toast.id"
        class="toast" :class="`toast-${state.toast.type}`"
        role="status" aria-live="polite"
        @click="state.toast = null"
      >
        <span class="toast-icon" aria-hidden="true">{{ {
          success: '✓', error: '!', warn: '!', info: '·',
        }[state.toast.type] || '·' }}</span>
        <span>{{ state.toast.text }}</span>
      </div>
    </Transition>

    <Transition name="modal">
      <div v-if="state.locked" ref="lockRoot" class="modal-backdrop">
        <div class="modal">
          <div class="modal-head"><h2>🔒 需要访问口令</h2></div>
          <p class="muted" style="font-size: 0.88rem">
            这台设备设置了访问口令。输入后会记在本机浏览器里，之后不用重复输入。
          </p>
          <input
            v-model="codeInput" class="input" type="password" placeholder="访问口令"
            style="margin-top: 14px" @keyup.enter="actions.unlock(codeInput)"
          />
          <div class="modal-foot">
            <button class="btn btn-primary" @click="actions.unlock(codeInput)">进入</button>
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
        <div class="user-pick-list">
          <button v-for="m in state.members" :key="m.id" class="user-pick-item" @click="pickUser(m.id)">
            <span class="avatar avatar-lg" :style="{ background: m.color }">{{ m.avatar }}</span>
            <span class="user-pick-name">{{ m.name }}</span>
            <span class="user-pick-arrow">›</span>
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
        :confirm-text="confirmState.confirmText" :cancel-text="confirmState.cancelText" :danger="confirmState.danger"
        @confirm="onConfirmOk" @cancel="onConfirmCancel"
      />
    </Transition>
  </div>
</template>
