<script setup lang="ts">
// Seed color: the preset swatches, a custom color picker and a hex field applied once valid.
import { computed, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { SEEDS, validHex } from '@/lib/theme'

const model = defineModel<string>({ required: true })
defineProps<{ label: string }>()

const isPreset = computed(() => SEEDS.some((s) => s.hex.toLowerCase() === model.value.toLowerCase()))
const same = (a: string) => a.toLowerCase() === model.value.toLowerCase()

const hex = ref(model.value)
watch(model, (s) => (hex.value = s))
watch(hex, (h) => {
  const v = h.startsWith('#') ? h : `#${h}`
  if (validHex(v) && !same(v)) model.value = v.toUpperCase()
})
function onPicker(e: Event) {
  model.value = (e.target as HTMLInputElement).value.toUpperCase()
}
</script>

<template>
  <div class="seed-picker">
    <div class="seeds" role="radiogroup" :aria-label="label">
      <button
        v-for="s in SEEDS"
        :key="s.hex"
        type="button"
        role="radio"
        class="seed"
        :class="{ on: same(s.hex) }"
        :aria-checked="same(s.hex)"
        :aria-label="s.name"
        v-tip="s.name"
        :style="{ background: s.hex }"
        @click="model = s.hex"
      />
      <label class="seed custom" :class="{ on: !isPreset }" v-tip="'Custom color'" :style="!isPreset ? { background: model } : undefined">
        <Icon v-if="isPreset" name="palette" :size="20" />
        <input type="color" class="sr-only" :value="model" aria-label="Custom color" @input="onPicker" />
      </label>
    </div>
    <label class="tf hex" :class="{ invalid: !validHex(hex.startsWith('#') ? hex : `#${hex}`) }">
      <span class="lbl">Hex</span>
      <span class="in">
        <span class="sw" :style="{ background: model }" />
        <input v-model="hex" maxlength="7" spellcheck="false" style="font-family: var(--mono)" />
      </span>
    </label>
  </div>
</template>

<style scoped>
.seed-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.seeds {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.seed {
  width: 44px;
  height: 44px;
  border-radius: 22px;
  border: none;
  padding: 0;
  transition: box-shadow 0.2s var(--ease);
}
.seed.on {
  box-shadow:
    0 0 0 3px var(--sc),
    0 0 0 5px var(--on-surface);
}
.seed.custom {
  display: grid;
  place-items: center;
  cursor: pointer;
  background: var(--schh);
  color: var(--on-sv);
  border: 1px dashed var(--outline);
}
.seed.custom.on {
  border: none;
}
.hex {
  max-width: 220px;
}
.sw {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  flex-shrink: 0;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--on-surface) 20%, transparent);
}
</style>
