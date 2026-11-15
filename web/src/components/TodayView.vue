<script setup>
import { inject, computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { friendlyDate, memberOf, dishImage, cuisineName, artStyle, isCuisineHidden, normalizeMeals, isMealExpired, rosterForDate, seasonOf, smartSuggest } from '../utils'
import { getRecipe } from '../recipes'
import { sparkleFromEvent, celebrate, vibrate } from '../feedback'
import DishDetail from './DishDetail.vue'
import DicePK from './DicePK.vue'
import ShoppingList from './ShoppingList.vue'
import WeekMenu from './WeekMenu.vue'

const { state, actions, pending, confirmDialog, choiceDialog } = inject('app')
const detailId = ref(null)
const detailDish = computed(() => state.dishes.find((d) => d.id === detailId.value) || null)
const pkOpen = ref(false)
const shoppingOpen = ref(false)

const orders = computed(() => state.plan.orders || [])
// 餐别完全来自后端下发：名字、图标、结束时刻都能在设置里改，这里不写死。
const meals = computed(() => normalizeMeals(state.meals))
// 默认自动跟随当前时段；用户手动选过之后全局生效（菜谱页/详情弹窗点菜也归这一餐）。
const activeMeal = computed(() => state.mealPick || state.meal)
function selectMeal(key) {
  // 再点一次已选中的餐别 = 取消手动指定，回到跟随时间
  actions.setMealPick(state.mealPick === key ? '' : key)
}
// 餐别切换下方的滑动指示器：跟随当前选中项平滑移动到底边
const chipsEl = ref(null)
const indicator = ref(null)
function updateIndicator() {
  const box = chipsEl.value
  const ind = indicator.value
  if (!box || !ind) return
  const active = box.querySelector('.meal-chip.active')
  if (!active) { ind.style.opacity = '0'; return }
  ind.style.opacity = '1'
  ind.style.width = `${active.offsetWidth}px`
  ind.style.transform = `translateX(${active.offsetLeft}px)`
}
watch([activeMeal, meals], updateIndicator, { flush: 'post' })
onMounted(() => { updateIndicator(); window.addEventListener('resize', updateIndicator) })
onUnmounted(() => window.removeEventListener('resize', updateIndicator))
// 当前餐别排最前，其余按配置里的结束时刻顺序，「其他」永远垫底；
// 已过饭点且没菜的组在模板里折叠成紧凑一行，避免把当前餐挤到屏幕外。
const mealGroups = computed(() => {
  const all = [...meals.value, { key: 'other', label: '其他', icon: '🍽️' }]
  const idx = new Map(meals.value.map((m, i) => [m.key, i]))
  const rank = (m) => m.key === 'other' ? 1000 : m.key === state.meal ? -1 : (idx.get(m.key) ?? 0) + 1
  return all.slice().sort((a, b) => rank(a) - rank(b))
})
const grouped = computed(() => {
  const g = { other: [] }
  for (const m of meals.value) g[m.key] = []
  for (const o of orders.value) {
    if (o.meal && g[o.meal]) g[o.meal].push(o)
    else g.other.push(o)
  }
  return g
})
// 分工支持多人：数组里可能有 0..N 个成员，这里统一转成成员对象列表
const dutyMembers = (ids) => (ids || []).map((id) => memberOf(id, state.members)).filter(Boolean)
// 分工按餐次独立：只读「现在这顿」，过了饭点 state.meal 自动变化，这里跟着换
const dutyNow = computed(() => state.plan.mealDuties?.[state.meal] || {})
const nowMeal = computed(() => (state.meals || []).find((m) => m.key === state.meal) || null)
const chefs = computed(() => dutyMembers(dutyNow.value.chefIds))
const washers = computed(() => dutyMembers(dutyNow.value.dishwasherIds))
const noteText = ref(state.plan.note || '')
const noteSaved = ref(false)
const noteDirty = computed(() => noteText.value !== (state.plan.note || ''))
let noteTimer = null
onUnmounted(() => clearTimeout(noteTimer))
const suggestions = ref([])
const suggestSalt = ref(0)
const suggesting = ref(false) // 换一批时播放一次整块淡入，不重建 DOM
const season = computed(() => seasonOf(state.today))
const selfMember = computed(() => state.members.find((m) => m.id === state.currentUserId) || null)
const pantryCount = computed(() => (state.settings.pantry || []).length)
// 近 7 天(不含今天)点过的菜，推荐时降权，避免「来来回回就那几样」
const recentIds = computed(() => {
  const set = new Set()
  const todayStr = state.today
  if (!todayStr) return set
  const todayT = new Date(todayStr + 'T12:00:00')
  for (const p of (state.plans || [])) {
    if (!p?.date || p.date >= todayStr) continue
    const days = (todayT - new Date(p.date + 'T12:00:00')) / 86400000
    if (days >= 1 && days <= 7) {
      for (const o of (p.orders || [])) if (o.dishId) set.add(o.dishId)
    }
  }
  return set
})

// 智能推荐：季节 + 冰箱食材 + 口味避雷 + 耗时 + 收藏，全是本地规则，不调用外部服务。
// 内置菜的配料由 recipes.js 动态生成，必须经 getRecipe() 展开才能匹配冰箱食材。
function computeSuggestions(excludeIds = [], limit = 3) {
  return smartSuggest(state.dishes, {
    today: state.today,
    pantry: state.settings.pantry || [],
    member: selfMember.value,
    hiddenCuisines: state.hiddenCuisines,
    orders: state.plan.orders || [],
    excludeIds,
    recentIds: recentIds.value,
    salt: suggestSalt.value,
    limit,
    ingredientsOf: (d) => getRecipe(d).ingredients || [],
  })
}
function refreshSuggestions() { suggestions.value = computeSuggestions([]) }
function playBatchAnim(flag) {
  flag.value = true
  clearTimeout(playBatchAnim.timer)
  playBatchAnim.timer = setTimeout(() => { flag.value = false }, 460)
}
function shuffle() {
  // 换一批时排除当前这三道，避免点一下还是同一批
  const exclude = suggestions.value.map((s) => s.dish.id)
  suggestSalt.value++
  suggestions.value = computeSuggestions(exclude)
  playBatchAnim(suggesting)
}

// 首页小吃专区：从「小吃/点心」标签的菜里挑 3 道，换一批可刷新
const SNACK_TAGS = ['小吃', '点心']
const snackSalt = ref(0)
const snackPicks = computed(() => {
  const pool = state.dishes.filter((d) =>
    (d.tags || []).some((t) => SNACK_TAGS.includes(String(t)))
    && !isCuisineHidden(d.cuisine, state.hiddenCuisines))
  if (!pool.length) return []
  return smartSuggest(pool, {
    today: state.today,
    pantry: state.settings.pantry || [],
    member: selfMember.value,
    hiddenCuisines: state.hiddenCuisines,
    orders: state.plan.orders || [],
    salt: snackSalt.value,
    limit: 3,
    ingredientsOf: (d) => getRecipe(d).ingredients || [],
  })
})
const snacking = ref(false)
function shuffleSnacks() {
  snackSalt.value += 1
  playBatchAnim(snacking)
}

// 「本周菜单」：按人数配「荤 + 素 + 汤（+ 主食）」，整周不重样，可导出图片
const weekOpen = ref(false)

// 每周排班提醒：只在「排班已开启 + 今天有安排 + 当前分工不一致」时出现，可一键套用。
const rosterToday = computed(() => rosterForDate(state.settings.roster, state.today))
const rosterNames = computed(() => {
  const r = rosterToday.value
  if (!r) return null
  return {
    chef: state.members.find((m) => m.id === r.chefId) || null,
    wash: state.members.find((m) => m.id === r.washId) || null,
  }
})
const rosterDiff = computed(() => {
  const r = rosterToday.value
  if (!r) return false
  // 分工是数组：排班上的人只要在名单里就算一致，不再只比第一个人
  const chefOk = !r.chefId || (dutyNow.value.chefIds || []).includes(r.chefId)
  const washOk = !r.washId || (dutyNow.value.dishwasherIds || []).includes(r.washId)
  return !(chefOk && washOk)
})
async function applyRoster() {
  const r = rosterToday.value
  if (!r) return
  // 排班没填的那一半保留今天已有的名单；填了的那一半以排班为准
  const chefIds = r.chefId ? [r.chefId] : (dutyNow.value.chefIds || [])
  const washIds = r.washId ? [r.washId] : (dutyNow.value.dishwasherIds || [])
  const dropped = [
    ...(dutyNow.value.chefIds || []).filter((id) => !chefIds.includes(id)),
    ...(dutyNow.value.dishwasherIds || []).filter((id) => !washIds.includes(id)),
  ]
  if (dropped.length) {
    const ok = await confirmDialog({
      title: '套用排班会替换名单',
      message: `今天已有 ${dropped.length} 位家人认领，套用排班后他们会从名单里移除，只留排班上的安排。`,
      confirmText: '仍然套用',
    })
    if (!ok) return
  }
  const done = await actions.setDuties(chefIds, washIds, state.meal)
  if (done) actions.toast('已按排班安排今天的分工', 'success')
}

// 有未保存的草稿时，实时同步 / 其它设备改动触发的刷新都不覆盖输入框内容
watch(() => state.plan.note, (v) => { if (!noteDirty.value) noteText.value = v || '' })
watch(
  () => [state.today, state.dishes, state.settings.pantry, state.hiddenCuisines, state.plan.orders],
  refreshSuggestions,
  { immediate: true },
)

// 分工是「认领」，且支持多人：每个人把自己的 id 加进数组，各设备各加各的。
// 掌勺和洗碗是两个独立集合，同一个人可以两边都在。
const meId = computed(() => state.currentUserId)
const justClaimed = ref('')
// 必须和名单同源：都读当前餐。读顶层旧字段会出现「没人认领却给退出按钮」的矛盾。
const iAmChef = computed(() => (dutyNow.value.chefIds || []).includes(meId.value))
const iAmWasher = computed(() => (dutyNow.value.dishwasherIds || []).includes(meId.value))

const namesOf = (list) => (list.length <= 2
  ? list.map((m) => m.name).join('、')
  : `${list[0].name} 等 ${list.length} 人`)

// 卡片副标题：空位 / 只有我 / 我在其中 / 别人在负责，四种状态文案分开
function dutyHint(list, mine, label) {
  if (!list.length) return '点「我来」加入'
  if (mine) return list.length === 1 ? '目前只有你一人' : `你在其中 · 共 ${list.length} 人`
  return `${list.length} 人在${label}`
}

function playClaimAnim(role) {
  justClaimed.value = role
  clearTimeout(playClaimAnim.timer)
  playClaimAnim.timer = setTimeout(() => { justClaimed.value = '' }, 700)
}

async function toggleDuty(role) {
  const uid = meId.value
  if (!uid) return
  const isChef = role === 'chef'
  const label = isChef ? '掌勺' : '洗碗'
  const otherLabel = isChef ? '洗碗' : '掌勺'
  const key = isChef ? 'chefIds' : 'dishwasherIds'
  const otherKey = isChef ? 'dishwasherIds' : 'chefIds'
  const list = [...(dutyNow.value[key] || [])]
  const at = list.indexOf(uid)

  // 已在名单里 → 退出
  if (at >= 0) {
    list.splice(at, 1)
    const done = await actions.setDutyList(role, list, state.meal)
    if (done) {
      const rest = (dutyNow.value[key] || []).filter((id) => id !== uid)
      actions.toast(rest.length ? `你退出了${label}，还有 ${rest.length} 人在${label}` : `你退出了${label}`, 'success')
    }
    return
  }

  // 名单里已经有别人：问一句「一起」还是「只我来」。
  // 多人模式原本没有「换我」入口，导致没办法把别人换下来，这里补回。
  if (list.length > 0) {
    const others = list.map((id) => memberOf(id, state.members)?.name).filter(Boolean)
    const choice = await choiceDialog({
      title: `一起${label}，还是换你？`,
      message: `${others.join('、')} 已经在${label}了。可以一起干，也可以把 TA 换下来只留你。`,
      confirmText: `一起${label}`,
      extraText: '只我来',
      cancelText: '算了',
    })
    if (choice === 'cancel') return
    if (choice === 'extra') {
      const replaced = await actions.setDutyList(role, [uid], state.meal)
      if (replaced) {
        playClaimAnim(role)
        actions.toast(`已经换成你${label}，${others.join('、')} 被换下了`, 'success')
      }
      return
    }
  }

  // 加入（空位直接加，非空位是用户选了「一起」）
  list.push(uid)
  const done = await actions.setDutyList(role, list, state.meal)
  if (!done) return
  playClaimAnim(role)

  // 提示要说清「和谁一起」，不能只说「今天你掌勺」——现在是多人分工
  const others = list.filter((id) => id !== uid).map((id) => memberOf(id, state.members)?.name).filter(Boolean)
  const inOther = (dutyNow.value[otherKey] || []).includes(uid)
  if (inOther) {
    const partner = others.length ? `，还有 ${others.join('、')} 一起${label}` : ''
    actions.toast(`掌勺 + 洗碗都是你，全包了 👨‍🍳🧽${partner}`, 'success')
    return
  }
  if (others.length) {
    actions.toast(`你加入了${label}，和 ${others.join('、')} 一起`, 'success')
    return
  }
  actions.toast(`你认领了今天的${label}${isChef ? ' 👨‍🍳' : ' 🧽'}，别人也可以加入`, 'success')
}

// 骰子 PK：全员掷骰，点大的掌勺、点小的洗碗
function openPK() {
  if (state.members.length < 2) {
    actions.toast('至少需要两位家人才能 PK', 'warn')
    return
  }
  pkOpen.value = true
}
async function applyPK({ chefId, dishwasherId }) {
  const extra = [
    ...(dutyNow.value.chefIds || []).filter((id) => id !== chefId),
    ...(dutyNow.value.dishwasherIds || []).filter((id) => id !== dishwasherId),
  ]
  if (extra.length) {
    const ok = await confirmDialog({
      title: '用骰子结果替换分工？',
      message: `今天还有 ${extra.length} 位家人在名单里，骰子结果只会留 ${memberOf(chefId, state.members)?.name} 掌勺、${memberOf(dishwasherId, state.members)?.name} 洗碗。`,
      confirmText: '用骰子结果',
    })
    if (!ok) return
  }
  const done = await actions.setDuties(chefId, dishwasherId, state.meal)
  if (done) {
    pkOpen.value = false
    actions.toast(`这顿的分工定好了：${memberOf(chefId, state.members)?.name} 掌勺、${memberOf(dishwasherId, state.members)?.name} 洗碗`, 'success')
  }
}

// 长按菜单行 → 快捷操作（标记上桌 / 撤掉），手机上不用瞄准两个小图标
let pressTimer = null
function cancelPress() {
  if (pressTimer) { clearTimeout(pressTimer); pressTimer = null }
}
function startPress(order, e) {
  if (e?.pointerType === 'mouse' && e.button !== 0) return
  cancelPress()
  pressTimer = setTimeout(async () => {
    pressTimer = null
    vibrate(12)
    const choice = await choiceDialog({
      title: order.dishName,
      message: order.done ? '这道已经上桌了。' : '这道还没上桌。',
      confirmText: order.done ? '改回没做' : '标记已上桌',
      extraText: '撤掉这道',
      cancelText: '取消',
    })
    if (choice === 'ok') actions.toggleOrder(order.id)
    else if (choice === 'extra') actions.removeOrder(order.id)
  }, 480)
}

async function saveNote() {
  if (!noteDirty.value) return
  const ok = await actions.setNote(noteText.value)
  if (ok) {
    actions.toast('今天的话记下啦 ✓', 'success')
    noteSaved.value = true
    clearTimeout(noteTimer)
    noteTimer = setTimeout(() => { noteSaved.value = false }, 2200)
  }
}

function isPicked(dish) {
  return orders.value.some((o) => o.dishId === dish.id)
}

// 点完菜保留当前这三张卡片，只标记「已点」，避免图片突然换掉让人以为点错了
async function quickOrder(dish, event) {
  if (isPicked(dish)) {
    actions.toast('这道已经在今日菜单啦', 'warn')
    return
  }
  const rect = event?.currentTarget?.getBoundingClientRect?.()
  const at = rect ? { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 } : null
  sparkleFromEvent(event, '🍚')
  const ok = await actions.order(dish.id, '', activeMeal.value)
  if (ok && at) {
    celebrate(at.x, at.y)
    vibrate(16)
  }
}


const doneCount = computed(() => orders.value.filter((o) => o.done).length)
const peopleCount = computed(() => new Set(orders.value.map((o) => o.userId)).size)
const totalMinutes = computed(() => orders.value.reduce((sum, o) => {
  const dish = state.dishes.find((d) => d.id === o.dishId)
  return sum + (dish?.minutes || 20)
}, 0))
</script>

<template>
  <section>
    <div class="today-hero">
      <span class="hero-deco">🍲</span>
      <div class="hero-label">今日份 · {{ friendlyDate(state.today) }}</div>
      <h1 class="hero-title"><span>今天已点 {{ orders.length }} 道</span></h1>
      <p class="hero-sub">
        <template v-if="chefs.length || washers.length">
          {{ chefs.length ? `${chefs[0].avatar} ${namesOf(chefs)} 掌勺` : '掌勺待定' }}<span class="hero-sep">·</span>{{ washers.length ? `${washers[0].avatar} ${namesOf(washers)} 洗碗` : '洗碗待定' }}<span class="hero-sep">·</span>
        </template>
        {{ doneCount ? `已上桌 ${doneCount} 道，` : '' }}{{ orders.length ? '就等开饭啦' : '先点几个，别让掌勺的人猜' }}
      </p>

      <div class="duty-head">
        <span class="duty-meal-tag">{{ nowMeal?.icon }} {{ nowMeal?.label }}</span>
        <span class="duty-meal-note">这顿的分工，过了饭点自动切下一顿</span>
      </div>

      <div class="duty-bar">
        <div class="duty-card" :class="{ 'is-taken': chefs.length, 'is-mine': iAmChef, 'just-claimed': justClaimed === 'chef' }">
          <span v-if="chefs.length" class="duty-stack" aria-hidden="true">
            <span v-for="m in chefs.slice(0, 3)" :key="m.id" class="duty-owner-avatar" :style="{ background: m.color }">{{ m.avatar }}</span>
          </span>
          <span v-else class="duty-claim-emoji" aria-hidden="true">👨‍🍳</span>
          <span class="duty-owner-info">
            <b>{{ chefs.length ? namesOf(chefs) : '还没人认领' }}</b>
            <small>{{ dutyHint(chefs, iAmChef, '掌勺') }}</small>
          </span>
          <button v-if="!iAmChef" class="duty-mini duty-mini-ghost" type="button" :disabled="pending('duty')" @click="toggleDuty('chef')">+ 我来</button>
          <button v-else class="duty-mini" type="button" :disabled="pending('duty')" @click="toggleDuty('chef')">退出</button>
        </div>

        <div class="duty-card" :class="{ 'is-taken': washers.length, 'is-mine': iAmWasher, 'just-claimed': justClaimed === 'wash' }">
          <span v-if="washers.length" class="duty-stack" aria-hidden="true">
            <span v-for="m in washers.slice(0, 3)" :key="m.id" class="duty-owner-avatar" :style="{ background: m.color }">{{ m.avatar }}</span>
          </span>
          <span v-else class="duty-claim-emoji" aria-hidden="true">🧽</span>
          <span class="duty-owner-info">
            <b>{{ washers.length ? namesOf(washers) : '还没人认领' }}</b>
            <small>{{ dutyHint(washers, iAmWasher, '洗碗') }}</small>
          </span>
          <button v-if="!iAmWasher" class="duty-mini duty-mini-ghost" type="button" :disabled="pending('duty')" @click="toggleDuty('wash')">+ 我来</button>
          <button v-else class="duty-mini" type="button" :disabled="pending('duty')" @click="toggleDuty('wash')">退出</button>
        </div>

        <button class="btn btn-soft btn-sm duty-roll" type="button" title="全员掷骰，点大的掌勺、点小的洗碗" @click="openPK">🎲 摇骰子</button>
      </div>
      <p class="duty-hint">只算「现在这顿」的分工；可以多人一起，一个人也能两头都包。</p>

      <div v-if="rosterToday && rosterNames" class="roster-hint" :class="{ 'is-done': !rosterDiff }">
        <span class="roster-hint-icon">{{ rosterDiff ? '📅' : '✅' }}</span>
        <span class="roster-hint-text">
          <template v-if="rosterDiff">
            按每周排班，今天该
            <b v-if="rosterNames.chef">{{ rosterNames.chef.avatar }} {{ rosterNames.chef.name }} 掌勺</b><b v-if="rosterNames.chef && rosterNames.wash">、</b><b v-if="rosterNames.wash">{{ rosterNames.wash.avatar }} {{ rosterNames.wash.name }} 洗碗</b>
          </template>
          <template v-else>今天的分工和排班一致</template>
        </span>
        <button v-if="rosterDiff" class="btn btn-primary btn-sm" :disabled="pending('duty')" @click="applyRoster">一键套用</button>
      </div>

      <div class="note-line" :class="{ 'is-dirty': noteDirty, 'is-saved': noteSaved }">
        <span class="note-line-icon" aria-hidden="true">💬</span>
        <input
          v-model="noteText" class="note-line-input" maxlength="200"
          placeholder="给今天留句话：想吃点清淡的、晚点开饭…"
          aria-label="今日留言"
          @blur="saveNote" @keyup.enter="$event.target.blur()"
        />
        <span v-if="pending('note')" class="note-line-hint">保存中…</span>
        <span v-else-if="noteSaved" class="note-line-saved">✓ 已存</span>
        <span v-else-if="noteDirty" class="note-line-hint">失焦自动保存</span>
      </div>
    </div>

    <div class="stat-strip">
      <div class="stat-item"><span class="stat-num value-pop" :key="`dishes-${orders.length}`">{{ orders.length }}</span> 道菜</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`people-${peopleCount}`">{{ peopleCount }}</span> 人点菜</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`minutes-${totalMinutes}`">{{ totalMinutes }}</span> 分钟预计耗时</div>
    </div>

    <div class="today-cols">
      <div id="today-menu-panel" class="panel">
        <div class="panel-head">
          <div>
            <div class="panel-title">🍽️ 今日菜单</div>
            <div class="section-sub">谁点的、做没做好，一眼看清</div>
          </div>
          <div class="panel-head-actions">
            <button class="btn btn-soft btn-sm" @click="shoppingOpen = true">🛒 购物清单</button>
            <button class="btn btn-soft btn-sm" @click="state.tab = 'menu'">去点菜 ＋</button>
          </div>
        </div>

        <div v-if="orders.length" class="meal-groups">
          <template v-for="m in mealGroups" :key="m.key">
            <!-- 只显示真正点了菜的那一餐：已过饭点或没点的分组不占版面 -->
            <div v-if="grouped[m.key].length" class="meal-group" :class="{ 'is-current': m.key === state.meal }">
              <div class="meal-group-head">
                <span class="meal-group-title">{{ m.icon }} {{ m.label }}</span>
                <span v-if="m.key === state.meal" class="meal-group-now">现在这顿</span>
                <span class="meal-group-count">{{ grouped[m.key].length }} 道</span>
              </div>
              <TransitionGroup name="list" tag="div" class="order-list">
                <div
                  v-for="o in grouped[m.key]" :key="o.id" class="order-row" :class="{ done: o.done }"
                  @pointerdown="startPress(o, $event)" @pointerup="cancelPress"
                  @pointerleave="cancelPress" @pointercancel="cancelPress"
                >
                  <div class="order-thumb">
                    <img :src="dishImage(o, state.cuisines, state.dishes, 'thumb')" :alt="o.dishName" :style="artStyle(o, state.dishes)" loading="lazy" decoding="async" class="img-fade" />
                  </div>
                  <div class="order-main">
                    <div class="order-name">{{ o.dishName }}</div>
                    <div class="order-meta">
                      <span>{{ o.userAvatar }} {{ o.userName }}</span>
                      <span>{{ cuisineName(o.dishCuisine, state.cuisines) }}</span>
                      <span v-if="o.note">「{{ o.note }}」</span>
                    </div>
                  </div>
                  <div class="order-actions">
                    <button
                      class="icon-btn" :aria-label="o.done ? '标记为还没做' : '标记已上桌'"
                      :title="o.done ? '标记为还没做' : '标记已上桌'"
                      :disabled="pending(`toggle:${o.id}`)" @click="actions.toggleOrder(o.id)"
                    >
                      {{ o.done ? '✅' : '⭕' }}
                    </button>
                    <button
                      class="icon-btn icon-btn-danger" aria-label="撤掉这道" title="撤掉这道"
                      :disabled="pending(`remove:${o.id}`)" @click="actions.removeOrder(o.id)"
                    >✕</button>
                  </div>
                </div>
              </TransitionGroup>
            </div>
          </template>
        </div>
        <div v-else class="empty">
          <span class="empty-icon">🥢</span>
          今天还没人点菜，去菜谱挑几道吧
        </div>
      </div>

      <div class="panel">
        <div class="panel-head">
          <div>
            <div class="panel-title">💡 今天吃啥</div>
            <div class="section-sub">{{ season.label }}天 · 按季节{{ pantryCount ? '、冰箱里的 ' + pantryCount + ' 样食材' : '' }}和口味避雷挑的</div>
          </div>
          <button class="btn btn-sm" @click="shuffle" aria-label="换一批推荐">🔄 换一批</button>
        </div>

        <div class="meal-switch" role="group" aria-label="点的是哪一餐">
          <span class="meal-switch-label">点的是</span>
          <div class="meal-chips" ref="chipsEl">
            <button
              v-for="m in meals" :key="m.key" type="button"
              class="meal-chip"
              :class="{ active: activeMeal === m.key, 'is-past': isMealExpired(m.key, state.now, state.meals) }"
              :aria-pressed="activeMeal === m.key"
              :title="`${m.label} · ${m.end} 前；${isMealExpired(m.key, state.now, state.meals) ? '已过饭点，选它属于补记' : '接下来点的菜归到这一餐'}${state.mealPick === m.key ? '（再点一次跟随时间）' : ''}`"
              @click="selectMeal(m.key)"
            >{{ m.icon }} {{ m.label }}</button>
            <span class="meal-indicator" ref="indicator" aria-hidden="true"></span>
          </div>
        </div>

        <div class="suggest-grid" :class="{ 'is-swapping': suggesting }">
          <div
            v-for="(s, si) in suggestions" :key="si" class="suggest-card"
            :class="{ picked: isPicked(s.dish) }"
            role="button" tabindex="0" :aria-label="`点这道：${s.dish.name}`"
            :aria-disabled="pending(`order:${s.dish.id}`)"
            @click="quickOrder(s.dish, $event)" @keydown.enter="quickOrder(s.dish, $event)" @keydown.space.prevent="quickOrder(s.dish, $event)"
          >
            <div class="suggest-photo">
              <img :src="dishImage(s.dish, state.cuisines, null, 'thumb')" :alt="s.dish.name" :style="artStyle(s.dish, state.dishes)" loading="lazy" decoding="async" class="img-fade" />
              <button class="suggest-recipe" title="看做法" aria-label="看做法" @click.stop="detailId = s.dish.id"><span class="badge-ico">📖</span></button>
              <span v-if="isPicked(s.dish)" class="suggest-picked">✓ 已点</span>
              <span v-else-if="pending(`order:${s.dish.id}`)" class="suggest-adding">加菜中…</span>
              <span v-else class="suggest-add" aria-hidden="true"><span class="badge-ico">＋</span></span>
            </div>
            <span class="suggest-name">{{ s.dish.name }}</span>
            <span class="suggest-reasons">{{ s.reasons.join(' · ') }}</span>
          </div>
        </div>

        <div class="snack-strip" v-if="snackPicks.length">
          <div class="snack-strip-head">
            <span class="snack-strip-title">🍢 来点小吃解馋</span>
            <button class="btn btn-ghost btn-sm" type="button" @click="shuffleSnacks">换一批</button>
          </div>
          <div class="snack-picks" :class="{ 'is-swapping': snacking }">
            <button
              v-for="(s, ni) in snackPicks" :key="ni" type="button"
              class="snack-pick" :class="{ picked: isPicked(s.dish) }"
              :aria-label="`点这道：${s.dish.name}`"
              @click="quickOrder(s.dish, $event)"
            >
              <span class="snack-pick-name">{{ s.dish.name }}</span>
              <span class="snack-pick-plus" aria-hidden="true">{{ isPicked(s.dish) ? '✓' : '＋' }}</span>
            </button>
          </div>
        </div>

        <div class="meal-plan">
          <div class="meal-plan-bar">
            <span class="meal-plan-title">🗓️ 本周菜单</span>
            <span class="tiny muted">按人数配荤素汤，整周不重样，可导出图片</span>
            <span class="grow"></span>
            <button class="btn btn-soft btn-sm" type="button" @click="weekOpen = true">生成一周菜单</button>
          </div>
        </div>

        <hr class="divider" />
      </div>
    </div>

    <Transition name="modal">
      <DishDetail v-if="detailDish" :dish="detailDish" :editable="false" @close="detailId = null" />
    </Transition>
    <Transition name="modal">
      <DicePK v-if="pkOpen" :members="state.members" @close="pkOpen = false" @apply="applyPK" />
    </Transition>
    <Transition name="modal">
      <ShoppingList v-if="shoppingOpen" @close="shoppingOpen = false" />
    </Transition>
    <Transition name="modal">
      <WeekMenu v-if="weekOpen" @close="weekOpen = false" />
    </Transition>
  </section>
</template>
