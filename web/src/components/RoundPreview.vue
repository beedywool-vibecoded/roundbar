<script setup lang="ts">
// The 360 x 360 round screen, laid out with the firmware's numbers (firmware/main/ui.c) and
// drawn with the device palette. Coordinates are relative to the screen center, y down.
// Buttons can be clicked to select them for editing.
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useAssets } from '@/lib/assets'
import { buttonPath, type ButtonShape } from '@/lib/shapes'
import { CLOCK_DEFAULTS, IMAGE_DEFAULTS, MEDIA_SHAPE_DEFAULTS, TIMER_DEFAULTS, defaultShape, isGif } from '@/lib/model'
import { config } from '@/lib/store'
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

// ---- clock: live time; weather is a static sample ----
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | undefined
watch(
  () => props.page.type === 'clock',
  (on) => {
    clearInterval(clockTimer)
    clockTimer = undefined
    if (!on) return
    now.value = new Date()
    clockTimer = setInterval(() => (now.value = new Date()), 1000)
  },
  { immediate: true },
)
onBeforeUnmount(() => clearInterval(clockTimer))
const pad = (n: number) => String(n).padStart(2, '0')
function zoneTime(tz: string, d: Date): string {
  try {
    return new Intl.DateTimeFormat('en-GB', { timeZone: tz, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(d)
  } catch {
    return '--:--'
  }
}
const clock = computed(() => {
  const pg = props.page
  if (pg.type !== 'clock') return null
  const d = now.value
  const h = d.getHours()
  const m = d.getMinutes()
  const sec = d.getSeconds()
  const w = config.value?.weather
  const weather = (pg.weather ?? CLOCK_DEFAULTS.weather) && w ? (w.units === 'imperial' ? '54°' : '12°') : ''
  const zones = (pg.zones ?? []).slice(0, 3).map((z) => `${z.label} ${zoneTime(z.tz, d)}`)
  const analog = (pg.style ?? CLOCK_DEFAULTS.style) === 'analog'
  // Same layout as the device: weather above the home time, world clocks below it.
  const base = analog ? -66 : -92 // weather row
  const rowsTop = analog ? 56 : 62
  const rows = zones.map((text, i) => ({ y: rowsTop + i * 20, text }))
  const hourAngle = ((h % 12) + m / 60) * 30 - 90
  const minAngle = (m + sec / 60) * 6 - 90
  const ticks = Array.from({ length: 12 }, (_, i) => {
    const big = i % 3 === 0
    const [x1, y1] = at(big ? 124 : 130, i * 30 - 90)
    const [x2, y2] = at(140, i * 30 - 90)
    return { x1, y1, x2, y2, big }
  })
  return {
    analog,
    base,
    time: `${pad(h)}:${pad(m)}`,
    date: d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
    timeY: -24, // baseline-centred like the device; date 50 px below
    hour: at(70, hourAngle),
    minute: at(108, minAngle),
    second: at(124, sec * 6 - 90),
    ticks,
    weather,
    rows,
  }
})

// ---- timer: full ring at 70 %, sample remaining time ----
const timer = computed(() => {
  const pg = props.page
  if (pg.type !== 'timer') return null
  const presets = (pg.minutes ?? TIMER_DEFAULTS.minutes).slice(0, 4).map((m) => `${m} min`)
  const gap = 8
  const widths = presets.map((t) => measure(t, 14, false) + 24)
  const total = widths.reduce((a, b) => a + b, 0) + gap * Math.max(0, presets.length - 1)
  let x = -total / 2
  const chips = presets.map((text, i) => {
    const c = { text, x, w: widths[i]!, cx: x + widths[i]! / 2 }
    x += widths[i]! + gap
    return c
  })
  return {
    track: `M0 ${-BEZEL_R} A${BEZEL_R} ${BEZEL_R} 0 1 1 0 ${BEZEL_R} A${BEZEL_R} ${BEZEL_R} 0 1 1 0 ${-BEZEL_R}`,
    ind: arcPath(BEZEL_R, 270, 270 + 360 * 0.7),
    label: ellipsize(pg.label || (pg.pomodoro ? 'Focus' : ''), 16, false, 220),
    pomodoro: pg.pomodoro === true,
    chips,
  }
})

// ---- mixer, windows, clipboard: sample content, drawn like the device lists ----
const MIXER_APPS = [
  { name: 'Spotify', icon: 'music_note', vol: 72 },
  { name: 'Firefox', icon: 'public', vol: 45 },
  { name: 'Discord', icon: 'headset_mic', vol: 88 },
]
const mixer = computed(() => {
  if (props.page.type !== 'mixer') return null
  const x0 = -78
  const x1 = 128
  const rows = MIXER_APPS.map((a, i) => {
    const y = -46 + i * 64
    const hx = x0 + ((x1 - x0) * a.vol) / 100
    return { ...a, y, x0, x1, hx }
  })
  const chips: { text: string; on: boolean; w: number }[] = [] // per-app volume only, no output picker
  const total = chips.reduce((a, c) => a + c.w, 0) + 8
  let x = -total / 2
  return {
    rows,
    chips: chips.map((c) => {
      const out = { ...c, x }
      x += c.w + 8
      return out
    }),
  }
})
const WINDOW_ICONS = ['public', 'terminal', 'folder', 'code', 'chat', 'music_note']
const windows = computed(() => {
  if (props.page.type !== 'windows') return null
  return WINDOW_ICONS.map((icon, i) => {
    const [x, y] = at(116, 270 + i * 60)
    return { icon, x, y, on: i === 0 }
  })
})
const CLIPS = [
  { text: 'Meeting moved to 15:30', when: 'Just now' },
  { text: 'https://open-meteo.com/en/docs', when: '2 min ago' },
  { text: 'git commit -m "fix layout"', when: '10 min ago' },
]
const clipboard = computed(() => {
  if (props.page.type !== 'clipboard') return null
  return CLIPS.map((c, i) => ({ text: ellipsize(c.text, 16, false, 220), when: c.when, y: -112 + i * 76 }))
})

// ---- discord: sample voice channel ----
const discord = computed(() => {
  if (props.page.type !== 'discord') return null
  // Five members spread over the top of the ring, clear of the buttons
  return [-150, -120, -90, -60, -30].map((deg, i) => {
    const [x, y] = at(132, deg)
    return { x, y, speaking: i === 1, muted: i === 3 }
  })
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

      <!-- Clock -->
      <template v-else-if="clock">
        <template v-if="clock.analog">
          <circle r="150" :fill="p.primaryContainer" fill-opacity="0.45" />
          <line
            v-for="(t, i) in clock.ticks"
            :key="i"
            :x1="t.x1"
            :y1="t.y1"
            :x2="t.x2"
            :y2="t.y2"
            :stroke="t.big ? p.primary : p.onSurfaceVariant"
            :stroke-width="t.big ? 6 : 4"
            stroke-linecap="round"
          />
        </template>
        <template v-else>
          <text class="txt num" :y="clock.timeY" font-size="72" font-weight="700" :fill="p.onSurface">{{ clock.time }}</text>
          <text class="txt" :y="clock.timeY + 50" font-size="16" :fill="p.onSurfaceVariant">{{ clock.date }}</text>
        </template>
        <text v-for="r in clock.rows" :key="r.text" class="txt num" :y="r.y" font-size="14" :fill="p.onSurfaceVariant">{{ r.text }}</text>
        <text v-if="clock.weather" class="txt" :y="clock.base" font-size="16" :fill="p.onSurface">
          <tspan class="sym-inline" font-size="20" :fill="p.tertiary">partly_cloudy_day</tspan><tspan dx="6">{{ clock.weather }}</tspan>
        </text>
        <template v-if="clock.analog">
          <line x1="0" y1="0" :x2="clock.hour[0]" :y2="clock.hour[1]" :stroke="p.primary" stroke-width="12" stroke-linecap="round" />
          <line x1="0" y1="0" :x2="clock.minute[0]" :y2="clock.minute[1]" :stroke="p.onSurface" stroke-width="8" stroke-linecap="round" />
          <circle r="7" :fill="p.onSurface" />
          <circle :cx="clock.second[0]" :cy="clock.second[1]" r="6" :fill="p.tertiary" />
        </template>
      </template>

      <!-- Timer -->
      <template v-else-if="timer">
        <path :d="timer.track" fill="none" :stroke="p.surfaceContainerHighest" stroke-width="14" />
        <path :d="timer.ind" fill="none" :stroke="p.primary" stroke-width="14" stroke-linecap="round" />
        <text v-if="timer.pomodoro" class="txt" y="-62" font-size="14" font-weight="700" :fill="p.tertiary">POMODORO 1/4</text>
        <text class="txt num" y="-14" font-size="56" font-weight="700" :fill="p.onSurface">17:32</text>
        <text v-if="timer.label" class="txt" y="30" font-size="16" :fill="p.onSurfaceVariant">{{ timer.label }}</text>
        <g v-for="c in timer.chips" :key="c.text">
          <rect :x="c.x" y="80" :width="c.w" height="30" rx="15" :fill="p.secondaryContainer" />
          <text class="txt" :x="c.cx" y="95" font-size="14" :fill="p.onSecondaryContainer">{{ c.text }}</text>
        </g>
      </template>

      <!-- Volume mixer -->
      <template v-else-if="mixer">
        <g v-for="c in mixer.chips" :key="c.text">
          <rect
            :x="c.x"
            y="-132"
            :width="c.w"
            height="32"
            rx="10"
            :fill="c.on ? p.secondaryContainer : 'none'"
            :stroke="c.on ? 'none' : p.outlineVariant"
            stroke-width="1.5"
          />
          <text v-if="c.on" class="sym" :x="c.x + 20" y="-116" font-size="18" :fill="p.onSecondaryContainer">check</text>
          <text class="txt" :x="c.on ? c.x + 20 + (c.w - 20) / 2 - 2 : c.x + c.w / 2" y="-116" font-size="14" :fill="c.on ? p.onSecondaryContainer : p.onSurfaceVariant">
            {{ c.text }}
          </text>
        </g>
        <g v-for="r in mixer.rows" :key="r.name">
          <circle cx="-112" :cy="r.y" r="22" :fill="p.surfaceContainerHighest" />
          <text class="sym" x="-112" :y="r.y" font-size="22" :fill="p.onSurface">{{ r.icon }}</text>
          <text class="txt start" :x="r.x0" :y="r.y - 14" font-size="14" :fill="p.onSurface">{{ r.name }}</text>
          <!-- M3 expressive slider: thick active track, a gap, the bar handle, the inactive track -->
          <line :x1="r.x0 + 6" :y1="r.y + 10" :x2="r.hx - 8" :y2="r.y + 10" :stroke="p.primary" stroke-width="12" stroke-linecap="round" />
          <line :x1="r.hx + 8" :y1="r.y + 10" :x2="r.x1 - 6" :y2="r.y + 10" :stroke="p.secondaryContainer" stroke-width="12" stroke-linecap="round" />
          <line :x1="r.hx" :y1="r.y - 2" :x2="r.hx" :y2="r.y + 22" :stroke="p.primary" stroke-width="4" stroke-linecap="round" />
        </g>
      </template>

      <!-- Windows -->
      <template v-else-if="windows">
        <g v-for="w in windows" :key="w.icon">
          <circle :cx="w.x" :cy="w.y" r="36" :fill="w.on ? p.primaryContainer : p.surfaceContainerHighest" />
          <text class="sym" :x="w.x" :y="w.y" font-size="32" :fill="w.on ? p.onPrimaryContainer : p.onSurface">{{ w.icon }}</text>
        </g>
        <text class="txt" y="0" font-size="22" font-weight="700" :fill="p.onSurface">Firefox</text>
      </template>

      <!-- Clipboard -->
      <template v-else-if="clipboard">
        <g v-for="(c, i) in clipboard" :key="i">
          <rect x="-132" :y="c.y" width="264" height="64" rx="20" :fill="i === 0 ? p.secondaryContainer : p.surfaceContainerHigh" />
          <text class="txt start" x="-112" :y="c.y + 24" font-size="16" :fill="i === 0 ? p.onSecondaryContainer : p.onSurface">{{ c.text }}</text>
          <text class="txt start" x="-112" :y="c.y + 46" font-size="12" :fill="i === 0 ? p.onSecondaryContainer : p.onSurfaceVariant">{{ c.when }}</text>
        </g>
      </template>

      <!-- Discord voice -->
      <template v-else-if="discord">
        <g v-for="(m, i) in discord" :key="i">
          <circle v-if="m.speaking" :cx="m.x" :cy="m.y" r="30" fill="none" stroke="#23A55A" stroke-width="4" />
          <circle :cx="m.x" :cy="m.y" r="28" :fill="p.surfaceContainerHighest" />
          <text class="sym" :x="m.x" :y="m.y" font-size="30" :fill="p.onSurfaceVariant">person</text>
          <g v-if="m.muted">
            <circle :cx="m.x + 20" :cy="m.y + 20" r="11" :fill="p.errorContainer" :stroke="p.surface" stroke-width="2" />
            <text class="sym" :x="m.x + 20" :y="m.y + 20" font-size="14" :fill="p.onErrorContainer">mic_off</text>
          </g>
        </g>
        <text class="txt" y="-64" font-size="16" :fill="p.onSurfaceVariant">General</text>
        <circle cx="-44" cy="6" r="38" :fill="p.surfaceContainerHighest" />
        <text class="sym" x="-44" y="6" font-size="34" :fill="p.onSurface">mic</text>
        <circle cx="44" cy="6" r="38" :fill="p.surfaceContainerHighest" />
        <text class="sym" x="44" y="6" font-size="34" :fill="p.onSurface">headphones</text>
        <circle cx="0" cy="78" r="24" :fill="p.error" />
        <text class="sym" x="0" y="78" font-size="24" :fill="p.surface">call_end</text>
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
.txt.start {
  text-anchor: start;
}
.sym-inline {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
}
.num {
  font-variant-numeric: tabular-nums;
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
