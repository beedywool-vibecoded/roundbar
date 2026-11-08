// Labels, defaults, id generation and validation for the config (docs/spec.md section 1).
import { comboError } from './keys'
import { BUTTON_SHAPES, type ButtonShape } from './shapes'
import { DEFAULT_SEED, devicePalette } from './theme'
import type {
  Action,
  ActionType,
  Button,
  ColorRole,
  Config,
  DialSource,
  MediaSettings,
  Page,
  PageType,
  MediaButton,
  Profile,
  StatItem,
  Step,
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
} as const

export const PAGE_TYPES: { value: PageType; label: string; icon: string; hint: string }[] = [
  { value: 'radial', label: 'Buttons', icon: 'apps', hint: 'Up to 8 buttons on a ring, plus an optional center button' },
  { value: 'media', label: 'Media', icon: 'music_note', hint: 'Now playing, transport controls and volume on the edge' },
  { value: 'stats', label: 'Stats', icon: 'speed', hint: 'Up to 4 gauges: CPU, GPU, memory and temperatures' },
  { value: 'dial', label: 'Dial', icon: 'tune', hint: 'One big arc you drag along the edge: volume, mic, brightness or custom' },
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
    device: { brightness: 80, rotation: 0, dimAfter: 30, sleepAfter: 300, longPressMs: 450, keyboardLayout: 'fr' },
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
  for (const p of cfg.profiles) {
    p.match ??= []
    p.pages ??= []
    p.name ??= p.id
    for (const pg of p.pages) {
      if (pg.type === 'radial') pg.buttons ??= []
      if (pg.type === 'stats') pg.items ??= []
    }
  }
  const d = cfg.profiles.findIndex((p) => p.id === 'default')
  if (d < 0) cfg.profiles.unshift(base.profiles[0]!)
  // The fallback is listed first
  else if (d > 0) cfg.profiles.unshift(...cfg.profiles.splice(d, 1))
  return cfg
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

  if (!c.profiles.some((p) => p.id === 'default')) out.push({ path: 'profiles', message: 'The default profile is missing' })
  const pids = new Set<string>()
  c.profiles.forEach((p, i) => {
    const path = `profiles[${i}]`
    if (!ID_RE.test(p.id)) out.push({ path: `${path}.id`, message: 'Profile id must be 1-24 of a-z, 0-9 and -' })
    if (pids.has(p.id)) out.push({ path: `${path}.id`, message: `Profile id "${p.id}" is used twice` })
    pids.add(p.id)
    if (!p.name.trim()) out.push({ path: `${path}.name`, message: 'Profile name is empty' })
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
