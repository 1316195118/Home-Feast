<template>
  <span class="brand-mark brand-logo" aria-hidden="true">
    <svg class="brand-logo-svg" viewBox="0 0 48 48" role="img">
      <path class="logo-steam steam-a" d="M18 15c-3-3 2-5 0-8" />
      <path class="logo-steam steam-b" d="M25 14c-3-3 3-5 0-8" />
      <path class="logo-steam steam-c" d="M31 15c-2-3 2-5 0-8" />
      <path class="logo-chopstick" d="M30 8 42 20" />
      <path class="logo-chopstick chopstick-b" d="M34 6 43 16" />
      <path class="logo-rice" d="M12 27c1-6 5-9 11-9s11 3 12 9Z" />
      <path class="logo-bowl" d="M9 27h30c-1 9-6 13-15 13S10 36 9 27Z" />
      <path class="logo-rim" d="M8.5 27h31" />
      <circle class="logo-tomato" cx="17.5" cy="23.5" r="3.2" />
      <path class="logo-leaf" d="M17 20c1-2 3-3 5-2-2 1-3 2-5 2Z" />
    </svg>
  </span>
</template>

<style scoped>
.brand-logo {
  padding: 6px;
  color: var(--brand);
  background:
    radial-gradient(circle at 28% 18%, rgba(255, 255, 255, .9), transparent 32%),
    linear-gradient(145deg, var(--surface), var(--paper-deep));
  transition: transform var(--dur-mid) var(--ease-spring), box-shadow var(--dur-mid) var(--ease), border-color var(--dur-mid) var(--ease);
}
.brand-logo:hover {
  transform: translateY(-1px) rotate(-2deg);
  border-color: color-mix(in srgb, var(--brand) 34%, var(--line));
  box-shadow: 0 7px 20px rgba(96, 70, 44, .14);
}
.brand-logo-svg { width: 100%; height: 100%; overflow: visible; }
.logo-steam {
  fill: none;
  stroke: var(--brand);
  stroke-width: 2.2;
  stroke-linecap: round;
  opacity: .52;
  transform-origin: center bottom;
  animation: logo-steam 2.8s var(--ease-in-out) infinite;
}
.steam-b { animation-delay: -.9s; opacity: .38; }
.steam-c { animation-delay: -1.8s; opacity: .46; }
.logo-chopstick {
  fill: none;
  stroke: var(--ink-soft);
  stroke-width: 2.1;
  stroke-linecap: round;
}
.chopstick-b { opacity: .72; }
.logo-rice {
  fill: color-mix(in srgb, var(--amber) 56%, var(--surface));
  stroke: var(--brand-dark);
  stroke-width: 1.5;
}
.logo-bowl {
  fill: color-mix(in srgb, var(--brand) 88%, #fff);
  stroke: var(--brand-dark);
  stroke-width: 1.4;
}
.logo-rim {
  fill: none;
  stroke: color-mix(in srgb, var(--brand-dark) 76%, #fff);
  stroke-width: 2.2;
  stroke-linecap: round;
}
.logo-tomato {
  fill: #d8583d;
  stroke: #a83d2a;
  stroke-width: .8;
}
.logo-leaf {
  fill: var(--green);
}
@keyframes logo-steam {
  0%, 100% { transform: translateY(1px) scaleY(.86); }
  45% { transform: translateY(-3px) scaleY(1.08); }
  70% { transform: translateY(-1px) scaleY(.96); }
}
@media (prefers-reduced-motion: reduce) {
  .logo-steam { animation: none; }
  .brand-logo { transition: none; }
}
</style>
