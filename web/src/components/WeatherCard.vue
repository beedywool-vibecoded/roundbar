<script setup lang="ts">
// Location for clock pages: a city search on Open-Meteo's geocoding API (called from the
// browser, no key), the units, and a way to drop it. The daemon fetches the forecast.
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from './Icon.vue'
import ShapeLoader from './ShapeLoader.vue'
import { config } from '@/lib/store'
import type { WeatherUnits } from '@/lib/types'

interface Place {
  id: number
  name: string
  latitude: number
  longitude: number
  country?: string
  admin1?: string
}

const weather = computed(() => config.value!.weather)
const q = ref('')
const results = ref<Place[]>([])
const searching = ref(false)
const error = ref('')
const searched = ref('')

let timer: ReturnType<typeof setTimeout> | undefined
let ctl: AbortController | null = null
watch(q, (v) => {
  clearTimeout(timer)
  const term = v.trim()
  if (term.length < 2) {
    ctl?.abort()
    results.value = []
    searching.value = false
    error.value = ''
    searched.value = ''
    return
  }
  timer = setTimeout(() => search(term), 350)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  ctl?.abort()
})

async function search(term: string) {
  ctl?.abort()
  ctl = new AbortController()
  const mine = ctl
  searching.value = true
  error.value = ''
  try {
    const url = `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(term)}&count=5&language=en&format=json`
    const res = await fetch(url, { signal: mine.signal })
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
    const data = (await res.json()) as { results?: Place[] }
    if (mine !== ctl) return
    results.value = (data.results ?? []).filter((p) => typeof p.latitude === 'number' && typeof p.longitude === 'number')
    searched.value = term
  } catch (e) {
    if (mine !== ctl || (e instanceof DOMException && e.name === 'AbortError')) return
    results.value = []
    error.value = 'The city search is not reachable. Check the internet connection of this PC.'
  } finally {
    if (mine === ctl) searching.value = false
  }
}

const round = (v: number) => Math.round(v * 100) / 100
function pick(p: Place) {
  config.value!.weather = {
    city: p.name,
    lat: round(p.latitude),
    lon: round(p.longitude),
    units: weather.value?.units ?? 'metric',
  }
  q.value = ''
}
function remove() {
  delete config.value!.weather
}
const where = (p: Place) => [p.admin1, p.country].filter(Boolean).join(', ')

const UNITS: { id: WeatherUnits; label: string }[] = [
  { id: 'metric', label: '°C' },
  { id: 'imperial', label: '°F' },
]
</script>

<template>
  <section id="weather" class="card">
    <div class="card-head"><h2>Weather</h2></div>
    <div class="form">
      <p class="muted small" style="margin: 0">For clock pages. The daemon gets the forecast from Open-Meteo for this place; nothing else leaves your PC.</p>

      <div v-if="weather" class="current">
        <span class="badge"><Icon name="partly_cloudy_day" :size="24" fill /></span>
        <div class="grow">
          <div class="city">{{ weather.city }}</div>
          <div class="muted small">{{ weather.lat.toFixed(2) }}, {{ weather.lon.toFixed(2) }}</div>
        </div>
        <div class="seg" role="radiogroup" aria-label="Units">
          <button
            v-for="u in UNITS"
            :key="u.id"
            type="button"
            role="radio"
            class="st"
            :class="{ on: weather.units === u.id }"
            :aria-checked="weather.units === u.id"
            v-tip="u.id === 'metric' ? 'Metric' : 'Imperial'"
            @click="weather.units = u.id"
          >
            <Icon v-if="weather.units === u.id" name="check" :size="18" />
            {{ u.label }}
          </button>
        </div>
        <button class="btn btn-text st" type="button" @click="remove"><Icon name="delete" :size="18" /> Remove</button>
      </div>

      <div class="search-box">
        <label class="field search">
          <Icon name="search" />
          <input v-model="q" :placeholder="weather ? 'Search another city' : 'Search a city'" aria-label="Search a city" autocomplete="off" spellcheck="false" />
          <ShapeLoader v-if="searching" small label="Searching" />
          <button v-else-if="q" class="clear" type="button" aria-label="Clear" @click="q = ''"><Icon name="close" :size="20" /></button>
        </label>
        <ul v-if="results.length" class="results" aria-label="Cities">
          <li v-for="p in results" :key="p.id">
            <button type="button" class="result st" @click="pick(p)">
              <Icon name="location_on" :size="20" />
              <span class="grow">
                <span class="name">{{ p.name }}</span>
                <span class="muted small">{{ where(p) }}</span>
              </span>
            </button>
          </li>
        </ul>
        <p v-else-if="error" class="tf-help err" style="margin: 0">{{ error }}</p>
        <p v-else-if="searched && !searching && q.trim() === searched" class="tf-help" style="margin: 0">No city named "{{ searched }}".</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.current {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px 14px;
  padding: 12px 12px 12px 14px;
  border-radius: 20px;
  background: var(--sch);
}
.current .grow {
  flex: 1 1 120px;
}
.badge {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 22px;
  background: var(--tc);
  color: var(--on-tc);
}
.city {
  font-size: 16px;
  font-weight: 500;
}
.search-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.clear {
  display: grid;
  place-items: center;
  border: none;
  background: none;
  color: var(--on-sv);
  padding: 0;
}
.results {
  list-style: none;
  margin: 0;
  padding: 6px;
  border-radius: 20px;
  background: var(--sch);
}
.result {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-radius: 14px;
  background: transparent;
  color: var(--on-surface);
  text-align: left;
}
.result .muted {
  display: block;
}
.name {
  display: block;
  font-size: 15px;
}
</style>
