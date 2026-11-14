<script setup>
import { inject, ref, watch, computed, reactive } from 'vue'
import { api, setAccessCode } from '../api'
import { isCuisineHidden, DEFAULT_MEALS, mealEndMinutes, normalizeMeals, mealIconChoices, WEEKDAYS } from '../utils'
import { useModalBehavior } from '../modal'
import { themeChoices, currentThemeMode, setThemeMode } from '../theme'
import { shake, bounce } from '../feedback'

const { state, actions, confirmDialog, pending } = inject('app')
const familyName = ref(state.settings.familyName || '我们家')
const importBusy = ref(false)
const cuisineRoot = ref(null)
const themeMode = ref(currentThemeMode())
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
  <section>
    <div class="section-head">
      <div>
        <h2 class="section-title">⚙️ 设置</h2>
        <div class="section-sub">数据都在你自己的设备里：家庭信息、菜系、时段、排班、备份与访问口令</div>
      </div>
    </div>

    <div class="settings-grid">
      <div class="card setting-card sc-name">
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

      <div class="card setting-card sc-roster">
        <div class="setting-title">
          📅 每周掌勺排班
          <span class="tag setting-title-tag" :class="rosterEnabled ? 'tag-green' : ''">
            {{ rosterEnabled ? '已开启' : '未开启' }}
          </span>
        </div>
        <p class="setting-desc">
          给每天定好谁掌勺、谁洗碗；今日页会提醒「按排班今天该谁」，可一键套用。
        </p>
        <!-- 开关与保存都收进弹窗，卡片里不再放第二个「保存排班」，避免功能重复 -->
        <button class="btn btn-soft btn-sm cuisine-manage-btn roster-manage-btn" @click="showRosterModal = true">
          管理每周排班
          <span class="cuisine-count">{{ rosterFilledDays }} / 7 天已安排</span>
          <span class="chev">›</span>
        </button>
      </div>

      <div class="card setting-card sc-theme">
        <div class="setting-title">🌙 外观</div>
        <p class="setting-desc">
          深色主题只影响这台设备；选「跟随系统」会随手机 / 电脑的深浅模式自动切换，设置会记在本机。
        </p>
        <div class="setting-actions theme-choices">
          <button
            v-for="t in themeChoices" :key="t.key" class="btn btn-sm"
            :class="themeMode === t.key ? 'btn-primary' : 'btn-soft'"
            :aria-pressed="themeMode === t.key" @click="chooseTheme(t.key, $event)"
          >{{ t.label }}</button>
        </div>
      </div>

      <div class="card setting-card sc-cuisine">
        <div class="setting-title">🍽️ 菜系显示</div>
        <p class="setting-desc">
          不想看到的菜系可以关掉，关掉后它在菜谱分类和「全部」里都不再出现；想看回来，随时打开即可。
        </p>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="showCuisineModal = true">
          管理菜系显示
          <span class="cuisine-count">{{ visibleCount }} / {{ allCuisines.length }} 显示中</span>
          <span class="chev">›</span>
        </button>
      </div>

      <div class="card setting-card sc-access">
        <div class="setting-title">
          🔒 访问口令
          <span class="tag setting-title-tag" :class="state.accessRequired ? 'tag-green' : ''">
            {{ state.accessRequired ? '已开启' : '未开启' }}
          </span>
        </div>
        <p class="setting-desc" v-if="state.accessEnvManaged">
          当前由服务端环境变量 <code>MENU_ACCESS_CODE</code> 控制，请改 docker-compose.yml 后重建容器。
        </p>
        <template v-else>
          <p class="setting-desc">
            开启后，其它设备打开网页要先输入口令。口令存在服务端数据目录，不会写进备份 zip；本机记住后无需重复输入。
          </p>
          <div class="setting-row setting-row-inline">
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
        </template>
      </div>

      <div class="card setting-card sc-storage">
        <div class="setting-title">📂 数据与配置在哪</div>
        <p class="setting-desc">
          两个平级目录，都是纯文本、不依赖数据库；换机器时挂回去即可，无需迁移。
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

      <div class="card setting-card sc-about">
        <div class="setting-title">🎨 关于与版权</div>
        <p class="setting-desc">插画由 AI 生成，文案自写，源码 MIT；未用第三方图库或模板。</p>
        <details class="setting-details">
          <summary>查看完整素材与授权说明</summary>
          <p class="setting-desc">
            界面里的菜品插画由 AI 绘制，属于本项目自生成素材；菜名与菜系是通用饮食常识，描述文字自写。未使用任何第三方图库照片、商业模板、字体或图标库。精确用量数值取自公共领域项目 HowToCook（Unlicense）。完整说明见仓库 <code>NOTICE.md</code>。
          </p>
        </details>
        <div class="setting-actions">
          <span class="tag tag-green">家里吃点啥 v{{ state.version }}</span>
          <span class="tag">Go + Vue 3 · 单文件服务端</span>
        </div>
      </div>

      <div class="card setting-card sc-meals">
        <div class="setting-title">🕐 用餐时段</div>
        <p class="setting-desc">
          给每一餐起名字、定收起时间；过了时间，今天这一餐的菜会自动从今日菜单收起，历史记录仍保留。
        </p>
        <button class="btn btn-soft btn-sm cuisine-manage-btn" @click="openMealsModal">
          管理用餐时段
          <span class="cuisine-count">{{ effectiveMeals.length }} 餐 · {{ mealSummary }}</span>
          <span class="chev">›</span>
        </button>
      </div>
    </div>

    <Teleport to="body">
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
          <p class="setting-desc">勾选的菜系会在菜谱里显示，取消勾选则隐藏（随时可恢复）。</p>
          <div class="cuisine-toggle-list modal-cuisine-list">
            <label v-for="c in allCuisines" :key="c.key" class="cuisine-toggle">
              <span class="cuisine-toggle-name">{{ c.icon }} {{ c.name }}</span>
              <input type="checkbox" :checked="!isHidden(c.key)" @change="toggleCuisine(c.key)" />
            </label>
          </div>
          <div class="modal-foot cuisine-modal-foot">
            <span class="cuisine-foot-count">{{ visibleCount }} / {{ allCuisines.length }} 显示中</span>
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
