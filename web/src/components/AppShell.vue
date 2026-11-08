<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import Icon from './Icon.vue'
import M3Shape from './M3Shape.vue'
import ShapeLoader from './ShapeLoader.vue'
import { canRedo, canUndo, config, daemonUp, dirty, discard, issues, live, redo, save, saving, undo } from '@/lib/store'
import { confirm } from '@/lib/ui'

const route = useRoute()

const NAV = [
  { to: '/', label: 'Profiles', icon: 'layers' },
  { to: '/appearance', label: 'Appearance', icon: 'palette' },
  { to: '/device', label: 'Device', icon: 'devices' },
]
const isActive = (to: string) => (to === '/' ? route.path === '/' || route.path.startsWith('/profiles') : route.path.startsWith(to))

const device = computed(() => {
  if (!live.connected && !live.status) return { tone: 'idle', text: 'Connecting…' }
  if (!live.connected) return { tone: 'bad', text: 'Configurator offline' }
  if (!live.status) return { tone: 'idle', text: 'Connecting…' }
  if (!daemonUp.value) return { tone: 'warn', text: 'Daemon not running' }
  if (!live.status.device?.connected) return { tone: 'idle', text: 'No device' }
  return { tone: 'ok', text: live.status.device.port ?? 'Connected' }
})

async function revert() {
  const ok = await confirm({ title: 'Discard changes?', body: 'Every edit since the last save is lost.', action: 'Discard', danger: true })
  if (ok) discard()
}

// Ctrl+Z / Ctrl+Shift+Z / Ctrl+Y undo and redo config edits, except while typing in a text
// field, where the browser's own undo of the text wins.
const TEXT_INPUTS = new Set(['text', 'search', 'url', 'tel', 'email', 'password', 'number'])
function typing(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement)) return false
  if (t.isContentEditable || t instanceof HTMLTextAreaElement) return true
  return t instanceof HTMLInputElement && TEXT_INPUTS.has(t.type)
}
function onKey(e: KeyboardEvent) {
  if (e.defaultPrevented || !(e.ctrlKey || e.metaKey) || e.altKey || !config.value || saving.value) return
  if (typing(e.target) || document.querySelector('dialog[open]')) return
  const k = e.key.toLowerCase()
  if (k === 'z' && !e.shiftKey) {
    e.preventDefault()
    undo()
  } else if ((k === 'z' && e.shiftKey) || (k === 'y' && !e.shiftKey)) {
    e.preventDefault()
    redo()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
const mod = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘' : 'Ctrl+'

// Ring of dots on the cookie: the round controller
const DOTS = Array.from({ length: 6 }, (_, k) => {
  const a = -Math.PI / 2 + (k * Math.PI) / 3
  return { cx: 50 + 24 * Math.cos(a), cy: 50 + 24 * Math.sin(a) }
})
</script>

<template>
  <div class="shell">
    <nav class="rail" aria-label="Sections">
      <RouterLink to="/" class="logo" aria-label="Roundbar home">
        <M3Shape kind="cookie" :size="48" fill="var(--pc)">
          <circle v-for="(d, i) in DOTS" :key="i" :cx="d.cx" :cy="d.cy" r="6" fill="var(--on-pc)" />
          <circle cx="50" cy="50" r="9" fill="var(--on-pc)" />
        </M3Shape>
      </RouterLink>
      <RouterLink
        v-for="n in NAV"
        :key="n.to"
        :to="n.to"
        class="nav-item"
        :class="{ active: isActive(n.to) }"
        :aria-current="isActive(n.to) ? 'page' : undefined"
      >
        <span class="pill st"><Icon :name="n.icon" :fill="isActive(n.to)" /></span>
        {{ n.label }}
      </RouterLink>
    </nav>

    <div class="main-col">
      <header class="topbar">
        <span class="brand">ROUNDBAR</span>
        <RouterLink to="/device" class="server" :class="device.tone">
          <span class="dot" aria-hidden="true" />
          <span class="ellipsis">{{ device.text }}</span>
        </RouterLink>
        <span v-if="live.status?.focus" class="chip chip-outline focus" v-tip="'Focused window'">
          <Icon name="desktop_windows" :size="16" /> {{ live.status.focus.id }}
        </span>
        <span class="spacer" />
        <template v-if="config">
          <span v-if="dirty && issues.length" class="chip issues" v-tip="issues[0] ? `${issues[0].message} (${issues[0].path})` : ''">
            <Icon name="warning" :size="16" /> {{ issues.length }} to fix
          </span>
          <span class="history">
            <button class="icon-btn st" type="button" aria-label="Undo" :disabled="!canUndo || saving" v-tip="`Undo (${mod}Z)`" @click="undo">
              <Icon name="undo" :size="20" />
            </button>
            <button class="icon-btn st" type="button" aria-label="Redo" :disabled="!canRedo || saving" v-tip="`Redo (${mod}Shift+Z)`" @click="redo">
              <Icon name="redo" :size="20" />
            </button>
          </span>
          <button v-if="dirty" class="btn btn-text st" type="button" :disabled="saving" @click="revert">Discard</button>
          <button class="btn btn-filled st save" type="button" :disabled="!dirty || saving" @click="save">
            <ShapeLoader v-if="saving" small label="Saving" />
            <Icon v-else name="save" :size="18" />
            {{ dirty ? 'Save' : 'Saved' }}
          </button>
        </template>
      </header>

      <main class="content">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  min-height: 100vh;
}
.rail {
  position: sticky;
  top: 0;
  height: 100vh;
  flex: 0 0 96px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 20px 0;
  border-right: 1px solid var(--sch);
  overflow-y: auto;
  overscroll-behavior: contain;
}
.logo {
  margin-bottom: 18px;
  display: grid;
}
.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  width: 80px;
  padding: 4px 0;
  color: var(--on-sv);
  font-size: 12px;
  font-weight: 500;
  text-decoration: none;
}
.pill {
  width: 56px;
  height: 32px;
  border-radius: 16px;
  display: grid;
  place-items: center;
  transition: background 0.2s var(--ease), width 0.3s var(--ease-decel);
}
.nav-item.active {
  color: var(--on-surface);
}
.nav-item.active .pill {
  background: var(--secc);
  color: var(--on-secc);
}
.main-col {
  flex: 1;
  min-width: 0;
  overflow-x: clip;
  padding: 0 32px 48px;
}
.content {
  max-width: 1340px;
}
/* Sticky so Save stays in reach while editing */
.topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin: 0 -32px 20px;
  padding: 16px 32px 12px;
  background: var(--surface);
}
.brand {
  font-size: 30px;
  font-weight: 500;
  letter-spacing: 0.14em;
  margin-right: 8px;
}
.server {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 320px;
  height: 40px;
  padding: 0 16px 0 14px;
  border-radius: 20px;
  border: 1px solid var(--outline-v);
  color: var(--on-surface);
  font-size: 14px;
  text-decoration: none;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 4px;
  background: var(--on-sv);
  flex-shrink: 0;
}
.server.ok .dot {
  background: var(--ok);
}
.server.warn .dot {
  background: var(--warn);
}
.server.bad .dot {
  background: var(--bad);
}
.focus {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.issues {
  background: var(--ec);
  color: var(--on-ec);
}
.save {
  min-width: 112px;
}
.history {
  display: inline-flex;
  gap: 2px;
}
.history .icon-btn {
  border: none;
}
.spacer {
  flex: 1;
}

/* Phones: the rail becomes a bottom navigation bar */
@media (max-width: 760px) {
  .rail {
    position: fixed;
    top: auto;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 10;
    height: 80px;
    flex-direction: row;
    justify-content: space-around;
    gap: 0;
    padding: 12px 0 0;
    border-right: none;
    background: var(--sc);
  }
  .logo {
    display: none;
  }
  .nav-item {
    width: auto;
    flex: 1;
    min-width: 0;
    font-size: 11px;
  }
  .pill {
    width: 48px;
  }
  .main-col {
    padding: 0 16px 112px;
  }
  .topbar {
    margin: 0 -16px 16px;
    padding: 12px 16px 10px;
    gap: 8px;
  }
  .brand,
  .focus {
    display: none;
  }
  .server {
    max-width: 50%;
  }
}
</style>
