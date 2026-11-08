<script setup lang="ts">
// Build and flash the firmware from this PC's checkout. The log streams in as SSE `flash`
// events on the status connection.
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api, onFlash } from '@/lib/api'
import { live } from '@/lib/store'
import type { FirmwareInfo, FlashEvent } from '@/lib/types'
import { confirm, toast, toastError } from '@/lib/ui'

const MAX_LINES = 200
const info = ref<FirmwareInfo | null>(null)
const loadError = ref('')
const lines = ref<string[]>([])
const running = ref(false)
const result = ref<boolean | null>(null)
const logBox = ref<HTMLElement>()
const starting = ref(false)

async function load() {
  try {
    info.value = await api.firmware()
    loadError.value = ''
    if (info.value.running) running.value = true
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
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
  running.value = false
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

const blocked = computed(() => {
  const i = info.value
  if (!i) return loadError.value ? 'Could not check this PC' : 'Checking…'
  if (!i.repoOk) return 'Needs the Roundbar source checkout on this PC (tools/flash.sh not found)'
  if (!i.idf) return 'Needs ESP-IDF installed (~/esp/esp-idf-*)'
  if (running.value || i.running) return 'An update is running'
  return ''
})

async function update() {
  if (blocked.value || starting.value) return
  const ok = await confirm({
    title: 'Update the firmware?',
    body: "The device restarts during the update (about 30 s). Don't unplug it.",
    action: 'Update',
  })
  if (!ok) return
  starting.value = true
  lines.value = []
  result.value = null
  stick = true
  try {
    await api.updateFirmware()
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
      <button class="icon-btn st" type="button" v-tip="'Check again'" aria-label="Check again" :disabled="running" @click="load"><Icon name="refresh" :size="20" /></button>
    </div>
    <dl class="facts">
      <dt>On the device</dt>
      <dd>{{ live.status?.device?.connected ? live.status.device.fw || 'unknown' : 'not connected' }}</dd>
      <dt>Built here</dt>
      <dd>
        <template v-if="info">{{ info.built || 'not built yet' }}</template>
        <span v-else-if="loadError" class="err-text">{{ loadError }}</span>
        <span v-else class="muted">…</span>
      </dd>
      <template v-if="info?.repo">
        <dt>Source</dt>
        <dd><code class="plain small">{{ info.repo }}</code></dd>
      </template>
    </dl>

    <div class="actions">
      <span v-if="blocked && info && !running" class="muted small grow">{{ blocked }}</span>
      <span v-else-if="running" class="busy grow"><ShapeLoader small label="Updating" /> Building and flashing…</span>
      <span v-else-if="result !== null" class="small grow" :class="result ? 'ok-text' : 'err-text'">
        <Icon :name="result ? 'check' : 'error'" :size="16" /> {{ result ? 'Update finished' : 'Update failed' }}
      </span>
      <span v-else class="grow" />
      <button class="btn btn-tonal st" type="button" :disabled="!!blocked || starting" v-tip="blocked" @click="update">
        <ShapeLoader v-if="starting" small label="Starting" />
        <Icon v-else name="download" :size="18" />
        Update firmware
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
  margin: 0;
  overflow-wrap: anywhere;
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
</style>
