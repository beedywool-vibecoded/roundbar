<script setup lang="ts">
// A `timer` page: minute presets, Pomodoro cycles and a label. Defaults are stored as
// missing keys.
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import M3Switch from './M3Switch.vue'
import TextField from './TextField.vue'
import { LIMITS, TIMER_DEFAULTS } from '@/lib/model'
import type { TimerPage } from '@/lib/types'

const props = defineProps<{ page: TimerPage }>()
const MIN = TIMER_DEFAULTS.minMinutes
const MAX = TIMER_DEFAULTS.maxMinutes

const sameAsDefault = (m: number[]) => m.length === TIMER_DEFAULTS.minutes.length && m.every((v, i) => v === TIMER_DEFAULTS.minutes[i])
const minutes = computed<number[]>({
  get: () => props.page.minutes ?? [...TIMER_DEFAULTS.minutes],
  set: (m) => {
    const sorted = [...new Set(m)].sort((a, b) => a - b)
    if (sameAsDefault(sorted)) delete props.page.minutes
    else props.page.minutes = sorted
  },
})
function removePreset(v: number) {
  if (minutes.value.length <= 1) return
  minutes.value = minutes.value.filter((x) => x !== v)
}

const draft = ref('')
const draftError = computed(() => {
  if (!draft.value.trim()) return null
  const n = Number(draft.value)
  if (!Number.isInteger(n) || n < MIN || n > MAX) return `${MIN} to ${MAX} minutes`
  if (minutes.value.includes(n)) return 'Already a preset'
  return null
})
const full = computed(() => minutes.value.length >= LIMITS.timerPresets)
function addPreset() {
  const n = Number(draft.value)
  if (!draft.value.trim() || draftError.value || full.value) return
  minutes.value = [...minutes.value, n]
  draft.value = ''
}

const pomodoro = computed({
  get: () => props.page.pomodoro === true,
  set: (v: boolean) => {
    if (v) props.page.pomodoro = true
    else delete props.page.pomodoro
  },
})
const label = computed({
  get: () => props.page.label ?? '',
  set: (v: string) => {
    if (v) props.page.label = v
    else delete props.page.label
  },
})
</script>

<template>
  <div class="card">
    <div class="card-head"><h2>Timer</h2></div>
    <div class="setting col">
      <div>
        <div>Presets <span class="muted small">({{ minutes.length }} / {{ LIMITS.timerPresets }})</span></div>
        <div class="muted small">Chips at the bottom of the screen. Turning the ring sets any other duration.</div>
      </div>
      <div class="chips">
        <span v-for="m in minutes" :key="m" class="fchip on preset">
          {{ m }} min
          <button v-if="minutes.length > 1" class="x" type="button" :aria-label="`Remove ${m} min`" @click="removePreset(m)"><Icon name="close" :size="16" /></button>
        </span>
      </div>
      <form class="add-row" @submit.prevent="addPreset">
        <label class="tf" :class="{ invalid: !!draftError }">
          <span class="lbl">New preset</span>
          <span class="in">
            <input v-model="draft" inputmode="numeric" :disabled="full" :placeholder="full ? 'At most 4 presets' : 'e.g. 10'" />
            <span class="suffix">min</span>
          </span>
        </label>
        <button class="btn btn-tonal st" type="submit" :disabled="full || !draft.trim() || !!draftError"><Icon name="add" :size="18" /> Add</button>
      </form>
      <p v-if="draftError" class="tf-help err" style="margin: 0">{{ draftError }}</p>
    </div>
    <div class="setting">
      <div class="grow">
        <div>Pomodoro</div>
        <div class="muted small">Alternates 25 min of work and 5 min breaks, with a 15 min break after the fourth</div>
      </div>
      <M3Switch v-model="pomodoro" label="Pomodoro" />
    </div>
    <div class="setting col">
      <TextField v-model="label" label="Label (optional)" placeholder="Shown under the time, like Focus or Tea" :max="LIMITS.timerLabel" :error="label.length > LIMITS.timerLabel ? `At most ${LIMITS.timerLabel} characters` : null" />
      <p class="muted small" style="margin: 0">Tap the center to start or pause, hold to reset. When time is up the screen pulses and the PC shows a notification.</p>
    </div>
  </div>
</template>

<style scoped>
.preset {
  font-variant-numeric: tabular-nums;
}
.add-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.add-row .tf {
  flex: 0 1 200px;
}
.add-row .btn {
  height: 40px;
}
</style>
