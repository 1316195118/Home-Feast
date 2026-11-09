<script setup>
import { ref } from 'vue'
import { useModalBehavior } from '../modal'

const props = defineProps({
  title: { type: String, default: '请确认' },
  message: { type: String, default: '' },
  confirmText: { type: String, default: '确定' },
  cancelText: { type: String, default: '取消' },
  danger: { type: Boolean, default: false },
  busy: { type: Boolean, default: false },
})
const emit = defineEmits(['confirm', 'cancel'])
const root = ref(null)

// Esc 取消 + 打开时锁背景滚动 + 焦点归还（栈式，嵌套弹窗只响应最上层）
useModalBehavior(() => { if (!props.busy) emit('cancel') }, root)
</script>

<template>
  <div ref="root" class="modal-backdrop confirm-backdrop" @click.self="emit('cancel')">
    <div class="modal confirm-modal" role="dialog" aria-modal="true" :aria-label="title">
      <div class="modal-head">
        <h2>{{ title }}</h2>
        <button class="icon-btn" @click="emit('cancel')" aria-label="关闭">✕</button>
      </div>
      <p class="confirm-msg">{{ message }}</p>
      <div class="modal-foot">
        <button class="btn" :disabled="busy" @click="emit('cancel')">{{ cancelText }}</button>
        <button
          class="btn" :class="danger ? 'btn-danger' : 'btn-primary'"
          :disabled="busy" @click="emit('confirm')"
        >
          <span v-if="busy" class="btn-spinner" aria-hidden="true" />
          {{ confirmText }}
        </button>
      </div>
    </div>
  </div>
</template>
