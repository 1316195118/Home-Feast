<script setup>
import { inject, ref, computed, watch } from 'vue'
import DishCard from './DishCard.vue'
import DishEditor from './DishEditor.vue'
import DishDetail from './DishDetail.vue'
import { isCuisineHidden, pantryGroups, pantryVisual, pantryMatches } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const { state, actions, pending } = inject('app')
const activeCuisine = ref('all')
const keyword = ref('')
const onlyFavorite = ref(false)
const editorOpen = ref(false)
const editorDish = ref(null)
// 详情按 id 引用，编辑保存后能自动拿到最新数据
const detailId = ref(null)
const detailDish = computed(() => state.dishes.find((d) => d.id === detailId.value) || null)

// 只展示没有被隐藏的菜系
const visibleCuisines = computed(() =>
  (state.cuisines || []).filter((c) => !isCuisineHidden(c.key, state.hiddenCuisines))
)

const counts = computed(() => {
  const map = {}
  for (const d of state.dishes) {
    if (!isCuisineHidden(d.cuisine, state.hiddenCuisines)) {
      map[d.cuisine] = (map[d.cuisine] || 0) + 1
    }
  }
  return map
})

const visibleDishCount = computed(() =>
  state.dishes.filter((d) => !isCuisineHidden(d.cuisine, state.hiddenCuisines)).length
)

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
  !!keyword.value.trim() || activeCuisine.value !== 'all' || onlyFavorite.value || priorityPantry.value,
)
const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  const base = state.dishes.filter((d) => {
    if (isCuisineHidden(d.cuisine, state.hiddenCuisines)) return false
    if (activeCuisine.value !== 'all' && d.cuisine !== activeCuisine.value) return false
    if (onlyFavorite.value && !d.favorite) return false
    if (!q) return true
    return [d.name, d.desc, ...(d.tags || [])].join(' ').toLowerCase().includes(q)
  })
  if (!priorityPantry.value) return base
  const pantry = state.settings.pantry || []
  return base
    .map((d, i) => ({ d, i, hits: pantryMatches(d, pantry, recipeIngredients).count }))
    .sort((a, b) => b.hits - a.hits || a.i - b.i)
    .map(({ d, hits }) => ({ ...d, _pantryHits: hits }))
})

// 当前选中的分类若是被隐藏了，自动退回「全部」
watch(
  () => [state.hiddenCuisines, activeCuisine.value],
  () => {
    if (activeCuisine.value !== 'all' && isCuisineHidden(activeCuisine.value, state.hiddenCuisines)) {
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
const pantryDraft = ref(new Set(state.settings.pantry || []))
const pantryCustom = ref('')
const pantryQuery = ref('')
useModalBehavior(() => { pantryOpen.value = false }, pantryRoot, pantryOpen)

// 70 种食材分 7 类，加搜索与分类快捷跳转，避免长滚动里找不到想要的那一项
const shownPantryGroups = computed(() => {
  const q = pantryQuery.value.trim().toLowerCase()
  if (!q) return pantryGroups
  return pantryGroups
    .map((g) => ({ ...g, items: g.items.filter((n) => n.toLowerCase().includes(q)) }))
    .filter((g) => g.items.length)
})
function jumpToGroup(key) {
  pantryQuery.value = ''
  const el = document.getElementById('pantry-group-' + key)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
function clearPantry() { pantryDraft.value = new Set() }

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
  pantryCustom.value = ''
  pantryQuery.value = ''
  pantryOpen.value = true
}
function togglePantry(name) {
  const next = new Set(pantryDraft.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  pantryDraft.value = next
}
// 实时反馈：当前勾选的食材能匹配到多少道菜，让用户知道选了有用
const pantryHitCount = computed(() => {
  if (!pantryDraft.value.size) return 0
  return state.dishes.filter((d) =>
    pantryMatches(d, [...pantryDraft.value], recipeIngredients).count > 0,
  ).length
})
function addCustomPantry() {
  const words = pantryCustom.value.split(/[、,，\s]+/).map((s) => s.trim()).filter(Boolean)
  const next = new Set(pantryDraft.value)
  for (const w of words) next.add(w.slice(0, 12))
  pantryDraft.value = next
  pantryCustom.value = ''
}
async function savePantry() {
  const ok = await actions.saveSettings({ pantry: [...pantryDraft.value] })
  if (ok) pantryOpen.value = false
}
</script>

<template>
  <section>
    <div class="section-head menu-head">
      <div>
        <h2 class="section-title">📖 家里的菜谱</h2>
        <div class="section-sub">{{ state.dishes.length }} 道菜 · 八大菜系和家常菜，都能自己改</div>
      </div>
      <div class="menu-head-actions">
        <button class="btn btn-soft btn-sm" @click="openPantry">
          🧊 冰箱食材 <span class="tiny">{{ (state.settings.pantry || []).length }}</span>
        </button>
        <button class="btn btn-primary" @click="openAdd">＋ 添加菜品</button>
      </div>
    </div>

    <div class="cuisine-strip">
      <button class="chip cuisine-chip" :class="{ active: activeCuisine === 'all' }" @click="activeCuisine = 'all'">
        🍽️ 全部 <span class="tiny">{{ visibleDishCount }}</span>
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
      <div class="search-box">
        <input v-model="keyword" class="input" placeholder="搜菜名、口味、标签…" aria-label="搜索菜品" />
      </div>
      <button class="chip" :class="{ active: onlyFavorite }" :aria-pressed="onlyFavorite" @click="onlyFavorite = !onlyFavorite">
        ⭐ 只看收藏
      </button>
      <button
        class="chip" :class="{ active: priorityPantry }" :aria-pressed="priorityPantry"
        title="按冰箱已选的食材把能做的菜排到前面；盐、油、葱蒜等佐料不参与"
        @click="togglePriority"
      >🥬 优先推荐</button>
      <span v-if="priorityPantry && !(state.settings.pantry || []).length" class="tiny muted">先点「🧊 冰箱食材」选几样</span>
      <span v-if="hasFilter" class="tiny muted">找到 {{ filtered.length }} 道</span>
    </div>

    <!-- 不用 TransitionGroup：切换菜系时整片卡片进出场 + move 会让画面明显晃一下。
         增删菜品是低频操作，稳定的静态网格比 0.3s 动画更重要。 -->
    <div v-if="filtered.length" class="dish-grid">
      <DishCard v-for="d in filtered" :key="d.id" :dish="d" @edit="openEdit" @open="openDetail" />
    </div>
    <Transition name="pop" mode="out-in">
      <div v-if="!filtered.length" key="empty" class="empty">
        <span class="empty-icon">🥕</span>
        没找到合适的菜，换个词试试，或者自己加一道
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

    <Transition name="modal">
      <div v-if="pantryOpen" class="modal-backdrop" @click.self="pantryOpen = false">
        <div ref="pantryRoot" class="modal modal-wide pantry-modal" role="dialog" aria-modal="true" aria-label="冰箱食材">
          <div class="modal-head">
            <h2>🧊 冰箱里有什么</h2>
            <button class="icon-btn" aria-label="关闭" @click="pantryOpen = false">✕</button>
          </div>
          <p class="setting-desc">
            点选家里现有的食材，「今天吃啥」和菜谱「优先推荐」会立刻按它们重排。按类别找更快，佐料不参与推荐。
          </p>
          <div class="pantry-toolbar">
            <input
              v-model="pantryQuery" class="input" placeholder="搜食材，例如：排骨、番茄…"
              aria-label="搜索食材" maxlength="20"
            />
            <button class="btn btn-soft btn-sm" :disabled="!pantryDraft.size" @click="clearPantry">清空已选</button>
          </div>
          <div class="pantry-jumps">
            <button v-for="g in pantryGroups" :key="g.key" type="button" class="chip" @click="jumpToGroup(g.key)">{{ g.label }}</button>
          </div>
          <div class="pantry-feedback" aria-live="polite">
            <Transition name="duty-pop">
              <span :key="pantryDraft.size + '-' + pantryHitCount">
                🥬 已选 <b>{{ pantryDraft.size }}</b> 样食材，菜谱里有 <b>{{ pantryHitCount }}</b> 道菜用得上
              </span>
            </Transition>
          </div>
          <div v-if="!shownPantryGroups.length && !pantryCustomCards.length" class="pantry-no-result tiny muted">没找到这个食材，可在下面「添加自定义食材」里加一个。</div>
          <div v-for="g in shownPantryGroups" :key="g.key" :id="'pantry-group-' + g.key" class="pantry-section">
            <div class="pantry-group-head" :style="{ borderColor: g.tint }">{{ g.label }}</div>
            <div class="pantry-grid">
              <button
                v-for="p in g.items" :key="p" type="button"
                class="pantry-card" :class="{ active: pantryDraft.has(p) }"
                :aria-pressed="pantryDraft.has(p)" @click="togglePantry(p)"
              >
                <span class="pantry-art" :style="{ background: pantryVisual(p).tint }">
                  <span class="pantry-art-icon">{{ pantryVisual(p).icon }}</span>
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
                :aria-pressed="pantryDraft.has(p.name)" @click="togglePantry(p.name)"
              >
                <span class="pantry-art" :style="{ background: p.tint }">
                  <span class="pantry-art-icon">{{ p.icon }}</span>
                </span>
                <span class="pantry-card-name">{{ p.name }}</span>
                <span v-if="pantryDraft.has(p.name)" class="pantry-check">✓</span>
              </button>
            </div>
          </div>
          <div class="pantry-add-row">
            <input
              v-model="pantryCustom" class="input" maxlength="60"
              placeholder="自定义食材，例如：排骨、玉米…"
              aria-label="自定义食材" @keyup.enter="addCustomPantry"
            />
            <button class="btn btn-soft btn-sm" @click="addCustomPantry">添加</button>
          </div>
          <div class="modal-foot">
            <span class="tiny muted">保存后立即生效</span>
            <button class="btn" @click="pantryOpen = false">取消</button>
            <button class="btn btn-primary" :disabled="pending('settings')" @click="savePantry">保存食材</button>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>
