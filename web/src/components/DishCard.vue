<script setup>
import { inject, computed } from 'vue'
import { imageSize, cuisineName, cuisineIcon, spicyText, artStyle, avoidConflicts, splitHighlight } from '../utils'
import { getRecipe } from '../recipes'
import { sparkleFromEvent, celebrate, vibrate } from '../feedback'
import DishPhoto from './DishPhoto.vue'

const props = defineProps({
  dish: { type: Object, required: true },
  highlight: { type: String, default: '' },
  multi: { type: Boolean, default: false },
  multiPicked: { type: Boolean, default: false },
  priority: { type: Boolean, default: false },
})
defineEmits(['edit', 'open', 'multi-toggle'])
const { state, actions, pending } = inject('app')

const cuisine = computed(() => cuisineName(props.dish.cuisine, state.cuisines))
const selfMember = computed(() => state.members.find((m) => m.id === state.currentUserId) || null)

async function onOrder(event, dishId) {
  const rect = event?.currentTarget?.getBoundingClientRect?.()
  const at = rect ? { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 } : null
  sparkleFromEvent(event, '🍚') // 点击即时反馈，先让手上有回应
  const ok = await actions.order(dishId)
  if (ok && at) {
    celebrate(at.x, at.y)
    vibrate(16)
  }
}
// 内置菜配料由 recipes.js 动态生成，一并纳入忌口匹配（香菜 / 蒜 / 花生这类才命中得到）
const avoidHits = computed(() => {
  const r = getRecipe(props.dish)
  const enriched = { ...props.dish, ingredients: [...(props.dish.ingredients || []), ...r.ingredients] }
  return avoidConflicts(enriched, selfMember.value)
})
</script>

<template>
  <article class="dish-card" :class="{ 'is-multi': multi, 'is-multi-picked': multiPicked }">
    <button
      v-if="multi" class="multi-check" type="button"
      :aria-label="multiPicked ? `取消选择 ${dish.name}` : `选择 ${dish.name}`"
      @click.stop="$emit('multi-toggle', dish)"
    >{{ multiPicked ? '✓' : '' }}</button>
    <div
      class="dish-photo" role="button" tabindex="0" :aria-label="`查看 ${dish.name} 的做法`"
      @click="multi ? $emit('multi-toggle', dish) : $emit('open', dish)"
      @keydown.enter="multi ? $emit('multi-toggle', dish) : $emit('open', dish)"
      @keydown.space.prevent="multi ? $emit('multi-toggle', dish) : $emit('open', dish)"
    >
      <DishPhoto
        :item="dish" :cuisines="state.cuisines" :dishes="state.dishes"
        :size="imageSize(state.settings.imageMode)" :alt="dish.name" :priority="priority"
        :style="artStyle(dish, state.dishes)" width="480" height="360"
        :loading="priority ? 'eager' : 'lazy'" :fetchpriority="priority ? 'high' : 'auto'"
        decoding="async"
      />
      <button class="dish-recipe" title="看做法" aria-label="看做法" @click.stop="$emit('open', dish)"><span class="badge-ico">📖</span></button>
      <button
        class="dish-fav" :class="{ 'is-fav': dish.favorite }"
        :title="dish.favorite ? '取消收藏' : '收藏这道菜'"
        :aria-label="dish.favorite ? '取消收藏' : '收藏这道菜'"
        :disabled="pending(`fav:${dish.id}`)"
        @click.stop="actions.toggleFavorite(dish)"
      ><span class="badge-ico">{{ dish.favorite ? '⭐' : '☆' }}</span></button>
    </div>
    <div class="dish-body">
      <div class="dish-title">
        <span class="dish-name-link" @click="$emit('open', dish)" title="点开看做法">
          <template v-for="(part, i) in splitHighlight(dish.name, highlight)" :key="i">
            <mark v-if="part.hit">{{ part.text }}</mark>
            <template v-else>{{ part.text }}</template>
          </template>
        </span>
        <span class="tiny muted nowrap">{{ cuisineIcon(dish.cuisine, state.cuisines) }} {{ cuisine }}</span>
      </div>
      <p class="dish-desc">{{ dish.desc || '家里人都懂的一道菜' }}</p>
      <div class="dish-tags">
        <span class="tag" v-for="t in (dish.tags || []).slice(0, 3)" :key="t">{{ t }}</span>
        <span class="tag tag-spicy" v-if="dish.spicy > 0">{{ spicyText(dish.spicy) }}</span>
        <span class="tag tag-pantry" v-if="dish._pantryHits > 0" title="冰箱里已有的主料命中数，佐料不计">🧊 {{ dish._pantryHits }} 样现成</span>
        <span
          v-if="avoidHits.length" class="tag tag-avoid"
          :title="`与 ${selfMember?.name} 的口味避雷冲突`"
        >⚠️ {{ avoidHits.join('、') }}</span>
      </div>
      <div class="dish-meta">
        <span>⏱ {{ dish.minutes || 20 }} 分钟</span>
        <span>{{ '🌶️'.repeat(dish.spicy || 0) || '清淡' }}</span>
      </div>
      <div class="dish-foot">
        <button
          class="btn btn-soft btn-sm" :disabled="pending(`order:${dish.id}`)"
          @click="onOrder($event, dish.id)"
        >
          <span v-if="pending(`order:${dish.id}`)" class="btn-spinner" aria-hidden="true" />
          {{ pending(`order:${dish.id}`) ? '添加中' : '点这道' }}
        </button>
        <button class="icon-btn" title="编辑菜品" aria-label="编辑菜品" @click="$emit('edit', dish)">✏️</button>
      </div>
    </div>
  </article>
</template>
