<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/Icon.vue'
import M3Dialog from '@/components/M3Dialog.vue'
import M3Shape from '@/components/M3Shape.vue'
import MatchChips from '@/components/MatchChips.vue'
import RoundPreview from '@/components/RoundPreview.vue'
import ShapeLoader from '@/components/ShapeLoader.vue'
import TextField from '@/components/TextField.vue'
import M3Switch from '@/components/M3Switch.vue'
import SeedPicker from '@/components/SeedPicker.vue'
import { api } from '@/lib/api'
import { CLOCK_DEFAULTS, ID_RE, LIMITS, PAGE_TYPES, TIMER_DEFAULTS, matchesEntry, newPage, pageTypeInfo, pastePage, syncProfilePalette } from '@/lib/model'
import { clipboardPage, copyPage } from '@/lib/clipboard'
import { config, daemonUp, dirty, findProfile, issuesUnder, matchesFocus, profileIndex, profilePalette } from '@/lib/store'
import { SEEDS } from '@/lib/theme'
import type { Page, PageType } from '@/lib/types'
import { confirm, toast, toastError } from '@/lib/ui'

const props = defineProps<{ pid: string }>()
const router = useRouter()

const profile = computed(() => findProfile(props.pid))
const index = computed(() => profileIndex(props.pid))
const palette = computed(() => profilePalette(profile.value))

// ---- profile color: its own seed, or the theme's ----
const ownColor = computed({
  get: () => profile.value?.seed !== undefined,
  set: (on: boolean) => {
    const p = profile.value
    if (!p) return
    if (on) {
      // Start from a preset that differs from the theme, so the change shows
      const theme = config.value!.theme.seed.toLowerCase()
      p.seed = SEEDS.find((s) => s.hex.toLowerCase() !== theme)!.hex
    } else delete p.seed
    syncProfilePalette(p, config.value!.theme.mode)
  },
})
const profileSeed = computed({
  get: () => profile.value?.seed ?? config.value!.theme.seed,
  set: (v: string) => {
    const p = profile.value
    if (!p) return
    p.seed = v
    syncProfilePalette(p, config.value!.theme.mode)
  },
})
const isDefault = computed(() => props.pid === 'default')

// The id is the route: renaming it moves to the new address.
const idDraft = ref(props.pid)
const idError = computed(() => {
  const id = idDraft.value
  if (!ID_RE.test(id)) return '1 to 24 of a-z, 0-9 and -'
  if (id !== props.pid && config.value!.profiles.some((p) => p.id === id)) return 'Already used by another profile'
  if (id === 'default') return '"default" is the fallback profile'
  return null
})
function commitId() {
  const p = profile.value
  if (!p || idError.value || idDraft.value === p.id) return
  p.id = idDraft.value
  router.replace(`/profiles/${p.id}`)
}

// A class matched by an earlier profile never reaches this one (wildcards included).
function takenBy(cls: string): string | null {
  const list = config.value!.profiles
  for (let i = 0; i < index.value; i++) if (list[i]!.match.some((m) => m === cls || matchesEntry(m, cls))) return list[i]!.name
  return null
}

const focusMatches = computed(() => !!profile.value && matchesFocus(profile.value.match))

// ---- pages ----
const adding = ref(false)
function addPage(t: PageType) {
  const p = profile.value
  if (!p || p.pages.length >= LIMITS.pages) return
  const page = newPage(
    t,
    p.pages.map((x) => x.id),
  )
  p.pages.push(page)
  adding.value = false
  router.push(`/profiles/${p.id}/pages/${page.id}`)
}
const full = computed(() => (profile.value?.pages.length ?? 0) >= LIMITS.pages)
function copy(pg: Page) {
  copyPage(pg)
  toast('Page copied')
}
function paste() {
  const p = profile.value
  const src = clipboardPage.value
  if (!p || !src || full.value) return
  p.pages.push(pastePage(src, p))
  toast('Page pasted')
}
function move(i: number, d: number) {
  const pages = profile.value!.pages
  const j = i + d
  if (j < 0 || j >= pages.length) return
  ;[pages[i], pages[j]] = [pages[j]!, pages[i]!]
}
async function remove(pg: Page, i: number) {
  const p = profile.value!
  const refs = p.pages.flatMap((x) =>
    x.type === 'radial' ? [...x.buttons, ...(x.center ? [x.center] : [])].filter((b) => (b.tap.type === 'page' && b.tap.page === pg.id) || (b.hold?.type === 'page' && b.hold.page === pg.id)) : [],
  )
  const ok = await confirm({
    title: `Delete ${pg.title || pageTypeInfo(pg.type).label}?`,
    body: refs.length ? `${refs.length} button${refs.length > 1 ? 's go' : ' goes'} to this page and will need a new target.` : 'Nothing is written until you save.',
    action: 'Delete',
    danger: true,
  })
  if (ok) p.pages.splice(i, 1)
}
const pageSummary = (pg: Page) => {
  switch (pg.type) {
    case 'radial':
      return `${pg.buttons.length} button${pg.buttons.length > 1 ? 's' : ''}${pg.center ? ' + center' : ''}`
    case 'media':
      return 'Now playing and volume'
    case 'stats':
      return pg.items.join(', ') || 'No gauge'
    case 'dial':
      return `${pg.label || pg.source} dial`
    case 'touchpad':
      return `Mouse, speed ${pg.speed ?? 5}`
    case 'clock': {
      const zones = pg.zones?.length ?? 0
      return [pg.style ?? CLOCK_DEFAULTS.style, ...(zones ? [`${zones} world clock${zones > 1 ? 's' : ''}`] : [])].join(', ')
    }
    case 'mixer':
      return 'Per-app volume and output'
    case 'windows':
      return 'Open windows on a ring'
    case 'clipboard':
      return 'Recent copies'
    case 'discord':
      return 'Voice channel controls'
    case 'obs':
      return 'Record button and live scenes'
    case 'game':
      return 'Focused game, session time, GPU'
    case 'timer':
      return `${(pg.minutes ?? TIMER_DEFAULTS.minutes).join(' / ')} min${pg.pomodoro ? ', Pomodoro' : ''}`
    case 'image':
      return !pg.images.length ? 'No image yet' : pg.images.length === 1 ? (pg.images[0]!.endsWith('.gif') ? 'GIF' : '1 picture') : `Slideshow of ${pg.images.length}`
  }
}

const showing = ref('')
async function show(pg: Page) {
  showing.value = pg.id
  try {
    await api.show(props.pid, pg.id)
  } catch (e) {
    toastError(e)
  } finally {
    showing.value = ''
  }
}
</script>

<template>
  <div v-if="!profile" class="card empty">
    <Icon name="error" :size="32" />
    <h2>No profile "{{ pid }}"</h2>
    <RouterLink class="btn btn-filled st" to="/">Back to profiles</RouterLink>
  </div>

  <div v-else class="page">
    <div class="page-title">
      <RouterLink class="icon-btn st" to="/" v-tip="'All profiles'"><Icon name="arrow_back" /></RouterLink>
      <h1 class="ellipsis">{{ profile.name || pid }}</h1>
      <span v-if="isDefault" class="chip">Fallback</span>
      <span v-if="focusMatches" class="chip active"><Icon name="desktop_windows" :size="14" /> Matches the focused window</span>
    </div>

    <div class="cols">
      <section class="card">
        <div class="card-head"><h2>Profile</h2></div>
        <div class="form">
          <TextField v-model="profile.name" label="Name" :error="!profile.name.trim() ? 'Give it a name' : null" />
          <TextField
            v-if="!isDefault"
            v-model="idDraft"
            label="Id"
            mono
            :error="idError"
            help="Used in the config file and by the daemon. Applied when you leave the field."
            @focusout="commitId"
          />
          <p v-else class="muted small" style="margin: 0">
            The default profile keeps the id <code>default</code> and is shown whenever no other profile matches.
          </p>
          <MatchChips v-if="!isDefault" v-model="profile.match" :taken-by="takenBy" />
          <p v-if="!isDefault" class="muted small" style="margin: 0">
            App id or window class, lowercase, as GNOME reports it (either one matches): <code>firefox</code>, <code>org.gnome.nautilus</code>, <code>code</code>.
          </p>
        </div>
        <div class="setting color-row">
          <div class="grow">
            <div>Profile color</div>
            <div class="muted small">{{ ownColor ? 'Its pages get their own palette, in the device mode set in Appearance.' : 'Uses the device theme. Turn on to give this profile its own color.' }}</div>
          </div>
          <M3Switch v-model="ownColor" label="Profile color" />
        </div>
        <div v-if="ownColor" class="setting col" style="border-top: none; padding-top: 0">
          <SeedPicker v-model="profileSeed" label="Profile color" />
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>Pages</h2>
          <span class="muted small">{{ profile.pages.length }} / {{ LIMITS.pages }}</span>
          <button
            v-if="clipboardPage"
            class="btn btn-text st"
            type="button"
            :disabled="full"
            v-tip="full ? `A profile has at most ${LIMITS.pages} pages` : `Paste ${clipboardPage.title || pageTypeInfo(clipboardPage.type).label}`"
            @click="paste"
          >
            <Icon name="content_paste" :size="18" /> Paste page
          </button>
          <button class="btn btn-tonal st" type="button" :disabled="full" @click="adding = true">
            <Icon name="add" :size="18" /> Add page
          </button>
        </div>
        <p class="muted small" style="margin: 0 0 8px">Swipe on the device to move between them, in this order.</p>
        <ul class="pages">
          <li v-for="(pg, i) in profile.pages" :key="pg.id" class="pg" :class="{ bad: issuesUnder(`profiles[${index}].pages[${i}]`).length }">
            <RouterLink :to="`/profiles/${pid}/pages/${pg.id}`" class="pg-main st">
              <div class="thumb"><RoundPreview :page="pg" :palette="palette" :page-index="i" :page-count="profile.pages.length" thumb /></div>
              <div class="grow pg-info">
                <div class="pg-name">
                  <span class="ellipsis">{{ pg.title || pageTypeInfo(pg.type).label }}</span>
                  <Icon v-if="issuesUnder(`profiles[${index}].pages[${i}]`).length" name="warning" :size="18" class="err-icon" />
                </div>
                <div class="muted small ellipsis"><Icon :name="pageTypeInfo(pg.type).icon" :size="14" /> {{ pageTypeInfo(pg.type).label }} · {{ pageSummary(pg) }}</div>
              </div>
            </RouterLink>
            <div class="pg-tools">
              <button class="icon-btn st" type="button" :disabled="showing === pg.id || !daemonUp" v-tip="dirty ? 'Show on device (saved version)' : 'Show on device'" @click="show(pg)">
                <ShapeLoader v-if="showing === pg.id" small />
                <Icon v-else name="cast" :size="20" />
              </button>
              <button class="icon-btn st" type="button" v-tip="'Copy page'" @click="copy(pg)"><Icon name="content_copy" :size="20" /></button>
              <button class="icon-btn st" type="button" :disabled="i === 0" v-tip="'Move up'" @click="move(i, -1)"><Icon name="arrow_upward" :size="20" /></button>
              <button class="icon-btn st" type="button" :disabled="i === profile.pages.length - 1" v-tip="'Move down'" @click="move(i, 1)"><Icon name="arrow_downward" :size="20" /></button>
              <button class="icon-btn st" type="button" :disabled="profile.pages.length <= 1" v-tip="profile.pages.length <= 1 ? 'A profile needs one page' : 'Delete'" @click="remove(pg, i)">
                <Icon name="delete" :size="20" />
              </button>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <M3Dialog v-model="adding" title="Add a page">
      <div class="types">
        <button v-for="(t, i) in PAGE_TYPES" :key="t.value" type="button" class="type st" @click="addPage(t.value)">
          <M3Shape :kind="(['cookie', 'clover', 'sunny', 'flower', 'burst'] as const)[i % 5]!" :size="52" fill="var(--pc)">
            <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="44" class="sym" fill="var(--on-pc)">{{ t.icon }}</text>
          </M3Shape>
          <span class="grow">
            <span class="type-name">{{ t.label }}</span>
            <span class="muted small">{{ t.hint }}</span>
          </span>
        </button>
      </div>
    </M3Dialog>
  </div>
</template>

<style scoped>
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
  gap: 16px;
  align-items: start;
}
.color-row {
  margin-top: 12px;
}
.chip.active {
  background: var(--pc);
  color: var(--on-pc);
}
.pages {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.pg {
  display: flex;
  align-items: center;
  gap: 6px;
  border-radius: 20px;
}
.pg.bad {
  box-shadow: inset 0 0 0 1px var(--error);
}
.pg-main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 8px 10px;
  border-radius: 20px;
  color: inherit;
  text-decoration: none;
}
.thumb {
  width: 56px;
  flex-shrink: 0;
  padding: 3px;
}
.thumb :deep(.round) {
  box-shadow: 0 0 0 3px #111;
}
.pg-info {
  min-width: 0;
}
.pg-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 16px;
  font-weight: 500;
}
.err-icon {
  color: var(--error);
}
.pg-info .muted {
  display: flex;
  align-items: center;
  gap: 4px;
}
.pg-tools {
  display: flex;
  gap: 2px;
}
.pg-tools .icon-btn {
  border: none;
}
.types {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.type {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 16px 12px 12px;
  border: none;
  border-radius: 20px;
  background: var(--sc);
  color: var(--on-surface);
  text-align: left;
}
.type svg path {
  transform-origin: 50px 50px;
  transition: rotate 0.5s var(--ease-decel);
}
.type:hover svg path {
  rotate: 30deg;
}
.type-name {
  display: block;
  font-size: 16px;
  font-weight: 500;
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
@media (max-width: 560px) {
  .pg {
    flex-wrap: wrap;
  }
  .pg-tools {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
