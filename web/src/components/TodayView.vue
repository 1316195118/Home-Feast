<script setup>
import { inject, computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { friendlyDate, memberOf, dishImage, cuisineName, artStyle, isCuisineHidden, normalizeMeals, isMealExpired, rosterForDate, seasonOf, smartSuggest } from '../utils'
import { getRecipe } from '../recipes'
import { sparkleFromEvent, celebrate, vibrate } from '../feedback'
import DishDetail from './DishDetail.vue'
import DicePK from './DicePK.vue'
import ShoppingList from './ShoppingList.vue'
import WeekMenu from './WeekMenu.vue'

const { state, actions, pending } = inject('app')
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
const chef = computed(() => memberOf(state.plan.chefId, state.members))
const washer = computed(() => memberOf(state.plan.dishwasherId, state.members))
const samePerson = computed(() =>
  !!(state.plan.chefId && state.plan.dishwasherId === state.plan.chefId)
)
const noteText = ref(state.plan.note || '')
const noteSaved = ref(false)
const noteDirty = computed(() => noteText.value !== (state.plan.note || ''))
const noteBtnClass = computed(() =>
  noteSaved.value ? 'btn-green' : noteDirty.value ? 'btn-primary' : 'btn-soft'
)
let noteTimer = null
onUnmounted(() => clearTimeout(noteTimer))
const suggestions = ref([])
const suggestSalt = ref(0)
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
function shuffle() {
  // 换一批时排除当前这三道，避免点一下还是同一批
  const exclude = suggestions.value.map((s) => s.dish.id)
  suggestSalt.value++
  suggestions.value = computeSuggestions(exclude)
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
function shuffleSnacks() { snackSalt.value += 1 }

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
  const chefOk = !r.chefId || state.plan.chefId === r.chefId
  const washOk = !r.washId || state.plan.dishwasherId === r.washId
  return !(chefOk && washOk)
})
function applyRoster() {
  const r = rosterToday.value
  if (!r) return
  // 排班没填的那一半保留今天已定的人，不误清空
  actions.setDuties(r.chefId || state.plan.chefId || '', r.washId || state.plan.dishwasherId || '')
}

// 有未保存的草稿时，实时同步 / 其它设备改动触发的刷新都不覆盖输入框内容
watch(() => state.plan.note, (v) => { if (!noteDirty.value) noteText.value = v || '' })
watch(
  () => [state.today, state.dishes, state.settings.pantry, state.hiddenCuisines, state.plan.orders],
  refreshSuggestions,
  { immediate: true },
)

// 分工：两个按钮就是给「当前身份」认领角色用的——点「我来掌勺」= 把自己定为掌勺，
// 再点一次取消。别人已认领时点它会接管（改主意），按钮上始终显示当前认领人。
const dutyPulse = ref('')
let dutyPulseTimer = null
function claimRole(role, event) {
  const me = selfMember.value
  if (!me) {
    actions.toast('先在右上角选一下「我是谁」', 'warn')
    return
  }
  const isChef = role === 'chef'
  const already = isChef ? state.plan.chefId === me.id : state.plan.dishwasherId === me.id
  if (!already) {
    sparkleFromEvent(event, isChef ? '👨‍🍳' : '🧽')
    vibrate(12)
  }
  if (isChef) {
    if (already) actions.clearChef()
    else actions.setChef(me.id)
  } else {
    if (already) actions.clearDishwasher()
    else actions.setDishwasher(me.id)
  }
  const verb = isChef ? '掌勺' : '洗碗'
  dutyPulse.value = { id: me.id, text: already ? `已取消${verb}` : `${me.avatar} ${me.name} 来${verb}啦！` }
  clearTimeout(dutyPulseTimer)
  dutyPulseTimer = setTimeout(() => { dutyPulse.value = '' }, 2400)
}
onUnmounted(() => clearTimeout(dutyPulseTimer))

// 骰子 PK：全员掷骰，点大的掌勺、点小的洗碗
function openPK() {
  if (state.members.length < 2) {
    actions.toast('至少需要两位家人才能 PK', 'warn')
    return
  }
  pkOpen.value = true
}
async function applyPK({ chefId, dishwasherId }) {
  const ok = await actions.setDuties(chefId, dishwasherId)
  if (ok) pkOpen.value = false
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

// 首屏速览：有菜时给一条「今天已点 N 道 · 前几道菜名」，点一下滚到今日菜单，
// 解决手机端首屏被分工流程占满、看不到今天到底吃什么的问题。
const orderPreview = computed(() => orders.value.slice(0, 3).map((o) => o.dishName))
function scrollToMenu() {
  document.getElementById('today-menu-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
      <h1 class="hero-title">
        <template v-if="chef || washer">
          <span>{{ chef ? `${chef.avatar} ${chef.name} 掌勺` : '掌勺待定' }}</span>
          <span class="hero-sep">·</span>
          <span>{{ washer ? `${washer.avatar} ${washer.name} 洗碗` : '洗碗待定' }}</span>
        </template>
        <template v-else>今天谁做饭、谁洗碗呀？</template>
      </h1>
      <p class="hero-sub">
        已经点了 {{ orders.length }} 道菜{{ doneCount ? `，上桌 ${doneCount} 道` : '' }}，
        {{ orders.length ? '就等开饭啦' : '先点几个，别让掌勺的人猜' }}
      </p>

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

      <!-- 骰子 PK 独立成一条醒目 CTA：以前挤在「谁洗碗」标题里，很容易被当成标题的一部分 -->
      <div class="pk-cta">
        <span class="pk-cta-icon">🎲</span>
        <div class="pk-cta-text">
          <b>摇骰子，一把定分工</b>
          <span class="tiny muted pk-cta-hint">全员掷骰，点大的掌勺、点小的洗碗</span>
        </div>
        <button class="btn btn-primary pk-cta-btn" @click="openPK">摇骰子</button>
      </div>

      <!-- 两个按钮就是给当前身份认领：点一下「我来」，再点取消；谁认领了一眼看到 -->
      <div class="duty-claim-grid">
        <button
          type="button" class="duty-claim duty-claim-chef"
          :class="{ active: chef && chef.id === state.currentUserId, taken: !!chef }"
          :disabled="pending('duty')" @click="claimRole('chef', $event)"
        >
          <span class="duty-claim-emoji">👨‍🍳</span>
          <span class="duty-claim-text">
            <b>我来掌勺</b>
            <span class="duty-claim-status">
              <template v-if="chef">
                <span class="avatar avatar-sm" :style="{ background: chef.color }">{{ chef.avatar }}</span>
                {{ chef.id === state.currentUserId ? '是你 · 再点取消' : chef.name + ' 已认领' }}
              </template>
              <template v-else>点一下认领</template>
            </span>
          </span>
          <span v-if="chef && chef.id === state.currentUserId" class="duty-claim-check">✓</span>
        </button>
        <button
          type="button" class="duty-claim duty-claim-wash"
          :class="{ active: washer && washer.id === state.currentUserId, taken: !!washer }"
          :disabled="pending('duty')" @click="claimRole('wash', $event)"
        >
          <span class="duty-claim-emoji">🧽</span>
          <span class="duty-claim-text">
            <b>我来洗碗</b>
            <span class="duty-claim-status">
              <template v-if="washer">
                <span class="avatar avatar-sm" :style="{ background: washer.color }">{{ washer.avatar }}</span>
                {{ washer.id === state.currentUserId ? '是你 · 再点取消' : washer.name + ' 已认领' }}
              </template>
              <template v-else>点一下认领</template>
            </span>
          </span>
          <span v-if="washer && washer.id === state.currentUserId" class="duty-claim-check">✓</span>
        </button>
        <Transition name="duty-pop">
          <div v-if="dutyPulse" :key="dutyPulse.text" class="duty-pulse" aria-live="polite">{{ dutyPulse.text }}</div>
        </Transition>
      </div>

      <p class="duty-summary" :class="{ muted: !chef || !washer }">
        <template v-if="samePerson">{{ chef?.name }} 又做饭又洗碗，辛苦啦 🥲</template>
        <template v-else-if="chef && washer">分工已定，就等开饭啦 🍚</template>
        <template v-else-if="!chef && !washer">分工还没定</template>
        <template v-else>{{ !chef ? '还差掌勺的人' : '还差洗碗的人' }}没定</template>
      </p>

      <div class="note-row" :class="{ 'is-dirty': noteDirty, 'is-saved': noteSaved }">
        <div class="note-input-wrap">
          <input
            v-model="noteText" class="input" maxlength="200"
            placeholder="留句话，例如：想吃点清淡的…"
            @blur="saveNote" @keyup.enter="$event.target.blur()"
          />
          <Transition name="page"><span v-if="noteSaved" class="note-check">✓</span></Transition>
        </div>
        <button
          class="btn btn-sm note-save-btn" :class="noteBtnClass"
          :disabled="!noteDirty || pending('note')" @click="saveNote"
        >
          <span v-if="pending('note')" class="btn-spinner" aria-hidden="true" />
          {{ pending('note') ? '保存中' : (noteSaved ? '✓ 已存好' : '保存留言') }}
        </button>
      </div>
    </div>

    <div class="stat-strip">
      <div class="stat-item"><span class="stat-num value-pop" :key="`dishes-${orders.length}`">{{ orders.length }}</span> 道菜</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`people-${peopleCount}`">{{ peopleCount }}</span> 人点菜</div>
      <div class="stat-item"><span class="stat-num value-pop" :key="`minutes-${totalMinutes}`">{{ totalMinutes }}</span> 分钟预计耗时</div>
    </div>

    <button v-if="orders.length" type="button" class="today-peek" @click="scrollToMenu">
      <span class="today-peek-label">🍽️ 今天已点 <span class="value-pop" :key="orders.length">{{ orders.length }}</span> 道</span>
      <span class="today-peek-names">{{ orderPreview.join('、') }}{{ orders.length > 3 ? '…' : '' }}</span>
      <span class="today-peek-go">查看菜单 ›</span>
    </button>

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
                <div v-for="o in grouped[m.key]" :key="o.id" class="order-row" :class="{ done: o.done }">
                  <div class="order-thumb">
                    <img :src="dishImage(o, state.cuisines, state.dishes)" :alt="o.dishName" :style="artStyle(o, state.dishes)" loading="lazy" class="img-fade" />
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
        <TransitionGroup name="swap" tag="div" class="suggest-grid">
          <div
            v-for="s in suggestions" :key="s.dish.id" class="suggest-card"
            :class="{ picked: isPicked(s.dish) }"
            role="button" tabindex="0" :aria-label="`点这道：${s.dish.name}`"
            :aria-disabled="pending(`order:${s.dish.id}`)"
            @click="quickOrder(s.dish, $event)" @keydown.enter="quickOrder(s.dish, $event)" @keydown.space.prevent="quickOrder(s.dish, $event)"
          >
            <div class="suggest-photo">
              <img :src="dishImage(s.dish, state.cuisines)" :alt="s.dish.name" :style="artStyle(s.dish, state.dishes)" loading="lazy" class="img-fade" />
              <button class="suggest-recipe" title="看做法" aria-label="看做法" @click.stop="detailId = s.dish.id"><span class="badge-ico">📖</span></button>
              <span v-if="isPicked(s.dish)" class="suggest-picked">✓ 已点</span>
              <span v-else-if="pending(`order:${s.dish.id}`)" class="suggest-adding">加菜中…</span>
              <span v-else class="suggest-add" aria-hidden="true"><span class="badge-ico">＋</span></span>
            </div>
            <span class="suggest-name">{{ s.dish.name }}</span>
            <span class="suggest-reasons">{{ s.reasons.join(' · ') }}</span>
          </div>
        </TransitionGroup>

        <div class="snack-strip" v-if="snackPicks.length">
          <div class="snack-strip-head">
            <span class="snack-strip-title">🍢 来点小吃解馋</span>
            <button class="btn btn-ghost btn-sm" type="button" @click="shuffleSnacks">换一批</button>
          </div>
          <div class="snack-picks">
            <button
              v-for="s in snackPicks" :key="s.dish.id" type="button"
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
