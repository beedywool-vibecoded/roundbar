// State shared by the setup steps (views/SetupView.vue, components/setup/*).
import { computed, reactive } from 'vue'
import { api } from './api'
import { matchesAny, uniqueId } from './model'
import { PRESETS, appsFor, matchFor, type Preset } from './presets'
import { config } from './store'
import type { InstalledApp, InstalledApps, Profile } from './types'

export type StepId = 'board' | 'firmware' | 'desktop' | 'keyboard' | 'apps' | 'extras' | 'done'

export const STEP_LABELS: Record<StepId, string> = {
  board: 'Board',
  firmware: 'Firmware',
  desktop: 'Desktop',
  keyboard: 'Keyboard',
  apps: 'Apps',
  extras: 'Extras',
  done: 'Done',
}

export const WEB_INSTALLER = 'https://beedywool-vibecoded.github.io/roundbar/'

export const wiz = reactive({
  apps: null as InstalledApps | null,
  appsError: '',
  /** steps that need nothing from the user right now (they move on by themselves) */
  auto: {} as Partial<Record<StepId, boolean>>,
  /** a step that must not be left (firmware update running) */
  busy: false,
  /** ids of the profiles the setup added */
  added: [] as string[],
  /** the board came with its own setup: jump to Done */
  finishEarly: false,
})

export async function loadApps() {
  if (wiz.apps) return
  try {
    wiz.apps = await api.apps()
    wiz.appsError = ''
  } catch (e) {
    wiz.apps = { apps: [], tools: {} }
    wiz.appsError = e instanceof Error ? e.message : String(e)
  }
}

const has = (ids: string[]) => (wiz.apps?.apps ?? []).filter((a) => ids.includes(a.id) || ids.some((id) => a.keys?.includes(id)))

export interface Offer {
  preset: Preset
  found: InstalledApp[]
  match: string[]
  /** an existing profile already handles these apps */
  covered: boolean
}

export const offers = computed<Offer[]>(() => {
  const apps = wiz.apps?.apps ?? []
  const profiles = config.value?.profiles ?? []
  return PRESETS.flatMap((preset) => {
    const found = appsFor(preset, apps)
    if (!found.length) return []
    const match = matchFor(preset, found)
    const covered = profiles.some((p) => p.id !== 'default' && (p.id === preset.id || matchesAny(p.match, match)))
    return [{ preset, found, match, covered }]
  })
})

export const detected = computed(() => ({
  steam: has(['steam', 'com.valvesoftware.steam']).length > 0,
  obs: has(['com.obsproject.studio', 'obs', 'obs-studio']).length > 0,
  discord: has(['discord', 'com.discordapp.discord', 'discord-canary', 'discord-ptb']).length > 0,
  vesktop: has(['vesktop', 'dev.vencord.vesktop']).length > 0,
  mangohud: !!wiz.apps?.tools.mangohud,
}))

export const hasExtras = computed(() => {
  const d = detected.value
  return d.steam || d.obs || d.discord || d.vesktop || d.mangohud
})

/** Adds a profile from a preset to the config being edited; returns it. */
export function addPreset(preset: Preset, match: string[]): Profile {
  const profiles = config.value!.profiles
  const id = uniqueId(preset.id, profiles.map((p) => p.id))
  const p: Profile = { id, name: preset.name, ...preset.build(), match }
  profiles.push(p)
  wiz.added.push(id)
  return p
}

/** The profile game pages belong in: one matching Steam games. */
export function gamesProfile(): Profile | undefined {
  return config.value?.profiles.find((p) => p.id !== 'default' && matchesAny(p.match, ['steam_app_1']))
}
