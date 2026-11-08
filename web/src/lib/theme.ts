// Material You on the web: the whole palette is derived from one seed color with the
// same tonal-spot scheme Android uses for wallpaper colors. The configurator follows the
// device's seed, in the system's dark or light mode; the device gets its own palette.
import {
  Hct,
  MaterialDynamicColors as C,
  SchemeTonalSpot,
  argbFromHex,
  hexFromArgb,
} from '@material/material-color-utilities'
import { ref, watch } from 'vue'
import type { Palette, PaletteRole, ThemeMode } from './types'

export const SEEDS = [
  { name: 'Purple', hex: '#6750A4' },
  { name: 'Teal', hex: '#006A6A' },
  { name: 'Blue', hex: '#0061A4' },
  { name: 'Pink', hex: '#984061' },
  { name: 'Green', hex: '#386A20' },
  { name: 'Orange', hex: '#8B5000' },
  { name: 'Red', hex: '#B3261E' },
  { name: 'Indigo', hex: '#4355B9' },
  { name: 'Olive', hex: '#5B6300' },
  { name: 'Slate', hex: '#5B5F72' },
] as const

export const DEFAULT_SEED = SEEDS[0].hex

export function validHex(s: string): boolean {
  return /^#[0-9a-f]{6}$/i.test(s)
}

function load(key: string, def: string): string {
  try {
    return localStorage.getItem(key) ?? def
  } catch {
    return def
  }
}
function save(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // private mode: the choice just lasts for this visit
  }
}

// Remembered so the first paint already has the right colors before the config loads.
export const seed = ref(load('roundbar.seed', DEFAULT_SEED))

type Role = { getArgb(s: unknown): number }
const role = (r: unknown) => r as Role

const ROLES: Record<string, unknown> = {
  '--surface': C.surface,
  '--scc': C.surfaceContainerLowest,
  '--scl': C.surfaceContainerLow,
  '--sc': C.surfaceContainer,
  '--sch': C.surfaceContainerHigh,
  '--schh': C.surfaceContainerHighest,
  '--primary': C.primary,
  '--on-primary': C.onPrimary,
  '--pc': C.primaryContainer,
  '--on-pc': C.onPrimaryContainer,
  '--secc': C.secondaryContainer,
  '--on-secc': C.onSecondaryContainer,
  '--tertiary': C.tertiary,
  '--tc': C.tertiaryContainer,
  '--on-tc': C.onTertiaryContainer,
  '--error': C.error,
  '--on-error': C.onError,
  '--ec': C.errorContainer,
  '--on-ec': C.onErrorContainer,
  '--on-surface': C.onSurface,
  '--on-sv': C.onSurfaceVariant,
  '--outline': C.outline,
  '--outline-v': C.outlineVariant,
}

// Device palette roles, exactly the names of spec section 1.
const DEVICE_ROLES: Record<PaletteRole, unknown> = {
  surface: C.surface,
  surfaceContainer: C.surfaceContainer,
  surfaceContainerHigh: C.surfaceContainerHigh,
  surfaceContainerHighest: C.surfaceContainerHighest,
  onSurface: C.onSurface,
  onSurfaceVariant: C.onSurfaceVariant,
  primary: C.primary,
  onPrimary: C.onPrimary,
  primaryContainer: C.primaryContainer,
  onPrimaryContainer: C.onPrimaryContainer,
  secondaryContainer: C.secondaryContainer,
  onSecondaryContainer: C.onSecondaryContainer,
  tertiary: C.tertiary,
  tertiaryContainer: C.tertiaryContainer,
  onTertiaryContainer: C.onTertiaryContainer,
  error: C.error,
  errorContainer: C.errorContainer,
  onErrorContainer: C.onErrorContainer,
  outline: C.outline,
  outlineVariant: C.outlineVariant,
}

function scheme(hex: string, dark: boolean) {
  return new SchemeTonalSpot(Hct.fromInt(argbFromHex(validHex(hex) ? hex : DEFAULT_SEED)), dark, 0)
}

// The full palette the device reads, as uppercase #RRGGBB strings.
export function devicePalette(hex: string, mode: ThemeMode): Palette {
  const s = scheme(hex, mode === 'dark')
  const out = {} as Palette
  for (const [name, r] of Object.entries(DEVICE_ROLES) as [PaletteRole, unknown][]) {
    out[name] = hexFromArgb(role(r).getArgb(s)).toUpperCase()
  }
  return out
}

const media = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null

export function isDark(): boolean {
  return media?.matches ?? true
}

export function applyTheme() {
  const dark = isDark()
  const s = scheme(seed.value, dark)
  const root = document.documentElement
  root.dataset.theme = dark ? 'dark' : 'light'
  for (const [name, r] of Object.entries(ROLES)) root.style.setProperty(name, hexFromArgb(role(r).getArgb(s)))
  // Key names and commands use a dark surface in light mode too.
  const night = dark ? s : scheme(seed.value, true)
  const tone = (r: unknown) => hexFromArgb(role(r).getArgb(night))
  root.style.setProperty('--code-bg', tone(C.surfaceContainerLowest))
  root.style.setProperty('--code-fg', tone(C.onSurface))
  root.style.setProperty('--code-border', tone(C.outlineVariant))
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', hexFromArgb(role(C.surface).getArgb(s)))
}

export function initTheme() {
  applyTheme()
  media?.addEventListener('change', applyTheme)
  watch(seed, () => {
    if (!validHex(seed.value)) return
    save('roundbar.seed', seed.value)
    applyTheme()
  })
}
