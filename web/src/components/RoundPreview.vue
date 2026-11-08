<script setup lang="ts">
// The 360 x 360 round screen, drawn with the device palette the way the firmware lays
// pages out. Buttons can be clicked to select them for editing.
import { computed } from 'vue'
import { shapePath } from '@/lib/shapes'
import { arc, polar, wavyArc } from '@/lib/geometry'
import { STAT_ITEMS } from '@/lib/model'
import type { Button, ColorRole, Page, Palette } from '@/lib/types'

const props = withDefaults(
  defineProps<{
    page: Page
    palette: Palette
    selected?: string | null
    editable?: boolean
    pageIndex?: number
    pageCount?: number
  }>(),
  { selected: null, editable: false, pageIndex: 0, pageCount: 1 },
)
const emit = defineEmits<{ select: [id: string]; addCenter: [] }>()

const C = 180
const uid = `screen-${Math.random().toString(36).slice(2, 9)}`
const p = computed(() => props.palette)

function pair(role: ColorRole): [string, string] {
  const q = p.value
  switch (role) {
    case 'primary':
      return [q.primaryContainer, q.onPrimaryContainer]
    case 'secondary':
      return [q.secondaryContainer, q.onSecondaryContainer]
    case 'tertiary':
      return [q.tertiaryContainer, q.onTertiaryContainer]
    case 'error':
      return [q.errorContainer, q.onErrorContainer]
    case 'surface':
      return [q.surfaceContainerHighest, q.onSurface]
  }
}

interface Placed {
  b: Button
  x: number
  y: number
  size: number
  center: boolean
}

const radial = computed(() => {
  const pg = props.page
  if (pg.type !== 'radial') return null
  const n = pg.buttons.length
  const hasCenter = !!pg.center
  const ring = n <= 4 ? 112 : 124
  const size = Math.min(n <= 4 ? 96 : 84, ((2 * Math.PI * ring) / Math.max(n, 1)) * 0.86)
  const placed: Placed[] = pg.buttons.map((b, i) => {
    const [x, y] = polar(C, C, ring, (360 / n) * i)
    return { b, x, y, size, center: false }
  })
  if (pg.center) placed.push({ b: pg.center, x: C, y: C, size: 108, center: true })
  return { placed, hasCenter, ring }
})

const selectedPlaced = computed(() => radial.value?.placed.find((x) => x.b.id === props.selected) ?? null)
const labelPos = computed(() => {
  const s = selectedPlaced.value
  if (!s) return null
  // Under the button, or above it in the lower part of the circle
  const below = s.center || s.y < C + 60
  return { x: s.x, y: below ? s.y + s.size / 2 + 6 : s.y - s.size / 2 - 6 }
})

// Sample values so the gauges read like the real thing
const SAMPLE: Record<string, number> = { cpu: 23, cpuTemp: 54, ram: 61, gpu: 12, gpuTemp: 47, vram: 35 }

const gauges = computed(() => {
  const pg = props.page
  if (pg.type !== 'stats') return []
  const n = pg.items.length
  const layouts: Record<number, { x: number; y: number; r: number }[]> = {
    1: [{ x: C, y: C, r: 110 }],
    2: [
      { x: 108, y: C, r: 62 },
      { x: 252, y: C, r: 62 },
    ],
    3: [
      { x: C, y: 106, r: 54 },
      { x: 108, y: 238, r: 54 },
      { x: 252, y: 238, r: 54 },
    ],
    4: [
      { x: 116, y: 116, r: 50 },
      { x: 244, y: 116, r: 50 },
      { x: 116, y: 244, r: 50 },
      { x: 244, y: 244, r: 50 },
    ],
  }
  const slots = layouts[Math.min(4, Math.max(1, n))] ?? []
  return pg.items.slice(0, 4).map((item, i) => {
    const info = STAT_ITEMS.find((s) => s.value === item)!
    const v = SAMPLE[item] ?? 0
    const slot = slots[i]!
    return { ...slot, info, v, frac: Math.min(1, v / 100), color: i % 2 ? p.value.tertiary : p.value.primary }
  })
})

const dots = computed(() => {
  const n = props.pageCount
  if (n <= 1) return []
  const span = (n - 1) * 5
  return Array.from({ length: n }, (_, i) => {
    const [x, y] = polar(C, C, 168, 180 + span / 2 - i * 5)
    return { x, y, on: i === props.pageIndex }
  })
})

const iconStyle = (fill = 1) => ({ fontVariationSettings: `'FILL' ${fill}` })

function pick(id: string) {
  if (props.editable) emit('select', id)
}
</script>

<template>
  <svg class="round" viewBox="0 0 360 360" role="img" :aria-label="`Preview of the ${page.type} page`">
    <defs>
      <clipPath :id="uid"><circle cx="180" cy="180" r="180" /></clipPath>
    </defs>
    <g :clip-path="`url(#${uid})`">
      <rect width="360" height="360" :fill="p.surface" />

      <!-- Buttons on a ring -->
      <template v-if="radial">
        <circle v-if="radial.placed.length > 1" cx="180" cy="180" :r="radial.ring" fill="none" :stroke="p.surfaceContainerHigh" stroke-width="1.5" />
        <g
          v-for="pl in radial.placed"
          :key="pl.b.id"
          class="btn-g"
          :class="{ editable, sel: pl.b.id === selected }"
          :role="editable ? 'button' : undefined"
          :tabindex="editable ? 0 : undefined"
          :aria-label="editable ? `Edit ${pl.b.label || pl.b.icon}` : undefined"
          :style="{ transformOrigin: `${pl.x}px ${pl.y}px` }"
          @click="pick(pl.b.id)"
          @keydown.enter.prevent="pick(pl.b.id)"
          @keydown.space.prevent="pick(pl.b.id)"
        >
          <path
            v-if="pl.b.id === selected && editable"
            :d="shapePath(pl.center ? 'sunny' : 'cookie', pl.x, pl.y, pl.size / 2 + 7)"
            fill="none"
            :stroke="p.primary"
            stroke-width="3"
          />
          <path :d="shapePath(pl.center ? 'sunny' : 'cookie', pl.x, pl.y, pl.size / 2 - 3)" :fill="pair(pl.b.color)[0]" />
          <text
            class="sym"
            :x="pl.x"
            :y="pl.y"
            :font-size="pl.center ? 52 : Math.round(pl.size * 0.44)"
            :fill="pair(pl.b.color)[1]"
            :style="iconStyle()"
            >{{ pl.b.icon }}</text
          >
        </g>
        <g v-if="editable && !radial.hasCenter" class="ghost" role="button" tabindex="0" aria-label="Add a center button" @click="emit('addCenter')" @keydown.enter.prevent="emit('addCenter')">
          <circle cx="180" cy="180" r="44" fill="transparent" :stroke="p.outlineVariant" stroke-width="2" stroke-dasharray="6 6" />
          <text class="sym" x="180" y="180" font-size="30" :fill="p.onSurfaceVariant" :style="iconStyle(0)">add</text>
        </g>
        <g v-if="labelPos && selectedPlaced?.b.label" class="label">
          <rect
            :x="labelPos.x - (selectedPlaced.b.label.length * 7 + 20) / 2"
            :y="labelPos.y - 11"
            :width="selectedPlaced.b.label.length * 7 + 20"
            height="22"
            rx="11"
            :fill="p.surfaceContainerHighest"
          />
          <text :x="labelPos.x" :y="labelPos.y + 1" font-size="13" font-weight="600" :fill="p.onSurface" text-anchor="middle" dominant-baseline="central">
            {{ selectedPlaced.b.label }}
          </text>
        </g>
      </template>

      <!-- Media: now playing, transport, volume on the edge -->
      <template v-else-if="page.type === 'media'">
        <path :d="arc(180, 180, 166, 210, 510)" fill="none" :stroke="p.surfaceContainerHighest" stroke-width="10" stroke-linecap="round" />
        <path :d="wavyArc(180, 180, 166, 210, 210 + 300 * 0.62, 2, 28)" fill="none" :stroke="p.primary" stroke-width="8" stroke-linecap="round" />
        <circle v-bind="{ cx: polar(180, 180, 166, 210 + 300 * 0.62)[0], cy: polar(180, 180, 166, 210 + 300 * 0.62)[1] }" r="9" :fill="p.primary" />
        <path :d="shapePath('clover', 180, 92, 34)" :fill="p.tertiaryContainer" />
        <text class="sym" x="180" y="92" font-size="34" :fill="p.onTertiaryContainer" :style="iconStyle()">music_note</text>
        <text x="180" y="152" text-anchor="middle" font-size="22" font-weight="600" :fill="p.onSurface">Song title</text>
        <text x="180" y="178" text-anchor="middle" font-size="15" :fill="p.onSurfaceVariant">Artist name</text>
        <path :d="shapePath('cookie', 106, 240, 30)" :fill="p.surfaceContainerHighest" />
        <text class="sym" x="106" y="240" font-size="30" :fill="p.onSurface" :style="iconStyle()">skip_previous</text>
        <path :d="shapePath('sunny', 180, 240, 44)" :fill="p.primaryContainer" />
        <text class="sym" x="180" y="240" font-size="44" :fill="p.onPrimaryContainer" :style="iconStyle()">pause</text>
        <path :d="shapePath('cookie', 254, 240, 30)" :fill="p.surfaceContainerHighest" />
        <text class="sym" x="254" y="240" font-size="30" :fill="p.onSurface" :style="iconStyle()">skip_next</text>
        <text class="sym" x="180" y="306" font-size="20" :fill="p.onSurfaceVariant" :style="iconStyle()">volume_up</text>
      </template>

      <!-- Stats: M3 expressive gauges -->
      <template v-else-if="page.type === 'stats'">
        <g v-for="g in gauges" :key="g.info.value">
          <path :d="arc(g.x, g.y, g.r, 225, 495)" fill="none" :stroke="p.surfaceContainerHighest" :stroke-width="g.r > 80 ? 14 : 9" stroke-linecap="round" />
          <path
            :d="wavyArc(g.x, g.y, g.r, 225, 225 + 270 * g.frac, g.r > 80 ? 2.6 : 1.8, g.r > 80 ? 34 : 22)"
            fill="none"
            :stroke="g.color"
            :stroke-width="g.r > 80 ? 12 : 8"
            stroke-linecap="round"
          />
          <text class="sym" :x="g.x" :y="g.y - g.r * 0.32" :font-size="g.r * 0.36" :fill="p.onSurfaceVariant" :style="iconStyle()">{{ g.info.icon }}</text>
          <text :x="g.x" :y="g.y + g.r * 0.12" text-anchor="middle" dominant-baseline="central" :font-size="g.r * 0.46" font-weight="600" :fill="p.onSurface">
            {{ g.v }}<tspan :font-size="g.r * 0.24" :fill="p.onSurfaceVariant">{{ g.info.unit }}</tspan>
          </text>
          <text :x="g.x" :y="g.y + g.r * 0.62" text-anchor="middle" dominant-baseline="central" :font-size="Math.max(11, g.r * 0.22)" :fill="p.onSurfaceVariant">{{ g.info.label }}</text>
        </g>
      </template>

      <!-- Dial: one big arc along the edge -->
      <template v-else-if="page.type === 'dial'">
        <path :d="arc(180, 180, 156, 225, 495)" fill="none" :stroke="p.surfaceContainerHighest" stroke-width="20" stroke-linecap="round" />
        <path :d="wavyArc(180, 180, 156, 225, 225 + 270 * 0.65, 3, 40)" fill="none" :stroke="p.primary" stroke-width="16" stroke-linecap="round" />
        <circle v-bind="{ cx: polar(180, 180, 156, 225 + 270 * 0.65)[0], cy: polar(180, 180, 156, 225 + 270 * 0.65)[1] }" r="15" :fill="p.primary" />
        <circle v-bind="{ cx: polar(180, 180, 156, 225 + 270 * 0.65)[0], cy: polar(180, 180, 156, 225 + 270 * 0.65)[1] }" r="6" :fill="p.onPrimary" />
        <path :d="shapePath('flower', 180, 128, 38)" :fill="p.primaryContainer" />
        <text class="sym" x="180" y="128" font-size="40" :fill="p.onPrimaryContainer" :style="iconStyle()">{{ page.icon }}</text>
        <text x="180" y="210" text-anchor="middle" dominant-baseline="central" font-size="64" font-weight="600" :fill="p.onSurface">
          {{ page.source === 'custom' ? '±' : '65' }}
        </text>
        <text x="180" y="262" text-anchor="middle" font-size="18" :fill="p.onSurfaceVariant">{{ page.label }}</text>
      </template>

      <circle v-for="(d, i) in dots" :key="i" :cx="d.x" :cy="d.y" :r="d.on ? 4 : 3" :fill="d.on ? p.onSurface : p.outlineVariant" />
    </g>
    <circle cx="180" cy="180" r="179" fill="none" :stroke="p.outlineVariant" stroke-width="2" class="bezel" />
  </svg>
</template>

<style scoped>
.round {
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 1;
  border-radius: 50%;
  box-shadow:
    0 0 0 10px #111,
    0 0 0 11px #2a2a2a,
    0 18px 40px rgba(0, 0, 0, 0.45);
  font-family: var(--font);
  user-select: none;
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  text-anchor: middle;
  dominant-baseline: central;
}
.btn-g {
  transition: transform 0.35s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.btn-g.editable {
  cursor: pointer;
}
.btn-g.editable:hover {
  transform: scale(1.06);
}
.btn-g.sel {
  transform: scale(1.04);
}
.btn-g:focus-visible {
  outline: none;
}
.btn-g:focus-visible path:last-of-type {
  stroke: var(--primary);
  stroke-width: 3;
}
.ghost {
  cursor: pointer;
  opacity: 0.75;
  transition: opacity 0.2s;
}
.ghost:hover {
  opacity: 1;
}
.ghost:focus-visible {
  outline: none;
}
.bezel {
  pointer-events: none;
}
</style>
