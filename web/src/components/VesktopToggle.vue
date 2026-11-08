<script setup lang="ts">
// Opt-in: live Discord voice through Vesktop needs it started with a localhost-only DevTools
// port. The server adds (or removes) one flag in vesktop-flags.conf; Vesktop picks it up
// when it restarts. Shows nothing when Vesktop isn't installed.
import { computed, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import M3Switch from './M3Switch.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api } from '@/lib/api'
import type { VesktopInfo } from '@/lib/types'
import { toastError } from '@/lib/ui'

const info = ref<VesktopInfo | null>(null)
const busy = ref(false)
const restart = ref(false)

onMounted(async () => {
  try {
    info.value = await api.vesktop()
  } catch {
    info.value = null
  }
})

const enabled = computed(() => !!info.value?.installs.length && info.value.installs.every((i) => i.enabled))

async function set(on: boolean) {
  if (busy.value) return
  busy.value = true
  try {
    const r = await api.setVesktop(on)
    info.value = { installs: r.installs, port: r.port }
    if (r.changed) restart.value = true
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="info?.installs.length" class="vesktop">
    <div class="row-top">
      <div class="grow">
        <div class="title">Live voice in Vesktop</div>
        <div class="muted small">Shows who is talking and lets you mute, deafen or leave from the device. No Discord app setup needed.</div>
      </div>
      <ShapeLoader v-if="busy" small label="Saving" />
      <M3Switch :model-value="enabled" label="Live voice in Vesktop" :disabled="busy" @update:model-value="set" />
    </div>
    <p class="note small">
      <Icon name="info" :size="16" />
      <span>
        This starts Vesktop with a debugging port on this PC only (port {{ info.port }}, not reachable from the network). While
        it is on, any program running as you could read your Discord session through it. Turn it off here any time.
      </span>
    </p>
    <p v-if="restart" class="restart small" role="status">
      <Icon name="restart_alt" :size="16" />
      {{ enabled ? 'Done. Quit Vesktop completely and start it again to turn it on.' : 'Turned off. Restart Vesktop to close the port.' }}
    </p>
  </div>
</template>

<style scoped>
.vesktop {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border-radius: 20px;
  background: var(--sch);
}
.row-top {
  display: flex;
  align-items: center;
  gap: 14px;
}
.title {
  font-weight: 500;
}
.note,
.restart {
  display: flex;
  gap: 8px;
  margin: 0;
  line-height: 1.45;
}
.note {
  color: var(--on-sv);
}
.note .msr,
.restart .msr {
  margin-top: 2px;
}
.restart {
  color: var(--primary);
  font-weight: 500;
}
</style>
