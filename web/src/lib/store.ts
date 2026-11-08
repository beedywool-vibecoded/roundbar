// App state: the config being edited, the last saved copy, the icon list and the live
// daemon status. Edits stay local until Save.
import { computed, reactive, ref, shallowRef, watch } from 'vue'
import { ApiError, api, streamStatus } from './api'
import { normalize, starterConfig, validate, type Issue } from './model'
import { devicePalette, seed } from './theme'
import type { Config, Profile, Status } from './types'
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

const json = computed(() => (config.value ? JSON.stringify(config.value) : ''))
export const dirty = computed(() => !!config.value && json.value !== saved.value)
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
    if (cfg.fileError) toast(`The config file is invalid, showing defaults: ${cfg.fileError}`, true)
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
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
  saving.value = true
  try {
    const sent = JSON.stringify(c)
    const back = await api.saveConfig(c)
    const written = back && typeof back === 'object' ? normalize(back) : (JSON.parse(sent) as Config)
    // Show what the server wrote, unless the user kept editing during the request
    if (JSON.stringify(config.value) === sent) config.value = written
    saved.value = JSON.stringify(written)
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
