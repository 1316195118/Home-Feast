<template>
  <div class="view-loading" role="status" aria-live="polite" aria-label="正在打开页面">
    <span class="view-loading-mark" aria-hidden="true">🍚</span>
    <span class="view-loading-copy">
      <b>正在摆好这一页</b>
      <i>内容马上就来</i>
    </span>
    <span class="view-loading-line" aria-hidden="true"><i></i></span>
  </div>
</template>

<style scoped>
.view-loading {
  min-height: 220px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 9px;
  color: var(--muted);
}
.view-loading-mark {
  width: 54px;
  height: 54px;
  display: grid;
  place-items: center;
  border-radius: 19px;
  background: var(--brand-soft);
  box-shadow: inset 0 0 0 1px rgba(192, 90, 60, .08);
  font-size: 1.55rem;
  animation: view-loading-breathe 1.2s var(--ease) infinite;
}
.view-loading-copy { display: grid; justify-items: center; gap: 2px; line-height: 1.4; }
.view-loading-copy b { color: var(--ink-soft); font-size: .82rem; }
.view-loading-copy i { font-size: .7rem; font-style: normal; }
.view-loading-line {
  width: 108px;
  height: 4px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--paper-deep);
}
.view-loading-line i {
  display: block;
  width: 42%;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--brand), var(--amber));
  animation: view-loading-slide 1s var(--ease) infinite;
}
@keyframes view-loading-breathe {
  0%, 100% { transform: translateY(0) rotate(-2deg); }
  50% { transform: translateY(-3px) rotate(2deg); }
}
@keyframes view-loading-slide {
  from { transform: translateX(-120%); }
  to { transform: translateX(340%); }
}
@media (prefers-reduced-motion: reduce) {
  .view-loading-mark, .view-loading-line i { animation: none; }
  .view-loading-line i { width: 100%; opacity: .55; }
}
</style>
