// Labels, defaults, id generation and validation for the config (docs/spec.md section 1).
import { comboError } from './keys'
import { BUTTON_SHAPES, type ButtonShape } from './shapes'
import { DEFAULT_SEED, devicePalette, validHex } from './theme'
import {
  PALETTE_ROLES,
  type Action,
  type ActionType,
  type Button,
  type ColorRole,
  type Config,
  type DialSource,
  type MediaSettings,
  type Page,
  type PageType,
  type MediaButton,
  type Profile,
  type StatItem,
  type Step,
  type ThemeMode,
} from './types'

export const ID_RE = /^[a-z0-9-]{1,24}$/
export const LIMITS = {
  pages: 8,
  buttons: 8,
  label: 16,
  text: 64,
  steps: 32,
  delay: 5000,
  stats: 4,
  images: 20,
  zones: 3,
  zoneLabel: 16,
  timerPresets: 4,
  timerLabel: 24,
} as const

export const CLOCK_DEFAULTS = { style: 'analog', weather: true } as const
export const TIMER_DEFAULTS = { minutes: [5, 15, 25] as readonly number[], minMinutes: 1, maxMinutes: 180 } as const
export const SCREENSAVER_DEFAULTS = { after: 60, minAfter: 10, maxAfter: 3600 } as const

// ---- time zones (IANA names, as the browser knows them) ----
let zoneList: string[] | null = null
export function timeZones(): string[] {
  if (!zoneList) {
    const fn = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf
    zoneList = fn ? fn('timeZone') : []
    if (!zoneList.includes('UTC')) zoneList = [...zoneList, 'UTC']
  }
  return zoneList
}
let zoneSet: Set<string> | null = null
export function validTimeZone(tz: string): boolean {
  zoneSet ??= new Set(timeZones())
  if (zoneSet.size > 1) return zoneSet.has(tz)
  // No list in this browser: ask the formatter
  try {
    new Intl.DateTimeFormat('en', { timeZone: tz })
    return tz.includes('/') || tz === 'UTC'
  } catch {
    return false
  }
}
// "America/Argentina/Buenos_Aires" → "Buenos Aires"
export function zoneCity(tz: string): string {
  return (tz.split('/').pop() ?? tz).replace(/_/g, ' ')
}

export const TOUCHPAD_DEFAULTS = { speed: 5 } as const
export const IMAGE_DEFAULTS = { interval: 8, fit: 'cover' } as const
export const ASSET_ID_RE = /^[0-9a-f]{12}\.(jpg|gif)$/
export const isGif = (id: string) => id.endsWith('.gif')

export const PAGE_TYPES: { value: PageType; label: string; icon: string; hint: string }[] = [
  { value: 'radial', label: 'Buttons', icon: 'apps', hint: 'Up to 8 buttons on a ring, plus an optional center button' },
  { value: 'media', label: 'Media', icon: 'music_note', hint: 'Now playing, transport controls and volume on the edge' },
  { value: 'stats', label: 'Stats', icon: 'speed', hint: 'Up to 4 gauges: CPU, GPU, memory and temperatures' },
  { value: 'dial', label: 'Dial', icon: 'tune', hint: 'One big arc you drag along the edge: volume, mic, brightness or custom' },
  { value: 'clock', label: 'Clock', icon: 'schedule', hint: 'Time, weather and world clocks' },
  { value: 'timer', label: 'Timer', icon: 'timer', hint: 'Countdown and Pomodoro' },
  { value: 'mixer', label: 'Volume mixer', icon: 'graphic_eq', hint: 'Per-app volume and output device' },
  { value: 'windows', label: 'Windows', icon: 'select_window', hint: 'Switch between open windows' },
  { value: 'clipboard', label: 'Clipboard', icon: 'content_paste', hint: 'Paste from your recent copies' },
  { value: 'discord', label: 'Discord voice', icon: 'headset_mic', hint: "Your voice channel: who's talking, mute and deafen" },
  { value: 'image', label: 'Image', icon: 'image', hint: 'A picture, a GIF, or a slideshow of pictures' },
  {
    value: 'touchpad',
    label: 'Touchpad',
    icon: 'mouse',
    hint: 'Use the screen as a mouse: drag to move, tap to click, hold to right-click, slide the ring to scroll',
  },
]
export const pageTypeInfo = (t: PageType) => PAGE_TYPES.find((p) => p.value === t)!

export const COLOR_ROLES: { value: ColorRole; label: string }[] = [
  { value: 'primary', label: 'Primary' },
  { value: 'secondary', label: 'Secondary' },
  { value: 'tertiary', label: 'Tertiary' },
  { value: 'error', label: 'Error' },
  { value: 'surface', label: 'Surface' },
]

export const ACTION_TYPES: { value: ActionType; label: string; icon: string; hint: string }[] = [
  { value: 'keys', label: 'Shortcut', icon: 'keyboard', hint: 'Press a key combo' },
  { value: 'text', label: 'Type text', icon: 'text_fields', hint: 'Type a short text' },
  { value: 'media', label: 'Media key', icon: 'play_pause', hint: 'Play, pause, skip, volume' },
  { value: 'page', label: 'Go to page', icon: 'layers', hint: 'Switch to another page of this profile' },
  { value: 'command', label: 'Run command', icon: 'terminal', hint: 'Run a shell command on the PC' },
  { value: 'open', label: 'Open', icon: 'open_in_new', hint: 'Open a URL, file or .desktop app' },
  { value: 'mic', label: 'Microphone', icon: 'mic', hint: 'Mute or unmute the default source' },
  { value: 'macro', label: 'Macro', icon: 'done_all', hint: 'Several steps in a row' },
]

export const MEDIA_CMDS = [
  { value: 'play_pause', label: 'Play / pause' },
  { value: 'next', label: 'Next track' },
  { value: 'prev', label: 'Previous track' },
  { value: 'vol_up', label: 'Volume up' },
  { value: 'vol_down', label: 'Volume down' },
  { value: 'mute', label: 'Mute' },
] as const
export const MIC_CMDS = [
  { value: 'toggle', label: 'Toggle mute' },
  { value: 'mute', label: 'Mute' },
  { value: 'unmute', label: 'Unmute' },
] as const

export const STAT_ITEMS: { value: StatItem; label: string; icon: string; unit: string }[] = [
  { value: 'cpu', label: 'CPU', icon: 'memory', unit: '%' },
  { value: 'cpuTemp', label: 'CPU temp', icon: 'thermostat', unit: '°C' },
  { value: 'ram', label: 'RAM', icon: 'developer_board', unit: '%' },
  { value: 'gpu', label: 'GPU', icon: 'sports_esports', unit: '%' },
  { value: 'gpuTemp', label: 'GPU temp', icon: 'local_fire_department', unit: '°C' },
  { value: 'vram', label: 'VRAM', icon: 'storage', unit: '%' },
]

export const DIAL_SOURCES: { value: DialSource; label: string; icon: string }[] = [
  { value: 'volume', label: 'Volume', icon: 'volume_up' },
  { value: 'mic', label: 'Microphone', icon: 'mic' },
  { value: 'brightness', label: 'Screen brightness', icon: 'brightness_high' },
  { value: 'custom', label: 'Custom actions', icon: 'tune' },
]

export const SHAPE_LABELS: Record<ButtonShape, string> = {
  circle: 'Circle',
  square: 'Rounded square',
  cookie: 'Cookie',
  clover: 'Clover',
  sunny: 'Sunny',
  flower: 'Flower',
  burst: 'Burst',
}
export const defaultShape = (center: boolean): ButtonShape => (center ? 'square' : 'circle')

// Media page transport buttons. `play` shows while paused, `playing` while music plays.
export const MEDIA_SHAPE_DEFAULTS: Record<MediaButton, ButtonShape> = {
  prev: 'circle',
  next: 'circle',
  play: 'circle',
  playing: 'square',
}
export const MEDIA_BUTTONS: { key: MediaButton; label: string }[] = [
  { key: 'prev', label: 'Previous' },
  { key: 'next', label: 'Next' },
  { key: 'play', label: 'Play (paused)' },
  { key: 'playing', label: 'Play (playing)' },
]

export const ROTATIONS = [0, 90, 180, 270] as const

export const MEDIA_DEFAULTS = { blur: 60, coverOpacity: 50 } as const

// ---- ids ----

export function uniqueId(base: string, taken: Iterable<string>): string {
  const used = new Set(taken)
  const b = slug(base) || 'item'
  if (!used.has(b)) return b
  for (let i = 2; ; i++) {
    const suffix = `-${i}`
    const id = b.slice(0, 24 - suffix.length) + suffix
    if (!used.has(id)) return id
  }
}

export function slug(s: string): string {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 24)
    .replace(/-+$/, '')
}

export function buttonIds(p: Page): string[] {
  if (p.type !== 'radial') return []
  return [...p.buttons.map((b) => b.id), ...(p.center ? [p.center.id] : [])]
}

export function nextButtonId(p: Page): string {
  const used = new Set(buttonIds(p))
  for (let i = 1; ; i++) if (!used.has(`b${i}`)) return `b${i}`
}

// ---- defaults ----

export function defaultAction(type: ActionType, pages: Page[] = []): Action {
  switch (type) {
    case 'keys':
      return { type, keys: '' }
    case 'text':
      return { type, text: '' }
    case 'media':
      return { type, cmd: 'play_pause' }
    case 'page':
      return { type, page: pages[0]?.id ?? '' }
    case 'command':
      return { type, cmd: '' }
    case 'open':
      return { type, target: '' }
    case 'mic':
      return { type, cmd: 'toggle' }
    case 'macro':
      return { type, steps: [] }
  }
}

export function newButton(p: Page, icon = 'star', label = ''): Button {
  return { id: nextButtonId(p), icon, label, color: 'primary', tap: { type: 'keys', keys: '' } }
}

export function newPage(type: PageType, taken: string[]): Page {
  const id = uniqueId(type, taken)
  switch (type) {
    case 'radial': {
      const p: Page = { id, type, buttons: [] }
      p.buttons.push({ id: 'b1', icon: 'play_pause', label: 'Play', color: 'primary', tap: { type: 'media', cmd: 'play_pause' } })
      return p
    }
    case 'media':
      return { id, type, title: 'Media' }
    case 'stats':
      return { id, type, title: 'System', items: ['cpu', 'ram', 'gpu', 'cpuTemp'] }
    case 'dial':
      return { id, type, title: 'Volume', icon: 'volume_up', label: 'Volume', source: 'volume' }
    case 'clock':
      return { id, type, title: 'Clock' }
    case 'timer':
      return { id, type, title: 'Timer' }
    case 'mixer':
      return { id, type, title: 'Mixer' }
    case 'windows':
      return { id, type, title: 'Windows' }
    case 'clipboard':
      return { id, type, title: 'Clipboard' }
    case 'discord':
      return { id, type, title: 'Discord' }
    case 'image':
      return { id, type, title: 'Image', images: [] }
    case 'touchpad':
      return { id, type, title: 'Touchpad', speed: TOUCHPAD_DEFAULTS.speed }
  }
}

const btn = (id: string, icon: string, label: string, color: ColorRole, tap: Action, hold?: Action): Button => ({
  id,
  icon,
  label,
  color,
  tap,
  ...(hold ? { hold } : {}),
})

// A useful first config when the file is missing or empty.
export function starterConfig(): Config {
  return {
    version: 1,
    theme: { seed: DEFAULT_SEED, mode: 'dark', palette: devicePalette(DEFAULT_SEED, 'dark') },
    media: { coverBackground: true, coverColors: true, ...MEDIA_DEFAULTS },
    device: {
      brightness: 80, rotation: 0, dimAfter: 30, sleepAfter: 300, longPressMs: 450, keyboardLayout: 'fr',
      followPc: true,
      edgeDoubleTap: 'page',
      micIndicator: false,
      screensaver: 'off',
      screensaverAfter: SCREENSAVER_DEFAULTS.after,
      screensaverImages: [],
      notifications: true,
      notifToast: true,
    },
    profiles: [
      {
        id: 'default',
        name: 'Default',
        match: [],
        pages: [
          {
            id: 'main',
            type: 'radial',
            title: 'Main',
            center: btn('b9', 'play_pause', 'Play', 'primary', { type: 'media', cmd: 'play_pause' }),
            buttons: [
              btn('b1', 'content_copy', 'Copy', 'secondary', { type: 'keys', keys: 'ctrl+c' }),
              btn('b2', 'content_paste', 'Paste', 'secondary', { type: 'keys', keys: 'ctrl+v' }),
              btn('b3', 'undo', 'Undo', 'tertiary', { type: 'keys', keys: 'ctrl+z' }, { type: 'keys', keys: 'ctrl+shift+z' }),
              btn('b4', 'screenshot_monitor', 'Screenshot', 'tertiary', { type: 'keys', keys: 'printscreen' }),
              btn('b5', 'mic', 'Mic', 'error', { type: 'mic', cmd: 'toggle' }),
              btn('b6', 'terminal', 'Terminal', 'surface', { type: 'command', cmd: 'kgx' }),
              btn('b7', 'public', 'Browser', 'primary', { type: 'open', target: 'https://' }),
              btn('b8', 'equalizer', 'Media', 'primary', { type: 'page', page: 'media' }),
            ],
          },
          { id: 'media', type: 'media', title: 'Media' },
          { id: 'volume', type: 'dial', title: 'Volume', icon: 'volume_up', label: 'Volume', source: 'volume' },
          { id: 'stats', type: 'stats', title: 'System', items: ['cpu', 'ram', 'gpu', 'cpuTemp'] },
        ],
      },
    ],
  }
}

const inRange = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v) && v >= 0 && v <= 100

function normalizeMedia(raw: unknown, base: MediaSettings): MediaSettings {
  const m = { ...base, ...(raw && typeof raw === 'object' ? (raw as Partial<MediaSettings>) : {}) }
  if (!inRange(m.blur)) m.blur = MEDIA_DEFAULTS.blur
  if (!inRange(m.coverOpacity)) m.coverOpacity = MEDIA_DEFAULTS.coverOpacity
  return m
}

// Fills what a hand-written or older file may lack, so the editors can bind safely.
export function normalize(raw: unknown): Config {
  const base = starterConfig()
  if (!raw || typeof raw !== 'object') return base
  const c = raw as Partial<Config>
  const theme = { ...base.theme, ...(c.theme ?? {}) }
  if (!theme.palette || typeof theme.palette !== 'object') theme.palette = devicePalette(theme.seed, theme.mode)
  const cfg: Config = {
    ...c,
    version: 1,
    theme,
    media: normalizeMedia(c.media, base.media),
    device: { ...base.device, ...(c.device ?? {}) },
    profiles: Array.isArray(c.profiles) && c.profiles.length ? c.profiles : base.profiles,
  }
  if (!(ROTATIONS as readonly unknown[]).includes(cfg.device.rotation)) cfg.device.rotation = 0
  if (typeof cfg.device.followPc !== 'boolean') cfg.device.followPc = true
  if (!['page', 'quickSettings', 'screenOff', 'off'].includes(cfg.device.edgeDoubleTap)) cfg.device.edgeDoubleTap = 'page'
  if (typeof cfg.device.micIndicator !== 'boolean') cfg.device.micIndicator = false
  if (typeof cfg.device.notifications !== 'boolean') cfg.device.notifications = true
  if (typeof cfg.device.notifToast !== 'boolean') cfg.device.notifToast = true
  if (!['off', 'clock', 'photos'].includes(cfg.device.screensaver)) cfg.device.screensaver = 'off'
  if (typeof cfg.device.screensaverAfter !== 'number' || !Number.isFinite(cfg.device.screensaverAfter))
    cfg.device.screensaverAfter = SCREENSAVER_DEFAULTS.after
  cfg.device.screensaverImages = Array.isArray(cfg.device.screensaverImages) ? cfg.device.screensaverImages.filter((x) => typeof x === 'string') : []
  if (cfg.weather != null && (typeof cfg.weather !== 'object' || Array.isArray(cfg.weather))) delete cfg.weather
  for (const p of cfg.profiles) {
    p.match ??= []
    p.pages ??= []
    p.name ??= p.id
    syncProfilePalette(p, cfg.theme.mode)
    for (const pg of p.pages) {
      if (pg.type === 'radial') pg.buttons ??= []
      if (pg.type === 'stats') pg.items ??= []
      if (pg.type === 'image') pg.images = Array.isArray(pg.images) ? pg.images.filter((x) => typeof x === 'string') : []
    }
  }
  const d = cfg.profiles.findIndex((p) => p.id === 'default')
  if (d < 0) cfg.profiles.unshift(base.profiles[0]!)
  // The fallback is listed first
  else if (d > 0) cfg.profiles.unshift(...cfg.profiles.splice(d, 1))
  return cfg
}

// A profile with its own seed carries the palette for the current mode; without one, none.
export function syncProfilePalette(p: Profile, mode: ThemeMode) {
  if (p.seed === undefined) delete p.palette
  else if (validHex(p.seed)) p.palette = devicePalette(p.seed, mode)
}

// ---- validation ----

export interface Issue {
  path: string
  message: string
}

const TEXT_RE = /^[\x20-\x7e éèàçù]*$/

export function validText(s: string): boolean {
  return TEXT_RE.test(s)
}

function checkStep(s: Step, path: string, out: Issue[]) {
  if ('keys' in s) {
    const e = comboError(s.keys)
    if (e) out.push({ path, message: e })
  } else if ('text' in s) {
    if (!s.text) out.push({ path, message: 'Text is empty' })
    else if (s.text.length > LIMITS.text) out.push({ path, message: `Text is longer than ${LIMITS.text} characters` })
    else if (!validText(s.text)) out.push({ path, message: 'Text has characters the device cannot type' })
  } else if ('delay' in s) {
    if (!Number.isInteger(s.delay) || s.delay < 0 || s.delay > LIMITS.delay)
      out.push({ path, message: `Delay must be 0 to ${LIMITS.delay} ms` })
  } else if ('command' in s) {
    if (!s.command.trim()) out.push({ path, message: 'Command is empty' })
  }
}

export function checkAction(a: Action, pages: Page[], path: string, out: Issue[]) {
  switch (a.type) {
    case 'keys': {
      const e = comboError(a.keys)
      if (e) out.push({ path, message: e })
      break
    }
    case 'text':
      if (!a.text) out.push({ path, message: 'Text is empty' })
      else if (a.text.length > LIMITS.text) out.push({ path, message: `Text is longer than ${LIMITS.text} characters` })
      else if (!validText(a.text)) out.push({ path, message: 'Text has characters the device cannot type (ASCII and é è à ç ù only)' })
      break
    case 'page':
      if (!pages.some((p) => p.id === a.page)) out.push({ path, message: 'Pick a page of this profile' })
      break
    case 'command':
      if (!a.cmd.trim()) out.push({ path, message: 'Command is empty' })
      break
    case 'open':
      if (!a.target.trim()) out.push({ path, message: 'Nothing to open' })
      break
    case 'macro':
      if (!a.steps.length) out.push({ path, message: 'The macro has no steps' })
      if (a.steps.length > LIMITS.steps) out.push({ path, message: `At most ${LIMITS.steps} steps` })
      a.steps.forEach((s, i) => checkStep(s, `${path}.steps[${i}]`, out))
      break
  }
}

function checkButton(b: Button, pages: Page[], icons: Set<string>, path: string, out: Issue[]) {
  if (icons.size && !icons.has(b.icon)) out.push({ path: `${path}.icon`, message: `Unknown icon "${b.icon}"` })
  if (b.label.length > LIMITS.label) out.push({ path: `${path}.label`, message: `Label is longer than ${LIMITS.label} characters` })
  if (!COLOR_ROLES.some((c) => c.value === b.color)) out.push({ path: `${path}.color`, message: 'Unknown color' })
  if (b.shape !== undefined && !(BUTTON_SHAPES as readonly string[]).includes(b.shape))
    out.push({ path: `${path}.shape`, message: `Unknown shape "${b.shape}"` })
  checkAction(b.tap, pages, `${path}.tap`, out)
  if (b.hold) checkAction(b.hold, pages, `${path}.hold`, out)
}

function checkPage(pg: Page, pages: Page[], icons: Set<string>, path: string, out: Issue[]) {
  if (!ID_RE.test(pg.id)) out.push({ path: `${path}.id`, message: 'Page id must be 1-24 of a-z, 0-9 and -' })
  switch (pg.type) {
    case 'radial': {
      if (pg.buttons.length < 1) out.push({ path: `${path}.buttons`, message: 'Add at least one button on the ring' })
      if (pg.buttons.length > LIMITS.buttons) out.push({ path: `${path}.buttons`, message: `At most ${LIMITS.buttons} buttons` })
      const ids = buttonIds(pg)
      if (new Set(ids).size !== ids.length) out.push({ path: `${path}.buttons`, message: 'Two buttons share an id' })
      pg.buttons.forEach((b, i) => checkButton(b, pages, icons, `${path}.buttons[${i}]`, out))
      if (pg.center) checkButton(pg.center, pages, icons, `${path}.center`, out)
      break
    }
    case 'media':
      if (pg.shapes !== undefined) {
        if (!pg.shapes || typeof pg.shapes !== 'object' || Array.isArray(pg.shapes)) {
          out.push({ path: `${path}.shapes`, message: 'Shapes must be an object' })
          break
        }
        for (const [k, v] of Object.entries(pg.shapes)) {
          if (!(k in MEDIA_SHAPE_DEFAULTS)) out.push({ path: `${path}.shapes.${k}`, message: `Unknown media button "${k}"` })
          else if (!(BUTTON_SHAPES as readonly string[]).includes(v as string))
            out.push({ path: `${path}.shapes.${k}`, message: `Unknown shape "${v}"` })
        }
      }
      break
    case 'stats':
      if (pg.items.length < 1 || pg.items.length > LIMITS.stats)
        out.push({ path: `${path}.items`, message: `Pick 1 to ${LIMITS.stats} gauges` })
      break
    case 'dial':
      if (icons.size && !icons.has(pg.icon)) out.push({ path: `${path}.icon`, message: `Unknown icon "${pg.icon}"` })
      if (pg.label.length > LIMITS.label) out.push({ path: `${path}.label`, message: `Label is longer than ${LIMITS.label} characters` })
      if (pg.source === 'custom') {
        if (pg.step != null && (!Number.isInteger(pg.step) || pg.step < 1 || pg.step > 20))
          out.push({ path: `${path}.step`, message: 'Step must be 1 to 20' })
        if (pg.onUp) checkAction(pg.onUp, pages, `${path}.onUp`, out)
        if (pg.onDown) checkAction(pg.onDown, pages, `${path}.onDown`, out)
        if (!pg.onUp && !pg.onDown) out.push({ path, message: 'A custom dial needs an action for up or down' })
      }
      break
    case 'clock':
      if (pg.style != null && pg.style !== 'analog' && pg.style !== 'digital')
        out.push({ path: `${path}.style`, message: 'Clock style must be analog or digital' })
      if (pg.weather != null && typeof pg.weather !== 'boolean') out.push({ path: `${path}.weather`, message: 'Weather must be on or off' })
      if (pg.zones != null) {
        if (!Array.isArray(pg.zones)) {
          out.push({ path: `${path}.zones`, message: 'World clocks must be a list' })
          break
        }
        if (pg.zones.length > LIMITS.zones) out.push({ path: `${path}.zones`, message: `At most ${LIMITS.zones} world clocks` })
        pg.zones.forEach((z, i) => {
          const zp = `${path}.zones[${i}]`
          if (!z || typeof z.tz !== 'string' || !validTimeZone(z.tz)) out.push({ path: `${zp}.tz`, message: `Unknown time zone "${z?.tz ?? ''}"` })
          if (typeof z?.label !== 'string' || !z.label.trim()) out.push({ path: `${zp}.label`, message: 'World clock label is empty' })
          else if (z.label.length > LIMITS.zoneLabel) out.push({ path: `${zp}.label`, message: `Label is longer than ${LIMITS.zoneLabel} characters` })
        })
      }
      break
    case 'timer':
      if (pg.minutes != null) {
        if (!Array.isArray(pg.minutes) || pg.minutes.length < 1 || pg.minutes.length > LIMITS.timerPresets)
          out.push({ path: `${path}.minutes`, message: `Pick 1 to ${LIMITS.timerPresets} presets` })
        else
          pg.minutes.forEach((m, i) => {
            if (!Number.isInteger(m) || m < TIMER_DEFAULTS.minMinutes || m > TIMER_DEFAULTS.maxMinutes)
              out.push({ path: `${path}.minutes[${i}]`, message: `Presets are ${TIMER_DEFAULTS.minMinutes} to ${TIMER_DEFAULTS.maxMinutes} minutes` })
          })
      }
      if (pg.pomodoro != null && typeof pg.pomodoro !== 'boolean') out.push({ path: `${path}.pomodoro`, message: 'Pomodoro must be on or off' })
      if (pg.label != null && (typeof pg.label !== 'string' || pg.label.length > LIMITS.timerLabel))
        out.push({ path: `${path}.label`, message: `Label is longer than ${LIMITS.timerLabel} characters` })
      break
    case 'image':
      if (pg.images.length < 1) out.push({ path: `${path}.images`, message: 'Add an image' })
      if (pg.images.length > LIMITS.images) out.push({ path: `${path}.images`, message: `At most ${LIMITS.images} images` })
      if (pg.images.length > 1 && pg.images.some(isGif)) out.push({ path: `${path}.images`, message: 'A GIF must be the only image of its page' })
      pg.images.forEach((id, i) => {
        if (!ASSET_ID_RE.test(id)) out.push({ path: `${path}.images[${i}]`, message: `Unknown image "${id}"` })
      })
      if (pg.interval != null && !(typeof pg.interval === 'number' && pg.interval >= 2 && pg.interval <= 3600))
        out.push({ path: `${path}.interval`, message: 'Slideshow interval must be 2 to 3600 s' })
      if (pg.fit != null && pg.fit !== 'cover' && pg.fit !== 'contain') out.push({ path: `${path}.fit`, message: 'Fit must be cover or contain' })
      break
    case 'mixer':
    case 'windows':
    case 'clipboard':
    case 'discord':
      break
    case 'touchpad':
      if (pg.speed != null && !(Number.isInteger(pg.speed) && pg.speed >= 1 && pg.speed <= 10))
        out.push({ path: `${path}.speed`, message: 'Pointer speed must be 1 to 10' })
      break
  }
}

export function validate(c: Config, icons: Set<string>): Issue[] {
  const out: Issue[] = []
  const d = c.device
  if (!(d.brightness >= 5 && d.brightness <= 100)) out.push({ path: 'device.brightness', message: 'Brightness must be 5 to 100' })
  if (!(ROTATIONS as readonly unknown[]).includes(d.rotation)) out.push({ path: 'device.rotation', message: 'Rotation must be 0, 90, 180 or 270' })
  if (!(d.dimAfter >= 0)) out.push({ path: 'device.dimAfter', message: 'Dim delay must be 0 or more' })
  if (!(d.sleepAfter >= 0)) out.push({ path: 'device.sleepAfter', message: 'Sleep delay must be 0 or more' })
  if (!(d.longPressMs >= 150 && d.longPressMs <= 3000)) out.push({ path: 'device.longPressMs', message: 'Long press must be 150 to 3000 ms' })
  if (!inRange(c.media.blur)) out.push({ path: 'media.blur', message: 'Blur must be 0 to 100' })
  if (!inRange(c.media.coverOpacity)) out.push({ path: 'media.coverOpacity', message: 'Album art opacity must be 0 to 100' })
  if (!/^#[0-9a-f]{6}$/i.test(c.theme.seed)) out.push({ path: 'theme.seed', message: 'Seed must be a #RRGGBB color' })
  if (typeof d.micIndicator !== 'boolean') out.push({ path: 'device.micIndicator', message: 'Mic indicator must be on or off' })
  if (typeof d.notifications !== 'boolean') out.push({ path: 'device.notifications', message: 'Notification panel must be on or off' })
  if (typeof d.notifToast !== 'boolean') out.push({ path: 'device.notifToast', message: 'New notifications must be on or off' })
  if (!['off', 'clock', 'photos'].includes(d.screensaver)) out.push({ path: 'device.screensaver', message: 'Screen saver must be off, clock or photos' })
  if (!(Number.isInteger(d.screensaverAfter) && d.screensaverAfter >= SCREENSAVER_DEFAULTS.minAfter && d.screensaverAfter <= SCREENSAVER_DEFAULTS.maxAfter))
    out.push({ path: 'device.screensaverAfter', message: `Screen saver delay must be ${SCREENSAVER_DEFAULTS.minAfter} to ${SCREENSAVER_DEFAULTS.maxAfter} s` })
  if (d.screensaverImages.length > LIMITS.images) out.push({ path: 'device.screensaverImages', message: `At most ${LIMITS.images} screen saver photos` })
  d.screensaverImages.forEach((id, i) => {
    if (!ASSET_ID_RE.test(id)) out.push({ path: `device.screensaverImages[${i}]`, message: `Unknown image "${id}"` })
    else if (isGif(id)) out.push({ path: `device.screensaverImages[${i}]`, message: 'The screen saver shows still pictures only, not GIFs' })
  })
  if (d.screensaver === 'photos' && !d.screensaverImages.length)
    out.push({ path: 'device.screensaverImages', message: 'Add a photo for the screen saver, or pick another mode' })
  const w = c.weather
  if (w) {
    if (typeof w.city !== 'string' || !w.city.trim()) out.push({ path: 'weather.city', message: 'Weather city is empty' })
    if (!(typeof w.lat === 'number' && w.lat >= -90 && w.lat <= 90)) out.push({ path: 'weather.lat', message: 'Latitude must be -90 to 90' })
    if (!(typeof w.lon === 'number' && w.lon >= -180 && w.lon <= 180)) out.push({ path: 'weather.lon', message: 'Longitude must be -180 to 180' })
    if (w.units !== 'metric' && w.units !== 'imperial') out.push({ path: 'weather.units', message: 'Units must be metric or imperial' })
  }

  if (!c.profiles.some((p) => p.id === 'default')) out.push({ path: 'profiles', message: 'The default profile is missing' })
  const pids = new Set<string>()
  c.profiles.forEach((p, i) => {
    const path = `profiles[${i}]`
    if (!ID_RE.test(p.id)) out.push({ path: `${path}.id`, message: 'Profile id must be 1-24 of a-z, 0-9 and -' })
    if (pids.has(p.id)) out.push({ path: `${path}.id`, message: `Profile id "${p.id}" is used twice` })
    pids.add(p.id)
    if (!p.name.trim()) out.push({ path: `${path}.name`, message: 'Profile name is empty' })
    if (p.seed !== undefined && !validHex(p.seed)) out.push({ path: `${path}.seed`, message: 'Profile color must be a #RRGGBB color' })
    if (p.palette !== undefined) {
      if (!p.palette || typeof p.palette !== 'object') out.push({ path: `${path}.palette`, message: 'Profile palette must be an object' })
      else
        for (const r of PALETTE_ROLES)
          if (typeof p.palette[r] !== 'string' || !validHex(p.palette[r])) out.push({ path: `${path}.palette.${r}`, message: `Palette role ${r} must be a #RRGGBB color` })
    }
    p.match.forEach((m, j) => {
      if (m !== m.toLowerCase() || !m.trim()) out.push({ path: `${path}.match[${j}]`, message: 'Window classes are lowercase' })
    })
    if (p.pages.length < 1 || p.pages.length > LIMITS.pages)
      out.push({ path: `${path}.pages`, message: `A profile has 1 to ${LIMITS.pages} pages` })
    const pg = new Set<string>()
    p.pages.forEach((page, j) => {
      if (pg.has(page.id)) out.push({ path: `${path}.pages[${j}].id`, message: `Page id "${page.id}" is used twice` })
      pg.add(page.id)
      checkPage(page, p.pages, icons, `${path}.pages[${j}]`, out)
    })
  })
  return out
}

export function duplicateProfile(p: Profile, taken: string[]): Profile {
  const copy = JSON.parse(JSON.stringify(p)) as Profile
  copy.id = uniqueId(p.id === 'default' ? 'profile' : p.id, taken)
  copy.name = `${p.name} copy`
  copy.match = []
  return copy
}

// A deep copy of `page` ready to append to `target`: a fresh page id, unique button ids and
// `page` actions pointing at pages the target lacks redirected to its first page.
export function pastePage(page: Page, target: Profile): Page {
  const copy = JSON.parse(JSON.stringify(page)) as Page
  const taken = target.pages.map((p) => p.id)
  copy.id = uniqueId(page.id, taken)
  const ids = new Set([...taken, copy.id])
  const first = target.pages[0]?.id ?? copy.id
  const fix = (a: Action | undefined) => {
    if (a?.type === 'page' && !ids.has(a.page)) a.page = first
  }
  if (copy.type === 'radial') {
    const used = new Set<string>()
    for (const b of [...copy.buttons, ...(copy.center ? [copy.center] : [])]) {
      if (!ID_RE.test(b.id) || used.has(b.id)) {
        let i = 1
        while (used.has(`b${i}`) || buttonIds(copy).includes(`b${i}`)) i++
        b.id = `b${i}`
      }
      used.add(b.id)
      fix(b.tap)
      fix(b.hold)
    }
  } else if (copy.type === 'dial') {
    fix(copy.onUp)
    fix(copy.onDown)
  } else if (copy.type === 'image') {
    // Asset ids are global: the copy shows the same files
    copy.images = Array.isArray(copy.images) ? copy.images : []
  }
  return copy
}

export function describeAction(a: Action | undefined, pages: Page[] = []): string {
  if (!a) return 'Nothing'
  switch (a.type) {
    case 'keys':
      return a.keys || 'No key yet'
    case 'text':
      return a.text ? `Type "${a.text}"` : 'No text yet'
    case 'media':
      return MEDIA_CMDS.find((m) => m.value === a.cmd)?.label ?? a.cmd
    case 'page': {
      const p = pages.find((x) => x.id === a.page)
      return p ? `Go to ${p.title || p.id}` : 'No page yet'
    }
    case 'command':
      return a.cmd || 'No command yet'
    case 'open':
      return a.target ? `Open ${a.target}` : 'Nothing to open yet'
    case 'mic':
      return `Mic: ${MIC_CMDS.find((m) => m.value === a.cmd)?.label.toLowerCase() ?? a.cmd}`
    case 'macro':
      return `${a.steps.length} step macro`
  }
}
