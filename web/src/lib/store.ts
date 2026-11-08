// App state: the config being edited, the last saved copy, the icon list and the live
// daemon status. Edits stay local until Save.
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { ApiError, api, streamStatus } from './api'
import { matchesAny, normalize, starterConfig, syncProfilePalette, validate, type Issue } from './model'
import { devicePalette, seed, validHex } from './theme'
import type { Config, Palette, Profile, Status, UIState } from './types'
import { toast } from './ui'

export const config = ref<Config | null>(null)
const saved = ref('')
export const loadError = ref('')
// No config file yet: the editor starts from a sample that Save writes out.
export const isNew = ref(false)
export const saving = ref(false)
export const icons = shallowRef<string[]>([])
export const iconSet = computed(() => new Set(icons.value))

export const live = reactive<{ status: Status | null; connected: boolean }>({ status: null, connected: false })
export const daemonUp = computed(() => live.status?.daemon === true)

// A window matches a profile entry by app id or by window class.
export function focusKeys(): string[] {
  const f = live.status?.focus
  return [f?.id, f?.class].filter((x): x is string => !!x).map((x) => x.toLowerCase())
}

// Does a profile's match list catch the focused window? Entries may hold `*` wildcards.
export function matchesFocus(match: string[]): boolean {
  const keys = focusKeys()
  return keys.length > 0 && matchesAny(match, keys)
}

const json = computed(() => (config.value ? JSON.stringify(config.value) : ''))
export const dirty = computed(() => !!config.value && json.value !== saved.value)

// ---- undo / redo of unsaved edits ----
// Snapshots are JSON strings (deep copies). A burst of changes (typing, a slider drag) settles
// for HISTORY_SETTLE_MS before it becomes one step. Cleared on load and after save.
const HISTORY_MAX = 50
const HISTORY_SETTLE_MS = 600
const past = shallowRef<string[]>([])
const future = shallowRef<string[]>([])
// The state the newest step leads to; edits after it are pending until they settle
let committed = ''
const pending = ref(false)
let settleTimer: ReturnType<typeof setTimeout> | undefined

function settle() {
  clearTimeout(settleTimer)
  settleTimer = undefined
  pending.value = false
  const now = json.value
  if (!now || now === committed) return
  if (committed) past.value = [...past.value, committed].slice(-HISTORY_MAX)
  committed = now
}

watch(json, (now) => {
  if (now === committed) {
    // Back where the last step left off (or an undo / redo just applied)
    clearTimeout(settleTimer)
    settleTimer = undefined
    pending.value = false
    return
  }
  if (future.value.length) future.value = []
  pending.value = true
  clearTimeout(settleTimer)
  settleTimer = setTimeout(settle, HISTORY_SETTLE_MS)
})

function resetHistory() {
  clearTimeout(settleTimer)
  settleTimer = undefined
  pending.value = false
  past.value = []
  future.value = []
  committed = json.value
}

export const canUndo = computed(() => past.value.length > 0 || pending.value)
export const canRedo = computed(() => future.value.length > 0 && !pending.value)

function apply(snapshot: string) {
  committed = snapshot
  config.value = JSON.parse(snapshot) as Config
}

export function undo() {
  if (pending.value) settle()
  const prev = past.value[past.value.length - 1]
  if (prev === undefined) return
  past.value = past.value.slice(0, -1)
  future.value = [...future.value, committed]
  apply(prev)
}

export function redo() {
  if (pending.value) return
  const next = future.value[future.value.length - 1]
  if (next === undefined) return
  future.value = future.value.slice(0, -1)
  past.value = [...past.value, committed].slice(-HISTORY_MAX)
  apply(next)
}
export const issues = computed<Issue[]>(() => (config.value ? validate(config.value, iconSet.value) : []))

export function issuesUnder(prefix: string): Issue[] {
  return issues.value.filter((i) => i.path === prefix || i.path.startsWith(`${prefix}.`) || i.path.startsWith(`${prefix}[`))
}

export function profileIndex(id: string): number {
  return config.value?.profiles.findIndex((p) => p.id === id) ?? -1
}
export function findProfile(id: string): Profile | undefined {
  return config.value?.profiles.find((p) => p.id === id)
}

export async function loadAll() {
  loadError.value = ''
  try {
    const [cfg, ic] = await Promise.all([
      // A missing file may come back as 404 or as null: both start from the sample.
      api.config().catch((e) => {
        if (e instanceof ApiError && e.status === 404) return { config: null, fileError: '' }
        throw e
      }),
      api.icons().catch(() => ({})),
    ])
    icons.value = Object.keys(ic ?? {}).sort()
    const c = normalize(cfg.config)
    saved.value = cfg.config ? JSON.stringify(c) : ''
    isNew.value = !cfg.config
    config.value = c
    resetHistory()
    if (!uiState.value) void loadUI()
    if (cfg.fileError) toast(`The config file is invalid, showing defaults: ${cfg.fileError}`, true)
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
}

// This PC's configurator state (ui.json): has the setup been done or skipped
export const uiState = ref<UIState | null>(null)
export async function loadUI() {
  try {
    uiState.value = await api.ui()
  } catch {
    uiState.value = { setupDone: true } // an older server: never force the setup
  }
}
export async function setSetupDone(done = true) {
  uiState.value = { ...(uiState.value ?? { setupDone: done }), setupDone: done }
  try {
    uiState.value = await api.saveUI({ setupDone: done })
  } catch {
    // kept for this tab
  }
}

export function discard() {
  config.value = saved.value ? (JSON.parse(saved.value) as Config) : starterConfig()
}

export async function save(): Promise<boolean> {
  const c = config.value
  if (!c) return false
  const first = issues.value[0]
  if (first) {
    toast(`${first.message} (${first.path})`, true)
    return false
  }
  // The device only reads the palette: recompute it from the seed and mode every time.
  c.theme.palette = devicePalette(c.theme.seed, c.theme.mode)
  for (const p of c.profiles) syncProfilePalette(p, c.theme.mode)
  saving.value = true
  try {
    const sent = JSON.stringify(c)
    const back = await api.saveConfig(c)
    const written = back && typeof back === 'object' ? normalize(back) : (JSON.parse(sent) as Config)
    // Show what the server wrote, unless the user kept editing during the request
    if (JSON.stringify(config.value) === sent) config.value = written
    saved.value = JSON.stringify(written)
    resetHistory()
    isNew.value = false
    toast('Saved. The daemon sends it to the device.')
    return true
  } catch (e) {
    if (e instanceof ApiError && e.status === 400) toast(e.path ? `${e.message} (${e.path})` : e.message, true)
    else toast(e instanceof Error ? e.message : String(e), true)
    return false
  } finally {
    saving.value = false
  }
}

// The configurator's own colors follow the device seed.
watch(
  () => config.value?.theme.seed,
  (s) => s && (seed.value = s),
)

// Profile palettes always match the device mode.
watch(
  () => config.value?.theme.mode,
  (mode) => {
    if (mode) for (const p of config.value!.profiles) syncProfilePalette(p, mode)
  },
)

// The palette a profile's pages are drawn with: its own color, else the theme's.
export function profilePalette(p: Profile | undefined): Palette {
  const t = config.value!.theme
  return devicePalette(p?.seed && validHex(p.seed) ? p.seed : t.seed, t.mode)
}

let stopStream: (() => void) | undefined
export function startLive() {
  stopStream?.()
  api
    .status()
    .then((s) => (live.status = s))
    .catch(() => {})
  stopStream = streamStatus({
    status: (s) => (live.status = s),
    connected: (up) => (live.connected = up),
  })
}
