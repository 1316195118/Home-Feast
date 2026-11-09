<script setup>
import { inject, computed } from 'vue'
import { dishImage, cuisineName, cuisineIcon, spicyText, artStyle } from '../utils'

const props = defineProps({ dish: { type: Object, required: true } })
defineEmits(['edit', 'open'])
const { state, actions, pending } = inject('app')

const image = computed(() => dishImage(props.dish, state.cuisines))
const cuisine = computed(() => cuisineName(props.dish.cuisine, state.cuisines))
</script>

<template>
  <article class="dish-card">
    <div
      class="dish-photo" role="button" tabindex="0" :aria-label="`查看 ${dish.name} 的做法`"
      @click="$emit('open', dish)" @keydown.enter="$emit('open', dish)"
      @keydown.space.prevent="$emit('open', dish)"
    >
      <img :src="image" :alt="dish.name" :style="artStyle(dish, state.dishes)" loading="lazy" class="img-fade" />
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
        <span class="dish-name-link" @click="$emit('open', dish)" title="点开看做法">{{ dish.name }}</span>
        <span class="tiny muted nowrap">{{ cuisineIcon(dish.cuisine, state.cuisines) }} {{ cuisine }}</span>
      </div>
      <p class="dish-desc">{{ dish.desc || '家里人都懂的一道菜' }}</p>
      <div class="dish-tags">
        <span class="tag" v-for="t in (dish.tags || []).slice(0, 3)" :key="t">{{ t }}</span>
        <span class="tag tag-spicy" v-if="dish.spicy > 0">{{ spicyText(dish.spicy) }}</span>
        <span class="tag tag-green" v-if="!dish.builtin">自家菜</span>
      </div>
      <div class="dish-meta">
        <span>⏱ {{ dish.minutes || 20 }} 分钟</span>
        <span>{{ '🌶️'.repeat(dish.spicy || 0) || '清淡' }}</span>
      </div>
      <div class="dish-foot">
        <button
          class="btn btn-soft btn-sm" :disabled="pending(`order:${dish.id}`)"
          @click="actions.order(dish.id)"
        >
          <span v-if="pending(`order:${dish.id}`)" class="btn-spinner" aria-hidden="true" />
          {{ pending(`order:${dish.id}`) ? '添加中' : '＋ 点这道' }}
        </button>
        <button class="icon-btn" title="编辑菜品" aria-label="编辑菜品" @click="$emit('edit', dish)">✏️</button>
      </div>
    </div>
  </article>
</template>
