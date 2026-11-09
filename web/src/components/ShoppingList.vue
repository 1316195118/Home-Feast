<script setup>
import { inject, computed, ref } from 'vue'
import { shoppingList } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const emit = defineEmits(['close'])
const { state, actions } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

// 内置菜的配料由 recipes.js 按「技法模板 + 主料表」动态生成，dish.ingredients 本身是空的；
// 用户在编辑器里自己填的配料优先（getRecipe 已处理），这里把它展开后再聚合。
const dishesWithIngredients = computed(() => state.dishes.map((d) => {
  const r = getRecipe(d)
  return { ...d, ingredients: r.ingredients.length ? r.ingredients : (d.ingredients || []) }
}))
const list = computed(() => shoppingList(state.plan.orders || [], dishesWithIngredients.value))
const STORE_KEY = `fm.shopping.${state.today}`

function readChecked() {
  try { return new Set(JSON.parse(localStorage.getItem(STORE_KEY) || '[]')) } catch { return new Set() }
}
const checked = ref(readChecked())
function persist() {
  try { localStorage.setItem(STORE_KEY, JSON.stringify([...checked.value])) } catch { /* 隐私模式忽略 */ }
}
function toggle(text) {
  const next = new Set(checked.value)
  if (next.has(text)) next.delete(text)
  else next.add(text)
  checked.value = next
  persist()
}
function clearChecked() {
  checked.value = new Set()
  persist()
}
async function copyList() {
  const lines = list.value.map(
    (x) => `${checked.value.has(x.text) ? '☑' : '☐'} ${x.text}${x.count > 1 ? ` ×${x.count}` : ''}`,
  )
  const text = `家里吃点啥 · ${state.today} 购物清单\n${lines.join('\n')}`
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    ta.remove()
  }
  actions.toast('清单已复制，可以直接发给买菜的人', 'success')
}
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide" role="dialog" aria-modal="true" aria-label="购物清单">
      <div class="modal-head">
        <h2>🛒 购物清单</h2>
        <button class="icon-btn" aria-label="关闭" @click="emit('close')">✕</button>
      </div>
      <p class="setting-desc">
        按今天已点菜品的配料自动归并，同名只列一条；勾选状态记在这台设备上（{{ state.today }}）。
      </p>
      <div v-if="list.length" class="shopping-list">
        <label v-for="item in list" :key="item.text" class="shopping-item" :class="{ done: checked.has(item.text) }">
          <input type="checkbox" :checked="checked.has(item.text)" @change="toggle(item.text)" />
          <span class="shopping-name">{{ item.text }}</span>
          <span v-if="item.count > 1" class="shopping-count">×{{ item.count }}</span>
          <span class="shopping-from tiny muted">{{ item.from.join('、') }}</span>
        </label>
      </div>
      <div v-else class="empty" style="padding: 18px">
        <span class="empty-icon">🧺</span>今天还没点菜，或者这些菜还没填配料
      </div>
      <div class="setting-actions">
        <button class="btn btn-soft btn-sm" :disabled="!list.length" @click="copyList">📋 复制清单</button>
        <button class="btn btn-ghost btn-sm" :disabled="!checked.size" @click="clearChecked">清空勾选</button>
        <span class="grow"></span>
        <button class="btn btn-primary btn-sm" @click="emit('close')">完成</button>
      </div>
    </div>
  </div>
</template>
