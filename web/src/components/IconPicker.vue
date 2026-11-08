<script setup lang="ts">
// Picks one of the icons baked into the firmware (GET /api/icons), with search.
import { computed, nextTick, ref, watch } from 'vue'
import M3Dialog from './M3Dialog.vue'
import Icon from './Icon.vue'
import { icons } from '@/lib/store'

const model = defineModel<string>({ required: true })
const props = withDefaults(defineProps<{ label?: string }>(), { label: 'Icon' })

const open = ref(false)
const q = ref('')
const search = ref<HTMLInputElement>()
const known = computed(() => icons.value.includes(model.value))

const shown = computed(() => {
  const words = q.value.toLowerCase().trim().split(/[\s_]+/).filter(Boolean)
  if (!words.length) return icons.value
  return icons.value.filter((n) => words.every((w) => n.includes(w)))
})

watch(open, async (v) => {
  if (!v) return
  q.value = ''
  await nextTick()
  search.value?.focus()
  document.querySelector<HTMLElement>('.ip-grid .on')?.scrollIntoView({ block: 'center' })
})

function pick(name: string) {
  model.value = name
  open.value = false
}
const pretty = (n: string) => n.replace(/_/g, ' ')
</script>

<template>
  <button type="button" class="trigger st" :class="{ invalid: !known }" @click="open = true">
    <span class="glyph"><Icon :name="model" :size="28" fill /></span>
    <span class="txt">
      <span class="lbl">{{ props.label }}</span>
      <span class="val">{{ known ? pretty(model) : `Unknown: ${model || 'none'}` }}</span>
    </span>
    <Icon name="edit" :size="20" class="muted" />
  </button>

  <M3Dialog v-model="open" :title="`Choose ${props.label.toLowerCase()}`" wide>
    <label class="field search">
      <Icon name="search" />
      <input ref="search" v-model="q" placeholder="Search icons" aria-label="Search icons" />
      <span class="muted small">{{ shown.length }}</span>
    </label>
    <div class="ip-grid" role="radiogroup" :aria-label="props.label">
      <button
        v-for="name in shown"
        :key="name"
        type="button"
        role="radio"
        class="opt st"
        :class="{ on: model === name }"
        :aria-checked="model === name"
        :aria-label="pretty(name)"
        @click="pick(name)"
      >
        <Icon :name="name" :size="28" :fill="model === name" />
        <span class="name">{{ pretty(name) }}</span>
      </button>
      <p v-if="!shown.length" class="muted">No icon matches "{{ q }}". Only the icons built into the firmware can be used.</p>
    </div>
  </M3Dialog>
</template>

<style scoped>
.trigger {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 6px 14px 6px 8px;
  border: none;
  border-bottom: 1px solid var(--on-sv);
  border-radius: 12px 12px 4px 4px;
  background: var(--schh);
  color: var(--on-surface);
  text-align: left;
  min-width: 0;
}
.trigger.invalid {
  border-bottom-color: var(--error);
}
.glyph {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: var(--pc);
  color: var(--on-pc);
}
.txt {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.lbl {
  font-size: 12px;
  font-weight: 500;
  color: var(--primary);
}
.invalid .lbl {
  color: var(--error);
}
.val {
  font-size: 15px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ip-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
  gap: 8px;
  height: min(60vh, 560px);
  align-content: start;
  overflow-y: auto;
  padding: 2px;
}
.opt {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 84px;
  padding: 10px 6px 8px;
  border: none;
  border-radius: 16px;
  background: var(--sc);
  color: var(--on-surface);
  transition: border-radius 0.25s var(--ease), background 0.2s var(--ease);
}
.opt.on {
  background: var(--pc);
  color: var(--on-pc);
  border-radius: 28px;
}
.name {
  font-size: 11px;
  line-height: 1.2;
  color: inherit;
  opacity: 0.8;
  text-align: center;
  word-break: break-word;
}
</style>
