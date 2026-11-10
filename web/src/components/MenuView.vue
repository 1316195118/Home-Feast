<script setup>
import { inject, ref, computed, watch } from 'vue'
import DishCard from './DishCard.vue'
import DishEditor from './DishEditor.vue'
import DishDetail from './DishDetail.vue'
import { isCuisineHidden } from '../utils'

const { state } = inject('app')
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

const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return state.dishes.filter((d) => {
    if (isCuisineHidden(d.cuisine, state.hiddenCuisines)) return false
    if (activeCuisine.value !== 'all' && d.cuisine !== activeCuisine.value) return false
    if (onlyFavorite.value && !d.favorite) return false
    if (!q) return true
    return [d.name, d.desc, ...(d.tags || [])].join(' ').toLowerCase().includes(q)
  })
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
</script>

<template>
  <section>
    <div class="section-head menu-head">
      <div>
        <h2 class="section-title">📖 家里的菜谱</h2>
        <div class="section-sub">内置 {{ state.dishes.length }} 道菜，八大菜系和日常饭菜都有；看不顺眼就自己改</div>
      </div>
      <button class="btn btn-primary" @click="openAdd">＋ 添加菜品</button>
    </div>

    <div class="cuisine-strip">
      <button class="chip cuisine-chip" :class="{ active: activeCuisine === 'all' }" @click="activeCuisine = 'all'">
        🍽️ 全部 <span class="tiny">{{ visibleDishCount }}</span>
      </button>
      <button
        v-for="c in visibleCuisines" :key="c.key"
        class="chip cuisine-chip" :class="{ active: activeCuisine === c.key }"
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
      <button class="chip" :class="{ active: onlyFavorite }" @click="onlyFavorite = !onlyFavorite">
        ⭐ 只看收藏
      </button>
      <span class="tiny muted">共 {{ filtered.length }} 道</span>
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
  </section>
</template>
