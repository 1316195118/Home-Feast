<script setup>
import { inject, computed, ref } from 'vue'
import { dishImage, cuisineName, cuisineIcon, spicyText, artStyle } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const props = defineProps({
  dish: { type: Object, required: true },
  editable: { type: Boolean, default: true },
})
const emit = defineEmits(['close', 'edit'])
const { state, actions, pending } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

const image = computed(() => dishImage(props.dish, state.cuisines))
const cuisine = computed(() => cuisineName(props.dish.cuisine, state.cuisines))
const recipe = computed(() => getRecipe(props.dish))
const ordering = computed(() => pending(`order:${props.dish.id}`))

// 下厨小贴士：按这道菜的辣度、耗时与主料给出可操作的补充要点，
// 让「做法」不只是步骤清单，也解释了火候和为什么这么做。
const tips = computed(() => {
  const d = props.dish
  const out = []
  out.push((d.spicy || 0) >= 2
    ? '豆瓣酱、干辣椒下锅后转小火慢慢炒出红油，再下主料，辣味才香而不呛。'
    : '起锅前先尝咸淡再补盐，家常菜宁可淡一点，上桌还能按口味蘸酱。')
  out.push((d.minutes || 20) <= 15
    ? '这道菜讲究大火快炒：锅先烧热、食材沥干水分，分批下锅才不容易出水。'
    : '炖煮类先大火烧开再转小火，锅盖留一条缝，汤汁会更浓亮。')
  const ing = [...(d.ingredients || []), ...recipe.value.ingredients].join(' ')
  out.push(/肉|鸡|牛|排骨|鱼|虾|翅|肝/.test(ing)
    ? '肉类提前用少许盐、料酒和淀粉抓匀腌 10 分钟，下锅更嫩不柴。'
    : '蔬菜临出锅再放蒜末或葱花，香气更足、颜色也更亮。')
  return out
})

async function order() {
  const ok = await actions.order(props.dish.id)
  if (ok) emit('close')
}
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide dish-detail" role="dialog" aria-modal="true" :aria-label="dish.name">
      <div class="modal-head">
        <h2>{{ dish.name }}</h2>
        <button class="icon-btn" @click="emit('close')" aria-label="关闭">✕</button>
      </div>

      <div class="detail-top">
        <div class="detail-photo">
          <img :src="image" :alt="dish.name" :style="artStyle(dish, state.dishes)" />
        </div>
        <div class="detail-info">
          <p class="detail-desc">{{ dish.desc || '家里人都懂的一道菜' }}</p>
          <div class="detail-meta">
            <span class="tag">{{ cuisineIcon(dish.cuisine, state.cuisines) }} {{ cuisine }}</span>
            <span class="tag">⏱ {{ dish.minutes || 20 }} 分钟</span>
            <span class="tag" v-if="dish.spicy > 0">{{ spicyText(dish.spicy) }}</span>
            <span class="tag tag-green" v-if="!dish.builtin">自家菜</span>
          </div>
          <div class="dish-tags" v-if="(dish.tags || []).length">
            <span class="tag" v-for="t in dish.tags" :key="t">{{ t }}</span>
          </div>
          <div class="detail-actions">
            <button class="btn btn-primary btn-sm" :disabled="ordering" @click="order">
              <span v-if="ordering" class="btn-spinner btn-spinner-light" aria-hidden="true" />
              {{ ordering ? '添加中…' : '＋ 点这道' }}
            </button>
            <button v-if="editable" class="btn btn-soft btn-sm" @click="emit('edit', dish)">✏️ 改做法</button>
          </div>
        </div>
      </div>

      <template v-if="recipe.ingredients.length || recipe.steps.length">
        <div class="detail-cols">
          <section class="recipe-col">
            <h3 class="recipe-title">🧺 配料</h3>
            <ul class="recipe-ingredients" v-if="recipe.ingredients.length">
              <li v-for="(item, i) in recipe.ingredients" :key="i">{{ item }}</li>
            </ul>
            <p v-else class="tiny muted">还没填配料</p>
          </section>
          <section class="recipe-col">
            <h3 class="recipe-title">👩‍🍳 做法</h3>
            <ol class="recipe-steps" v-if="recipe.steps.length">
              <li v-for="(step, i) in recipe.steps" :key="i">{{ step }}</li>
            </ol>
            <p v-else class="tiny muted">还没填步骤</p>
          </section>
        </div>
        <p class="recipe-note">
          {{ recipe.custom ? '这是你们家自己记下的做法' : '家常做法参考，可按自家口味在「改做法」里调整' }}
        </p>
        <section class="recipe-tips">
          <h3 class="recipe-title">🧑‍🍳 下厨小贴士</h3>
          <ul>
            <li v-for="(t, i) in tips" :key="i">{{ t }}</li>
          </ul>
        </section>
      </template>
      <div v-else class="recipe-empty">
        <span class="empty-icon">📝</span>
        这道菜还没写做法，点下面的按钮把自家的方子记下来。
        <button v-if="editable" class="btn btn-soft btn-sm" @click="emit('edit', dish)">补上做法</button>
      </div>
    </div>
  </div>
</template>
