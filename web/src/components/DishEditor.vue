<script setup>
import { inject, reactive, ref, computed } from 'vue'
import { api } from '../api'
import { dishImage } from '../utils'
import { getRecipe } from '../recipes'
import { useModalBehavior } from '../modal'

const props = defineProps({ dish: { type: Object, default: null } })
const emit = defineEmits(['close'])
const { state, actions, confirmDialog } = inject('app')
const root = ref(null)
useModalBehavior(() => emit('close'), root)

// 新建菜品时，默认选第一个「可见」的菜系（比如重庆菜），而不是被隐藏的家常菜。
function firstVisibleCuisine() {
  const hidden = state.hiddenCuisines || []
  return (state.cuisines || []).find((c) => !hidden.includes(c.key))?.key || 'homestyle'
}

// 编辑已有菜时，若还没存过自家做法，就把内置参考做法预填进表单，方便在此基础上改。
const reference = props.dish ? getRecipe(props.dish) : { ingredients: [], steps: [] }
const ownIngredients = props.dish?.ingredients || []
const ownSteps = props.dish?.steps || []

const form = reactive({
  name: props.dish?.name || '',
  cuisine: props.dish?.cuisine || firstVisibleCuisine(),
  desc: props.dish?.desc || '',
  tags: (props.dish?.tags || []).join('、'),
  spicy: props.dish?.spicy || 0,
  minutes: props.dish?.minutes || 20,
  image: props.dish?.image || '',
  ingredients: (ownIngredients.length ? ownIngredients : reference.ingredients).join('\n'),
  steps: (ownSteps.length ? ownSteps : reference.steps).join('\n'),
})
const uploading = ref(false)
const saving = ref(false)

// 下拉里只列「可见」的菜系；但若正在编辑一道被隐藏菜系的菜，保留它以免选项消失。
const cuisineOptions = computed(() => {
  const hidden = state.hiddenCuisines || []
  const opts = (state.cuisines || []).filter((c) => !hidden.includes(c.key))
  if (form.cuisine && !opts.some((c) => c.key === form.cuisine)) {
    const cur = (state.cuisines || []).find((c) => c.key === form.cuisine)
    if (cur) opts.push(cur)
  }
  return opts
})

const preview = computed(() => form.image || dishImage({ cuisine: form.cuisine }, state.cuisines))

async function onFile(event) {
  const file = event.target.files?.[0]
  if (!file) return
  const data = new FormData()
  data.append('file', file)
  uploading.value = true
  try {
    const result = await api.upload(data)
    form.image = result.url
    actions.toast('照片已上传', 'success')
  } catch (err) {
    actions.toast(err.message, 'error')
  } finally {
    uploading.value = false
    event.target.value = ''
  }
}

async function save() {
  if (!form.name.trim()) {
    actions.toast('先给这道菜起个名字', 'warn')
    return
  }
  const payload = {
    name: form.name.trim(),
    cuisine: form.cuisine,
    desc: form.desc.trim(),
    tags: form.tags.split(/[、,，\s]+/).filter(Boolean).slice(0, 6),
    spicy: Number(form.spicy) || 0,
    minutes: Number(form.minutes) || 20,
    image: form.image,
    ingredients: form.ingredients.split('\n').map((s) => s.trim()).filter(Boolean).slice(0, 20),
    steps: form.steps.split('\n').map((s) => s.trim()).filter(Boolean).slice(0, 20),
  }
  saving.value = true
  const ok = props.dish
    ? await actions.updateDish(props.dish.id, payload)
    : await actions.addDish(payload)
  saving.value = false
  if (ok) emit('close')
}

async function remove() {
  if (!props.dish) return
  const ok = await confirmDialog({
    title: '移除菜品',
    message: `确定从菜谱移除「${props.dish.name}」吗？已点过的历史记录仍会保留菜名。`,
    confirmText: '移除', cancelText: '再想想', danger: true,
  })
  if (!ok) return
  const ok2 = await actions.deleteDish(props.dish.id)
  if (ok2) emit('close')
}
</script>

<template>
  <div ref="root" class="modal-backdrop" @click.self="emit('close')">
    <div class="modal modal-wide" role="dialog" aria-modal="true" :aria-label="dish ? '编辑菜品' : '添加一道自家的菜'">
      <div class="modal-head">
        <h2>{{ dish ? '编辑菜品' : '添加一道自家的菜' }}</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </div>

      <div class="form-grid">
        <div class="span-2">
          <div class="preview-photo"><img :src="preview" alt="菜品效果预览" /></div>
          <div class="row" style="margin-top: 8px">
            <label class="btn btn-sm" :class="{ 'btn-soft': !uploading, 'is-busy': uploading }">
              <span v-if="uploading" class="btn-spinner" aria-hidden="true" />
              {{ uploading ? '上传中…' : '📷 上传自己的照片' }}
              <input type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden :disabled="uploading" @change="onFile" />
            </label>
            <button class="btn btn-sm btn-ghost" v-if="form.image" @click="form.image = ''">改用菜系插画</button>
            <span class="tiny muted">不传照片就沿用 AI 绘制的菜系插画</span>
          </div>
        </div>

        <div class="field span-2">
          <label for="dish-name">菜名</label>
          <input id="dish-name" v-model="form.name" class="input" maxlength="40" placeholder="比如：妈妈牌红烧肉" />
        </div>

        <div class="field">
          <label for="dish-cuisine">属于哪一类</label>
          <select id="dish-cuisine" v-model="form.cuisine" class="select">
            <option v-for="c in cuisineOptions" :key="c.key" :value="c.key">{{ c.icon }} {{ c.name }}</option>
          </select>
        </div>
        <div class="field">
          <label for="dish-minutes">大概要多久（分钟）</label>
          <input id="dish-minutes" v-model="form.minutes" class="input" type="number" min="0" max="600" />
        </div>

        <div class="field span-2">
          <label for="dish-desc">一句话描述</label>
          <input id="dish-desc" v-model="form.desc" class="input" maxlength="200" placeholder="写点家里人一看就懂的话" />
        </div>

        <div class="field">
          <label for="dish-tags">标签（顿号或逗号分隔）</label>
          <input id="dish-tags" v-model="form.tags" class="input" placeholder="下饭、快手、周末" />
        </div>
        <div class="field">
          <label for="dish-spicy">辣度</label>
          <select id="dish-spicy" v-model="form.spicy" class="select">
            <option :value="0">不辣</option>
            <option :value="1">微辣 🌶️</option>
            <option :value="2">中辣 🌶️🌶️</option>
            <option :value="3">特辣 🌶️🌶️🌶️</option>
          </select>
        </div>

        <div class="field span-2">
          <label for="dish-ingredients">🧺 配料（每行一项，可写用量）</label>
          <textarea
            id="dish-ingredients" v-model="form.ingredients" class="textarea" rows="5"
            placeholder="嫩豆腐 1 盒&#10;牛肉末 100g&#10;郫县豆瓣酱 1 勺"
          ></textarea>
        </div>
        <div class="field span-2">
          <label for="dish-steps">👩‍🍳 做法步骤（每行一步）</label>
          <textarea
            id="dish-steps" v-model="form.steps" class="textarea" rows="6"
            placeholder="豆腐切块，淡盐水泡 5 分钟去豆腥&#10;热油炒香肉末和豆瓣酱，加水烧开&#10;推入豆腐小火煮 5 分钟，淋薄芡收汁"
          ></textarea>
          <span class="tiny muted">留空就不显示做法；已填的内容会成为这道菜自家的方子。</span>
        </div>
      </div>

      <div class="modal-foot">
        <button v-if="dish" class="btn btn-danger" @click="remove">移除菜品</button>
        <span class="grow"></span>
        <button class="btn" :disabled="saving" @click="emit('close')">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="save">
          <span v-if="saving" class="btn-spinner btn-spinner-light" aria-hidden="true" />
          {{ saving ? '保存中…' : (dish ? '保存修改' : '加入菜谱') }}
        </button>
      </div>
    </div>
  </div>
</template>
