<script setup lang="ts">
// Firmware updates (logic in lib/firmware.ts, shared with the setup).
import { nextTick, ref } from 'vue'
import Icon from './Icon.vue'
import ShapeLoader from './ShapeLoader.vue'
import { live } from '@/lib/store'
import { mb, useFirmware } from '@/lib/firmware'

const logBox = ref<HTMLElement>()
let stick = true
function onScroll() {
  const el = logBox.value
  if (el) stick = el.scrollHeight - el.scrollTop - el.clientHeight < 24
}
const fw = useFirmware({
  onLine: async () => {
    if (!stick) return
    await nextTick()
    logBox.value?.scrollTo({ top: logBox.value.scrollHeight })
  },
})
const { info, release, checking, loadError, lines, result, progress, starting, connected, canBuild, busy, releaseNote, installLabel, installBlocked, load, buildAndFlash } =
  fw
function install() {
  stick = true
  void fw.install()
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
