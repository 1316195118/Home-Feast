<script setup>
import { computed, ref, watch } from 'vue'
import Food3D from './Food3D.vue'
import photoMap from '../pantry-photo-map.json'
import { foodShape } from '../utils'

const props = defineProps({
  name: { type: String, required: true },
  size: { type: String, default: 'thumb' }, // thumb | full
  eager: { type: Boolean, default: false },
})

const broken = ref(false)
watch(() => props.name, () => { broken.value = false })

const source = computed(() => {
  const slug = photoMap[props.name]
  if (!slug || broken.value) return ''
  const folder = props.size === 'full' ? '' : 'thumb/'
  return `/pantry/${folder}${slug}.webp`
})
const fallbackShape = computed(() => foodShape(props.name))
</script>

<template>
  <span class="pantry-photo" :class="`is-${size}`">
    <img
      v-if="source"
      :src="source"
      alt=""
      :loading="eager ? 'eager' : 'lazy'"
      decoding="async"
      draggable="false"
      @error="broken = true"
    />
    <Food3D v-else :name="name" v-bind="fallbackShape" />
  </span>
</template>

<style scoped>
.pantry-photo {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  overflow: hidden;
  pointer-events: none;
}
.pantry-photo img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
  object-position: center;
  mix-blend-mode: multiply;
  user-select: none;
}
.pantry-photo.is-thumb img {
  filter: saturate(.96) contrast(1.02);
}
.pantry-photo :deep(.food3d) {
  transform: scale(1.1);
}
</style>
