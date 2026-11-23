<script setup>
defineEmits(['home', 'menu', 'family'])
</script>

<template>
  <section class="not-found">
    <span class="not-found-emoji" aria-hidden="true">🍽️</span>
    <span class="legal-eyebrow">404 · 页面未找到</span>
    <h1>这页好像被端错了</h1>
    <p>链接可能已经失效，也可能地址少了一位。菜和家人都还在，换个入口继续就好。</p>
    <div class="not-found-actions">
      <button class="btn btn-primary btn-lg" type="button" @click="$emit('home')">回到今天</button>
      <button class="btn btn-soft" type="button" @click="$emit('menu')">去菜谱挑菜</button>
      <button class="btn btn-soft" type="button" @click="$emit('family')">看看家人</button>
    </div>
  </section>
</template>
