<script setup lang="ts">
// Port of Pelote's loading indicator (components/bundle.js): every 700 ms the path morphs
// to the next shape and turns 140° with a 650 ms spring. The first step comes after 20 ms.
// Paused off screen and in hidden tabs; static under prefers-reduced-motion.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { LOADER_SEQUENCE, LOADER_SHAPES, LOADER_STEP_MS, LOADER_TURN_DEG } from '@/lib/loaderShapes'

const props = withDefaults(
  defineProps<{ small?: boolean; label?: string; contained?: boolean }>(),
  { small: false, label: 'Loading', contained: false },
)

const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
const step = ref(0)
const turn = ref(0)
const host = ref<HTMLSpanElement>()
let start: ReturnType<typeof setTimeout> | undefined
let timer: ReturnType<typeof setInterval> | undefined
let observer: IntersectionObserver | undefined
let visible = true

function beat() {
  if (!visible || document.hidden) return
  step.value = (step.value + 1) % LOADER_SEQUENCE.length
  turn.value += LOADER_TURN_DEG
}

onMounted(() => {
  if (reduce) return
  if (host.value && 'IntersectionObserver' in window) {
    observer = new IntersectionObserver((entries) => {
      visible = entries[0]?.isIntersecting ?? false
    })
    observer.observe(host.value)
  }
  start = setTimeout(() => {
    beat()
    timer = setInterval(beat, LOADER_STEP_MS)
  }, 20)
})

onBeforeUnmount(() => {
  clearTimeout(start)
  clearInterval(timer)
  observer?.disconnect()
})

const d = computed(() => LOADER_SHAPES[LOADER_SEQUENCE[step.value] ?? 'cookie9'])
const box = computed(() => (props.small ? 20 : 48))
const glyph = computed(() => (props.small ? 18 : 38))
</script>

<template>
  <span
    ref="host"
    class="shape-loader"
    :class="{ contained, small }"
    :style="{ width: `${box}px`, height: `${box}px` }"
    role="progressbar"
    :aria-label="label"
  >
    <svg :width="glyph" :height="glyph" viewBox="0 0 100 100" aria-hidden="true">
      <path :d="d" fill="currentColor" :style="{ transform: `rotate(${turn}deg)` }" />
    </svg>
  </span>
</template>

<style scoped>
.shape-loader {
  display: inline-grid;
  place-items: center;
  flex: none;
  color: var(--primary);
}
.shape-loader.contained {
  border-radius: 24px;
  background: var(--pc);
  color: var(--on-pc);
}
/* Inside a button, the indicator takes the label's colour, as in Pelote */
.btn .shape-loader {
  color: currentColor;
}
path {
  transform-origin: 50px 50px;
  transition: transform 650ms cubic-bezier(0.34, 1.56, 0.64, 1);
}
@media (prefers-reduced-motion: reduce) {
  path {
    transition: none;
  }
}
</style>
