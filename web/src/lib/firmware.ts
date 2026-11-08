// Firmware release check and update, shared by the Firmware card and the setup. The normal
// path installs the latest GitHub release (downloaded and flashed by `roundbar config`
// itself, no toolchain needed); developers with the source checkout and ESP-IDF can also
// build and flash from it. Progress streams in as SSE `flash` events.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api, onFlash } from './api'
import { live } from './store'
import type { FirmwareInfo, FirmwareRelease, FlashEvent } from './types'
import { confirm, toast, toastError } from './ui'

const MAX_LINES = 200

export function useFirmware(opts: { onLine?: () => void } = {}) {
  const info = ref<FirmwareInfo | null>(null)
  const release = ref<FirmwareRelease | null>(null)
  const checking = ref(false)
  const loadError = ref('')
  const lines = ref<string[]>([])
  const running = ref(false)
  const result = ref<boolean | null>(null)
  const progress = ref<number | null>(null)
  const starting = ref(false)

  async function load(refresh = false) {
    try {
      info.value = await api.firmware()
      loadError.value = ''
      if (info.value.running) running.value = true
    } catch (e) {
      loadError.value = e instanceof Error ? e.message : String(e)
    }
    checking.value = true
    try {
      release.value = await api.release(refresh)
    } catch (e) {
      release.value = { error: e instanceof Error ? e.message : String(e) }
    } finally {
      checking.value = false
    }
  }

  function handle(e: FlashEvent) {
    if ('line' in e) {
      running.value = true
      lines.value.push(e.line)
      if (lines.value.length > MAX_LINES) lines.value.splice(0, lines.value.length - MAX_LINES)
      opts.onLine?.()
      return
    }
    if ('progress' in e) {
      running.value = true
      progress.value = e.progress.total ? Math.min(100, Math.round((e.progress.done / e.progress.total) * 100)) : null
      return
    }
    running.value = false
    progress.value = null
    result.value = e.ok
    if (e.ok) toast('Firmware updated. The device is restarting.')
    else toast('The firmware update failed. See the log for details.', true)
    void load()
  }
  let off: (() => void) | undefined
  onMounted(() => {
    off = onFlash(handle)
    void load()
  })
  onBeforeUnmount(() => off?.())

  const connected = computed(() => !!live.status?.device?.connected)
  const canBuild = computed(() => !!info.value?.repoOk && !!info.value?.idf)
  const busy = computed(() => running.value || !!info.value?.running)

  const releaseNote = computed(() => {
    const r = release.value
    if (!r || r.error) return ''
    switch (r.status) {
      case 'upToDate':
        return 'Up to date'
      case 'updateAvailable':
        return 'Update available'
      case 'deviceNewer':
        return 'The device runs a newer build'
      default:
        return ''
    }
  })

  const installLabel = computed(() => {
    const r = release.value
    if (!r?.version) return 'Install release'
    return r.status === 'upToDate' ? `Reinstall ${r.version}` : `Install ${r.version}`
  })

  const installBlocked = computed(() => {
    if (busy.value) return 'An update is running'
    const r = release.value
    if (!r) return 'Checking for releases…'
    if (r.error) return "Couldn't reach GitHub releases"
    return ''
  })

  async function start(source: 'release' | 'build') {
    starting.value = true
    lines.value = []
    result.value = null
    progress.value = null
    try {
      await api.updateFirmware(source)
      running.value = true
    } catch (e) {
      toastError(e)
    } finally {
      starting.value = false
    }
  }

  async function install() {
    if (installBlocked.value || starting.value) return
    const ok = await confirm({
      title: `Install ${release.value?.version ?? 'the latest release'}?`,
      body: connected.value
        ? "The device restarts in download mode, gets the new firmware and restarts again (under a minute; longer if the font pack changed). Don't unplug it."
        : "The device isn't connected. When asked, unplug it, hold its BOOT button while plugging it back in, then release the button. Don't unplug it during the update.",
      action: 'Install',
    })
    if (ok) await start('release')
  }

  async function buildAndFlash() {
    if (!canBuild.value || busy.value || starting.value) return
    const ok = await confirm({
      title: 'Build and flash from source?',
      body: `Builds ${info.value?.repo}/firmware with ESP-IDF and flashes it with esptool. The device restarts during the update (about 30 s). Don't unplug it.`,
      action: 'Build & flash',
    })
    if (ok) await start('build')
  }

  return {
    info,
    release,
    checking,
    loadError,
    lines,
    running,
    result,
    progress,
    starting,
    connected,
    canBuild,
    busy,
    releaseNote,
    installLabel,
    installBlocked,
    load,
    install,
    buildAndFlash,
  }
}

export function mb(n?: number) {
  return n ? `${(n / 1048576).toFixed(1)} MB` : ''
}
