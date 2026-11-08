<script setup lang="ts">
// A `clock` page: analog or digital face, the weather line and up to 3 world clocks.
// Defaults are stored as missing keys.
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from './Icon.vue'
import M3Dialog from './M3Dialog.vue'
import M3Switch from './M3Switch.vue'
import TextField from './TextField.vue'
import { CLOCK_DEFAULTS, LIMITS, timeZones, zoneCity } from '@/lib/model'
import { config } from '@/lib/store'
import type { ClockPage, ClockStyle, ClockZone } from '@/lib/types'

const props = defineProps<{ page: ClockPage }>()

const STYLES: { id: ClockStyle; label: string }[] = [
  { id: 'analog', label: 'Analog' },
  { id: 'digital', label: 'Digital' },
]
const style = computed({
  get: () => props.page.style ?? CLOCK_DEFAULTS.style,
  set: (v: ClockStyle) => {
    if (v === CLOCK_DEFAULTS.style) delete props.page.style
    else props.page.style = v
  },
})
const showWeather = computed({
  get: () => props.page.weather ?? CLOCK_DEFAULTS.weather,
  set: (v: boolean) => {
    if (v === CLOCK_DEFAULTS.weather) delete props.page.weather
    else props.page.weather = v
  },
})
const city = computed(() => config.value!.weather?.city)

// ---- world clocks ----
const zones = computed<ClockZone[]>(() => props.page.zones ?? [])
function setZones(z: ClockZone[]) {
  if (z.length) props.page.zones = z
  else delete props.page.zones
}
function setLabel(i: number, label: string) {
  setZones(zones.value.map((z, j) => (j === i ? { ...z, label } : z)))
}
function move(i: number, d: number) {
  const z = [...zones.value]
  const j = i + d
  if (j < 0 || j >= z.length) return
  ;[z[i], z[j]] = [z[j]!, z[i]!]
  setZones(z)
}
function remove(i: number) {
  setZones(zones.value.filter((_, j) => j !== i))
}
const labelError = (l: string) => (!l.trim() ? 'Give it a name' : l.length > LIMITS.zoneLabel ? `At most ${LIMITS.zoneLabel} characters` : null)

// The current time, refreshed every 15 s, for the list and the picker
const now = ref(new Date())
const tick = setInterval(() => (now.value = new Date()), 15_000)
onBeforeUnmount(() => clearInterval(tick))
const fmts = new Map<string, Intl.DateTimeFormat>()
function timeIn(tz: string): string {
  let f = fmts.get(tz)
  if (!f) {
    try {
      f = new Intl.DateTimeFormat(undefined, { timeZone: tz, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
    } catch {
      return '--:--'
    }
    fmts.set(tz, f)
  }
  return f.format(now.value)
}

// ---- time zone picker ----
const picking = ref(false)
const q = ref('')
const search = ref<HTMLInputElement>()
const allZones = timeZones()
const found = computed(() => {
  const words = q.value.toLowerCase().trim().split(/[\s_/]+/).filter(Boolean)
  const taken = new Set(zones.value.map((z) => z.tz))
  const list = allZones.filter((tz) => !taken.has(tz))
  if (!words.length) return list
  return list.filter((tz) => {
    const hay = tz.toLowerCase().replace(/_/g, ' ')
    return words.every((w) => hay.includes(w))
  })
})
watch(picking, async (v) => {
  if (!v) return
  q.value = ''
  now.value = new Date()
  await nextTick()
  search.value?.focus()
})
function add(tz: string) {
  if (zones.value.length >= LIMITS.zones) return
  setZones([...zones.value, { label: zoneCity(tz).slice(0, LIMITS.zoneLabel), tz }])
  picking.value = false
}
const region = (tz: string) => tz.split('/').slice(0, -1).join(' / ').replace(/_/g, ' ')
</script>

<template>
  <div class="card">
    <div class="card-head"><h2>Clock</h2></div>
    <div class="setting">
      <div class="grow">
        <div>Style</div>
        <div class="muted small">{{ style === 'analog' ? 'A dial with hands' : 'Large numbers with the date' }}</div>
      </div>
      <div class="seg" role="radiogroup" aria-label="Clock style">
        <button v-for="s in STYLES" :key="s.id" type="button" role="radio" class="st" :class="{ on: style === s.id }" :aria-checked="style === s.id" @click="style = s.id">
          <Icon v-if="style === s.id" name="check" :size="18" />
          {{ s.label }}
        </button>
      </div>
    </div>
    <div class="setting">
      <div class="grow">
        <div>Show weather</div>
        <div v-if="city" class="muted small">Current conditions in {{ city }}</div>
        <div v-else class="muted small">
          No city set yet. <RouterLink to="/appearance#weather">Pick one in Appearance, Weather</RouterLink>
        </div>
      </div>
      <M3Switch v-model="showWeather" label="Show weather" />
    </div>

    <div class="setting col">
      <div class="row-head">
        <div>
          <div>World clocks</div>
          <div class="muted small">Small rows under the time, {{ zones.length }} / {{ LIMITS.zones }}</div>
        </div>
        <button class="btn btn-tonal st" type="button" :disabled="zones.length >= LIMITS.zones" @click="picking = true">
          <Icon name="add" :size="18" /> Add
        </button>
      </div>
      <ol v-if="zones.length" class="zones">
        <li v-for="(z, i) in zones" :key="z.tz" class="zone">
          <span class="time">{{ timeIn(z.tz) }}</span>
          <div class="grow zone-body">
            <TextField :model-value="z.label" label="Label" :max="LIMITS.zoneLabel" :error="labelError(z.label)" @update:model-value="setLabel(i, $event)" />
            <code class="plain muted small">{{ z.tz }}</code>
          </div>
          <div class="tools">
            <button class="icon-btn st" type="button" :disabled="i === 0" v-tip="'Move up'" @click="move(i, -1)"><Icon name="arrow_upward" :size="18" /></button>
            <button class="icon-btn st" type="button" :disabled="i === zones.length - 1" v-tip="'Move down'" @click="move(i, 1)"><Icon name="arrow_downward" :size="18" /></button>
            <button class="icon-btn st" type="button" v-tip="'Remove'" @click="remove(i)"><Icon name="delete" :size="18" /></button>
          </div>
        </li>
      </ol>
    </div>

    <M3Dialog v-model="picking" title="Add a world clock">
      <label class="field search">
        <Icon name="search" />
        <input ref="search" v-model="q" placeholder="Search a city or region" aria-label="Search time zones" />
        <span class="muted small">{{ found.length }}</span>
      </label>
      <ul class="tz-list" aria-label="Time zones">
        <li v-for="tz in found" :key="tz">
          <button type="button" class="tz st" @click="add(tz)">
            <span class="grow">
              <span class="tz-city">{{ zoneCity(tz) }}</span>
              <span class="muted small">{{ region(tz) || tz }}</span>
            </span>
            <span class="tz-time">{{ timeIn(tz) }}</span>
          </button>
        </li>
        <li v-if="!found.length" class="muted" style="padding: 12px">No time zone matches "{{ q }}".</li>
      </ul>
    </M3Dialog>
  </div>
</template>

<style scoped>
.row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.zones {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.zone {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px;
  border-radius: 16px;
  background: var(--sch);
}
.time {
  display: grid;
  place-items: center;
  min-width: 64px;
  height: 56px;
  border-radius: 14px;
  background: var(--secc);
  color: var(--on-secc);
  font-size: 18px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.zone-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1 1 140px;
}
.zone-body code {
  padding-left: 4px;
}
.tools {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.tools .icon-btn {
  width: 32px;
  height: 32px;
  border: none;
}
.tz-list {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
  max-height: min(56vh, 480px);
  overflow-y: auto;
}
.tz {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 14px;
  border: none;
  border-radius: 14px;
  background: transparent;
  color: var(--on-surface);
  text-align: left;
}
.tz-city {
  display: block;
  font-size: 15px;
}
.tz .muted {
  display: block;
}
.tz-time {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  color: var(--on-sv);
}
</style>
