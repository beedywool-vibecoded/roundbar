<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/Icon.vue'
import M3Dialog from '@/components/M3Dialog.vue'
import RoundPreview from '@/components/RoundPreview.vue'
import TextField from '@/components/TextField.vue'
import { ID_RE, duplicateProfile, newPage, slug, uniqueId } from '@/lib/model'
import { PRESETS } from '@/lib/presets'
import { toast } from '@/lib/ui'
import { config, focusKeys, isNew, issuesUnder, live, profilePalette } from '@/lib/store'
import type { Profile } from '@/lib/types'
import { confirm } from '@/lib/ui'

const router = useRouter()
const profiles = computed(() => config.value!.profiles)
const focused = computed(() => focusKeys())
const activeId = computed(() => live.status?.profile ?? live.status?.view?.profile ?? '')

// Default stays first: it's the fallback, not one of the matched profiles.
function move(i: number, d: number) {
  const list = profiles.value
  const j = i + d
  if (list[i]?.id === 'default' || list[j]?.id === 'default' || j < 0 || j >= list.length) return
  ;[list[i], list[j]] = [list[j]!, list[i]!]
}

function duplicate(p: Profile, i: number) {
  const copy = duplicateProfile(
    p,
    profiles.value.map((x) => x.id),
  )
  profiles.value.splice(i + 1, 0, copy)
}

async function remove(p: Profile, i: number) {
  const ok = await confirm({
    title: `Delete ${p.name}?`,
    body: `Its ${p.pages.length} page${p.pages.length > 1 ? 's' : ''} go with it. Nothing is written until you save.`,
    action: 'Delete',
    danger: true,
  })
  if (ok) profiles.value.splice(i, 1)
}

// ---- new profile ----
const creating = ref(false)
const presetId = ref('') // '' = blank
const preset = computed(() => PRESETS.find((p) => p.id === presetId.value))
function pickPreset(id: string) {
  const before = preset.value
  presetId.value = id
  const now = preset.value
  if (now && (!newName.value.trim() || newName.value === before?.name)) newName.value = now.name
  if (!now && before && newName.value === before.name) newName.value = ''
}
const newName = ref('')
const newId = ref('')
const idEdited = ref(false)
const suggestedId = computed(() =>
  uniqueId(
    slug(newName.value) || 'profile',
    profiles.value.map((p) => p.id),
  ),
)
const effectiveId = computed(() => (idEdited.value ? newId.value : suggestedId.value))
const idError = computed(() => {
  const id = effectiveId.value
  if (!ID_RE.test(id)) return '1 to 24 of a-z, 0-9 and -'
  if (profiles.value.some((p) => p.id === id)) return 'Already used'
  return null
})
function openCreate() {
  const unmatched = focused.value.length && !profiles.value.some((p) => focused.value.some((k) => p.match.includes(k)))
  newName.value = unmatched ? (live.status?.focus?.name ?? '') : ''
  newId.value = ''
  idEdited.value = false
  presetId.value = ''
  creating.value = true
}
function create() {
  if (!newName.value.trim() || idError.value) return
  const id = effectiveId.value
  const focus = live.status?.focus
  const match = focus && newName.value === focus.name ? [focus.id.toLowerCase()] : []
  const p = preset.value
  if (p) {
    const built = p.build()
    profiles.value.push({ id, name: newName.value.trim(), ...built })
    if (p.setup) toast(p.setup)
  } else {
    profiles.value.push({ id, name: newName.value.trim(), match, pages: [newPage('radial', [])] })
  }
  creating.value = false
  router.push(`/profiles/${id}`)
}
</script>

<template>
  <div class="page">
    <div class="page-title">
      <h1>Profiles</h1>
      <button class="btn btn-filled st" type="button" @click="openCreate"><Icon name="add" :size="18" /> New profile</button>
    </div>
    <p class="muted intro">
      The device switches profile when the focused window matches one. <strong>Default</strong> is used for everything else.
    </p>

    <div v-if="isNew" class="card new-banner">
      <Icon name="auto_awesome" :size="24" />
      <div class="grow">
        <div>No config yet</div>
        <div class="small">This is a starter setup. Change what you like, then Save to create <code>~/.config/roundbar/config.json</code>.</div>
      </div>
    </div>

    <section class="list">
      <article v-for="(p, i) in profiles" :key="p.id" class="card prof" :class="{ bad: issuesUnder(`profiles[${i}]`).length }">
        <RouterLink :to="`/profiles/${p.id}`" class="main st" :aria-label="`Edit ${p.name}`">
          <div class="thumb">
            <RoundPreview v-if="p.pages[0]" :page="p.pages[0]" :palette="profilePalette(p)" :page-count="p.pages.length" thumb />
          </div>
          <div class="grow info">
            <div class="name-row">
              <h2 class="ellipsis">{{ p.name }}</h2>
              <span v-if="p.id === 'default'" class="chip">Fallback</span>
              <span v-if="activeId === p.id" class="chip active"><Icon name="radio_button_checked" :size="14" /> On device</span>
              <span v-if="issuesUnder(`profiles[${i}]`).length" class="chip err"><Icon name="warning" :size="14" /> {{ issuesUnder(`profiles[${i}]`).length }} to fix</span>
            </div>
            <div class="muted small">
              <code class="plain">{{ p.id }}</code> · {{ p.pages.length }} page{{ p.pages.length > 1 ? 's' : '' }}
            </div>
            <div v-if="p.match.length" class="chips">
              <span v-for="m in p.match.slice(0, 4)" :key="m" class="chip chip-outline" :class="{ focused: focused.includes(m) }">{{ m }}</span>
              <span v-if="p.match.length > 4" class="chip chip-outline">+{{ p.match.length - 4 }}</span>
            </div>
            <div v-else-if="p.id !== 'default'" class="small warn-text">No window matches it yet</div>
          </div>
          <Icon name="chevron_right" class="muted chev" />
        </RouterLink>
        <div class="tools">
          <button class="icon-btn st" type="button" :disabled="p.id === 'default' || i <= 1" v-tip="'Move up'" @click="move(i, -1)">
            <Icon name="arrow_upward" :size="20" />
          </button>
          <button class="icon-btn st" type="button" :disabled="p.id === 'default' || i >= profiles.length - 1" v-tip="'Move down'" @click="move(i, 1)">
            <Icon name="arrow_downward" :size="20" />
          </button>
          <button class="icon-btn st" type="button" v-tip="'Duplicate'" @click="duplicate(p, i)"><Icon name="content_copy" :size="20" /></button>
          <button class="icon-btn st" type="button" :disabled="p.id === 'default'" v-tip="p.id === 'default' ? 'The default profile stays' : 'Delete'" @click="remove(p, i)">
            <Icon name="delete" :size="20" />
          </button>
        </div>
      </article>
    </section>
    <p class="muted small">When several profiles match a window, the first one in this list wins.</p>

    <M3Dialog v-model="creating" title="New profile">
      <form class="form" @submit.prevent="create">
        <div class="seg" role="radiogroup" aria-label="Start from">
          <button type="button" role="radio" class="st" :class="{ on: !presetId }" :aria-checked="!presetId" @click="pickPreset('')">
            <Icon v-if="!presetId" name="check" :size="18" /> Blank
          </button>
          <button
            v-for="p in PRESETS"
            :key="p.id"
            type="button"
            role="radio"
            class="st"
            :class="{ on: presetId === p.id }"
            :aria-checked="presetId === p.id"
            v-tip="p.hint"
            @click="pickPreset(p.id)"
          >
            <Icon :name="presetId === p.id ? 'check' : p.icon" :size="18" /> {{ p.name }}
          </button>
        </div>
        <TextField v-model="newName" label="Name" placeholder="Firefox, Code, Games…" />
        <TextField
          :model-value="effectiveId"
          label="Id"
          mono
          :error="idError"
          help="Used in the config file. Made from the name unless you change it."
          @update:model-value="((newId = $event), (idEdited = true))"
        />
        <p v-if="preset" class="muted small" style="margin: 0">{{ preset.hint }}. {{ preset.setup }}</p>
        <p v-else class="muted small" style="margin: 0">It starts with one page of buttons. Add the window classes it should match on the next screen.</p>
        <button type="submit" class="sr-only" tabindex="-1">Create</button>
      </form>
      <template #actions>
        <button class="btn btn-text st" type="button" @click="creating = false">Cancel</button>
        <button class="btn btn-filled st" type="button" :disabled="!newName.trim() || !!idError" @click="create">Create</button>
      </template>
    </M3Dialog>
  </div>
</template>

<style scoped>
.new-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  background: var(--tc);
  color: var(--on-tc);
}
.intro {
  margin: -6px 0 0;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.prof {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px 8px 8px;
}
.prof.bad {
  box-shadow: inset 0 0 0 1px var(--error);
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 10px 12px;
  border-radius: 22px;
  color: inherit;
  text-decoration: none;
}
.thumb {
  width: 76px;
  flex-shrink: 0;
  padding: 4px;
}
.thumb :deep(.round) {
  box-shadow: 0 0 0 3px #111;
}
.info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.name-row h2 {
  font-size: 20px;
  min-width: 0;
}
.chip.active {
  background: var(--pc);
  color: var(--on-pc);
}
.chip.err {
  background: var(--ec);
  color: var(--on-ec);
}
.chip.focused {
  border-color: var(--primary);
  color: var(--primary);
}
.chips {
  margin-top: 4px;
  gap: 6px;
}
.warn-text {
  color: var(--warn);
}
.tools {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.tools .icon-btn {
  border: none;
}
@media (max-width: 760px) {
  .prof {
    flex-direction: column;
    align-items: stretch;
  }
  .chev {
    display: none;
  }
  .main {
    gap: 12px;
  }
  .thumb {
    width: 60px;
  }
}
</style>
