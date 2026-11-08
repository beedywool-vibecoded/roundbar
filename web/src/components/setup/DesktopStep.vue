<script setup lang="ts">
// Step 3: which desktop the daemon follows and what works there, in plain words.
import { computed, watchEffect } from 'vue'
import Icon from '../Icon.vue'
import SetupHero from './SetupHero.vue'
import { wiz } from '@/lib/setup'
import { daemonUp, live } from '@/lib/store'

const NAMES: Record<string, string> = {
  gnome: 'GNOME',
  kde: 'KDE Plasma',
  wayland: 'a Wayland desktop',
  x11: 'an X11 desktop',
  none: 'a desktop Roundbar can’t follow',
}

const d = computed(() => live.status?.desktop)
const noExtension = computed(() => d.value?.backend === 'gnome' && live.status?.extension === false)

const features = computed(() => {
  const x = d.value
  if (!x) return []
  return [
    { ok: x.focus && !noExtension.value, icon: 'select_window', name: 'Follows the app you use', off: noExtension.value ? 'After you log out and back in' : 'Profiles stay on Default' },
    { ok: x.windows && !noExtension.value, icon: 'grid_view', name: 'Window switcher page', off: noExtension.value ? 'After you log out and back in' : 'Stays empty here' },
    { ok: x.notifications && !noExtension.value, icon: 'notifications', name: 'Notifications on the board', off: noExtension.value ? 'After you log out and back in' : 'Not available here' },
    { ok: x.clipboard && !noExtension.value, icon: 'content_paste', name: 'Clipboard history page', off: noExtension.value ? 'After you log out and back in' : 'Stays empty here' },
  ]
})
const allOk = computed(() => features.value.length > 0 && features.value.every((f) => f.ok))

watchEffect(() => (wiz.auto.desktop = !daemonUp.value || allOk.value))
</script>

<template>
  <div class="step">
    <SetupHero icon="desktop_windows" kind="sunny" :tone="allOk ? 'primary' : 'tertiary'" />
    <template v-if="!d">
      <h1>Your desktop</h1>
      <p class="lead">Start Roundbar's helper to see what works on this desktop. You can come back to this later.</p>
    </template>
    <template v-else>
      <h1>You're on {{ NAMES[d.backend] ?? d.backend }}</h1>
      <p class="lead" v-if="allOk">Everything works here.</p>
      <p class="lead" v-else-if="noExtension">One more thing, then everything works.</p>
      <p class="lead" v-else>Most things work. A few pages can't get what they need from this desktop:</p>

      <div v-if="noExtension" class="card-inner">
        <Icon name="logout" :size="24" />
        <div class="grow">
          <div class="strong">Log out and back in once</div>
          <div class="muted small">
            Roundbar adds a small GNOME extension so the board knows which app you're using. GNOME only loads new extensions
            when you log in. If it's still off afterwards, turn on <b>Roundbar</b> in the Extensions app.
          </div>
        </div>
      </div>

      <ul class="features">
        <li v-for="f in features" :key="f.name" :class="{ off: !f.ok }">
          <span class="ic"><Icon :name="f.icon" :size="20" /></span>
          <span class="grow">
            <span class="name">{{ f.name }}</span>
            <span v-if="!f.ok" class="muted small">{{ f.off }}</span>
          </span>
          <Icon :name="f.ok ? 'check' : noExtension ? 'schedule' : 'block'" :size="20" class="mark" />
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.features {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
  text-align: left;
}
.features li {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 14px;
  border-radius: 16px;
  background: var(--sch);
}
.features .grow {
  display: flex;
  flex-direction: column;
}
.ic {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: var(--pc);
  color: var(--on-pc);
}
.off .ic {
  background: var(--schh);
  color: var(--on-sv);
}
.mark {
  color: var(--primary);
}
.off .mark {
  color: var(--on-sv);
}
</style>
