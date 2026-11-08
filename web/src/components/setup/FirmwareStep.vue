<script setup lang="ts">
// Step 2: the release check and update of the Firmware card (lib/firmware.ts).
import { computed, watchEffect } from 'vue'
import Icon from '../Icon.vue'
import ShapeLoader from '../ShapeLoader.vue'
import SetupHero from './SetupHero.vue'
import { mb, useFirmware } from '@/lib/firmware'
import { wiz } from '@/lib/setup'
import { live } from '@/lib/store'

const fw = useFirmware()
const { release, checking, lines, result, progress, starting, connected, busy } = fw

const state = computed<'checking' | 'nodevice' | 'offline' | 'update' | 'busy' | 'current' | 'newer'>(() => {
  if (busy.value || starting.value) return 'busy'
  if (!release.value || (checking.value && !release.value.version)) return 'checking'
  if (release.value.error) return 'offline'
  if (!connected.value) return 'nodevice'
  if (release.value.status === 'updateAvailable') return 'update'
  if (release.value.status === 'deviceNewer') return 'newer'
  return 'current'
})

watchEffect(() => {
  wiz.busy = state.value === 'busy'
  wiz.auto.firmware = ['nodevice', 'offline', 'current', 'newer'].includes(state.value) && result.value !== false
})
</script>

<template>
  <div class="step">
    <template v-if="state === 'checking'">
      <SetupHero icon="download" kind="flower" tone="neutral" />
      <h1>Checking for updates…</h1>
      <div class="center"><ShapeLoader /></div>
    </template>

    <template v-else-if="state === 'busy'">
      <SetupHero icon="download" kind="flower" />
      <h1>Updating your Roundbar</h1>
      <p class="lead">Keep it plugged in. It restarts a couple of times; this takes about a minute.</p>
      <div class="bar" role="progressbar" :aria-valuenow="progress ?? undefined" aria-valuemin="0" aria-valuemax="100">
        <span :class="{ indeterminate: progress === null }" :style="progress !== null ? { width: `${progress}%` } : {}" />
      </div>
      <div class="wait"><ShapeLoader small label="Updating" /> {{ progress !== null ? `${progress} %` : lines[lines.length - 1] || 'Starting…' }}</div>
    </template>

    <template v-else-if="state === 'update'">
      <SetupHero icon="download" kind="flower" />
      <h1>An update is ready</h1>
      <p class="lead">
        Version <strong>{{ release?.version }}</strong> is out; your board has {{ live.status?.device?.fw }}. Updating takes
        about a minute and keeps your setup.
      </p>
      <div class="actions">
        <button class="btn btn-filled btn-lg st" type="button" @click="fw.install">
          <Icon name="download" :size="20" /> Update now
        </button>
      </div>
      <p class="muted small">{{ mb(release?.appSize) }} download</p>
    </template>

    <template v-else-if="state === 'offline'">
      <SetupHero icon="cloud" kind="clover" tone="neutral" />
      <h1>Couldn't check for updates</h1>
      <p class="lead">This computer couldn't reach GitHub right now. That's fine: you can update later from the Device page.</p>
    </template>

    <template v-else-if="state === 'nodevice'">
      <SetupHero icon="usb" kind="clover" tone="neutral" />
      <h1>Firmware</h1>
      <p class="lead">Plug the board in to check its firmware. The latest version is {{ release?.version }}.</p>
    </template>

    <template v-else>
      <SetupHero icon="check" kind="sunny" />
      <h1>{{ state === 'newer' ? 'Running a development build' : 'Firmware is up to date' }}</h1>
      <p class="lead">
        <template v-if="state === 'newer'">Your board runs {{ live.status?.device?.fw }}, newer than the latest release ({{ release?.version }}).</template>
        <template v-else>Your board runs {{ release?.version }}, the latest version.</template>
      </p>
    </template>

    <p v-if="result === false" class="err small" role="alert">
      <Icon name="error" :size="16" /> The update didn't finish. Your board still works; try again, or later from the Device page.
    </p>
    <details v-if="lines.length" class="log-wrap">
      <summary class="small">Details</summary>
      <pre class="log">{{ lines.join('\n') }}</pre>
    </details>
  </div>
</template>

<style scoped>
.bar {
  height: 8px;
  border-radius: 4px;
  background: var(--schh);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  border-radius: 4px;
  background: var(--primary);
  transition: width 0.2s;
}
.bar span.indeterminate {
  width: 30%;
  animation: slide 1.4s var(--ease) infinite;
}
@keyframes slide {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(340%);
  }
}
.wait {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  justify-self: center;
  color: var(--on-sv);
  font-size: 14px;
}
.log-wrap summary {
  cursor: pointer;
  color: var(--on-sv);
}
.log {
  margin: 8px 0 0;
  max-height: 200px;
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
  text-align: left;
}
@media (prefers-reduced-motion: reduce) {
  .bar span.indeterminate {
    animation: none;
  }
}
</style>
