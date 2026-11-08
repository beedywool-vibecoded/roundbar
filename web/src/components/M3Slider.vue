<script setup lang="ts">
// M3 slider, in place of the browser's <input type="range">. Pointer drag, click on the
// track, arrows (Shift for big steps), Home and End.
import { computed, ref } from 'vue'

const value = defineModel<number>({ required: true })
const props = withDefaults(defineProps<{ min?: number; max?: number; step?: number; label: string; format?: (v: number) => string }>(), {
  min: 0,
  max: 1,
  step: 0.01,
})
const emit = defineEmits<{ commit: [v: number] }>()
const track = ref<HTMLElement>()
const dragging = ref(false)
const pct = computed(() => ((value.value - props.min) / (props.max - props.min)) * 100)
const text = computed(() => (props.format ? props.format(value.value) : String(value.value)))

function clamp(v: number) {
  const stepped = Math.round((v - props.min) / props.step) * props.step + props.min
  return Math.min(props.max, Math.max(props.min, +stepped.toFixed(4)))
}
function fromPointer(e: PointerEvent) {
  const r = track.value!.getBoundingClientRect()
  value.value = clamp(props.min + ((e.clientX - r.left) / r.width) * (props.max - props.min))
}
function down(e: PointerEvent) {
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  fromPointer(e)
}
function up() {
  if (!dragging.value) return
  dragging.value = false
  emit('commit', value.value)
}
function key(e: KeyboardEvent) {
  const big = (props.max - props.min) / 10
  const d = { ArrowRight: 1, ArrowUp: 1, ArrowLeft: -1, ArrowDown: -1 }[e.key]
  if (d) value.value = clamp(value.value + d * (e.shiftKey ? big : props.step))
  else if (e.key === 'Home') value.value = props.min
  else if (e.key === 'End') value.value = props.max
  else return
  e.preventDefault()
  emit('commit', value.value)
}
</script>

<template>
  <div
    ref="track"
    class="slider"
    :class="{ dragging }"
    role="slider"
    tabindex="0"
    :aria-label="label"
    :aria-valuemin="min"
    :aria-valuemax="max"
    :aria-valuenow="value"
    :aria-valuetext="text"
    @pointerdown="down"
    @pointermove="dragging && fromPointer($event)"
    @pointerup="up"
    @pointercancel="up"
    @keydown="key"
  >
    <span class="active" :style="{ width: `calc(${pct}% - 6px)` }" />
    <span class="inactive" :style="{ left: `calc(${pct}% + 6px)` }" />
    <span class="handle" :style="{ left: `${pct}%` }"><span class="bubble">{{ text }}</span></span>
  </div>
</template>

<style scoped>
.slider {
  position: relative;
  height: 44px;
  cursor: pointer;
  touch-action: none;
  outline: none;
}
.active,
.inactive {
  position: absolute;
  top: 50%;
  height: 16px;
  translate: 0 -50%;
}
.active {
  left: 0;
  border-radius: 8px 2px 2px 8px;
  background: var(--primary);
}
.inactive {
  right: 0;
  border-radius: 2px 8px 8px 2px;
  background: var(--secc);
}
.handle {
  position: absolute;
  top: 50%;
  width: 4px;
  height: 44px;
  translate: -50% -50%;
  border-radius: 2px;
  background: var(--primary);
}
.bubble {
  position: absolute;
  bottom: calc(100% + 6px);
  left: 50%;
  translate: -50% 4px;
  padding: 6px 10px;
  border-radius: 14px;
  background: var(--on-surface);
  color: var(--surface);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  opacity: 0;
  transition: opacity 0.15s, translate 0.15s var(--ease);
  pointer-events: none;
}
.dragging .bubble,
.slider:focus-visible .bubble {
  opacity: 1;
  translate: -50% 0;
}
.slider:focus-visible .handle {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 35%, transparent);
}
</style>
