<script setup lang="ts">
// App ids / window classes a profile matches, as input chips. The focused and recent apps
// the daemon has seen are offered by app id; an entry matches a window's id or its class.
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { live } from '@/lib/store'
import type { App } from '@/lib/types'

const model = defineModel<string[]>({ required: true })
const props = defineProps<{ takenBy: (id: string) => string | null }>()
const draft = ref('')

const suggestions = computed(() => {
  const seen = new Set(model.value)
  const out: (App & { focused: boolean })[] = []
  const add = (a: App | null | undefined, focused: boolean) => {
    if (!a?.id) return
    const id = a.id.toLowerCase()
    if (seen.has(id)) return
    seen.add(id)
    out.push({ id, name: a.name, class: a.class, focused })
  }
  add(live.status?.focus, true)
  for (const a of live.status?.recentApps ?? []) add(a, false)
  return out
})

function add(v: string) {
  const id = v.trim().toLowerCase()
  if (!id || model.value.includes(id)) return
  model.value = [...model.value, id]
}
function commit() {
  for (const part of draft.value.split(/[\s,]+/)) add(part)
  draft.value = ''
}
function remove(id: string) {
  model.value = model.value.filter((m) => m !== id)
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
    e.preventDefault()
    commit()
  } else if (e.key === 'Backspace' && !draft.value && model.value.length) {
    model.value = model.value.slice(0, -1)
  }
}
</script>

<template>
  <div class="mc">
    <label class="tf box">
      <span class="lbl">Apps (id or window class)</span>
      <span class="in wrap">
        <span v-for="m in model" :key="m" class="fchip on chip-in">
          {{ m }}
          <span v-if="props.takenBy(m)" class="also" v-tip="`Also matched by ${props.takenBy(m)}, which comes first`"><Icon name="warning" :size="14" /></span>
          <button type="button" class="x" :aria-label="`Remove ${m}`" @click.prevent="remove(m)"><Icon name="close" :size="16" /></button>
        </span>
        <input v-model="draft" placeholder="Type a class and press Enter" @keydown="onKey" @blur="commit" />
      </span>
    </label>
    <div v-if="suggestions.length" class="sugg">
      <span class="muted small">Seen recently</span>
      <div class="chips">
        <button v-for="s in suggestions" :key="s.id" type="button" class="fchip st" :class="{ focusedapp: s.focused }" v-tip="s.class && s.class !== s.id ? `${s.name} · class ${s.class}` : s.name" @click="add(s.id)">
          <Icon :name="s.focused ? 'desktop_windows' : 'add'" :size="16" />
          {{ s.id }}
          <span v-if="s.focused" class="tag">focused</span>
        </button>
      </div>
    </div>
    <p v-else class="muted small" style="margin: 0">
      Apps you focus show up here as suggestions while the daemon and the GNOME extension run.
    </p>
  </div>
</template>

<style scoped>
.mc {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.wrap {
  flex-wrap: wrap;
  padding: 4px 0;
}
.wrap input {
  flex: 1 1 160px;
  min-height: 32px;
}
.chip-in {
  padding-right: 4px;
}
.also {
  display: inline-grid;
  color: var(--warn);
}
.sugg {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.focusedapp {
  border-color: var(--primary);
  color: var(--primary);
}
.tag {
  font-size: 11px;
  opacity: 0.8;
}
</style>
