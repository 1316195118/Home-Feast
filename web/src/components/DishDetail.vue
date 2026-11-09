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
      </template>
      <div v-else class="recipe-empty">
        <span class="empty-icon">📝</span>
        这道菜还没写做法，点下面的按钮把自家的方子记下来。
        <button v-if="editable" class="btn btn-soft btn-sm" @click="emit('edit', dish)">补上做法</button>
      </div>
    </div>
  </div>
</template>
