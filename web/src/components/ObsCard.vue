<script setup lang="ts">
// OBS setup for `obs` actions and pages: the daemon talks to OBS through obs-websocket. The
// password goes straight to the server, which keeps it in ~/.config/roundbar/obs.json; it is
// never kept in the config, in the page or in browser storage.
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import M3Shape from './M3Shape.vue'
import ShapeLoader from './ShapeLoader.vue'
import TextField from './TextField.vue'
import { api } from '@/lib/api'
import { daemonUp, live } from '@/lib/store'
import { toast, toastError } from '@/lib/ui'

const obs = computed(() => live.status?.obs)
const state = computed<'ok' | 'error' | 'none'>(() => (obs.value?.connected ? 'ok' : obs.value?.error ? 'error' : 'none'))

const host = ref('127.0.0.1')
const port = ref('4455')
const password = ref('')
const showPassword = ref(false)
const connecting = ref(false)

const hostError = computed(() => (!host.value.trim() ? 'Enter the host OBS runs on' : /\s/.test(host.value.trim()) ? 'No spaces in a host name' : null))
const portNum = computed(() => Number(port.value))
const portError = computed(() => (Number.isInteger(portNum.value) && portNum.value >= 1 && portNum.value <= 65535 ? null : 'A port is 1 to 65535'))
const ready = computed(() => !hostError.value && !portError.value)

async function connect() {
  if (!ready.value || connecting.value) return
  connecting.value = true
  try {
    await api.connectObs(host.value.trim(), portNum.value, password.value)
    // Not kept in the page once the server has it
    password.value = ''
    showPassword.value = false
    toast('Sent. Roundbar connects to OBS now.')
  } catch (e) {
    toastError(e)
  } finally {
    connecting.value = false
  }
}
</script>

<template>
  <section id="obs" class="card">
    <div class="card-head"><h2>OBS</h2></div>

    <div class="state">
      <M3Shape :kind="state === 'ok' ? 'sunny' : state === 'error' ? 'clover' : 'cookie'" :size="56" :fill="state === 'ok' ? 'var(--pc)' : state === 'error' ? 'var(--ec)' : 'var(--sch)'">
        <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="46" class="sym" :fill="state === 'ok' ? 'var(--on-pc)' : state === 'error' ? 'var(--on-ec)' : 'var(--on-sv)'">
          videocam
        </text>
      </M3Shape>
      <div class="grow">
        <div class="big">
          <template v-if="state === 'ok'">Connected</template>
          <template v-else-if="state === 'error'">Not connected</template>
          <template v-else>Not set up</template>
        </div>
        <div v-if="state === 'ok'" class="live small">
          <span class="chip" :class="obs!.recording ? 'rec' : 'chip-outline'">
            <Icon :name="obs!.recording ? 'radio_button_checked' : 'circle'" :size="14" />
            {{ obs!.recording ? 'Recording' : 'Not recording' }}
          </span>
          <span class="chip" :class="obs!.streaming ? 'onair' : 'chip-outline'">
            <Icon name="cast" :size="14" />
            {{ obs!.streaming ? 'Streaming' : 'Not streaming' }}
          </span>
          <span v-if="obs!.current" class="muted">Scene: {{ obs!.current }}</span>
        </div>
        <div v-else-if="state === 'error'" class="err-text small">{{ obs!.error }}</div>
        <div v-else class="muted small">OBS buttons and pages talk to OBS directly, so they work even when it isn't focused.</div>
      </div>
    </div>

    <ol class="guide">
      <li>In OBS: <b>Tools → WebSocket Server Settings → Enable</b>, copy the password (<b>Show Connect Info</b>).</li>
      <li>Paste it below and click <b>Connect</b>. OBS must be running.</li>
    </ol>

    <form class="form" autocomplete="off" @submit.prevent="connect">
      <div class="form-row">
        <TextField v-model="host" label="Host" mono :error="hostError" />
        <TextField v-model="port" label="Port" mono :error="portError" />
      </div>
      <TextField v-model="password" label="Password" mono :type="showPassword ? 'text' : 'password'" help="Leave empty if authentication is off in OBS">
        <template #trailing>
          <button class="eye" type="button" :aria-label="showPassword ? 'Hide password' : 'Show password'" v-tip="showPassword ? 'Hide' : 'Show'" @click="showPassword = !showPassword">
            <Icon :name="showPassword ? 'visibility_off' : 'visibility'" :size="20" />
          </button>
        </template>
      </TextField>
      <p class="muted small" style="margin: 0">Stored only on this PC (<code>~/.config/roundbar/obs.json</code>), never in the config.</p>
      <div class="actions">
        <button class="btn btn-filled st" type="submit" :disabled="!ready || connecting || !daemonUp" v-tip="!daemonUp ? 'Needs the daemon running' : ''">
          <ShapeLoader v-if="connecting" small label="Connecting" />
          <Icon v-else name="link" :size="18" />
          {{ state === 'ok' ? 'Reconnect' : 'Connect' }}
        </button>
      </div>
    </form>
  </section>
</template>

<style scoped>
#obs {
  scroll-margin-top: 84px;
}
.state {
  display: flex;
  align-items: center;
  gap: 16px;
}
.big {
  font-size: 18px;
  font-weight: 500;
}
.live {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 8px;
  margin-top: 4px;
}
.chip.rec {
  background: var(--ec);
  color: var(--on-ec);
}
.chip.onair {
  background: var(--pc);
  color: var(--on-pc);
}
.err-text {
  color: var(--error);
  overflow-wrap: anywhere;
}
.guide {
  margin: 16px 0;
  padding-left: 22px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 14px;
  line-height: 1.45;
}
.eye {
  display: grid;
  place-items: center;
  border: none;
  background: none;
  color: var(--on-sv);
  padding: 0 0 0 6px;
}
.actions {
  display: flex;
  justify-content: flex-end;
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
}
</style>
