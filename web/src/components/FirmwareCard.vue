<script setup lang="ts">
// Firmware updates. The normal path installs the latest GitHub release (downloaded and
// flashed by `roundbar config` itself, no toolchain needed); developers with the source
// checkout and ESP-IDF can also build and flash from it. Progress streams in as SSE
// `flash` events on the status connection.
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api, onFlash } from '@/lib/api'
import { live } from '@/lib/store'
import type { FirmwareInfo, FirmwareRelease, FlashEvent } from '@/lib/types'
import { confirm, toast, toastError } from '@/lib/ui'

const MAX_LINES = 200
const info = ref<FirmwareInfo | null>(null)
const release = ref<FirmwareRelease | null>(null)
const checking = ref(false)
const loadError = ref('')
const lines = ref<string[]>([])
const running = ref(false)
const result = ref<boolean | null>(null)
const progress = ref<number | null>(null)
const logBox = ref<HTMLElement>()
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

let stick = true
function onScroll() {
  const el = logBox.value
  if (el) stick = el.scrollHeight - el.scrollTop - el.clientHeight < 24
}
async function append(line: string) {
  lines.value.push(line)
  if (lines.value.length > MAX_LINES) lines.value.splice(0, lines.value.length - MAX_LINES)
  if (!stick) return
  await nextTick()
  logBox.value?.scrollTo({ top: logBox.value.scrollHeight })
}
function handle(e: FlashEvent) {
  if ('line' in e) {
    running.value = true
    void append(e.line)
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
  if (!r) return ''
  if (r.error) return ''
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

function mb(n?: number) {
  return n ? `${(n / 1048576).toFixed(1)} MB` : ''
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

async function start(source: 'release' | 'build') {
  starting.value = true
  lines.value = []
  result.value = null
  progress.value = null
  stick = true
  try {
    await api.updateFirmware(source)
    running.value = true
  } catch (e) {
    toastError(e)
  } finally {
    starting.value = false
  }
}
</script>

<template>
  <section id="firmware" class="card">
    <div class="card-head">
      <h2>Firmware</h2>
      <button class="icon-btn st" type="button" v-tip="'Check again'" aria-label="Check again" :disabled="busy || checking" @click="load(true)">
        <Icon name="refresh" :size="20" />
      </button>
    </div>
    <dl class="facts">
      <dt>On the device</dt>
      <dd>{{ connected ? live.status?.device?.fw || 'unknown' : 'not connected' }}</dd>
      <dt>Latest release</dt>
      <dd>
        <span v-if="checking && !release" class="muted">checking…</span>
        <span v-else-if="release?.error" class="err-text small">{{ release.error }}</span>
        <template v-else-if="release?.version">
          {{ release.version }}
          <span v-if="releaseNote" class="chip" :class="{ 'chip-outline': release.status !== 'updateAvailable' }">{{ releaseNote }}</span>
          <span class="muted small">· {{ mb(release.appSize) }}</span>
        </template>
      </dd>
      <template v-if="info?.repo">
        <dt>Built here</dt>
        <dd>
          {{ info.built || 'not built yet' }}
          <span class="muted small">· <code class="plain small">{{ info.repo }}</code></span>
        </dd>
      </template>
    </dl>
    <p v-if="loadError" class="err-text small">{{ loadError }}</p>

    <div v-if="busy && progress !== null" class="bar" role="progressbar" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100">
      <span :style="{ width: `${progress}%` }" />
    </div>

    <div class="actions">
      <span v-if="busy" class="busy grow">
        <ShapeLoader small label="Updating" /> {{ progress !== null ? `Updating… ${progress} %` : 'Updating…' }}
      </span>
      <span v-else-if="result !== null" class="small grow" :class="result ? 'ok-text' : 'err-text'">
        <Icon :name="result ? 'check' : 'error'" :size="16" /> {{ result ? 'Update finished' : 'Update failed' }}
      </span>
      <span v-else-if="!connected && release?.version" class="muted small grow">
        Not connected: you can still install, holding BOOT while plugging the board in.
      </span>
      <span v-else class="grow" />
      <button v-if="canBuild" class="btn btn-text st" type="button" :disabled="busy || starting" @click="buildAndFlash">
        <Icon name="build" :size="18" /> Build from source
      </button>
      <button class="btn btn-tonal st" type="button" :disabled="!!installBlocked || starting" v-tip="installBlocked" @click="install">
        <ShapeLoader v-if="starting" small label="Starting" />
        <Icon v-else name="download" :size="18" />
        {{ installLabel }}
      </button>
    </div>

    <pre v-if="lines.length" ref="logBox" class="log" aria-label="Update log" aria-live="polite" @scroll="onScroll">{{ lines.join('\n') }}</pre>
  </section>
</template>

<style scoped>
#firmware {
  scroll-margin-top: 84px;
}
.facts {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 8px 20px;
  margin: 0;
  font-size: 14px;
}
.facts dt {
  color: var(--on-sv);
}
.facts dd {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  overflow-wrap: anywhere;
}
.bar {
  height: 6px;
  margin-top: 16px;
  border-radius: 3px;
  background: var(--schh);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--primary);
  transition: width 0.2s;
}
.actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}
.busy {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}
.ok-text,
.err-text {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.ok-text {
  color: var(--primary);
}
.err-text {
  color: var(--error);
}
.log {
  margin: 14px 0 0;
  max-height: 260px;
  overflow: auto;
  padding: 12px 14px;
  border-radius: 14px;
  background: var(--code-bg);
  color: var(--code-fg);
  border: 1px solid var(--code-border);
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
@media (prefers-reduced-motion: reduce) {
  .bar span {
    transition: none;
  }
}
</style>
