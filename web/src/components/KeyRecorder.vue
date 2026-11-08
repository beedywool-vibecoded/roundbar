<script setup lang="ts">
// Key combo in the spec syntax. Record captures the next key press; the modifier chips and
// the key menu cover what the browser can't catch (Super, Ctrl+W and the like).
import { computed, onBeforeUnmount, ref } from 'vue'
import Icon from './Icon.vue'
import M3Select from './M3Select.vue'
import { KEY_NAMES, MODIFIERS, comboError, comboFromEvent, formatCombo, heldMods, modifierOf, parseCombo, prettyKey, type Modifier } from '@/lib/keys'

const model = defineModel<string>({ required: true })
defineProps<{ label?: string }>()

const combo = computed(() => parseCombo(model.value))
const error = computed(() => comboError(model.value))
const recording = ref(false)
const held = ref<Modifier[]>([])
const box = ref<HTMLElement>()

const keyOptions = [
  { value: '', label: 'None', hint: 'One modifier alone, like Super for the overview' },
  ...KEY_NAMES.map((k) => {
    const p = prettyKey(k)
    return { value: k, label: p.toLowerCase() === k ? p : `${p}  ·  ${k}` }
  }),
]
const key = computed({
  get: () => combo.value.key,
  set: (k: string) => (model.value = formatCombo({ ...combo.value, key: k })),
})

function toggleMod(m: Modifier) {
  const mods = combo.value.mods.includes(m) ? combo.value.mods.filter((x) => x !== m) : [...combo.value.mods, m]
  model.value = formatCombo({ mods, key: combo.value.key })
}

// A modifier pressed and released on its own records just that modifier.
let lone: Modifier | null = null
function onKey(e: KeyboardEvent) {
  e.preventDefault()
  e.stopPropagation()
  held.value = heldMods(e)
  const m = modifierOf(e)
  if (m) {
    lone = held.value.length === 1 ? m : null
    return
  }
  lone = null
  const c = comboFromEvent(e)
  if (!c) return
  model.value = formatCombo(c)
  stop()
}
function onKeyUp(e: KeyboardEvent) {
  e.preventDefault()
  e.stopPropagation()
  held.value = heldMods(e)
  if (lone && modifierOf(e) === lone && !held.value.length) {
    model.value = lone
    stop()
  }
  lone = null
}
function start() {
  recording.value = true
  lone = null
  held.value = []
  window.addEventListener('keydown', onKey, true)
  window.addEventListener('keyup', onKeyUp, true)
  window.addEventListener('blur', stop)
  box.value?.focus()
}
function stop() {
  recording.value = false
  window.removeEventListener('keydown', onKey, true)
  window.removeEventListener('keyup', onKeyUp, true)
  window.removeEventListener('blur', stop)
}
onBeforeUnmount(stop)
</script>

<template>
  <div class="kr">
    <div ref="box" class="caps" :class="{ recording, invalid: !!error && !recording }" tabindex="-1">
      <template v-if="recording">
        <span v-for="m in held" :key="m" class="cap">{{ prettyKey(m) }}</span>
        <span class="hint">{{ held.length ? '+ a key, or release for the modifier alone…' : 'Press a key combo…' }}</span>
      </template>
      <template v-else-if="model">
        <template v-for="(m, i) in MODIFIERS.filter((x) => combo.mods.includes(x))" :key="m">
          <span v-if="i > 0" class="plus">+</span>
          <span class="cap">{{ prettyKey(m) }}</span>
        </template>
        <span v-if="combo.key && combo.mods.length" class="plus">+</span>
        <span v-if="combo.key" class="cap">{{ prettyKey(combo.key) }}</span>
        <code class="raw">{{ model }}</code>
      </template>
      <span v-else class="hint">No key yet</span>
      <span class="spacer" />
      <button v-if="!recording" class="btn btn-tonal st rec" type="button" @click="start">
        <Icon name="radio_button_checked" :size="18" /> Record
      </button>
      <button v-else class="btn btn-text st rec" type="button" @click="stop">Cancel</button>
    </div>
    <p v-if="error && !recording" class="tf-help err">{{ error }}</p>
    <div class="manual">
      <div class="chips" role="group" aria-label="Modifiers">
        <button
          v-for="m in MODIFIERS"
          :key="m"
          type="button"
          class="fchip st"
          :class="{ on: combo.mods.includes(m) }"
          :aria-pressed="combo.mods.includes(m)"
          @click="toggleMod(m)"
        >
          <Icon v-if="combo.mods.includes(m)" name="check" :size="16" />
          {{ prettyKey(m) }}
        </button>
      </div>
      <M3Select v-model="key" class="key-select" :options="keyOptions" label="Key" />
    </div>
  </div>
</template>

<style scoped>
.kr {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.caps {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-height: 56px;
  padding: 8px 8px 8px 14px;
  border-radius: 12px 12px 4px 4px;
  border-bottom: 1px solid var(--on-sv);
  background: var(--schh);
  outline: none;
}
.caps.recording {
  border-bottom-color: var(--primary);
  box-shadow: inset 0 -1px 0 var(--primary);
  background: color-mix(in srgb, var(--primary) 10%, var(--schh));
}
.caps.invalid {
  border-bottom-color: var(--error);
}
.cap {
  display: inline-grid;
  place-items: center;
  min-width: 34px;
  height: 32px;
  padding: 0 10px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: inset 0 -2px 0 var(--outline-v);
  border: 1px solid var(--outline-v);
  font-size: 13px;
  font-weight: 600;
}
.plus {
  color: var(--on-sv);
  font-size: 13px;
}
.raw {
  margin-left: 6px;
  font-size: 12px;
}
.hint {
  color: var(--on-sv);
  font-size: 14px;
}
.recording .hint {
  color: var(--primary);
  animation: blink 1.2s infinite;
}
@keyframes blink {
  50% {
    opacity: 0.5;
  }
}
.spacer {
  flex: 1;
}
.rec {
  height: 36px;
  padding: 0 14px;
}
.manual {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
}
.key-select {
  flex: 1 1 160px;
  min-width: 160px;
}
</style>
