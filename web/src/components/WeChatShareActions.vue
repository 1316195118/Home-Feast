<script setup>
import { computed, inject, ref } from 'vue'
import {
  canShareImageFile, copyPlainText, dataUrlToFile, downloadDataUrl,
  isWeChatBrowser, shareImageFile,
} from '../share'

const props = defineProps({
  imageData: { type: String, default: '' },
  filename: { type: String, required: true },
  title: { type: String, default: '' },
  text: { type: String, default: '' },
  saveLabel: { type: String, default: '保存图片' },
})

const { actions } = inject('app')
const busy = ref(false)
const guide = ref('')
const copied = ref(false)
const inWeChat = isWeChatBrowser()
const fallbackCopy = computed(() => props.text || props.title || '家里吃点啥')

async function shareToWeChat() {
  if (busy.value || !props.imageData) return
  busy.value = true
  guide.value = ''
  copied.value = false
  try {
    const file = dataUrlToFile(props.imageData, props.filename)
    if (!canShareImageFile(file)) {
      const copyOk = await copyPlainText(fallbackCopy.value)
      copied.value = copyOk
      if (inWeChat) {
        guide.value = 'wechat'
        actions.toast('微信内请长按上方图片，选「发送给朋友」', 'info')
      } else {
        downloadDataUrl(props.imageData, props.filename)
        guide.value = 'desktop'
        actions.toast(copyOk ? '图片已保存，文案已复制，打开微信粘贴发送' : '图片已保存，打开微信发送这张图片', 'info')
      }
      return
    }
    const result = await shareImageFile(file, { title: props.title, text: props.text })
    if (result === 'shared') actions.toast('分享面板已打开，选择微信发给家人', 'success')
  } catch (err) {
    actions.toast(err?.message || '没能打开分享，请长按图片保存后发送', 'error')
  } finally {
    busy.value = false
  }
}

function saveImage() {
  if (!props.imageData) return
  downloadDataUrl(props.imageData, props.filename)
  actions.toast(inWeChat ? '已尝试保存；也可长按图片保存' : '图片已保存到下载目录', 'info')
}

async function copyAgain() {
  copied.value = await copyPlainText(fallbackCopy.value)
  actions.toast(copied.value ? '文案已复制，打开微信粘贴发送' : '复制失败，请长按图片发送', copied.value ? 'success' : 'warn')
}
</script>

<template>
  <div class="wechat-share-actions">
    <button
      class="btn btn-primary btn-sm wechat-share-btn" type="button"
      :disabled="busy || !imageData" :aria-busy="busy" @click="shareToWeChat"
    >
      <span class="wechat-share-icon" aria-hidden="true"><i></i><i></i></span>
      <span>{{ busy ? '正在准备…' : '分享到微信' }}</span>
    </button>
    <button class="btn btn-soft btn-sm" type="button" :disabled="!imageData" @click="saveImage">
      📱 {{ saveLabel }}
    </button>
    <div v-if="guide" class="wechat-share-guide" role="status">
      <span class="wechat-guide-badge" aria-hidden="true">微信</span>
      <div class="wechat-guide-copy">
        <strong>{{ guide === 'wechat' ? '长按图片即可直接发微信' : '图片和文案已经准备好' }}</strong>
        <span v-if="guide === 'wechat'">微信内置浏览器不允许网页直接打开分享面板；长按上方图片，选择「发送给朋友」。</span>
        <span v-else>图片已保存，文案{{ copied ? '已复制' : '可在下方复制' }}；打开微信粘贴发送即可。</span>
      </div>
      <button class="btn btn-ghost btn-sm" type="button" @click="copyAgain">{{ copied ? '再复制一次' : '复制文案' }}</button>
    </div>
  </div>
</template>
