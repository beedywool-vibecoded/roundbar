<script setup lang="ts">
// Step 4: the keyboard layout the board types with (device.keyboardLayout).
import { computed } from 'vue'
import M3Select from '../M3Select.vue'
import SetupHero from './SetupHero.vue'
import { LAYOUTS, layoutName } from '@/lib/layouts'
import { config, daemonUp, live } from '@/lib/store'
import type { KeyboardLayout } from '@/lib/types'

const layout = computed<KeyboardLayout>({
  get: () => config.value!.device.keyboardLayout || 'auto',
  set: (v) => (config.value!.device.keyboardLayout = v),
})
const detected = computed(() => live.status?.keyboard?.detected)
const options = computed<{ value: KeyboardLayout; label: string; hint?: string }[]>(() => [
  {
    value: 'auto',
    label: detected.value ? `Automatic (detected: ${layoutName(detected.value)})` : 'Automatic',
    hint: detected.value ? undefined : daemonUp.value ? 'not detected, types as US' : 'needs the helper running',
  },
  ...LAYOUTS.map((l) => ({ value: l.id as KeyboardLayout, label: l.name, hint: l.xkb })),
])
</script>

<template>
  <div class="step">
    <SetupHero icon="keyboard" kind="flower" />
    <h1>Your keyboard layout</h1>
    <p class="lead">
      The board presses keys like a keyboard, so it needs your layout for shortcuts and text to come out right.
      <template v-if="detected">Automatic follows the layout you use on this PC, also when you switch.</template>
    </p>
    <div class="pick">
      <M3Select v-model="layout" :options="options" label="Keyboard layout" />
    </div>
  </div>
</template>

<style scoped>
.pick {
  width: min(360px, 100%);
  justify-self: center;
  text-align: left;
}
</style>
