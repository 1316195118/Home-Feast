<template>
  <div class="view-load-error" role="alert">
    <span class="view-load-error-icon" aria-hidden="true">!</span>
    <div>
      <b>这一页没有打开</b>
      <p>网络可能刚断了一下，重新加载即可继续使用。</p>
    </div>
    <button class="btn btn-soft btn-sm" type="button" @click="reload">重新加载</button>
  </div>
</template>

<script setup>
function reload() {
  window.location.reload()
}
</script>

<style scoped>
.view-load-error {
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 11px;
  padding: 22px;
  border: 1px solid rgba(169, 70, 46, .18);
  border-radius: var(--radius-lg);
  background: linear-gradient(145deg, var(--error-soft), var(--surface));
  color: var(--ink-soft);
}
.view-load-error-icon {
  width: 34px;
  height: 34px;
  flex: none;
  display: grid;
  place-items: center;
  border-radius: 12px;
  background: var(--error);
  color: var(--surface);
  font-weight: 900;
}
.view-load-error b { display: block; color: var(--ink); font-size: .85rem; }
.view-load-error p { margin-top: 2px; font-size: .72rem; line-height: 1.5; }
.view-load-error .btn { margin-left: 8px; flex: none; }
@media (max-width: 520px) {
  .view-load-error { flex-wrap: wrap; justify-content: flex-start; }
  .view-load-error .btn { margin-left: 45px; }
}
</style>
