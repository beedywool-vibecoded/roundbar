<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/Icon.vue'
import M3Shape from '@/components/M3Shape.vue'
import M3Slider from '@/components/M3Slider.vue'
import NumField from '@/components/NumField.vue'
import { PAGE_TYPES } from '@/lib/model'
import { config, daemonUp, findProfile, live } from '@/lib/store'
import type { KeyboardLayout } from '@/lib/types'

const dev = computed(() => config.value!.device)
const st = computed(() => live.status)

const LAYOUTS: { id: KeyboardLayout; label: string; hint: string }[] = [
  { id: 'fr', label: 'French (AZERTY)', hint: 'fr' },
  { id: 'us', label: 'US (QWERTY)', hint: 'us' },
]

const view = computed(() => {
  const v = st.value?.view
  if (!v) return null
  const prof = findProfile(v.profile)
  const page = prof?.pages.find((p) => p.id === v.page)
  return {
    profile: prof?.name ?? v.profile,
    page: page?.title || PAGE_TYPES.find((t) => t.value === v.kind)?.label || v.page,
    awake: v.awake,
    to: prof && page ? `/profiles/${prof.id}/pages/${page.id}` : null,
  }
})

const seconds = (s: number) => (s === 0 ? 'never' : s < 60 ? `${s} s` : s % 60 ? `${Math.floor(s / 60)} min ${s % 60} s` : `${s / 60} min`)
</script>

<template>
  <div class="page">
    <div class="page-title"><h1>Device</h1></div>

    <div class="cols">
      <section class="card status" :class="{ off: !daemonUp || !st?.device?.connected }">
        <div class="card-head"><h2>Status</h2></div>

        <div v-if="!live.connected && st" class="state">
          <M3Shape kind="clover" :size="72" fill="var(--ec)" />
          <div>
            <div class="big">Configurator offline</div>
            <p class="muted small">The connection to <code>roundbar config</code> dropped. Reconnecting… If it stopped, run it again.</p>
          </div>
        </div>

        <div v-else-if="!st" class="state">
          <M3Shape kind="cookie" :size="72" fill="var(--sch)" />
          <div class="big">Connecting…</div>
        </div>

        <div v-else-if="!daemonUp" class="state">
          <M3Shape kind="clover" :size="72" fill="var(--tc)" />
          <div>
            <div class="big">The daemon isn't running</div>
            <p class="muted small">
              You can still edit and save: the daemon picks the config up when it starts. To start it now and at every login:
            </p>
            <pre class="cmd"><code>systemctl --user enable --now roundbar</code></pre>
          </div>
        </div>

        <template v-else>
          <div class="state">
            <M3Shape :kind="st.device?.connected ? 'sunny' : 'cookie'" :size="72" :fill="st.device?.connected ? 'var(--pc)' : 'var(--sch)'">
              <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="44" class="sym" :fill="st.device?.connected ? 'var(--on-pc)' : 'var(--on-sv)'">
                {{ st.device?.connected ? 'usb' : 'link_off' }}
              </text>
            </M3Shape>
            <div>
              <div class="big">{{ st.device?.connected ? 'Connected' : 'No device' }}</div>
              <p v-if="!st.device?.connected" class="muted small">Plug the roundbar in over USB. Settings are sent as soon as it shows up.</p>
            </div>
          </div>
          <dl class="facts">
            <template v-if="st.device?.connected">
              <dt>Port</dt>
              <dd><code class="plain">{{ st.device.port ?? '—' }}</code></dd>
              <dt>Firmware</dt>
              <dd>{{ st.device.fw ?? '—' }}</dd>
              <dt>Config</dt>
              <dd>
                <span v-if="st.device.inSync === false" class="warn-text">Sending the latest config…</span>
                <span v-else>Up to date</span>
                <code v-if="st.device.cfg" class="plain muted small" style="margin-left: 8px">{{ st.device.cfg.slice(0, 8) }}</code>
              </dd>
              <dt>On screen</dt>
              <dd>
                <template v-if="view">
                  <RouterLink v-if="view.to" :to="view.to">{{ view.profile }} · {{ view.page }}</RouterLink>
                  <span v-else>{{ view.profile }} · {{ view.page }}</span>
                  <span class="chip chip-outline" style="margin-left: 8px">{{ view.awake ? 'awake' : 'asleep' }}</span>
                </template>
                <span v-else class="muted">—</span>
              </dd>
            </template>
            <dt>Focused app</dt>
            <dd>
              <template v-if="st.focus">
                {{ st.focus.name }} <code class="plain muted">{{ st.focus.id }}</code>
                <code v-if="st.focus.class && st.focus.class !== st.focus.id" class="plain muted"> · class {{ st.focus.class }}</code>
              </template>
              <span v-else-if="st.extension === false" class="warn-text">The GNOME extension isn't running, so profiles can't follow the focused window. Enable it in Extensions.</span>
              <span v-else class="muted">—</span>
            </dd>
          </dl>
        </template>
      </section>

      <section class="card">
        <div class="card-head"><h2>Screen</h2></div>
        <div class="setting col">
          <div class="row-head">
            <span>Brightness</span>
            <span class="muted">{{ dev.brightness }} %</span>
          </div>
          <M3Slider v-model="dev.brightness" label="Brightness" :min="5" :max="100" :step="5" :format="(v) => `${v} %`" />
        </div>
        <div class="setting col">
          <div class="form-row">
            <NumField v-model="dev.dimAfter" label="Dim after" suffix="s" :min="0" :help="`Idle time before dimming to 20 %: ${seconds(dev.dimAfter)}`" />
            <NumField v-model="dev.sleepAfter" label="Screen off after" suffix="s" :min="0" :help="`Idle time before the screen turns off: ${seconds(dev.sleepAfter)}`" />
          </div>
          <p class="muted small" style="margin: 0">0 means never.</p>
        </div>

        <div class="card-head" style="margin-top: 12px"><h2>Input</h2></div>
        <div class="setting col">
          <div class="row-head">
            <span>Long press</span>
            <span class="muted">{{ dev.longPressMs }} ms</span>
          </div>
          <M3Slider v-model="dev.longPressMs" label="Long press" :min="200" :max="1500" :step="25" :format="(v) => `${v} ms`" />
          <p class="muted small" style="margin: 0">How long a button is held before its long-press action runs.</p>
        </div>
        <div class="setting">
          <div class="grow">
            <div>Keyboard layout</div>
            <div class="muted small">The layout set on this PC, so typed text and shortcuts come out right.</div>
          </div>
          <div class="seg" role="radiogroup" aria-label="Keyboard layout">
            <button
              v-for="l in LAYOUTS"
              :key="l.id"
              type="button"
              role="radio"
              class="st"
              :class="{ on: dev.keyboardLayout === l.id }"
              :aria-checked="dev.keyboardLayout === l.id"
              v-tip="l.label"
              @click="dev.keyboardLayout = l.id"
            >
              <Icon v-if="dev.keyboardLayout === l.id" name="check" :size="18" />
              {{ l.hint.toUpperCase() }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.state {
  display: flex;
  align-items: center;
  gap: 18px;
}
.state p {
  margin: 4px 0 0;
}
.big {
  font-size: 22px;
  font-weight: 500;
}
.cmd {
  margin: 10px 0 0;
}
.cmd code {
  display: inline-block;
  padding: 8px 12px;
  font-size: 13px;
  user-select: all;
}
.facts {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 10px 20px;
  margin: 20px 0 0;
  font-size: 14px;
}
.facts dt {
  color: var(--on-sv);
}
.facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.warn-text {
  color: var(--warn);
}
.row-head {
  display: flex;
  justify-content: space-between;
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
}
@media (max-width: 1000px) {
  .cols {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
