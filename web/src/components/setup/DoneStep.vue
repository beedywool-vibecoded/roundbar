<script setup lang="ts">
// Step 7: what's set up, a try-it nudge, and Finish (saves and remembers the setup is done).
import { computed, ref } from 'vue'
import Icon from '../Icon.vue'
import ShapeLoader from '../ShapeLoader.vue'
import SetupHero from './SetupHero.vue'
import { api } from '@/lib/api'
import { layoutName } from '@/lib/layouts'
import { detected, wiz } from '@/lib/setup'
import { config, dirty, findProfile, live, save } from '@/lib/store'
import { toast, toastError } from '@/lib/ui'

const st = computed(() => live.status)
const added = computed(() => wiz.added.map((id) => findProfile(id)).filter((p) => !!p))

const rows = computed(() => {
  const s = st.value
  const out: { icon: string; ok: boolean; text: string }[] = []
  out.push(
    s?.device?.connected
      ? { icon: 'usb', ok: true, text: `Board connected${s.device.fw ? ` · firmware ${s.device.fw}` : ''}` }
      : { icon: 'link_off', ok: false, text: 'Board not connected yet: it gets this setup when you plug it in' },
  )
  const kb = config.value!.device.keyboardLayout || 'auto'
  out.push({
    icon: 'keyboard',
    ok: true,
    text: kb === 'auto' ? `Keyboard: ${s?.keyboard?.detected ? `${layoutName(s.keyboard.detected)}, ` : ''}automatic` : `Keyboard: ${layoutName(kb)}`,
  })
  if (added.value.length) out.push({ icon: 'layers', ok: true, text: `New profiles: ${added.value.map((p) => p.name).join(', ')}` })
  else out.push({ icon: 'layers', ok: true, text: `${config.value!.profiles.length} profile${config.value!.profiles.length > 1 ? 's' : ''} ready` })
  if (detected.value.obs) out.push({ icon: 'videocam', ok: !!s?.obs?.connected, text: s?.obs?.connected ? 'OBS connected' : 'OBS not connected (Device page, any time)' })
  if (detected.value.discord || detected.value.vesktop)
    out.push({ icon: 'headset_mic', ok: !!s?.discord?.connected, text: s?.discord?.connected ? 'Discord connected' : 'Discord not connected (Device page, any time)' })
  return out
})

// The page to show: the first new profile's first page, else what the device shows now.
const target = computed(() => {
  const p = added.value[0] ?? config.value!.profiles[0]
  return p && p.pages[0] ? { profile: p.id, page: p.pages[0].id, name: p.name } : null
})

const showing = ref(false)
async function show() {
  if (!target.value) return
  showing.value = true
  try {
    if (dirty.value && !(await save())) return
    await api.show(target.value.profile, target.value.page)
    toast(`Look at your Roundbar: it shows ${target.value.name}`)
  } catch (e) {
    toastError(e)
  } finally {
    showing.value = false
  }
}
</script>

<template>
  <div class="step">
    <SetupHero icon="celebration" kind="burst" />
    <h1>You're all set</h1>
    <p class="lead">Use your apps as usual: the board switches profiles with them. Change anything later in Profiles and Device.</p>
    <ul class="summary">
      <li v-for="r in rows" :key="r.text" :class="{ off: !r.ok }">
        <Icon :name="r.icon" :size="20" />
        <span class="grow">{{ r.text }}</span>
        <Icon v-if="r.ok" name="check" :size="20" class="mark" />
      </li>
    </ul>
    <div v-if="target && st?.device?.connected" class="card-inner try">
      <Icon name="touch_app" :size="24" />
      <div class="grow">
        <div class="strong">Try it</div>
        <div class="muted small">Show {{ target.name }} on the board now{{ dirty ? ' (saves first)' : '' }}.</div>
      </div>
      <button class="btn btn-tonal st" type="button" :disabled="showing" @click="show">
        <ShapeLoader v-if="showing" small label="Showing" />
        <Icon v-else name="visibility" :size="18" />
        Show on device
      </button>
    </div>
  </div>
</template>

<style scoped>
.summary {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 6px;
  text-align: left;
}
.summary li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 16px;
  background: var(--sch);
  font-size: 14px;
}
.summary .msr:first-child {
  color: var(--primary);
}
.summary .off .msr:first-child {
  color: var(--on-sv);
}
.step .card-inner.try {
  background: var(--pc);
  color: var(--on-pc);
}
.mark {
  color: var(--primary);
}
</style>
