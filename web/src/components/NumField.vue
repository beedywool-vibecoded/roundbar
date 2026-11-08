<script setup lang="ts">
// M3 filled text field for a whole number. Out-of-range values show an error and are kept
// as typed so the user can finish typing; validation blocks saving.
import { computed } from 'vue'

const model = defineModel<number>({ required: true })
const props = defineProps<{ label: string; min?: number; max?: number; suffix?: string; help?: string }>()

const text = computed({
  get: () => (Number.isFinite(model.value) ? String(model.value) : ''),
  set: (v: string) => {
    const n = Number.parseInt(v, 10)
    model.value = Number.isNaN(n) ? Number.NaN : n
  },
})
const error = computed(() => {
  const v = model.value
  if (!Number.isFinite(v)) return 'Enter a number'
  if (props.min != null && v < props.min) return `At least ${props.min}`
  if (props.max != null && v > props.max) return `At most ${props.max}`
  return ''
})
</script>

<template>
  <div class="tf-wrap">
    <label class="tf" :class="{ invalid: !!error }">
      <span class="lbl">{{ label }}</span>
      <span class="in">
        <input v-model="text" inputmode="numeric" />
        <span v-if="suffix" class="suffix">{{ suffix }}</span>
      </span>
    </label>
    <p v-if="error" class="tf-help err">{{ error }}</p>
    <p v-else-if="help" class="tf-help">{{ help }}</p>
  </div>
</template>

<style scoped>
.tf-wrap {
  min-width: 0;
}
</style>
