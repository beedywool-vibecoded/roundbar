// Config, status and action types, mirroring docs/spec.md sections 1, 3 and 4.
import type { ButtonShape } from './shapes'

export type { ButtonShape }

export type ThemeMode = 'dark' | 'light'

export const PALETTE_ROLES = [
  'surface',
  'surfaceContainer',
  'surfaceContainerHigh',
  'surfaceContainerHighest',
  'onSurface',
  'onSurfaceVariant',
  'primary',
  'onPrimary',
  'primaryContainer',
  'onPrimaryContainer',
  'secondaryContainer',
  'onSecondaryContainer',
  'tertiary',
  'tertiaryContainer',
  'onTertiaryContainer',
  'error',
  'errorContainer',
  'onErrorContainer',
  'outline',
  'outlineVariant',
] as const
export type PaletteRole = (typeof PALETTE_ROLES)[number]
export type Palette = Record<PaletteRole, string>

export interface Theme {
  seed: string
  mode: ThemeMode
  palette: Palette
}

export type KeyboardLayout = 'fr' | 'us'

export type Rotation = 0 | 90 | 180 | 270

export interface DeviceSettings {
  brightness: number
  /** Degrees clockwise */
  rotation: Rotation
  dimAfter: number
  sleepAfter: number
  longPressMs: number
  keyboardLayout: KeyboardLayout
  /** Screen off when the PC goes away, on when it comes back. Missing = true */
  followPc: boolean
  /** Double tap near the left/right edge. Missing = "page" */
  edgeDoubleTap: EdgeDoubleTap
  /** Replaces the dim step after screensaverAfter seconds idle. Missing = "off" */
  screensaver: Screensaver
  /** Seconds, 10..3600. Missing = 60 */
  screensaverAfter: number
  /** Asset ids for "photos", stills only, up to 20 */
  screensaverImages: string[]
  /** Pull up from the bottom edge for the notification panel. Missing = true */
  notifications: boolean
  /** Show new notifications briefly. Missing = true */
  notifToast: boolean
}

export type Screensaver = 'off' | 'clock' | 'photos'

export type WeatherUnits = 'metric' | 'imperial'
export interface Weather {
  city: string
  lat: number
  lon: number
  units: WeatherUnits
}

export type EdgeDoubleTap = 'page' | 'quickSettings' | 'screenOff' | 'off'

export type MediaCmd = 'play_pause' | 'next' | 'prev' | 'vol_up' | 'vol_down' | 'mute'
export type MicCmd = 'toggle' | 'mute' | 'unmute'

export type Step = { keys: string } | { text: string } | { delay: number } | { command: string }

export type Action =
  | { type: 'keys'; keys: string }
  | { type: 'text'; text: string }
  | { type: 'media'; cmd: MediaCmd }
  | { type: 'page'; page: string }
  | { type: 'command'; cmd: string }
  | { type: 'open'; target: string }
  | { type: 'mic'; cmd: MicCmd }
  | { type: 'macro'; steps: Step[] }

export type ActionType = Action['type']

export type ColorRole = 'primary' | 'secondary' | 'tertiary' | 'error' | 'surface'

export interface Button {
  id: string
  icon: string
  label: string
  color: ColorRole
  // Missing: circle on the ring, rounded square in the center
  shape?: ButtonShape
  tap: Action
  hold?: Action
}

export type StatItem = 'cpu' | 'cpuTemp' | 'ram' | 'gpu' | 'gpuTemp' | 'vram'
export type DialSource = 'volume' | 'mic' | 'brightness' | 'custom'

interface PageBase {
  id: string
  title?: string
}
export interface RadialPage extends PageBase {
  type: 'radial'
  center?: Button
  buttons: Button[]
}
export type MediaButton = 'prev' | 'next' | 'play' | 'playing'
export interface MediaPage extends PageBase {
  type: 'media'
  /** Transport button shapes; missing keys use MEDIA_SHAPE_DEFAULTS */
  shapes?: Partial<Record<MediaButton, ButtonShape>>
}
export interface StatsPage extends PageBase {
  type: 'stats'
  items: StatItem[]
}
export interface DialPage extends PageBase {
  type: 'dial'
  icon: string
  label: string
  source: DialSource
  step?: number
  onUp?: Action
  onDown?: Action
}
export type ImageFit = 'cover' | 'contain'
export interface ImagePage extends PageBase {
  type: 'image'
  /** Asset ids (docs/spec.md, Assets). A `.gif` must be the only one. */
  images: string[]
  /** Slideshow seconds per image, 2..3600; missing = IMAGE_DEFAULTS.interval */
  interval?: number
  /** Missing = cover */
  fit?: ImageFit
}
export interface TouchpadPage extends PageBase {
  type: 'touchpad'
  /** Pointer speed 1..10; missing = TOUCHPAD_DEFAULTS.speed */
  speed?: number
}
export type ClockStyle = 'analog' | 'digital'
export interface ClockZone {
  label: string
  /** IANA time zone, e.g. "Asia/Tokyo" */
  tz: string
}
export interface ClockPage extends PageBase {
  type: 'clock'
  /** Missing = analog */
  style?: ClockStyle
  /** Missing = true; needs the top-level weather */
  weather?: boolean
  /** World clocks, 0..3 */
  zones?: ClockZone[]
}
export interface TimerPage extends PageBase {
  type: 'timer'
  /** Presets in minutes, 1..4 values of 1..180; missing = TIMER_DEFAULTS.minutes */
  minutes?: number[]
  /** 25 min work / 5 min break cycles */
  pomodoro?: boolean
  label?: string
}
/** Pages without options: the daemon sends their content live */
export interface MixerPage extends PageBase {
  type: 'mixer'
}
export interface WindowsPage extends PageBase {
  type: 'windows'
}
export interface ClipboardPage extends PageBase {
  type: 'clipboard'
}
export interface DiscordPage extends PageBase {
  type: 'discord'
}
export type Page =
  | RadialPage
  | MediaPage
  | StatsPage
  | DialPage
  | ClockPage
  | TimerPage
  | MixerPage
  | WindowsPage
  | ClipboardPage
  | DiscordPage
  | ImagePage
  | TouchpadPage
export type PageType = Page['type']

export interface Profile {
  id: string
  name: string
  match: string[]
  /** Own seed color; the configurator writes `palette` from it for theme.mode */
  seed?: string
  palette?: Palette
  pages: Page[]
}

export interface MediaSettings {
  coverBackground: boolean
  coverColors: boolean
  /** 0 (nearly sharp) … 100 (very soft) */
  blur: number
  /** How much the album art shows through the surface-colored scrim, 0..100 */
  coverOpacity: number
}

export interface Config {
  version: 1
  theme: Theme
  media: MediaSettings
  /** Location for clock pages; missing = no weather */
  weather?: Weather
  device: DeviceSettings
  profiles: Profile[]
}

// POST /api/assets
export interface AssetInfo {
  id: string
  kind: 'image' | 'gif'
  w: number
  h: number
  size: number
}

export interface App {
  id: string
  name: string
  class?: string
}

export interface View {
  profile: string
  page: string
  kind: PageType
  awake: boolean
}

// GET /api/status and the SSE `status` event. Only `daemon` is set when the daemon is down.
export interface Status {
  daemon: boolean
  device?: { connected: boolean; port?: string; fw?: string; cfg?: string; inSync?: boolean }
  focus?: App | null
  profile?: string
  recentApps?: App[]
  view?: View | null
  extension?: boolean
  /** Discord RPC session of the daemon */
  discord?: DiscordStatus
}

export interface DiscordStatus {
  connected: boolean
  user?: string
  /** "" or missing: not in a voice channel */
  channel?: string
  error?: string
}

// GET /api/firmware: can this machine build and flash, and which version is built
export interface FirmwareInfo {
  repo: string
  repoOk: boolean
  /** ESP-IDF export script, "" when not found */
  idf: string
  /** Version of the last build, "" when none */
  built: string
  running: boolean
}

// SSE `flash` events during a firmware update
export type FlashEvent = { line: string } | { done: true; ok: boolean }
