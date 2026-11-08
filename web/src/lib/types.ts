// Config, status and action types, mirroring docs/spec.md sections 1, 3 and 4.

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

export interface DeviceSettings {
  brightness: number
  dimAfter: number
  sleepAfter: number
  longPressMs: number
  keyboardLayout: KeyboardLayout
}

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
export interface MediaPage extends PageBase {
  type: 'media'
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
export type Page = RadialPage | MediaPage | StatsPage | DialPage
export type PageType = Page['type']

export interface Profile {
  id: string
  name: string
  match: string[]
  pages: Page[]
}

export interface Config {
  version: 1
  theme: Theme
  device: DeviceSettings
  profiles: Profile[]
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
}
