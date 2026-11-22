<script setup>
import { inject, ref, watch, computed, reactive, onMounted, onBeforeUnmount, onActivated, onDeactivated } from 'vue'
import { api, setAccessCode } from '../api'
import { isCuisineHidden, DEFAULT_MEALS, mealEndMinutes, normalizeMeals, mealIconChoices, WEEKDAYS, isLanHost, imageSize } from '../utils'
import { useModalBehavior } from '../modal'
import HelpTip from './HelpTip.vue'
import { themeChoices, currentThemeMode, setThemeMode, fontChoices, currentFontScale, setFontScale } from '../theme'
import { shake, bounce } from '../feedback'

const { state, actions, confirmDialog, pending } = inject('app')
const familyName = ref(state.settings.familyName || '我们家')
const importBusy = ref(false)
const cuisineRoot = ref(null)
const themeMode = ref(currentThemeMode())
const fontScale = ref(currentFontScale())
function chooseFont(v) {
  fontScale.value = v
  setFontScale(v)
}
function chooseTheme(mode, event) {
  const anchor = event && Number.isFinite(event.clientX)
    ? { x: event.clientX, y: event.clientY }
    : null
  themeMode.value = setThemeMode(mode, anchor)
}
const realtimeText = computed(() => ({
  online: '🟢 已连接', connecting: '🟡 连接中', reconnecting: '🟡 断线重连中',
  unsupported: '⚪ 浏览器不支持', off: '⚪ 未开启',
}[state.realtime] || '⚪ 未开启'))

// ---------- 自动备份 ----------
const backups = ref([])
const backupBusy = ref(false)
const showBackupModal = ref(false)
const backupRoot = ref(null)
useModalBehavior(() => { showBackupModal.value = false }, backupRoot, showBackupModal)

// 版本更新：历史记录随二进制走，首次打开时拉一次
const showVersionModal = ref(false)
const versionRoot = ref(null)
const changelog = ref([])
const changelogLoading = ref(false)
useModalBehavior(() => { showVersionModal.value = false }, versionRoot, showVersionModal)
const KIND_CLASS = { 新增: 'k-add', 修复: 'k-fix', 优化: 'k-opt' }
const kindClass = (kind) => KIND_CLASS[kind] || 'k-opt'

// 拉一次即可（内容随二进制走，运行期不会变）；卡片上要显示版本总数，挂载时先静默取一次
async function ensureChangelog(notify = false) {
  if (changelog.value.length || changelogLoading.value) return
  changelogLoading.value = true
  try {
    const list = await api.changelog()
    changelog.value = Array.isArray(list) ? list : []
  } catch (err) {
    if (notify) actions.toast(err.message || '读取更新记录失败', 'error')
  } finally {
    changelogLoading.value = false
  }
}
function openVersionModal() {
  showVersionModal.value = true
  ensureChangelog(true)
}
onMounted(() => { ensureChangelog(false) })

// 构建时间只取日期，卡片上不必显示到秒
const buildDay = computed(() => (state.buildTime || '').slice(0, 10))

// 设置项较多，用页面内快捷导航直达四组配置，不把功能藏进页签。
const settingsCategories = [
  { id: 'family', icon: '🏠', title: '家庭与日常', hint: '名字 · 用餐 · 排班', count: 3 },
  { id: 'display', icon: '🍽️', title: '菜谱与显示', hint: '菜系 · 配图 · 外观', count: 3 },
  { id: 'data', icon: '🛡️', title: '安全与数据', hint: '口令 · 备份 · 存储', count: 3 },
  { id: 'system', icon: '⬆️', title: '系统与更新', hint: '更新 · 版本 · 关于', count: 3 },
]
const activeSettingsSection = ref(settingsCategories[0].id)
let settingsScrollFrame = 0
function syncActiveSettingsSection() {
  settingsScrollFrame = 0
  const probe = window.innerWidth <= 720 ? 132 : 110
  let current = settingsCategories[0].id
  const atPageEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4
  if (atPageEnd) {
    current = settingsCategories[settingsCategories.length - 1].id
  } else {
    for (const category of settingsCategories) {
      const section = document.getElementById(`settings-${category.id}`)
      if (section && section.getBoundingClientRect().top <= probe) current = category.id
    }
  }
  activeSettingsSection.value = current
}
function queueActiveSettingsSync() {
  if (settingsScrollFrame) return
  settingsScrollFrame = requestAnimationFrame(syncActiveSettingsSection)
}
function jumpToSettings(id) {
  const target = document.getElementById(`settings-${id}`)
  if (!target) return
  activeSettingsSection.value = id
  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  target.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' })
  target.focus({ preventScroll: true })
}
function attachSettingsListeners() {
  window.addEventListener('scroll', queueActiveSettingsSync, { passive: true })
  window.addEventListener('resize', queueActiveSettingsSync, { passive: true })
  queueActiveSettingsSync()
}
function detachSettingsListeners() {
  window.removeEventListener('scroll', queueActiveSettingsSync)
  window.removeEventListener('resize', queueActiveSettingsSync)
  if (settingsScrollFrame) cancelAnimationFrame(settingsScrollFrame)
  settingsScrollFrame = 0
}
onActivated(attachSettingsListeners)
onDeactivated(detachSettingsListeners)
onBeforeUnmount(detachSettingsListeners)

// ---- 容器内在线更新 ----
const updateInfo = ref({ current: state.version, latest: '', available: false, note: '', published: '' })
const updateChecking = ref(false)
const updateApplying = ref(false)
const updateBusy = computed(() => updateChecking.value || updateApplying.value)
const updateProgress = ref(0)
const updateMsg = ref('')
const updateErr = ref('')
const updateCard = ref(null)
const updateStatusVisible = computed(() => updateBusy.value || !!updateMsg.value || !!updateErr.value)
const autoCheck = computed(() => state.settings.autoCheckUpdate !== false) // 默认开
const LAST_CHECK_KEY = 'fm.update.lastCheck'

function fmtNote(note) {
  return String(note || '').replace(/\*\*([^*]+)\*\*/g, '【$1】').split('\n').filter(Boolean).slice(0, 3)
}

const noteLines = computed(() => fmtNote(updateInfo.value.note))
const updateVisualState = computed(() => {
  if (updateErr.value) return 'error'
  if (updateApplying.value) return 'working'
  if (updateChecking.value) return 'checking'
  if (updateInfo.value.available) return 'available'
  if (updateMsg.value.startsWith('已是最新')) return 'success'
  return 'idle'
})
const updateStatusLabel = computed(() => ({
  error: '更新遇到问题',
  working: updateProgress.value >= 85 ? '正在安装新版本' : '正在下载新版本',
  checking: '正在检查更新',
  available: '新版本已经准备好',
  success: '当前已经是最新版本',
  idle: '更新状态',
}[updateVisualState.value]))
const updateStatusIcon = computed(() => ({
  error: '!',
  working: updateProgress.value >= 85 ? '↻' : '↓',
  checking: '↻',
  available: '↑',
  success: '✓',
  idle: '·',
}[updateVisualState.value]))
const updateTagText = computed(() => {
  if (updateVisualState.value === 'error') return '需处理'
  if (updateApplying.value) return `${updateProgress.value}%`
  if (updateChecking.value) return '检查中'
  if (updateInfo.value.available) return `发现 v${updateInfo.value.latest}`
  if (updateVisualState.value === 'success') return '已是最新'
  return `当前 v${state.version}`
})
const updateTagClass = computed(() => {
  if (updateVisualState.value === 'error') return 'tag-error'
  if (['working', 'checking'].includes(updateVisualState.value)) return 'tag-update-live'
  if (['available', 'success'].includes(updateVisualState.value)) return 'tag-green'
  return ''
})
const updatePhaseHint = computed(() => {
  if (!updateApplying.value) return ''
  if (updateProgress.value >= 85) return '正在准备切换服务，页面会短暂重连'
  return '下载期间可以刷新页面，进度会自动接回'
})

async function checkUpdate(silent = false) {
  if (updateBusy.value) return
  updateChecking.value = true
  updateErr.value = ''
  updateProgress.value = 0
  updateMsg.value = '正在连接更新源，检查最新版本…'
  try {
    const r = await api.checkUpdate()
    updateInfo.value = r
    try { localStorage.setItem(LAST_CHECK_KEY, String(Date.now())) } catch { /* 隐私模式忽略 */ }
    bounce(updateCard.value)
    if (r.available) {
      updateMsg.value = `发现新版本 v${r.latest}，可以立即更新`
      if (!silent) actions.toast(`发现新版本 v${r.latest}`, 'success')
    } else {
      updateMsg.value = `已是最新版本 v${r.current}`
      if (!silent) actions.toast(`已是最新版本 v${r.current}`, 'success')
    }
  } catch (e) {
    updateErr.value = e.message || '检查失败'
    updateMsg.value = '检查未完成'
    shake(updateCard.value)
    if (!silent) actions.toast('检查更新失败：' + updateErr.value, 'error')
  } finally {
    updateChecking.value = false
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function pollUpdate() {
  const started = Date.now()
  let sawTask = false
  let taskError = null
  let completion = null
  while (Date.now() - started < 10 * 60 * 1000) {
    try {
      const st = await api.updateStatus()
      const progress = Number(st.progress) || 0
      updateProgress.value = progress
      updateMsg.value = st.message || ''
      if (st.downloading) sawTask = true
      if (completion && !st.downloading) {
        if (st.lastError) throw new Error(st.lastError)
        return completion
      }
      if (st.lastError && !st.downloading) {
        taskError = new Error(st.lastError)
        break
      }
      if (!st.downloading) {
        if (progress >= 100 || st.message === '已是最新版本') return st
        if (sawTask) {
          taskError = new Error('更新任务提前结束，当前版本未改变')
          break
        }
      }
      // 已替换二进制后服务还会再等 600ms 才 Exec。再读一次状态，
      // 这样 Exec 立即失败并回滚时也能拿到真实错误，而不是误报“已安装”。
      if (progress >= 100 && st.downloading) {
        completion = st
        await sleep(650)
        continue
      }
      if (completion) return completion
    } catch {
      if (completion) return completion
      updateMsg.value = '服务正在重启，等待重新连接…'
    }
    await sleep(650)
  }
  if (taskError) throw taskError
  throw new Error('更新等待超时，请稍后刷新页面查看版本')
}

async function waitForVersion(target) {
  const deadline = Date.now() + 60 * 1000
  while (Date.now() < deadline) {
    try {
      const info = await api.ping()
      if (!target || info.version === target) return true
    } catch { /* 服务切换期间短暂断开是正常的 */ }
    await sleep(800)
  }
  return false
}

async function doUpdate(force = false) {
  if (updateBusy.value) return
  const latest = updateInfo.value.latest || state.version
  const ok = await confirmDialog({
    title: force ? '强制重装' : '确认在线更新',
    message: force
      ? `强制重新下载并安装 v${latest}（即使已是最新）。服务会中断几秒，数据不受影响。确认继续？`
      : `将当前 v${state.version} 更新到 v${latest}。更新时服务会短暂中断几秒，页面会自动重连。确认继续？`,
    confirmText: force ? '强制重装' : '立即更新', danger: true,
  })
  if (!ok) return
  updateApplying.value = true
  updateErr.value = ''
  updateProgress.value = 0
  updateMsg.value = '正在启动更新任务…'
  try {
    const r = await api.applyUpdate({ force })
    updateMsg.value = r.message || '更新任务已开始'
    actions.toast('更新任务已开始，可查看实时进度', 'info')
    bounce(updateCard.value)
    const st = await pollUpdate()
    if (st?.message === '已是最新版本') {
      updateMsg.value = `已是最新版本 v${state.version}`
      updateApplying.value = false
      return
    }
    updateProgress.value = 100
    updateMsg.value = '正在重启并连接新版本…'
    const ready = await waitForVersion(st?.latest || updateInfo.value.latest)
    if (!ready) throw new Error('新版本已安装，但服务暂未重新连接，请稍后刷新页面')
    updateMsg.value = '更新完成，正在刷新页面…'
    actions.toast('更新完成，正在重新连接…', 'success')
    await sleep(500)
    window.location.reload()
  } catch (e) {
    updateErr.value = e.message || '更新失败'
    updateMsg.value = '更新未完成'
    shake(updateCard.value)
    actions.toast('更新失败：' + updateErr.value, 'error')
    updateApplying.value = false
    updateProgress.value = 0
  }
}

// 页面刷新或中途重新进入设置时，接管尚未结束的后台更新并继续显示进度。
async function resumeUpdate() {
  try {
    const st = await api.updateStatus()
    if (!st.downloading) return
    updateApplying.value = true
    updateProgress.value = Number(st.progress) || 0
    updateMsg.value = st.message || '更新正在进行中…'
    const final = await pollUpdate()
    updateProgress.value = 100
    updateMsg.value = '正在重启并连接新版本…'
    const ready = await waitForVersion(final?.latest || st.latest)
    if (!ready) throw new Error('新版本已安装，但服务暂未重新连接，请稍后刷新页面')
    updateMsg.value = '更新完成，正在刷新页面…'
    bounce(updateCard.value)
    actions.toast('更新完成，正在重新连接…', 'success')
    await sleep(500)
    window.location.reload()
  } catch (e) {
    updateErr.value = e.message || '更新失败'
    updateMsg.value = '更新未完成'
    shake(updateCard.value)
    updateApplying.value = false
    updateProgress.value = 0
  }
}

// 更新设置收进二级弹窗：代理这类低频配置不该把卡片撑长
const showUpdateSettings = ref(false)
const updateSettingsRoot = ref(null)
useModalBehavior(() => { showUpdateSettings.value = false }, updateSettingsRoot, showUpdateSettings)

// 代理：只用于在线更新下载（GitHub 直连不稳时填这里）
const proxyUrl = ref(state.settings.proxyUrl || '')
const proxyTesting = ref(false)
const proxyResult = ref(null)
async function saveProxy() {
  const v = proxyUrl.value.trim()
  const ok = await actions.saveSettings({ proxyUrl: v })
  if (ok) actions.toast(v ? '代理已保存，更新会走它' : '已改为直连', 'success')
}
async function testProxy() {
  proxyTesting.value = true
  proxyResult.value = null
  try {
    const r = await api.testProxy({ proxyUrl: proxyUrl.value.trim() })
    proxyResult.value = r
    if (r.ok) actions.toast(`代理可用，${r.ms}ms`, 'success')
    else actions.toast('代理不通：' + (r.error || `HTTP ${r.status}`), 'error')
  } catch (e) {
    proxyResult.value = { ok: false, error: e.message }
    actions.toast('测试失败：' + e.message, 'error')
  } finally {
    proxyTesting.value = false
  }
}

function toggleAutoCheck() {
  actions.saveSettings({ autoCheckUpdate: !autoCheck.value })
}

onMounted(() => {
  resumeUpdate()
  if (!autoCheck.value) return
  let last = 0
  try { last = Number(localStorage.getItem(LAST_CHECK_KEY) || 0) } catch { /* 忽略 */ }
  // 每天自动检查一次（24 小时）
  if (Date.now() - last > 24 * 3600 * 1000) {
    setTimeout(() => checkUpdate(true), 2000)
  }
})

// 关于与版权：说明文字较长，卡片只留摘要，完整素材与授权说明进二级弹窗
const showAboutModal = ref(false)
const aboutRoot = ref(null)
useModalBehavior(() => { showAboutModal.value = false }, aboutRoot, showAboutModal)

// 访问口令：改口令/关口令都是低频敏感操作，卡片只留状态，操作收进二级弹窗
const showAccessModal = ref(false)
const accessRoot = ref(null)
useModalBehavior(() => { showAccessModal.value = false }, accessRoot, showAccessModal)
// 配图清晰度：列表统一先用缩略图保证秒开，详情页始终加载原图。
const imageMode = computed(() => state.settings.imageMode || 'auto')
const lanNow = computed(() => isLanHost())
const currentModeLabel = computed(() => imageSize(imageMode.value) === '' ? '原图' : '缩略图')
const imageChoices = [
  { key: 'auto', label: '智能', icon: '⚡', hint: '列表缩略图秒开 · 详情自动原图' },
  { key: 'full', label: '原图', icon: '🖼️', hint: '始终加载清晰原图' },
  { key: 'thumb', label: '省流量', icon: '📶', hint: '始终加载缩略图' },
]
function chooseImageMode(m) {
  if (imageMode.value === m) return
  actions.saveSettings({ imageMode: m })
}

const autoBackup = computed(() => state.settings.autoBackup !== false)
const backupKeep = computed(() => state.settings.backupKeep || 7)
async function loadBackups() {
  try { backups.value = await api.listBackups() } catch { backups.value = [] }
}
function toggleAutoBackup() { actions.saveSettings({ autoBackup: !autoBackup.value }) }
async function runBackup() {
  backupBusy.value = true
  try {
    const r = await api.runBackupNow()
    actions.toast(`备份好了：${r.name}`, 'success')
    await actions.refresh()
    await loadBackups()
  } catch (err) {
    actions.toast(err.message || '备份失败', 'error')
  } finally {
    backupBusy.value = false
  }
}
loadBackups()

watch(() => state.settings.familyName, (v) => { familyName.value = v || '我们家' })

async function saveName() {
  if (!familyName.value.trim()) {
    actions.toast('家的名字不能为空', 'warn')
    shake('input[aria-label="家的名字"]')
    return
  }
  const ok = await actions.saveSettings({ familyName: familyName.value.trim() })
  if (ok) bounce('input[aria-label="家的名字"]')
}

// 菜系显示开关：关掉后，该菜系及其菜品在菜谱里不再出现（「全部」里也不会），随时可再打开。
const allCuisines = computed(() => state.cuisines || [])
const showCuisineModal = ref(false)
useModalBehavior(() => { showCuisineModal.value = false }, cuisineRoot, showCuisineModal)
const visibleCount = computed(() => (allCuisines.value || []).length - (state.hiddenCuisines || []).length)
function isHidden(key) {
  return isCuisineHidden(key, state.hiddenCuisines)
}
function toggleCuisine(key) {
  const hidden = new Set(state.hiddenCuisines || [])
  if (hidden.has(key)) hidden.delete(key)
  else hidden.add(key)
  actions.setHiddenCuisines([...hidden])
}

// 菜系排序：顺序存进 settings.cuisineOrder，后端 bootstrap 时按它返回，
// 所有设备看到的是同一个顺序。本地先重排，避免等下一次拉取。
async function moveCuisine(key, delta) {
  const list = allCuisines.value.map((c) => c.key)
  const i = list.indexOf(key)
  const j = i + delta
  if (i < 0 || j < 0 || j >= list.length) return
  const [moved] = list.splice(i, 1)
  list.splice(j, 0, moved)
  const map = new Map(allCuisines.value.map((c) => [c.key, c]))
  state.cuisines = list.map((k) => map.get(k)).filter(Boolean)
  try {
    await api.saveSettings({ cuisineOrder: list })
  } catch (err) {
    actions.toast(err.message, 'error')
  }
}
async function resetCuisineOrder() {
  try {
    await api.saveSettings({ cuisineOrder: [] })
    // actions.refresh 是空操作，这里直接拉一次拿到内置顺序
    const data = await api.bootstrap()
    if (data?.cuisines) state.cuisines = data.cuisines
    actions.toast('已恢复默认顺序', 'success')
  } catch (err) {
    actions.toast(err.message, 'error')
  }
}
function showAllCuisines() {
  actions.setHiddenCuisines([])
}
function hideAllCuisines() {
  actions.setHiddenCuisines((allCuisines.value || []).map((c) => c.key))
}

async function doExport() {
  try {
    const blob = await api.exportBackup()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `family-menu-${new Date().toISOString().slice(0, 10)}.zip`
    document.body.appendChild(link)
    link.click()
    link.remove()
    setTimeout(() => URL.revokeObjectURL(url), 3000)
    actions.toast('备份已下载，记得放到 NAS 另一个文件夹', 'success')
  } catch (err) {
    actions.toast(err.message, 'error')
  }
}

async function doExportExcel() {
  try {
    const blob = await api.exportExcel()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `家里吃点啥-${new Date().toISOString().slice(0, 10)}.xlsx`
    document.body.appendChild(link)
    link.click()
    link.remove()
    setTimeout(() => URL.revokeObjectURL(url), 3000)
    actions.toast('Excel 已下载：今日菜单 + 菜谱清单两个工作表', 'success')
  } catch (err) {
    actions.toast(err.message, 'error')
  }
}

async function doImport(event) {
  const file = event.target.files?.[0]
  if (!file) return
  const ok = await confirmDialog({
    title: '恢复备份',
    message: '恢复备份会覆盖当前的菜品、成员和点菜记录，确定继续吗？建议先导出一份当前数据。',
    confirmText: '恢复', cancelText: '取消', danger: true,
  })
  if (!ok) { event.target.value = ''; return }
  const data = new FormData()
  data.append('file', file)
  importBusy.value = true
  await actions.importBackup(data)
  importBusy.value = false
  event.target.value = ''
}

function forgetCode() {
  setAccessCode('')
  actions.toast('已清除本机口令，刷新后需要重新输入', 'success')
}

// ---- 访问口令：在网页里直接开启 / 修改 / 关闭 ----
const accessInput = ref('')
const accessBusy = ref(false)
async function saveAccess() {
  const code = accessInput.value.trim()
  if (code.length < 4) { actions.toast('口令至少 4 个字符', 'warn'); shake('input[aria-label="访问口令"]'); return }
  accessBusy.value = true
  const ok = await actions.setWebAccessCode(code)
  accessBusy.value = false
  if (ok) accessInput.value = ''
}
async function clearAccess() {
  const ok = await confirmDialog({
    title: '关闭访问口令',
    message: '关闭后，同一网络里的任何人打开网页都能查看和修改菜单。确定要关闭吗？',
    confirmText: '关闭口令', cancelText: '取消', danger: true,
  })
  if (!ok) return
  accessBusy.value = true
  await actions.setWebAccessCode('')
  accessBusy.value = false
  accessInput.value = ''
}

// ---- 用餐时段：名字、图标、收起时间都由用户自定义 ----
const cloneMeals = (list) => (Array.isArray(list) && list.length ? list : DEFAULT_MEALS)
  .map((m) => ({ key: m.key, label: m.label, icon: m.icon, end: m.end }))
const mealDraft = ref(cloneMeals(state.meals))
// 只有后端下发的配置真的变了才覆盖草稿，避免编辑中被其它设置保存动作打断
watch(() => JSON.stringify(state.meals || []), () => { mealDraft.value = cloneMeals(state.meals) })
// 卡片只显示摘要，编辑放进弹窗，这样卡片尺寸和其它卡片一致
const showMealsModal = ref(false)
// 哪一餐展开了预设图标面板；-1 表示都收起
const openIconRow = ref(-1)
function pickMealIcon(i, icon) {
  mealDraft.value[i].icon = icon
  openIconRow.value = -1
}
const mealsRoot = ref(null)
useModalBehavior(() => { showMealsModal.value = false }, mealsRoot, showMealsModal)
const effectiveMeals = computed(() => normalizeMeals(state.meals))
const mealSummary = computed(() => effectiveMeals.value.map((m) => m.end).join(' / '))
function openMealsModal() {
  mealDraft.value = cloneMeals(state.meals)
  openIconRow.value = -1
  showMealsModal.value = true
}

function nextEnd(prev) {
  const v = mealEndMinutes(prev)
  if (v < 0) return '20:00'
  const n = Math.min(v + 180, 23 * 60 + 59)
  return `${String(Math.floor(n / 60)).padStart(2, '0')}:${String(n % 60).padStart(2, '0')}`
}
function addMeal() {
  if (mealDraft.value.length >= 8) return
  const last = mealDraft.value[mealDraft.value.length - 1]
  mealDraft.value.push({
    key: `m${Date.now().toString(36)}`,
    label: '新的一餐', icon: '🍽️', end: last ? nextEnd(last.end) : '20:00',
  })
}
function removeMeal(i) {
  if (mealDraft.value.length <= 1) return
  mealDraft.value.splice(i, 1)
}
function resetMeals() {
  mealDraft.value = cloneMeals(DEFAULT_MEALS)
  actions.toast('已恢复默认三餐，点保存后生效', 'info')
}
async function saveMeals() {
  const list = mealDraft.value.map((m) => ({
    key: String(m.key || '').trim(),
    label: String(m.label || '').trim(),
    icon: String(m.icon || '').trim() || '🍽️',
    end: String(m.end || '').trim(),
  }))
  for (let i = 0; i < list.length; i++) {
    const m = list[i]
    if (!m.label) { actions.toast(`第 ${i + 1} 餐还没起名字`, 'warn'); shake(document.querySelectorAll('.meal-edit-label')[i]); return }
    const v = mealEndMinutes(m.end)
    if (v < 0) { actions.toast(`第 ${i + 1} 餐的时间格式应为 HH:MM`, 'warn'); shake(document.querySelectorAll('.meal-edit-time')[i]); return }
    if (i > 0 && v <= mealEndMinutes(list[i - 1].end)) {
      actions.toast(`第 ${i + 1} 餐的收起时间要晚于上一餐`, 'warn')
      return
    }
  }
  const ok = await actions.saveSettings({ meals: list })
  if (ok) showMealsModal.value = false
}

// ---------- 每周掌勺 / 洗碗排班 ----------
// 放在 settings.json 里，随备份一起走；只做提醒与一键套用，不自动改写已定好的分工。
const rosterEnabled = ref(false)
const rosterDays = reactive({
  0: { chefId: '', washId: '' }, 1: { chefId: '', washId: '' },
  2: { chefId: '', washId: '' }, 3: { chefId: '', washId: '' },
  4: { chefId: '', washId: '' }, 5: { chefId: '', washId: '' },
  6: { chefId: '', washId: '' },
})
watch(() => state.settings.roster, (r) => {
  rosterEnabled.value = !!r?.enabled
  for (const k of Object.keys(rosterDays)) {
    rosterDays[k].chefId = r?.days?.[k]?.chefId || ''
    rosterDays[k].washId = r?.days?.[k]?.washId || ''
  }
}, { immediate: true, deep: true })
const memberOptions = computed(() => state.members || [])
const rosterFilledDays = computed(() =>
  Object.values(rosterDays).filter((d) => d.chefId || d.washId).length)
// 排班表放弹窗里，卡片本身与「菜系显示」等卡片完全同尺寸
const showRosterModal = ref(false)
const rosterRoot = ref(null)
useModalBehavior(() => { showRosterModal.value = false }, rosterRoot, showRosterModal)
function clearRosterDay(k) { rosterDays[k].chefId = ''; rosterDays[k].washId = '' }
async function saveRoster() {
  const days = {}
  for (const k of Object.keys(rosterDays)) {
    if (rosterDays[k].chefId || rosterDays[k].washId) days[k] = { ...rosterDays[k] }
  }
  const ok = await actions.saveSettings({ roster: { enabled: rosterEnabled.value, days } })
  if (ok) showRosterModal.value = false
}
</script>

<template>
  <section class="settings-screen">
    <div class="section-head settings-head">
      <div class="settings-head-copy">
        <span class="settings-eyebrow"><span aria-hidden="true">✦</span> 家庭控制台</span>
        <h2 class="section-title">⚙️ 设置</h2>
        <div class="section-sub">数据都在你自己的设备里：家庭信息、菜系、时段、排班、口令与备份</div>
      </div>
      <span class="settings-local-pill">
        <span class="settings-local-dot" aria-hidden="true"></span>
        <span class="settings-local-label">仅存本机 / NAS</span>
      </span>
    </div>

    <nav class="settings-category-nav" aria-label="设置分类快捷导航">
      <button
        v-for="(category, i) in settingsCategories"
        :key="category.id"
        type="button"
        class="settings-jump"
        :class="[`is-${category.id}`, { 'is-active': activeSettingsSection === category.id }]"
        :aria-controls="`settings-${category.id}`"
        :aria-current="activeSettingsSection === category.id ? 'location' : undefined"
        @click="jumpToSettings(category.id)"
      >
        <span class="settings-jump-index" aria-hidden="true">{{ String(i + 1).padStart(2, '0') }}</span>
        <span class="settings-jump-icon" aria-hidden="true">{{ category.icon }}</span>
        <span class="settings-jump-copy">
          <b>{{ category.title }}</b>
          <small>{{ category.hint }}</small>
        </span>
        <span class="settings-jump-count">{{ category.count }} 项</span>
        <span class="settings-jump-arrow" aria-hidden="true">↓</span>
      </button>
    </nav>

    <div class="settings-sections">
      <section
        id="settings-family"
        class="settings-section settings-section-family"
        data-settings-section
        aria-labelledby="settings-family-title"
        tabindex="-1"
      >
        <header class="settings-section-head">
          <span class="settings-section-icon" aria-hidden="true">🏠</span>
          <div class="settings-section-copy">
            <span class="settings-section-kicker">01 · 家庭</span>
            <h3 id="settings-family-title">先把这个家调成你们习惯的样子</h3>
            <p>家里的称呼、吃饭时间和每周分工。</p>
          </div>
          <span class="settings-section-total">3 项设置</span>
        </header>

        <div class="settings-section-grid settings-family-grid">
          <div class="card setting-card setting-card-wide sc-name">
        <div class="setting-title">🏠 家的名字</div>
        <p class="setting-desc">显示在左上角，改成你们家习惯的叫法。</p>
        <div class="setting-row setting-row-inline">
          <input v-model="familyName" class="input" maxlength="24" aria-label="家的名字" @keyup.enter="saveName" />
          <button class="btn btn-primary btn-sm" :disabled="pending('settings')" @click="saveName">
            <span v-if="pending('settings')" class="btn-spinner btn-spinner-light" aria-hidden="true" />
            {{ pending('settings') ? '保存中' : '保存名字' }}
          </button>
        </div>
      </div>

          <div class="card setting-card sc-roster">
        <div class="setting-title">
          📅 每周掌勺排班
          <span class="tag setting-title-tag" :class="rosterEnabled ? 'tag-green' : ''">
            {{ rosterEnabled ? '已开启' : '未开启' }}
          </span>
        </div>
        <p class="setting-desc">
          定好每天谁掌勺、谁洗碗 <HelpTip text="今日页会提醒「按排班今天该谁」，可一键套用；排班只作提醒，不会改写已定好的分工。" />
        </p>
        <!-- 开关与保存都收进弹窗，卡片里不再放第二个「保存排班」，避免功能重复 -->
        <button class="btn btn-soft btn-sm cuisine-manage-btn roster-manage-btn" @click="showRosterModal = true">
          管理每周排班
          <span class="cuisine-count">{{ rosterFilledDays }} / 7 天已安排</span>
          <span class="chev">›</span>
        </button>
      </div>

          <div class="card setting-card sc-meals">
        <div class="setting-title">🕐 用餐时段</div>
        <p class="setting-desc">
          给每一餐起名字、定收起时间；过了时间，今天这一餐的菜会自动从今日菜单收起，历史记录仍保留。
        </p>
        <div class="setting-meta tiny muted">收起时间 {{ mealSummary }}</div>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="openMealsModal">
          管理用餐时段
          <span class="cuisine-count">{{ effectiveMeals.length }} 餐</span>
          <span class="chev">›</span>
        </button>
      </div>
        </div>
      </section>

      <section
        id="settings-display"
        class="settings-section settings-section-display"
        data-settings-section
        aria-labelledby="settings-display-title"
        tabindex="-1"
      >
        <header class="settings-section-head">
          <span class="settings-section-icon" aria-hidden="true">🍽️</span>
          <div class="settings-section-copy">
            <span class="settings-section-kicker">02 · 内容</span>
            <h3 id="settings-display-title">只留下你们爱看、顺手的内容</h3>
            <p>控制菜谱范围，以及这台设备上的图片和外观。</p>
          </div>
          <span class="settings-section-total">3 项设置</span>
        </header>

        <div class="settings-section-grid settings-display-grid">
          <div class="card setting-card sc-cuisine">
        <div class="setting-title">🍽️ 菜系显示</div>
        <p class="setting-desc">
          选择菜谱里显示哪些菜系 <HelpTip text="关掉的菜系在分类栏和「全部」里都不再出现，想看回来随时打开；用 ↑↓ 调整它们在顶部的顺序。" />
        </p>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showCuisineModal = true">
          管理菜系显示
          <span class="cuisine-count">{{ visibleCount }} / {{ allCuisines.length }} 显示中</span>
          <span class="chev">›</span>
        </button>
      </div>

          <div class="card setting-card sc-image">
        <div class="setting-title">
          🖼️ 配图清晰度
          <span class="tag setting-title-tag" :class="lanNow ? 'tag-green' : ''">
            {{ lanNow ? '局域网' : '外网' }} · 当前{{ currentModeLabel }}
          </span>
        </div>
        <p class="setting-desc">
          菜谱列表和今日菜单的配图大小 <HelpTip text="「按网络」会自动判断：家里局域网（10./192.168./172.16-31. 网段）加载 1200px 原图更清晰；从外网访问自动换成 400px 缩略图省流量。详情页始终用原图，不受此项影响。" />
        </p>
        <div class="setting-actions image-choices">
          <button
            v-for="c in imageChoices" :key="c.key" class="btn btn-sm"
            :class="imageMode === c.key ? 'btn-primary' : 'btn-soft'"
            :aria-pressed="imageMode === c.key" :title="c.hint" @click="chooseImageMode(c.key)"
          >{{ c.icon }} {{ c.label }}</button>
        </div>
        <div class="setting-meta tiny muted">{{ imageChoices.find((c) => c.key === imageMode)?.hint }}</div>
      </div>

          <div class="card setting-card setting-card-wide sc-theme">
        <div class="setting-title">🌙 外观</div>
        <p class="setting-desc">
          主题与字号只影响这台设备 <HelpTip text="选「自动」会随手机或系统的深浅模式切换；字号三档改的是全站文字大小，两项设置都记在本机，不进备份。" />
        </p>
        <div class="setting-actions theme-choices">
          <button
            v-for="t in themeChoices" :key="t.key" class="btn btn-sm"
            :class="themeMode === t.key ? 'btn-primary' : 'btn-soft'"
            :aria-pressed="themeMode === t.key" :title="t.label" @click="chooseTheme(t.key, $event)"
          >{{ t.icon }} {{ t.short }}</button>
        </div>
        <div class="font-row">
          <span class="font-label">字号</span>
          <div class="setting-actions font-choices">
            <button
              v-for="f in fontChoices" :key="f.key" class="btn btn-sm"
              :class="fontScale === f.key ? 'btn-primary' : 'btn-soft'"
              :aria-pressed="fontScale === f.key" @click="chooseFont(f.key)"
            >{{ f.label }}</button>
          </div>
        </div>
      </div>
        </div>
      </section>

      <section
        id="settings-data"
        class="settings-section settings-section-data"
        data-settings-section
        aria-labelledby="settings-data-title"
        tabindex="-1"
      >
        <header class="settings-section-head">
          <span class="settings-section-icon" aria-hidden="true">🛡️</span>
          <div class="settings-section-copy">
            <span class="settings-section-kicker">03 · 安全</span>
            <h3 id="settings-data-title">守住访问，也守住每一份家常数据</h3>
            <p>口令与备份放一起，数据位置和同步状态也能随时确认。</p>
          </div>
          <span class="settings-section-total">3 项设置</span>
        </header>

        <div class="settings-section-grid settings-data-grid">
          <div class="card setting-card sc-access">
        <div class="setting-title">
          🔒 访问口令
          <span class="tag setting-title-tag" :class="state.accessRequired ? 'tag-green' : ''">
            {{ state.accessRequired ? '已开启' : '未开启' }}
          </span>
        </div>
        <p class="setting-desc" v-if="state.accessEnvManaged">
          当前由服务端环境变量 <code>MENU_ACCESS_CODE</code> 控制，改 docker-compose.yml 后重建容器生效。
        </p>
        <template v-else>
          <p class="setting-desc">
            {{ state.accessRequired ? '其它设备要先输口令；本机已记住' : '开启后，其它设备要先输口令' }}
            <HelpTip :text="state.accessRequired ? '本机浏览器已记住口令，无需重复输入。口令只存服务端数据目录，不会写进备份 zip。' : '开启后，其它设备打开网页要先输入口令才能点菜。口令存在服务端数据目录，不会写进备份 zip。'" />
          </p>
          <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showAccessModal = true">
            管理访问口令
            <span class="chev">›</span>
          </button>
        </template>
      </div>

          <div class="card setting-card sc-backup">
        <div class="setting-title">
          💾 备份与恢复
          <span class="tag setting-title-tag" :class="autoBackup ? 'tag-green' : ''">
            {{ autoBackup ? '自动备份中' : '仅手动' }}
          </span>
        </div>
        <p class="setting-desc">
          每天 03:30 自动打包数据与图片，保留最近 {{ backupKeep }} 份；也可随时导出 / 恢复。
        </p>
        <div class="setting-meta tiny muted">
          共 <span class="value-pop" :key="backups.length">{{ backups.length }}</span> 份备份<template v-if="backups.length"> · 最近 {{ backups[0].at }}</template>
        </div>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showBackupModal = true">
          管理备份
          <span class="cuisine-count">{{ backups.length }} 份</span>
          <span class="chev">›</span>
        </button>
      </div>

          <div class="card setting-card setting-card-wide sc-storage">
        <div class="setting-title">📂 数据与配置在哪</div>
        <p class="setting-desc">
          两个平级目录，换机器挂回去即可 <HelpTip text="data/ 存菜品、成员、计划、口令哈希与图片；config/ 存设置。都是纯文本，不依赖数据库，无需迁移。" />
        </p>
        <details class="setting-details">
          <summary>查看目录结构</summary>
          <div class="dish-tags mt-8">
            <span class="tag">data/ · dishes / members / plans / pins / uploads / backups</span>
            <span class="tag">config/ · settings.json</span>
          </div>
        </details>
        <div class="setting-meta tiny muted">
          🔄 多人实时同步：<b>{{ realtimeText }}</b>
        </div>
      </div>
        </div>
      </section>

      <section
        id="settings-system"
        class="settings-section settings-section-system"
        data-settings-section
        aria-labelledby="settings-system-title"
        tabindex="-1"
      >
        <header class="settings-section-head">
          <span class="settings-section-icon" aria-hidden="true">⬆️</span>
          <div class="settings-section-copy">
            <span class="settings-section-kicker">04 · 系统</span>
            <h3 id="settings-system-title">更新、版本和来源，都集中在这里</h3>
            <p>检查与安装更新，查看每次改了什么，以及素材授权说明。</p>
          </div>
          <span class="settings-section-total">3 项设置</span>
        </header>

        <div class="settings-section-grid settings-system-grid">
          <div ref="updateCard" class="card setting-card setting-card-wide setting-card-feature sc-update" :class="`is-${updateVisualState}`">
        <span class="update-card-aura" aria-hidden="true"></span>
        <div class="setting-title">
          ⬆️ 在线更新
          <span class="tag setting-title-tag" :class="updateTagClass">
            {{ updateTagText }}
          </span>
        </div>
        <p class="setting-desc">
          从 GitHub 拉取新版本，在容器内直接更新，菜和图片都不会动
          <HelpTip text="更新包带 SHA256 校验，下载后原子替换程序并原地重启，容器不会重建。若新版启动失败会自动回滚到旧版；检查更新默认每天自动做一次。" />
        </p>
        <div v-if="noteLines.length" class="update-note">
          <span class="update-note-label">本次更新</span>
          <div class="update-note-lines">
            <div v-for="(line, i) in noteLines" :key="i">{{ line }}</div>
          </div>
        </div>
        <div
          v-if="updateStatusVisible"
          class="update-progress" :class="`is-${updateVisualState}`"
          role="status" aria-live="polite"
        >
          <div class="update-state-head">
            <span
              class="update-state-icon" :class="{ 'is-spinning': updateBusy, 'is-result': !updateBusy }"
              aria-hidden="true"
            >{{ updateStatusIcon }}</span>
            <span class="update-state-copy">
              <strong>{{ updateStatusLabel }}</strong>
              <span>{{ updateMsg }}</span>
            </span>
            <b v-if="updateApplying" class="update-percent">
              {{ updateProgress }}<small>%</small>
            </b>
          </div>
          <div
            v-if="updateBusy"
            class="update-bar"
            :class="{ 'is-indeterminate': updateChecking, 'is-installing': updateApplying && updateProgress >= 85 }"
            role="progressbar"
            :aria-valuenow="updateChecking ? undefined : updateProgress"
            aria-valuemin="0" aria-valuemax="100"
            :aria-label="updateChecking ? '正在检查更新' : '在线更新进度'"
          ><span :style="{ width: updateProgress + '%' }"><i aria-hidden="true"></i></span></div>
          <span v-if="updatePhaseHint" class="update-phase-hint">{{ updatePhaseHint }}</span>
        </div>
        <div v-if="updateErr" class="update-error tiny" role="alert">
          <span class="update-error-icon" aria-hidden="true">!</span>
          <span>{{ updateErr }}</span>
        </div>
        <div class="setting-actions update-actions">
          <button
            class="btn btn-soft btn-sm update-check-btn"
            :disabled="updateBusy" :aria-busy="updateChecking" @click="checkUpdate(false)"
          >
            <span v-if="updateChecking" class="btn-spinner" aria-hidden="true" />
            <span v-else class="update-action-icon" aria-hidden="true">⌕</span>
            {{ updateChecking ? '正在检查…' : '检查更新' }}
          </button>
          <button v-if="updateInfo.available" class="btn btn-primary btn-sm" :disabled="updateBusy" :aria-busy="updateApplying" @click="doUpdate(false)">
            <span v-if="updateApplying" class="btn-spinner btn-spinner-light" aria-hidden="true" />
            <span v-else class="update-action-icon" aria-hidden="true">↑</span>
            {{ updateApplying ? `更新中 ${updateProgress}%` : `更新到 v${updateInfo.latest}` }}
          </button>
          <button v-else class="btn btn-soft btn-sm" :disabled="updateBusy" :aria-busy="updateApplying" @click="doUpdate(true)">
            <span v-if="updateApplying" class="btn-spinner" aria-hidden="true" />
            <span v-else class="update-action-icon" aria-hidden="true">↻</span>
            {{ updateApplying ? `更新中 ${updateProgress}%` : '⚡ 强制重装' }}
          </button>
        </div>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showUpdateSettings = true">
          更新设置
          <span class="cuisine-count">{{ proxyUrl ? '已配代理' : (autoCheck ? '每日检查' : '仅手动') }}</span>
          <span class="chev">›</span>
        </button>
      </div>

          <div class="card setting-card sc-version">
        <div class="setting-title">
          📋 版本更新
          <span class="tag setting-title-tag">v{{ state.version }}</span>
        </div>
        <p class="setting-desc">这次改了什么，以及之前每个版本都做了什么。</p>
        <div class="setting-meta tiny muted">{{ buildDay }} 构建<template v-if="changelog.length"> · 共 {{ changelog.length }} 个版本</template></div>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="openVersionModal">
          查看更新记录
          <span class="chev">›</span>
        </button>
      </div>

          <div class="card setting-card sc-about">
        <div class="setting-title">🎨 关于与版权</div>
        <p class="setting-desc">插画由 AI 生成，文案自写，源码 MIT；未用第三方图库或模板。</p>
        <div class="setting-meta tiny muted">Go + Vue 3 · 单文件服务端 · 数据全在本地</div>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showAboutModal = true">
          查看素材与授权
          <span class="chev">›</span>
        </button>
      </div>
        </div>
      </section>
    </div>

    <Teleport to="body">
      <Transition name="modal">
        <div v-if="showAccessModal" ref="accessRoot" class="modal-backdrop" @click.self="showAccessModal = false">
          <div class="modal" role="dialog" aria-modal="true" aria-label="访问口令">
            <div class="modal-head">
              <h2>🔒 访问口令</h2>
              <button class="icon-btn" aria-label="关闭" @click="showAccessModal = false">✕</button>
            </div>
            <p class="setting-desc">
              开启后，其它设备打开网页要先输入口令。口令存在服务端数据目录，不会写进备份 zip；本机记住后无需重复输入。
            </p>
            <div class="setting-row setting-row-inline mt-12">
              <input
                v-model="accessInput" class="input" type="password" maxlength="64"
                placeholder="设置访问口令（至少 4 位）" autocomplete="new-password"
                aria-label="访问口令" @keyup.enter="saveAccess"
              />
              <button class="btn btn-primary btn-sm" :disabled="accessBusy" @click="saveAccess">
                {{ accessBusy ? '保存中' : (state.accessRequired ? '改口令' : '开启口令') }}
              </button>
            </div>
            <div class="setting-actions mt-10" v-if="state.accessRequired">
              <button class="btn btn-sm btn-ghost" :disabled="accessBusy" @click="clearAccess">关闭口令</button>
              <button class="btn btn-sm btn-ghost" @click="forgetCode">本机重新输入</button>
            </div>
            <div class="modal-foot">
              <button class="btn btn-primary btn-sm" @click="showAccessModal = false">完成</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showUpdateSettings" ref="updateSettingsRoot" class="modal-backdrop" @click.self="showUpdateSettings = false">
          <div class="modal" role="dialog" aria-modal="true" aria-label="更新设置">
            <div class="modal-head">
              <h2>⬆️ 更新设置</h2>
              <button class="icon-btn" aria-label="关闭" @click="showUpdateSettings = false">✕</button>
            </div>
            <p class="setting-desc">
              在线更新从 GitHub 拉取新版本，在新版启动失败时会自动回滚
              <HelpTip text="更新包带 SHA256 校验，下载后原子替换程序并原地重启，容器不会重建、数据不会丢。代理只影响更新下载，业务接口始终直连。" />
            </p>

            <div class="field">
              <label class="update-option">
                <input type="checkbox" :checked="autoCheck" @change="toggleAutoCheck" />
                <span>
                  <b>每天自动检查新版本</b>
                  <span class="tiny muted block">打开设置页时每天静默检查一次，有更新才提示</span>
                </span>
              </label>
            </div>

            <div class="field">
              <label for="proxy-url">更新走代理</label>
              <div class="proxy-row">
                <input
                  id="proxy-url" v-model="proxyUrl" class="input" maxlength="120"
                  placeholder="http://10.10.10.254:7890" aria-label="代理地址"
                  @keyup.enter="saveProxy"
                />
                <button class="btn btn-soft btn-sm" :disabled="proxyTesting" @click="testProxy">
                  {{ proxyTesting ? '测试中…' : '测试' }}
                </button>
                <button class="btn btn-primary btn-sm" :disabled="pending('settings')" @click="saveProxy">保存</button>
              </div>
              <span class="tiny muted">留空表示直连；支持 http:// / https:// / socks5://</span>
            </div>
            <div v-if="proxyResult" class="proxy-result" :class="proxyResult.ok ? 'ok' : 'bad'">
              {{ proxyResult.ok ? `✓ 代理可用（${proxyResult.ms}ms，HTTP ${proxyResult.status}）` : `✗ ${proxyResult.error || ('HTTP ' + proxyResult.status)}` }}
            </div>

            <div class="modal-foot">
              <span class="grow"></span>
              <button class="btn" @click="showUpdateSettings = false">关闭</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showVersionModal" ref="versionRoot" class="modal-backdrop" @click.self="showVersionModal = false">
          <div class="modal modal-wide version-modal" role="dialog" aria-modal="true" aria-label="版本更新">
            <div class="modal-head">
              <h2>📋 版本更新</h2>
              <button class="icon-btn" @click="showVersionModal = false" aria-label="关闭">✕</button>
            </div>

            <div class="version-current">
              <span class="version-badge">v{{ state.version }}</span>
              <span class="tiny muted">{{ state.buildTime }} 构建</span>
            </div>

            <div class="version-list">
              <div v-if="changelogLoading" class="version-tip">读取中…</div>
              <div v-else-if="!changelog.length" class="version-tip">暂无更新记录</div>
              <article v-for="(v, i) in changelog" :key="v.version" class="version-item" :class="{ 'is-current': i === 0 }">
                <header class="version-head">
                  <span class="version-no">v{{ v.version }}</span>
                  <span class="version-date">{{ v.date }}</span>
                  <span v-if="i === 0" class="tag tag-green">当前</span>
                </header>
                <div v-for="sec in v.sections" :key="sec.kind" class="version-sec">
                  <span class="version-kind" :class="kindClass(sec.kind)">{{ sec.kind }}</span>
                  <ul class="version-lines">
                    <li v-for="(line, j) in sec.lines" :key="j">{{ line }}</li>
                  </ul>
                </div>
              </article>
            </div>

            <div class="modal-foot">
              <button class="btn btn-primary btn-sm" @click="showVersionModal = false">知道了</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showAboutModal" ref="aboutRoot" class="modal-backdrop" @click.self="showAboutModal = false">
          <div class="modal" role="dialog" aria-modal="true" aria-label="关于与版权">
            <div class="modal-head">
              <h2>🎨 关于与版权</h2>
              <button class="icon-btn" aria-label="关闭" @click="showAboutModal = false">✕</button>
            </div>

            <div class="about-list">
              <div class="about-item">
                <span class="about-ico">📜</span>
                <div>
                  <div class="about-label">开源许可</div>
                  <div class="about-text">本项目以 MIT 许可开源，可自由使用、修改、商用与再分发，保留版权与许可声明即可。第三方依赖仅 Vue 3、Vite（MIT）与 Go 标准库、golang.org/x/image（BSD-3-Clause）。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">🖼️</span>
                <div>
                  <div class="about-label">图片素材</div>
                  <div class="about-text">菜系封面与菜品图均由 AI 生成，不含任何图库照片或菜谱网站图片；图标为 emoji 与自绘 SVG，字体走系统字体栈。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">📝</span>
                <div>
                  <div class="about-label">菜名与文案</div>
                  <div class="about-text">菜名、菜系与地域属通用饮食常识；描述、标签、辣度、耗时与做法步骤均为自写，未摘录任何菜谱网站或图书。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">📐</span>
                <div>
                  <div class="about-label">精确用量</div>
                  <div class="about-text">184 道菜的克数/毫升取自公共领域项目 HowToCook（Unlicense），其余按家常份量自写，已标注来源。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">🚫</span>
                <div>
                  <div class="about-label">未使用的素材</div>
                  <div class="about-text">未使用任何第三方图库照片、商业模板、UI 组件库、外部字体或图标包，也不含任何音频文件（音效为实时合成）。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">⚠️</span>
                <div>
                  <div class="about-label">免责声明</div>
                  <div class="about-text">菜谱用量与步骤为家庭做法整理，未经专业营养或食品安全审核，请按自家口味与食材情况调整。所有数据仅保存在你自己的设备上，本项目不收集任何信息。</div>
                </div>
              </div>
              <div class="about-item">
                <span class="about-ico">📄</span>
                <div>
                  <div class="about-label">授权与完整说明</div>
                  <div class="about-text">源码以 MIT 授权；完整素材清单见仓库根目录 <code>NOTICE.md</code>。</div>
                </div>
              </div>
            </div>

            <div class="about-stack">
              <span class="tag">Go + Vue 3</span>
              <span class="tag">单文件服务端</span>
              <span class="tag">数据全在本地</span>
            </div>

            <div class="modal-foot">
              <button class="btn btn-primary btn-sm" @click="showAboutModal = false">知道了</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showBackupModal" ref="backupRoot" class="modal-backdrop" @click.self="showBackupModal = false">
          <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="备份与恢复">
            <div class="modal-head">
              <h2>💾 备份与恢复</h2>
              <button class="icon-btn" aria-label="关闭" @click="showBackupModal = false">✕</button>
            </div>
            <p class="setting-desc">
              自动备份每天 03:30 把四份数据和图片打包到 <code>backups/</code>，保留最近 {{ backupKeep }} 份；恢复会覆盖当前数据，操作前会再确认。
            </p>
            <label class="switch-row">
              <input type="checkbox" :checked="autoBackup" @change="toggleAutoBackup" />
              <span>每天自动备份</span>
            </label>
            <div class="setting-actions">
              <button class="btn btn-primary btn-sm" :disabled="backupBusy" @click="runBackup">
                {{ backupBusy ? '备份中…' : '⏱ 立即备份' }}
              </button>
              <button class="btn btn-soft btn-sm" @click="doExport">⬇️ 导出 zip</button>
              <button class="btn btn-soft btn-sm" @click="doExportExcel">📊 导出 Excel</button>
              <label class="btn btn-soft btn-sm" :class="{ 'btn-ghost': importBusy }">
                {{ importBusy ? '恢复中…' : '⬆️ 从备份恢复' }}
                <input type="file" accept=".zip,application/zip" hidden @change="doImport" />
              </label>
            </div>
            <div v-if="backups.length" class="backup-list">
              <div v-for="b in backups" :key="b.name" class="backup-row tiny">
                <span>{{ b.at }}</span>
                <span class="muted">{{ (b.size / 1024 / 1024).toFixed(1) }} MB</span>
              </div>
            </div>
            <div v-else class="tiny muted mt-10">还没有备份，点「立即备份」生成第一份。</div>
            <div class="modal-foot">
              <button class="btn btn-primary btn-sm" @click="showBackupModal = false">完成</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showRosterModal" ref="rosterRoot" class="modal-backdrop" @click.self="showRosterModal = false">
          <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="每周掌勺排班">
            <div class="modal-head">
              <h2>📅 每周掌勺排班</h2>
              <button class="icon-btn" aria-label="关闭" @click="showRosterModal = false">✕</button>
            </div>
            <p class="setting-desc">留空表示那天不安排。改完点「保存排班」才生效；今日页只提醒，不改写已定好的分工。</p>
            <label class="switch-row">
              <input type="checkbox" v-model="rosterEnabled" />
              <span>启用每周排班提醒</span>
            </label>
            <div class="roster-grid">
              <div class="roster-row roster-head">
                <span>星期</span><span>👨‍🍳 掌勺</span><span>🧽 洗碗</span><span></span>
              </div>
              <div v-for="(label, idx) in WEEKDAYS" :key="idx" class="roster-row">
                <span class="roster-day">{{ label }}</span>
                <select v-model="rosterDays[String(idx)].chefId" class="input roster-select" :aria-label="`${label}掌勺`">
                  <option value="">不安排</option>
                  <option v-for="m in memberOptions" :key="m.id" :value="m.id">{{ m.avatar }} {{ m.name }}</option>
                </select>
                <select v-model="rosterDays[String(idx)].washId" class="input roster-select" :aria-label="`${label}洗碗`">
                  <option value="">不安排</option>
                  <option v-for="m in memberOptions" :key="m.id" :value="m.id">{{ m.avatar }} {{ m.name }}</option>
                </select>
                <button class="btn btn-ghost btn-sm roster-clear" @click="clearRosterDay(String(idx))" :aria-label="`清空${label}`">✕</button>
              </div>
            </div>
            <div class="modal-foot">
              <button class="btn" @click="showRosterModal = false">关闭</button>
              <button class="btn btn-primary" :disabled="pending('settings')" @click="saveRoster">保存排班</button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showMealsModal" ref="mealsRoot" class="modal-backdrop" @click.self="showMealsModal = false">
          <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="用餐时段">
            <div class="modal-head">
              <h2>🕐 用餐时段</h2>
              <button class="icon-btn" @click="showMealsModal = false" aria-label="关闭">✕</button>
            </div>
            <p class="setting-desc">
              名字、图标和收起时间都能改（24 小时制，时间需逐餐递增）。删掉某一餐后，之前点的菜会归到「其他」，不会丢。
            </p>
            <div class="meal-edit-list">
              <div v-for="(m, i) in mealDraft" :key="m.key" class="meal-edit-row">
                <div class="meal-edit-main">
                  <div class="meal-icon-wrap">
                    <button
                      type="button" class="input meal-icon-trigger"
                      :aria-label="`第 ${i + 1} 餐图标，点开选择`" :aria-expanded="openIconRow === i"
                      @click="openIconRow = openIconRow === i ? -1 : i"
                    >{{ m.icon || '🍽️' }}</button>
                    <div v-if="openIconRow === i" class="meal-icon-panel">
                      <button
                        v-for="ic in mealIconChoices" :key="ic" type="button"
                        class="meal-icon-option" :class="{ active: m.icon === ic }"
                        :aria-label="`选图标 ${ic}`" @click="pickMealIcon(i, ic)"
                      >{{ ic }}</button>
                      <input
                        v-model="m.icon" class="input meal-icon-custom" maxlength="4"
                        placeholder="或自定义一个 emoji" :aria-label="`自定义第 ${i + 1} 餐图标`"
                      />
                    </div>
                  </div>
                  <input v-model="m.label" class="input meal-edit-label" maxlength="12" placeholder="这一餐叫什么" :aria-label="`第 ${i + 1} 餐名字`" />
                  <button
                    class="icon-btn icon-btn-danger" :disabled="mealDraft.length <= 1"
                    :aria-label="`删除第 ${i + 1} 餐`" title="删除这一餐" @click="removeMeal(i)"
                  >✕</button>
                </div>
                <div class="meal-edit-sub">
                  <span class="meal-edit-to">收起时间</span>
                  <input v-model="m.end" type="time" class="input meal-edit-time" :aria-label="`第 ${i + 1} 餐收起时间`" />
                  <span class="meal-edit-hint tiny muted">过后自动收起今日菜单</span>
                </div>
              </div>
            </div>
            <div class="setting-actions meal-edit-actions">
              <button class="btn btn-soft btn-sm" :disabled="mealDraft.length >= 8" @click="addMeal">＋ 添加一餐</button>
              <span class="grow"></span>
              <button class="btn btn-ghost btn-sm" @click="resetMeals">恢复默认</button>
              <button class="btn btn-primary btn-sm" :disabled="pending('settings')" @click="saveMeals">
                <span v-if="pending('settings')" class="btn-spinner btn-spinner-light" aria-hidden="true" />
                {{ pending('settings') ? '保存中' : '保存时段' }}
              </button>
            </div>
          </div>
        </div>
      </Transition>

      <Transition name="modal">
        <div v-if="showCuisineModal" ref="cuisineRoot" class="modal-backdrop" @click.self="showCuisineModal = false">
          <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="菜系显示">
          <div class="modal-head">
            <h2>🍽️ 菜系显示</h2>
            <button class="icon-btn" @click="showCuisineModal = false" aria-label="关闭">✕</button>
          </div>
          <p class="setting-desc">勾选的菜系会在菜谱里显示，取消勾选则隐藏（随时可恢复）。用 ↑↓ 调整它们在菜谱顶部的排列顺序。</p>
          <div class="cuisine-toggle-list modal-cuisine-list">
            <div v-for="(c, i) in allCuisines" :key="c.key" class="cuisine-toggle" :class="{ 'is-hidden': isHidden(c.key) }">
              <label class="cuisine-toggle-pick">
                <input type="checkbox" :checked="!isHidden(c.key)" @change="toggleCuisine(c.key)" />
                <span class="cuisine-toggle-name">{{ c.icon }} {{ c.name }}</span>
              </label>
              <span class="cuisine-order-actions">
                <button type="button" class="order-btn" :disabled="i === 0"
                        aria-label="上移" title="上移" @click.prevent="moveCuisine(c.key, -1)">↑</button>
                <button type="button" class="order-btn" :disabled="i === allCuisines.length - 1"
                        aria-label="下移" title="下移" @click.prevent="moveCuisine(c.key, 1)">↓</button>
              </span>
            </div>
          </div>
          <div class="modal-foot cuisine-modal-foot">
            <span class="cuisine-foot-count">{{ visibleCount }} / {{ allCuisines.length }} 显示中</span>
            <button class="btn btn-ghost btn-sm" @click="resetCuisineOrder">恢复默认顺序</button>
            <button class="btn btn-ghost btn-sm" @click="showAllCuisines">全选显示</button>
            <button class="btn btn-ghost btn-sm" @click="hideAllCuisines">全部隐藏</button>
            <button class="btn btn-primary btn-sm" @click="showCuisineModal = false">完成</button>
          </div>
        </div>
      </div>
      </Transition>
    </Teleport>
  </section>
</template>
