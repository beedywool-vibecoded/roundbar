<script setup lang="ts">
// Step 1: wait for the board, fix access to its USB port, notice a setup it brought along.
import { computed, ref, watch, watchEffect } from 'vue'
import Icon from '../Icon.vue'
import ShapeLoader from '../ShapeLoader.vue'
import SetupHero from './SetupHero.vue'
import { api } from '@/lib/api'
import { WEB_INSTALLER, wiz } from '@/lib/setup'
import { daemonUp, dirty, live, loadAll } from '@/lib/store'

const emit = defineEmits<{ finish: [] }>()

const dev = computed(() => live.status?.device)
const state = computed<'connecting' | 'nodaemon' | 'permission' | 'waiting' | 'ok'>(() => {
  if (!live.status) return 'connecting'
  if (!daemonUp.value) return 'nodaemon'
  if (dev.value?.connected) return 'ok'
  if (dev.value?.error === 'permission') return 'permission'
  return 'waiting'
})
const synced = computed(() => state.value === 'ok' && !!dev.value?.syncNote)

watchEffect(() => (wiz.auto.board = state.value === 'ok' && !synced.value))

// The daemon just replaced config.json with the board's: show that one, unless there are edits.
watch(
  synced,
  (s) => {
    if (s && !dirty.value) void loadAll()
  },
  { immediate: true },
)

const fixing = ref(false)
const fixError = ref('')
async function allow() {
  fixing.value = true
  fixError.value = ''
  try {
    await api.installUdev()
  } catch (e) {
    fixError.value = e instanceof Error ? e.message : String(e)
  } finally {
    fixing.value = false
  }
}
</script>

<template>
  <div class="step">
    <template v-if="state === 'connecting'">
      <SetupHero board waiting tone="neutral" />
      <h1>Looking for your Roundbar…</h1>
      <div class="center"><ShapeLoader /></div>
    </template>

    <template v-else-if="state === 'nodaemon'">
      <SetupHero icon="power_settings_new" kind="clover" tone="tertiary" />
      <h1>Roundbar's helper isn't running</h1>
      <p class="lead">
        A small background service talks to the board. It starts by itself at login once it is enabled. Paste this in a
        terminal to start it now:
      </p>
      <pre class="cmd"><code>systemctl --user enable --now roundbar</code></pre>
      <p class="muted small">This page notices as soon as it runs.</p>
    </template>

    <template v-else-if="state === 'permission'">
      <SetupHero board tone="tertiary" />
      <h1>Almost there: allow access</h1>
      <p class="lead">
        Your Roundbar is plugged in, but Linux doesn't let your account talk to it yet. Allow it once; your computer asks for
        your password.
      </p>
      <div class="actions">
        <button class="btn btn-filled btn-lg st" type="button" :disabled="fixing" @click="allow">
          <ShapeLoader v-if="fixing" small label="Waiting for the password prompt" />
          <Icon v-else name="lock_open" :size="20" />
          Allow access
        </button>
      </div>
      <p v-if="fixing" class="muted small">Enter your password in the window that just opened.</p>
      <p v-if="fixError" class="err small" role="alert">
        <Icon name="error" :size="16" /> {{ fixError }}. You can also run <code>roundbar udev</code> in a terminal.
      </p>
    </template>

    <template v-else-if="state === 'waiting'">
      <SetupHero board waiting tone="neutral" />
      <h1>Plug in your Roundbar</h1>
      <p class="lead">Connect it to this computer with a USB-C cable. This page moves on by itself when it shows up.</p>
      <div class="wait"><ShapeLoader small label="Waiting for the board" /> Waiting for the board…</div>
      <ul class="tips">
        <li>
          <Icon name="usb" :size="20" />
          <span>Nothing happens? Some cables only charge. Try another cable, and a port on the computer itself rather than a hub.</span>
        </li>
        <li>
          <Icon name="rocket_launch" :size="20" />
          <span>
            Brand-new board? Put the Roundbar software on it first with the
            <a :href="WEB_INSTALLER" target="_blank" rel="noopener">web installer</a> (Chrome or Edge), then plug it in here.
          </span>
        </li>
      </ul>
    </template>

    <template v-else>
      <SetupHero board />
      <h1>Roundbar connected</h1>
      <p class="lead">
        Found on <code class="plain">{{ dev?.port }}</code><template v-if="dev?.fw"> · firmware {{ dev.fw }}</template>
      </p>
      <div v-if="synced" class="card-inner">
        <Icon name="restore" :size="24" />
        <div class="grow">
          <div class="strong">Loaded the setup stored on your Roundbar</div>
          <div class="muted small">Its profiles, pages and colors came along with it. You can finish right away, or keep going to check the rest.</div>
        </div>
        <button class="btn btn-tonal st" type="button" @click="emit('finish')">Finish now</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.wait {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  justify-self: center;
  color: var(--on-sv);
  font-size: 14px;
}
.tips {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  display: grid;
  gap: 10px;
  text-align: left;
}
.tips li {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 12px 14px;
  border-radius: 16px;
  background: var(--sch);
  font-size: 14px;
}
.tips .msr {
  color: var(--primary);
  margin-top: 1px;
}
.cmd {
  margin: 0;
  justify-self: center;
}
.cmd code {
  display: inline-block;
  padding: 8px 12px;
  border: 1px solid var(--code-border);
  border-radius: 10px;
  background: var(--code-bg);
  font-size: 13px;
  user-select: all;
}
</style>
