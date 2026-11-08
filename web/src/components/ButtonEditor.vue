<script setup lang="ts">
// Edits one radial button: icon, label, color role, tap and optional hold action.
import { computed } from 'vue'
import Icon from './Icon.vue'
import IconPicker from './IconPicker.vue'
import TextField from './TextField.vue'
import ActionEditor from './ActionEditor.vue'
import M3Switch from './M3Switch.vue'
import M3Shape from './M3Shape.vue'
import { COLOR_ROLES, LIMITS } from '@/lib/model'
import type { Action, Button, ColorRole, Page, Palette } from '@/lib/types'

const model = defineModel<Button>({ required: true })
const props = defineProps<{ pages: Page[]; palette: Palette; center: boolean; index: number; count: number }>()
const emit = defineEmits<{ remove: []; move: [d: number]; close: [] }>()

function set<K extends keyof Button>(k: K, v: Button[K]) {
  model.value = { ...model.value, [k]: v }
}
const hasHold = computed({
  get: () => !!model.value.hold,
  set: (on: boolean) => {
    const { hold: _drop, ...rest } = model.value
    model.value = on ? { ...rest, hold: { type: 'keys', keys: '' } } : rest
  },
})

function swatch(c: ColorRole): [string, string] {
  const p = props.palette
  return {
    primary: [p.primaryContainer, p.onPrimaryContainer],
    secondary: [p.secondaryContainer, p.onSecondaryContainer],
    tertiary: [p.tertiaryContainer, p.onTertiaryContainer],
    error: [p.errorContainer, p.onErrorContainer],
    surface: [p.surfaceContainerHighest, p.onSurface],
  }[c] as [string, string]
}
</script>

<template>
  <div class="be">
    <div class="be-head">
      <h2>{{ center ? 'Center button' : `Button ${index + 1}` }}</h2>
      <template v-if="!center">
        <button class="icon-btn st" type="button" :disabled="index === 0" v-tip="'Move back on the ring'" @click="emit('move', -1)">
          <Icon name="arrow_back" :size="20" />
        </button>
        <button class="icon-btn st" type="button" :disabled="index >= count - 1" v-tip="'Move forward on the ring'" @click="emit('move', 1)">
          <Icon name="arrow_back" :size="20" style="rotate: 180deg" />
        </button>
      </template>
      <button class="icon-btn st" type="button" :disabled="!center && count <= 1" v-tip="center ? 'Remove the center button' : count <= 1 ? 'A page needs one button' : 'Remove this button'" @click="emit('remove')">
        <Icon name="delete" :size="20" />
      </button>
      <button class="icon-btn st close" type="button" aria-label="Close" @click="emit('close')"><Icon name="close" :size="20" /></button>
    </div>

    <div class="form">
      <IconPicker :model-value="model.icon" @update:model-value="set('icon', $event)" />
      <TextField
        :model-value="model.label"
        label="Label"
        :max="LIMITS.label"
        placeholder="Shown under the button when touched"
        :error="model.label.length > LIMITS.label ? `At most ${LIMITS.label} characters` : null"
        @update:model-value="set('label', $event)"
      />
      <div>
        <p class="section-label">Color</p>
        <div class="colors" role="radiogroup" aria-label="Color">
          <button
            v-for="c in COLOR_ROLES"
            :key="c.value"
            type="button"
            role="radio"
            class="color st"
            :class="{ on: model.color === c.value }"
            :aria-checked="model.color === c.value"
            @click="set('color', c.value)"
          >
            <M3Shape kind="cookie" :size="40" :fill="swatch(c.value)[0]">
              <circle v-if="model.color === c.value" cx="50" cy="50" r="13" :fill="swatch(c.value)[1]" />
            </M3Shape>
            <span>{{ c.label }}</span>
          </button>
        </div>
      </div>

      <p class="section-label"><Icon name="touch_app" :size="18" /> Tap</p>
      <ActionEditor :model-value="model.tap" :pages="pages" @update:model-value="set('tap', $event as Action)" />

      <div class="hold-row">
        <div class="grow">
          <div>Long press</div>
          <div class="muted small">A second action when the button is held</div>
        </div>
        <M3Switch v-model="hasHold" label="Long press action" />
      </div>
      <ActionEditor v-if="model.hold" :model-value="model.hold" :pages="pages" @update:model-value="set('hold', $event as Action)" />
    </div>
  </div>
</template>

<style scoped>
.be {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.be-head {
  display: flex;
  align-items: center;
  gap: 6px;
}
.be-head h2 {
  flex: 1;
  font-size: 20px;
}
.be-head .icon-btn {
  border: none;
}
.close {
  display: none;
}
.colors {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}
.color {
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
.color svg {
  transition: rotate 0.4s var(--ease-decel);
}
.color.on {
  color: var(--on-surface);
  background: var(--sch);
}
.color.on svg {
  rotate: 40deg;
}
.section-label {
  display: flex;
  align-items: center;
  gap: 6px;
}
.hold-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--sch);
}
@media (max-width: 1100px) {
  .close {
    display: inline-grid;
  }
}
</style>
