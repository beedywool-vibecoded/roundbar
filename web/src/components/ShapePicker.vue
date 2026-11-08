<script setup lang="ts">
// Row of shape chips, each drawing its shape. Picking the fallback emits it like any other;
// callers decide whether that clears the stored value.
import { SHAPE_LABELS } from '@/lib/model'
import { BUTTON_SHAPES, buttonPath, type ButtonShape } from '@/lib/shapes'

defineProps<{ modelValue: ButtonShape; fallback: ButtonShape; fill: string; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [s: ButtonShape] }>()
</script>

<template>
  <div class="shapes" role="radiogroup" :aria-label="label">
    <button
      v-for="s in BUTTON_SHAPES"
      :key="s"
      type="button"
      role="radio"
      class="shape st"
      :class="{ on: modelValue === s }"
      :aria-checked="modelValue === s"
      v-tip="s === fallback ? `${SHAPE_LABELS[s]} (default here)` : SHAPE_LABELS[s]"
      @click="emit('update:modelValue', s)"
    >
      <svg width="40" height="40" viewBox="0 0 100 100" aria-hidden="true">
        <path :d="buttonPath(s, 50, 50, 92)" :fill="modelValue === s ? fill : 'var(--schh)'" />
      </svg>
      <span>{{ s === 'square' ? 'Square' : SHAPE_LABELS[s] }}</span>
    </button>
  </div>
</template>

<style scoped>
.shapes {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}
.shape {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  width: 72px;
  padding: 8px 0 6px;
  border: none;
  border-radius: 16px;
  background: transparent;
  color: var(--on-sv);
  font-size: 12px;
  font-weight: 500;
}
.shape.on {
  color: var(--on-surface);
  background: var(--sch);
}
</style>
