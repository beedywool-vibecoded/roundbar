<script setup lang="ts">
// The big shape at the top of a setup step: an M3 Expressive shape with a symbol on it.
// `board` draws the round Roundbar itself with its USB cable instead.
import { computed } from 'vue'
import M3Shape from '../M3Shape.vue'
import type { ShapeKind } from '@/lib/shapes'

const props = withDefaults(
  defineProps<{ icon?: string; kind?: ShapeKind; tone?: 'primary' | 'tertiary' | 'error' | 'neutral' | 'ok'; board?: boolean; waiting?: boolean }>(),
  { icon: 'check', kind: 'cookie', tone: 'primary', board: false, waiting: false },
)

const colors = computed(() => {
  switch (props.tone) {
    case 'tertiary':
      return ['var(--tc)', 'var(--on-tc)']
    case 'error':
      return ['var(--ec)', 'var(--on-ec)']
    case 'neutral':
      return ['var(--sch)', 'var(--on-sv)']
    default:
      return ['var(--pc)', 'var(--on-pc)']
  }
})

// Six buttons on the ring and one in the middle, like the device's radial page
const DOTS = Array.from({ length: 6 }, (_, k) => {
  const a = -Math.PI / 2 + (k * Math.PI) / 3
  return { cx: 70 + 27 * Math.cos(a), cy: 62 + 27 * Math.sin(a) }
})
</script>

<template>
  <div class="hero" :class="{ waiting }" aria-hidden="true">
    <svg v-if="board" class="board" width="140" height="150" viewBox="0 0 140 150">
      <g class="cable">
        <path d="M70 150 V128" stroke="var(--outline)" stroke-width="6" stroke-linecap="round" fill="none" />
        <rect x="61" y="110" width="18" height="20" rx="5" fill="var(--outline)" />
      </g>
      <circle cx="70" cy="62" r="56" fill="#111" />
      <circle cx="70" cy="62" r="49" :fill="colors[0]" />
      <circle v-for="(d, i) in DOTS" :key="i" class="dot" :style="{ animationDelay: `${i * 0.18}s` }" :cx="d.cx" :cy="d.cy" r="7" :fill="colors[1]" />
      <circle cx="70" cy="62" r="11" :fill="colors[1]" />
    </svg>
    <M3Shape v-else :kind="kind" :size="120" :fill="colors[0]" class="shape">
      <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="48" class="sym" :fill="colors[1]">{{ icon }}</text>
    </M3Shape>
  </div>
</template>

<style scoped>
.hero {
  display: grid;
  place-items: center;
  height: 150px;
}
.shape {
  animation: pop 0.5s var(--ease-decel) both;
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
}
.waiting .cable {
  animation: plug 2.4s var(--ease) infinite;
}
.waiting .dot {
  animation: blink 1.6s ease-in-out infinite;
}
@keyframes plug {
  0%,
  100% {
    transform: translateY(14px);
  }
  50% {
    transform: translateY(0);
  }
}
@keyframes blink {
  0%,
  100% {
    opacity: 0.35;
  }
  50% {
    opacity: 1;
  }
}
@keyframes pop {
  from {
    transform: scale(0.7) rotate(-30deg);
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .shape,
  .waiting .cable,
  .waiting .dot {
    animation: none;
  }
}
</style>
