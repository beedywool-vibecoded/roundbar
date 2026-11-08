<script setup lang="ts">
// The 360 x 360 round screen, laid out with the firmware's numbers (firmware/main/ui.c) and
// drawn with the device palette. Coordinates are relative to the screen center, y down.
// Buttons can be clicked to select them for editing.
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useAssets } from '@/lib/assets'
import { buttonPath, type ButtonShape } from '@/lib/shapes'
import { IMAGE_DEFAULTS, MEDIA_SHAPE_DEFAULTS, defaultShape, isGif } from '@/lib/model'
import type { Button, ColorRole, Page, Palette, StatItem } from '@/lib/types'

const props = withDefaults(
  defineProps<{
    page: Page
    palette: Palette
    selected?: string | null
    editable?: boolean
    pageIndex?: number
    pageCount?: number
    /** Small, non-interactive copy (inside a link) */
    thumb?: boolean
    /** Image pages: cross-fade through the pictures (the page editor); otherwise the first one stays */
    slideshow?: boolean
  }>(),
  { selected: null, editable: false, pageIndex: 0, pageCount: 1, thumb: false, slideshow: false },
)
const emit = defineEmits<{ select: [id: string]; addCenter: [] }>()

const uid = `screen-${Math.random().toString(36).slice(2, 9)}`
const p = computed(() => props.palette)

// ---- text measuring, for the widths the device wraps or cuts text at ----
const FONT = "'Google Sans Flex', 'Google Sans', system-ui, sans-serif"
const fontsReady = ref(0)
document.fonts?.ready.then(() => fontsReady.value++)
const ctx = document.createElement('canvas').getContext('2d')
function measure(s: string, px: number, bold: boolean): number {
  void fontsReady.value
  if (!ctx) return s.length * px * 0.55
  ctx.font = `${bold ? 700 : 400} ${px}px ${FONT}`
  return ctx.measureText(s).width
}
// LV_LABEL_LONG_DOT
function ellipsize(s: string, px: number, bold: boolean, width: number): string {
  if (measure(s, px, bold) <= width) return s
  let t = s
  while (t && measure(`${t}…`, px, bold) > width) t = t.slice(0, -1)
  return `${t.trimEnd()}…`
}
// LV_LABEL_LONG_WRAP, centered
function wrap(s: string, px: number, bold: boolean, width: number): string[] {
  const lines: string[] = []
  let line = ''
  for (const w of s.split(/\s+/).filter(Boolean)) {
    const next = line ? `${line} ${w}` : w
    if (!line || measure(next, px, bold) <= width) line = next
    else {
      lines.push(line)
      line = w
    }
  }
  if (line) lines.push(line)
  return lines
}

// ---- colors ----
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
// lv_color_mix(white, c, 30): the pressed tint
function lighten(hex: string, amount = 30 / 255): string {
  const n = Number.parseInt(hex.slice(1), 16)
  const ch = (v: number) => Math.round(v + (255 - v) * amount)
  return `rgb(${ch((n >> 16) & 255)}, ${ch((n >> 8) & 255)}, ${ch(n & 255)})`
}

// ---- geometry ----
const rad = (deg: number) => (deg * Math.PI) / 180
// Screen angles: 0° at 3 o'clock, clockwise
const at = (r: number, deg: number): [number, number] => [r * Math.cos(rad(deg)), r * Math.sin(rad(deg))]
function arcPath(r: number, a0: number, a1: number): string {
  if (a1 - a0 <= 0.01) a1 = a0 + 0.01
  const [x0, y0] = at(r, a0)
  const [x1, y1] = at(r, a1)
  return `M${x0.toFixed(2)} ${y0.toFixed(2)} A${r} ${r} 0 ${a1 - a0 > 180 ? 1 : 0} 1 ${x1.toFixed(2)} ${y1.toFixed(2)}`
}
// lv_arc: background 135° → 45° clockwise, value 0..100 along it
const ARC_START = 135
const ARC_SWEEP = 270
const valueAngle = (v: number) => ARC_START + (ARC_SWEEP * Math.min(100, Math.max(0, v))) / 100

// Bezel arc: 348 px box, width 14, stroke centered inside the box
const BEZEL_R = 348 / 2 - 14 / 2
function bezel(value: number, color: string) {
  const end = valueAngle(value)
  const [kx, ky] = at(BEZEL_R, end)
  return {
    track: arcPath(BEZEL_R, ARC_START, ARC_START + ARC_SWEEP),
    ind: arcPath(BEZEL_R, ARC_START, end),
    kx,
    ky,
    color,
  }
}

// ---- radial ----
interface Placed {
  b: Button
  x: number
  y: number
  size: number
  icon: number
  shape: ButtonShape
}
const radial = computed(() => {
  const pg = props.page
  if (pg.type !== 'radial') return null
  const btns = pg.buttons.slice(0, 8)
  const n = btns.length
  const size = n <= 4 ? 92 : n <= 6 ? 82 : 72
  const radius = n <= 4 ? 110 : n <= 6 ? 120 : 128
  const icon = n <= 6 ? 32 : 24
  const placed: Placed[] = btns.map((b, i) => {
    const [x, y] = at(radius, 270 + (i * 360) / n)
    return { b, x, y, size, icon, shape: b.shape ?? defaultShape(false) }
  })
  if (pg.center) placed.push({ b: pg.center, x: 0, y: 0, size: 112, icon: 44, shape: pg.center.shape ?? defaultShape(true) })
  const title = pg.center ? [] : wrap(pg.title ?? '', 22, true, 150)
  return { placed, title, hasCenter: !!pg.center }
})
const selectedButton = computed(() => radial.value?.placed.find((x) => x.b.id === props.selected)?.b ?? null)
const bubble = computed(() => {
  const label = selectedButton.value?.label
  if (!label) return null
  const w = measure(label, 16, false) + 32
  return { label, w, h: 16 * 1.25 + 16 }
})

const pressed = ref<string | null>(null)
function pick(id: string) {
  if (props.editable) emit('select', id)
}

// ---- media ----
const SAMPLE_VOLUME = 42
// Shown playing; clicking the play button pauses the sample, to see both shapes
const playing = ref(true)
const media = computed(() => {
  const pg = props.page
  if (pg.type !== 'media') return null
  const shape = (k: keyof typeof MEDIA_SHAPE_DEFAULTS) => pg.shapes?.[k] ?? MEDIA_SHAPE_DEFAULTS[k]
  return {
    prev: buttonPath(shape('prev'), -94, 44, 64),
    next: buttonPath(shape('next'), 94, 44, 64),
    play: buttonPath(shape(playing.value ? 'playing' : 'play'), 0, 44, 100),
    title: ellipsize('Song title', 22, true, 236),
    artist: ellipsize('Artist name', 16, false, 220),
    arc: bezel(SAMPLE_VOLUME, p.value.primary),
  }
})

// ---- stats ----
const SAMPLE: Record<StatItem, number> = { cpu: 23, cpuTemp: 54, ram: 61, gpu: 12, gpuTemp: 47, vram: 35 }
const CAPTION: Record<StatItem, string> = { cpu: 'CPU', cpuTemp: 'CPU °C', ram: 'RAM', gpu: 'GPU', gpuTemp: 'GPU °C', vram: 'VRAM' }
const LAYOUT: Record<number, { size: number; at: [number, number][] }> = {
  1: { size: 300, at: [[0, 0]] },
  2: { size: 160, at: [[-84, 0], [84, 0]] },
  3: { size: 148, at: [[0, -80], [-82, 62], [82, 62]] },
  4: { size: 136, at: [[-74, -74], [74, -74], [-74, 74], [74, 74]] },
}
const gauges = computed(() => {
  const pg = props.page
  if (pg.type !== 'stats') return []
  const items = pg.items.slice(0, 4)
  const lay = LAYOUT[Math.max(1, items.length)]!
  const one = items.length === 1
  const width = one ? 20 : 12
  const r = lay.size / 2 - width / 2
  return items.map((item, i) => {
    const [x, y] = lay.at[i]!
    const v = SAMPLE[item]
    const temp = item === 'cpuTemp' || item === 'gpuTemp'
    const color = item === 'cpu' || item === 'cpuTemp' ? p.value.primary : item === 'ram' ? p.value.onSecondaryContainer : p.value.tertiary
    return {
      key: item,
      x,
      y,
      width,
      track: arcPath(r, ARC_START, ARC_START + ARC_SWEEP),
      ind: arcPath(r, ARC_START, valueAngle(v)),
      color,
      value: `${v}${temp ? '°' : '%'}`,
      caption: CAPTION[item],
      valuePx: one ? 56 : 34,
      captionPx: one ? 16 : 14,
    }
  })
})

// ---- dial ----
const dial = computed(() => {
  const pg = props.page
  if (pg.type !== 'dial') return null
  const custom = pg.source === 'custom'
  const v = custom ? 50 : pg.source === 'brightness' ? 80 : 65
  return {
    icon: pg.icon || 'tune',
    label: pg.label,
    value: custom ? '' : String(v),
    arc: bezel(v, pg.source === 'mic' ? p.value.tertiary : p.value.primary),
  }
})

// ---- image: full screen, cover or contain on the surface color ----
const imagePage = computed(() => (props.page.type === 'image' ? props.page : null))
const sliding = computed(() => {
  const pg = imagePage.value
  return !!pg && props.slideshow && !props.thumb && pg.images.length > 1 && !pg.images.some(isGif)
})
const shownImages = computed(() => {
  const pg = imagePage.value
  if (!pg) return []
  return sliding.value ? pg.images : pg.images.slice(0, 1)
})
const assets = useAssets(() => shownImages.value)
const slide = ref(0)
let slideTimer: ReturnType<typeof setInterval> | undefined
watch(
  () => [sliding.value, imagePage.value?.interval, shownImages.value.length] as const,
  ([on, interval, n]) => {
    clearInterval(slideTimer)
    slideTimer = undefined
    if (slide.value >= n) slide.value = 0
    if (!on) {
      slide.value = 0
      return
    }
    const ms = Math.min(3600, Math.max(2, interval ?? IMAGE_DEFAULTS.interval)) * 1000
    slideTimer = setInterval(() => (slide.value = (slide.value + 1) % shownImages.value.length), ms)
  },
  { immediate: true },
)
onBeforeUnmount(() => clearInterval(slideTimer))
const image = computed(() => {
  const pg = imagePage.value
  if (!pg) return null
  const ids = shownImages.value
  return {
    aspect: (pg.fit ?? IMAGE_DEFAULTS.fit) === 'contain' ? 'xMidYMid meet' : 'xMidYMid slice',
    layers: ids.map((id, i) => ({ id, url: assets.src(id), on: i === slide.value })),
    state: !ids.length ? 'empty' : assets.failed(ids[slide.value] ?? ids[0]!) ? 'missing' : assets.src(ids[slide.value] ?? ids[0]!) ? 'ok' : 'loading',
  }
})

// ---- touchpad: the scroll ring between r 150 and 172 ----
const touchpad = computed(() => props.page.type === 'touchpad')

// ---- page dots: bottom center, 8 px above the edge, gap 6 ----
const dots = computed(() => {
  const n = props.pageCount
  if (n < 2) return []
  const widths = Array.from({ length: n }, (_, i) => (i === props.pageIndex ? 18 : 6))
  const total = widths.reduce((a, b) => a + b, 0) + 6 * (n - 1)
  let x = -total / 2
  return widths.map((w, i) => {
    const d = { x, w, on: i === props.pageIndex }
    x += w + 6
    return d
  })
})
</script>

<template>
  <svg class="round" viewBox="-180 -180 360 360" role="img" :aria-label="`Preview of the ${page.type} page`">
    <defs>
      <clipPath :id="uid"><circle r="180" /></clipPath>
    </defs>
    <g :clip-path="`url(#${uid})`">
      <circle r="180" :fill="p.surface" />

      <!-- Buttons on a ring, optional center button -->
      <template v-if="radial">
        <text v-for="(line, i) in radial.title" :key="i" :y="(i - (radial.title.length - 1) / 2) * 27" class="txt" font-size="22" font-weight="700" :fill="p.onSurfaceVariant">
          {{ line }}
        </text>
        <g
          v-for="pl in radial.placed"
          :key="pl.b.id"
          class="btn-g"
          :class="{ editable, down: pressed === pl.b.id }"
          :role="editable ? 'button' : undefined"
          :tabindex="editable ? 0 : undefined"
          :aria-label="editable ? `Edit ${pl.b.label || pl.b.icon}` : undefined"
          :style="{ transformOrigin: `${pl.x}px ${pl.y}px` }"
          @pointerdown="editable && (pressed = pl.b.id)"
          @pointerup="pressed = null"
          @pointerleave="pressed = null"
          @click="pick(pl.b.id)"
          @keydown.enter.prevent="pick(pl.b.id)"
          @keydown.space.prevent="pick(pl.b.id)"
        >
          <!-- Editing aid only: the device has no selection mark -->
          <path
            v-if="editable && pl.b.id === selected"
            :d="buttonPath(pl.shape, pl.x, pl.y, pl.size + 12)"
            fill="none"
            :stroke="p.primary"
            stroke-width="2.5"
            stroke-dasharray="5 4"
          />
          <path :d="buttonPath(pl.shape, pl.x, pl.y, pl.size)" :fill="pressed === pl.b.id ? lighten(pair(pl.b.color)[0]) : pair(pl.b.color)[0]" />
          <text class="sym" :x="pl.x" :y="pl.y" :font-size="pl.icon" :fill="pair(pl.b.color)[1]">{{ pl.b.icon }}</text>
        </g>
        <g v-if="editable && !radial.hasCenter && !radial.title.length" class="ghost" role="button" tabindex="0" aria-label="Add a center button" @click="emit('addCenter')" @keydown.enter.prevent="emit('addCenter')">
          <circle r="40" fill="transparent" :stroke="p.outlineVariant" stroke-width="2" stroke-dasharray="6 6" />
          <text class="sym outline" font-size="28" :fill="p.onSurfaceVariant">add</text>
        </g>
        <!-- Label bubble, as after a press on the device -->
        <g v-if="bubble" class="bubble">
          <rect :x="-bubble.w / 2" :y="-bubble.h / 2" :width="bubble.w" :height="bubble.h" :rx="bubble.h / 2" :fill="p.surfaceContainerHighest" />
          <text class="txt" font-size="16" :fill="p.onSurface">{{ bubble.label }}</text>
        </g>
      </template>

      <!-- Media -->
      <template v-else-if="media">
        <path :d="media.arc.track" fill="none" :stroke="p.surfaceContainerHighest" stroke-width="14" stroke-linecap="round" />
        <path :d="media.arc.ind" fill="none" :stroke="media.arc.color" stroke-width="14" stroke-linecap="round" />
        <circle :cx="media.arc.kx" :cy="media.arc.ky" r="8" :fill="p.onPrimary" :stroke="media.arc.color" stroke-width="4" />
        <text class="txt" y="-104" font-size="14" :fill="p.onSurfaceVariant">Spotify</text>
        <text class="txt" y="-66" font-size="22" font-weight="700" :fill="p.onSurface">{{ media.title }}</text>
        <text class="txt" y="-36" font-size="16" :fill="p.onSurfaceVariant">{{ media.artist }}</text>
        <path :d="media.prev" :fill="p.secondaryContainer" />
        <text class="sym" x="-94" y="44" font-size="32" :fill="p.onSecondaryContainer">skip_previous</text>
        <path :d="media.next" :fill="p.secondaryContainer" />
        <text class="sym" x="94" y="44" font-size="32" :fill="p.onSecondaryContainer">skip_next</text>
        <!-- The play button takes its `playing` shape while music plays, `play` when paused -->
        <g
          :class="{ 'play-g': !thumb }"
          :role="thumb ? undefined : 'button'"
          :tabindex="thumb ? undefined : 0"
          :aria-label="thumb ? undefined : playing ? 'Preview paused state' : 'Preview playing state'"
          @click="!thumb && (playing = !playing)"
          @keydown.enter.prevent="playing = !playing"
          @keydown.space.prevent="playing = !playing"
        >
          <path :d="media.play" :fill="p.primaryContainer" />
          <text class="sym" x="0" y="44" font-size="44" :fill="p.onPrimaryContainer">{{ playing ? 'pause' : 'play_arrow' }}</text>
        </g>
        <text class="txt" y="132" font-size="14" :fill="p.onSurfaceVariant">{{ SAMPLE_VOLUME }}</text>
      </template>

      <!-- Stats -->
      <template v-else-if="gauges.length">
        <g v-for="g in gauges" :key="g.key" :transform="`translate(${g.x} ${g.y})`">
          <path :d="g.track" fill="none" :stroke="p.surfaceContainerHighest" :stroke-width="g.width" stroke-linecap="round" />
          <path :d="g.ind" fill="none" :stroke="g.color" :stroke-width="g.width" stroke-linecap="round" />
          <text class="txt" y="-6" :font-size="g.valuePx" font-weight="700" :fill="p.onSurface">{{ g.value }}</text>
          <text class="txt" y="26" :font-size="g.captionPx" :fill="p.onSurfaceVariant">{{ g.caption }}</text>
        </g>
      </template>

      <!-- Dial -->
      <template v-else-if="dial">
        <path :d="dial.arc.track" fill="none" :stroke="p.surfaceContainerHighest" stroke-width="14" stroke-linecap="round" />
        <path :d="dial.arc.ind" fill="none" :stroke="dial.arc.color" stroke-width="14" stroke-linecap="round" />
        <circle :cx="dial.arc.kx" :cy="dial.arc.ky" r="8" :fill="p.onPrimary" :stroke="dial.arc.color" stroke-width="4" />
        <text class="sym" y="-54" font-size="64" :fill="p.primary">{{ dial.icon }}</text>
        <text class="txt" y="14" font-size="56" font-weight="700" :fill="p.onSurface">{{ dial.value }}</text>
        <text class="txt" y="62" font-size="16" :fill="p.onSurfaceVariant">{{ dial.label }}</text>
      </template>

      <!-- Touchpad -->
      <template v-else-if="touchpad">
        <circle r="161" fill="none" :stroke="p.surfaceContainerHigh" stroke-width="22" />
        <text class="sym" y="-14" font-size="56" :fill="p.onSurfaceVariant">mouse</text>
        <text class="txt" y="34" font-size="16" :fill="p.onSurfaceVariant">Touchpad</text>
      </template>

      <!-- Image -->
      <template v-else-if="image">
        <image
          v-for="l in image.layers"
          :key="l.id"
          v-show="l.url"
          class="img-layer"
          :class="{ on: l.on }"
          :href="l.url || undefined"
          x="-180"
          y="-180"
          width="360"
          height="360"
          :preserveAspectRatio="image.aspect"
        />
        <template v-if="image.state === 'empty' || image.state === 'missing'">
          <text class="sym" y="-14" font-size="64" :fill="p.onSurfaceVariant">image</text>
          <text v-if="!thumb" class="txt" y="44" font-size="16" :fill="p.onSurfaceVariant">{{ image.state === 'empty' ? 'No image yet' : 'Image not found' }}</text>
        </template>
      </template>

      <rect v-for="(d, i) in dots" :key="i" :x="d.x" :y="180 - 8 - 6" :width="d.w" height="6" rx="3" :fill="d.on ? p.primary : p.outlineVariant" />
    </g>
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
  user-select: none;
}
.txt {
  font-family: 'Google Sans Flex', 'Google Sans', system-ui, sans-serif;
  text-anchor: middle;
  dominant-baseline: central;
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
  text-anchor: middle;
  dominant-baseline: central;
}
.sym.outline {
  font-variation-settings: 'FILL' 0;
}
/* Press: scale to 91 % (transform_scale 232/256) and lighten; the shape stays */
.btn-g {
  transition: transform 0.15s var(--ease);
}
.btn-g.down {
  transform: scale(0.906);
}
.btn-g.editable {
  cursor: pointer;
}
.btn-g:focus-visible {
  outline: none;
}
.btn-g:focus-visible path:last-of-type {
  stroke: var(--primary);
  stroke-width: 3;
}
.bubble {
  pointer-events: none;
}
.play-g {
  cursor: pointer;
}
.play-g:focus-visible {
  outline: none;
}
.play-g:focus-visible path {
  stroke: var(--primary);
  stroke-width: 3;
}
.img-layer {
  opacity: 0;
  transition: opacity 0.8s ease-in-out;
}
.img-layer.on {
  opacity: 1;
}
@media (prefers-reduced-motion: reduce) {
  .img-layer {
    transition: none;
  }
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
</style>
