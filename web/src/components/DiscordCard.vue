<script setup lang="ts">
// Discord setup for `discord` pages: the daemon talks to the Discord app over its local RPC
// socket with the user's own application. The credentials go straight to the server, which
// keeps them in ~/.config/roundbar/discord.json; they are never kept in the page.
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import M3Shape from './M3Shape.vue'
import ShapeLoader from './ShapeLoader.vue'
import TextField from './TextField.vue'
import { api } from '@/lib/api'
import { daemonUp, live } from '@/lib/store'
import { toast, toastError } from '@/lib/ui'

const discord = computed(() => live.status?.discord)
const state = computed<'ok' | 'error' | 'none'>(() => (discord.value?.connected ? 'ok' : discord.value?.error ? 'error' : 'none'))

const clientId = ref('')
const clientSecret = ref('')
const showSecret = ref(false)
const connecting = ref(false)
const idError = computed(() => (clientId.value && !/^\d{15,22}$/.test(clientId.value.trim()) ? 'A Client ID is a long number' : null))
const ready = computed(() => !!clientId.value.trim() && !!clientSecret.value.trim() && !idError.value)

async function connect() {
  if (!ready.value || connecting.value) return
  connecting.value = true
  try {
    await api.connectDiscord(clientId.value.trim(), clientSecret.value.trim())
    // Not kept in the page once the server has it
    clientSecret.value = ''
    showSecret.value = false
    toast('Sent. Approve the prompt in the Discord app.')
  } catch (e) {
    toastError(e)
  } finally {
    connecting.value = false
  }
}
</script>

<template>
  <section id="discord" class="card">
    <div class="card-head"><h2>Discord</h2></div>

    <div class="state">
      <M3Shape :kind="state === 'ok' ? 'sunny' : state === 'error' ? 'clover' : 'cookie'" :size="56" :fill="state === 'ok' ? 'var(--pc)' : state === 'error' ? 'var(--ec)' : 'var(--sch)'">
        <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="46" class="sym" :fill="state === 'ok' ? 'var(--on-pc)' : state === 'error' ? 'var(--on-ec)' : 'var(--on-sv)'">
          headset_mic
        </text>
      </M3Shape>
      <div class="grow">
        <div class="big">
          <template v-if="state === 'ok'">Connected as {{ discord!.user || 'you' }}</template>
          <template v-else-if="state === 'error'">Not connected</template>
          <template v-else>Not set up</template>
        </div>
        <div v-if="state === 'ok'" class="muted small">{{ discord!.channel ? `In ${discord!.channel}` : 'Not in a voice channel' }}</div>
        <div v-else-if="state === 'error'" class="err-text small">{{ discord!.error }}</div>
        <div v-else class="muted small">Discord voice pages need your own Discord application. It takes two minutes:</div>
      </div>
    </div>

    <ol class="guide">
      <li>
        Open <a href="https://discord.com/developers/applications" target="_blank" rel="noopener">discord.com/developers/applications</a> and click
        <b>New Application</b> (name it Roundbar).
      </li>
      <li>In <b>OAuth2</b>, copy the <b>Client ID</b> and reset / copy the <b>Client Secret</b>.</li>
      <li>In <b>OAuth2 → Redirects</b> add <code>http://127.0.0.1</code> and save.</li>
      <li>Paste both below and click <b>Connect</b>, then approve the prompt that appears in the Discord app.</li>
    </ol>

    <form class="form" autocomplete="off" @submit.prevent="connect">
      <div class="form-row">
        <TextField v-model="clientId" label="Client ID" mono :error="idError" />
        <TextField v-model="clientSecret" label="Client Secret" mono :type="showSecret ? 'text' : 'password'">
          <template #trailing>
            <button class="eye" type="button" :aria-label="showSecret ? 'Hide secret' : 'Show secret'" v-tip="showSecret ? 'Hide' : 'Show'" @click="showSecret = !showSecret">
              <Icon :name="showSecret ? 'visibility_off' : 'visibility'" :size="20" />
            </button>
          </template>
        </TextField>
      </div>
      <p class="muted small" style="margin: 0">Stored only on this PC (<code>~/.config/roundbar/discord.json</code>). Discord must be running.</p>
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
#discord {
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
.guide a {
  color: var(--primary);
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
