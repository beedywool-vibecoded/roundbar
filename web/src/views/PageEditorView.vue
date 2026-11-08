<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '@/components/Icon.vue'
import RoundPreview from '@/components/RoundPreview.vue'
import ButtonEditor from '@/components/ButtonEditor.vue'
import ActionEditor from '@/components/ActionEditor.vue'
import IconPicker from '@/components/IconPicker.vue'
import TextField from '@/components/TextField.vue'
import NumField from '@/components/NumField.vue'
import M3Select from '@/components/M3Select.vue'
import M3Switch from '@/components/M3Switch.vue'
import M3Shape from '@/components/M3Shape.vue'
import ShapeLoader from '@/components/ShapeLoader.vue'
import { api } from '@/lib/api'
import { DIAL_SOURCES, LIMITS, STAT_ITEMS, newButton, nextButtonId, pageTypeInfo } from '@/lib/model'
import { config, daemonUp, dirty, findProfile, issuesUnder, profileIndex } from '@/lib/store'
import { devicePalette } from '@/lib/theme'
import { lockScroll, unlockScroll } from '@/lib/scrollLock'
import type { Action, Button, DialPage, DialSource, Page, StatItem } from '@/lib/types'
import { run } from '@/lib/ui'

const props = defineProps<{ pid: string; page: string }>()

const profile = computed(() => findProfile(props.pid))
const pageIdx = computed(() => profile.value?.pages.findIndex((p) => p.id === props.page) ?? -1)
const page = computed<Page | undefined>(() => profile.value?.pages[pageIdx.value])
const palette = computed(() => devicePalette(config.value!.theme.seed, config.value!.theme.mode))
const pagePath = computed(() => `profiles[${profileIndex(props.pid)}].pages[${pageIdx.value}]`)
const problems = computed(() => issuesUnder(pagePath.value))

// ---- selection and the bottom sheet on narrow screens ----
const narrowMq = window.matchMedia('(max-width: 1100px)')
const narrow = ref(narrowMq.matches)
const onMq = () => (narrow.value = narrowMq.matches)
onMounted(() => narrowMq.addEventListener('change', onMq))
onBeforeUnmount(() => narrowMq.removeEventListener('change', onMq))

const selected = ref<string | null>(null)
onMounted(() => {
  const p = page.value
  if (p?.type === 'radial' && !narrow.value) selected.value = p.buttons[0]?.id ?? null
})

const selectedButton = computed<Button | null>(() => {
  const p = page.value
  if (p?.type !== 'radial' || !selected.value) return null
  if (p.center?.id === selected.value) return p.center
  return p.buttons.find((b) => b.id === selected.value) ?? null
})
const sheetOpen = computed(() => narrow.value && !!selectedButton.value)
let locked = false
watch(
  sheetOpen,
  (v) => {
    if (v && !locked) lockScroll()
    if (!v && locked) unlockScroll()
    locked = v
  },
  { immediate: true },
)
onBeforeUnmount(() => locked && unlockScroll())
const selectedIsCenter = computed(() => page.value?.type === 'radial' && page.value.center?.id === selected.value)
const selectedIndex = computed(() => (page.value?.type === 'radial' ? page.value.buttons.findIndex((b) => b.id === selected.value) : -1))

function updateSelected(b: Button) {
  const p = page.value
  if (p?.type !== 'radial') return
  if (selectedIsCenter.value) p.center = b
  else {
    const i = p.buttons.findIndex((x) => x.id === b.id)
    if (i >= 0) p.buttons[i] = b
  }
}
function addButton() {
  const p = page.value
  if (p?.type !== 'radial' || p.buttons.length >= LIMITS.buttons) return
  const b = newButton(p, 'star', '')
  p.buttons.push(b)
  selected.value = b.id
}
function addCenter() {
  const p = page.value
  if (p?.type !== 'radial' || p.center) return
  p.center = { id: nextButtonId(p), icon: 'play_pause', label: '', color: 'primary', tap: { type: 'media', cmd: 'play_pause' } }
  selected.value = p.center.id
}
function removeSelected() {
  const p = page.value
  if (p?.type !== 'radial') return
  if (selectedIsCenter.value) {
    delete p.center
    selected.value = narrow.value ? null : (p.buttons[0]?.id ?? null)
    return
  }
  const i = selectedIndex.value
  if (i < 0 || p.buttons.length <= 1) return
  p.buttons.splice(i, 1)
  selected.value = narrow.value ? null : (p.buttons[Math.min(i, p.buttons.length - 1)]?.id ?? null)
}
function moveSelected(d: number) {
  const p = page.value
  const i = selectedIndex.value
  if (p?.type !== 'radial' || i < 0) return
  const j = i + d
  if (j < 0 || j >= p.buttons.length) return
  const arr = p.buttons
  ;[arr[i], arr[j]] = [arr[j]!, arr[i]!]
}

// ---- page fields ----
const title = computed({
  get: () => page.value?.title ?? '',
  set: (v: string) => {
    const p = page.value
    if (!p) return
    if (v) p.title = v
    else delete p.title
  },
})

function toggleStat(item: StatItem) {
  const p = page.value
  if (p?.type !== 'stats') return
  if (p.items.includes(item)) p.items = p.items.filter((x) => x !== item)
  else if (p.items.length < LIMITS.stats) p.items = [...p.items, item]
}

const dial = computed(() => (page.value?.type === 'dial' ? (page.value as DialPage) : null))
const sourceOptions = DIAL_SOURCES.map((s) => ({ value: s.value, label: s.label }))
const dialSource = computed({
  get: () => dial.value?.source ?? 'volume',
  set: (s: DialSource) => {
    const d = dial.value
    if (!d) return
    const prev = DIAL_SOURCES.find((x) => x.value === d.source)
    const next = DIAL_SOURCES.find((x) => x.value === s)!
    // Follow the source while icon and label are still the defaults
    if (!d.icon || d.icon === prev?.icon) d.icon = next.icon
    if (!d.label || d.label === prev?.label) d.label = next.label
    d.source = s
    if (s === 'custom') {
      d.step ??= 1
      d.onUp ??= { type: 'keys', keys: 'up' }
      d.onDown ??= { type: 'keys', keys: 'down' }
    } else {
      delete d.step
      delete d.onUp
      delete d.onDown
    }
  },
})
const dialStep = computed({
  get: () => dial.value?.step ?? 1,
  set: (v: number) => dial.value && (dial.value.step = v),
})
function toggleDialAction(k: 'onUp' | 'onDown', on: boolean) {
  const d = dial.value
  if (!d) return
  if (on) d[k] = { type: 'keys', keys: k === 'onUp' ? 'up' : 'down' }
  else delete d[k]
}

// ---- device ----
const showing = ref(false)
function show() {
  run(showing, () => api.show(props.pid, props.page), 'Shown on the device')
}
</script>

<template>
  <div v-if="!profile || !page" class="card empty">
    <Icon name="error" :size="32" />
    <h2>This page doesn't exist</h2>
    <p class="muted" style="margin: 0">It may have been deleted, or its profile renamed.</p>
    <RouterLink class="btn btn-filled st" to="/">Back to profiles</RouterLink>
  </div>

  <div v-else class="page">
    <div class="page-title">
      <RouterLink class="icon-btn st" :to="`/profiles/${pid}`" v-tip="`Back to ${profile.name}`"><Icon name="arrow_back" /></RouterLink>
      <h1 class="ellipsis">{{ page.title || pageTypeInfo(page.type).label }}</h1>
      <span class="chip type-chip"><Icon :name="pageTypeInfo(page.type).icon" :size="16" /> {{ pageTypeInfo(page.type).label }}</span>
      <button class="btn btn-outlined st" type="button" :disabled="showing || !daemonUp" v-tip="!daemonUp ? 'Needs the daemon running' : dirty ? 'The device shows the saved config: save first to see your edits' : 'Jump to this page on the device'" @click="show">
        <ShapeLoader v-if="showing" small label="Sending" />
        <Icon v-else name="cast" :size="18" />
        Show on device
      </button>
    </div>

    <nav class="page-tabs" aria-label="Pages of this profile">
      <RouterLink
        v-for="(p, i) in profile.pages"
        :key="p.id"
        :to="`/profiles/${pid}/pages/${p.id}`"
        class="fchip st"
        :class="{ on: i === pageIdx }"
        :aria-current="i === pageIdx ? 'page' : undefined"
      >
        <Icon :name="pageTypeInfo(p.type).icon" :size="16" /> {{ p.title || pageTypeInfo(p.type).label }}
      </RouterLink>
    </nav>

    <div class="editor">
      <section class="preview-col">
        <div class="card preview-card">
          <div class="preview-wrap">
            <RoundPreview
              :page="page"
              :palette="palette"
              :selected="selected"
              :editable="page.type === 'radial'"
              :page-index="pageIdx"
              :page-count="profile.pages.length"
              @select="selected = $event"
              @add-center="addCenter"
            />
          </div>
          <template v-if="page.type === 'radial'">
            <div class="strip" role="listbox" aria-label="Buttons">
              <button
                v-for="(b, i) in page.buttons"
                :key="b.id"
                type="button"
                role="option"
                class="mini st"
                :class="{ on: selected === b.id }"
                :aria-selected="selected === b.id"
                v-tip="b.label || `Button ${i + 1}`"
                @click="selected = b.id"
              >
                <Icon :name="b.icon" :size="20" :fill="selected === b.id" />
              </button>
              <button
                v-if="page.center"
                type="button"
                role="option"
                class="mini center st"
                :class="{ on: selected === page.center.id }"
                :aria-selected="selected === page.center.id"
                v-tip="page.center.label || 'Center button'"
                @click="selected = page.center.id"
              >
                <Icon :name="page.center.icon" :size="20" :fill="selected === page.center.id" />
              </button>
              <button class="btn btn-tonal st add" type="button" :disabled="page.buttons.length >= LIMITS.buttons" @click="addButton">
                <Icon name="add" :size="18" /> Button
              </button>
              <button v-if="!page.center" class="btn btn-text st add" type="button" @click="addCenter">
                <Icon name="add" :size="18" /> Center
              </button>
            </div>
            <p class="muted small hint">
              {{ page.buttons.length }} / {{ LIMITS.buttons }} on the ring. Click a button to edit it. Its label shows in the middle, as after a press.{{ page.center ? '' : ' Without a center button the page title is shown there.' }}
            </p>
          </template>
          <p class="muted small hint">Drawn with the device palette ({{ config!.theme.mode }}), with sample values.</p>
        </div>
      </section>

      <section class="side-col">
        <div v-if="problems.length" class="card problems">
          <Icon name="warning" :size="20" />
          <div class="grow">
            <div v-for="(p, i) in problems.slice(0, 4)" :key="i" class="small">{{ p.message }} <span class="muted">({{ p.path.replace(pagePath, 'page') }})</span></div>
            <div v-if="problems.length > 4" class="small muted">and {{ problems.length - 4 }} more</div>
          </div>
        </div>

        <div class="card">
          <div class="card-head"><h2>Page</h2></div>
          <div class="form">
            <TextField v-model="title" label="Title (optional)" :placeholder="page.type === 'radial' ? 'Shown in the middle when there is no center button' : 'Shown in this configurator'" :max="24" />

            <p v-if="page.type === 'media'" class="muted small" style="margin: 0">
              Shows the current player (MPRIS) with previous, play / pause and next, and the volume on the edge arc. Nothing to set up.
            </p>

            <template v-else-if="page.type === 'stats'">
              <div>
                <p class="section-label" style="margin-top: 0">Gauges <span class="muted small">({{ page.items.length }} / {{ LIMITS.stats }})</span></p>
                <div class="chips" style="margin-top: 8px">
                  <button
                    v-for="s in STAT_ITEMS"
                    :key="s.value"
                    type="button"
                    class="fchip st"
                    :class="{ on: page.items.includes(s.value) }"
                    :aria-pressed="page.items.includes(s.value)"
                    :disabled="!page.items.includes(s.value) && page.items.length >= LIMITS.stats"
                    @click="toggleStat(s.value)"
                  >
                    <Icon :name="page.items.includes(s.value) ? 'check' : s.icon" :size="16" /> {{ s.label }}
                  </button>
                </div>
                <p class="tf-help" :class="{ err: !page.items.length }" style="margin-left: 0">
                  {{ page.items.length ? 'Shown in the order picked. Collected only while this page is on screen.' : 'Pick at least one gauge' }}
                </p>
              </div>
            </template>

            <template v-else-if="dial">
              <M3Select v-model="dialSource" :options="sourceOptions" label="Controls" />
              <div class="form-row">
                <IconPicker v-model="dial.icon" />
                <TextField v-model="dial.label" label="Label" :max="LIMITS.label" :error="dial.label.length > LIMITS.label ? `At most ${LIMITS.label} characters` : null" />
              </div>
              <template v-if="dial.source === 'custom'">
                <NumField v-model="dialStep" label="Step" :min="1" :max="20" help="Dial movement per action, 1 to 20" />
                <div class="hold-row">
                  <div class="grow"><div>Turn up</div><div class="muted small">Runs once per step clockwise</div></div>
                  <M3Switch :model-value="!!dial.onUp" label="Turn up action" @update:model-value="toggleDialAction('onUp', $event)" />
                </div>
                <ActionEditor v-if="dial.onUp" :model-value="dial.onUp" :pages="profile.pages" @update:model-value="dial.onUp = $event as Action" />
                <div class="hold-row">
                  <div class="grow"><div>Turn down</div><div class="muted small">Runs once per step counter-clockwise</div></div>
                  <M3Switch :model-value="!!dial.onDown" label="Turn down action" @update:model-value="toggleDialAction('onDown', $event)" />
                </div>
                <ActionEditor v-if="dial.onDown" :model-value="dial.onDown" :pages="profile.pages" @update:model-value="dial.onDown = $event as Action" />
              </template>
            </template>
          </div>
        </div>

        <template v-if="page.type === 'radial'">
          <div v-if="selectedButton && !narrow" class="card">
            <ButtonEditor
              :key="selectedButton.id"
              :model-value="selectedButton"
              :pages="profile.pages"
              :palette="palette"
              :center="selectedIsCenter"
              :index="selectedIndex"
              :count="page.buttons.length"
              @update:model-value="updateSelected"
              @remove="removeSelected"
              @move="moveSelected"
              @close="selected = null"
            />
          </div>
          <div v-else-if="!narrow" class="card empty">
            <M3Shape kind="cookie" :size="64" fill="var(--secc)" />
            <p class="muted" style="margin: 0">Select a button in the preview to edit it.</p>
          </div>
        </template>
      </section>
    </div>

    <!-- Bottom sheet for the button editor on narrow screens -->
    <Transition name="sheet">
      <div v-if="sheetOpen && page.type === 'radial' && selectedButton" class="scrim" @click.self="selected = null">
        <div class="sheet" role="dialog" aria-label="Edit button">
          <span class="handle" aria-hidden="true" />
          <ButtonEditor
            :key="selectedButton.id"
            :model-value="selectedButton"
            :pages="profile.pages"
            :palette="palette"
            :center="selectedIsCenter"
            :index="selectedIndex"
            :count="page.buttons.length"
            @update:model-value="updateSelected"
            @remove="removeSelected"
            @move="moveSelected"
            @close="selected = null"
          />
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.page-title h1 {
  flex: 1 1 140px;
  min-width: 0;
}
@media (max-width: 560px) {
  .type-chip {
    display: none;
  }
}
.page-tabs {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
}
.page-tabs .fchip {
  text-decoration: none;
}
.editor {
  display: grid;
  grid-template-columns: minmax(300px, 440px) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.preview-col {
  position: sticky;
  top: 84px;
}
.preview-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 32px 22px 20px;
}
.preview-wrap {
  width: min(100%, 380px);
  padding: 11px;
}
.strip {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
}
.mini {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border: none;
  border-radius: 14px;
  background: var(--sch);
  color: var(--on-surface);
  transition: border-radius 0.25s var(--ease), background 0.2s var(--ease);
}
.mini.center {
  border-radius: 22px;
  box-shadow: inset 0 0 0 2px var(--outline-v);
}
.mini.on {
  background: var(--pc);
  color: var(--on-pc);
  border-radius: 22px;
}
.add {
  height: 44px;
  border-radius: 22px;
}
.hint {
  margin: 0;
  text-align: center;
}
.side-col {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.problems {
  display: flex;
  gap: 12px;
  background: var(--ec);
  color: var(--on-ec);
  padding: 14px 18px;
  border-radius: 20px;
}
.problems .muted {
  color: inherit;
  opacity: 0.75;
}
.hold-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--sch);
}
.scrim {
  display: none;
}

@media (max-width: 1100px) {
  .editor {
    grid-template-columns: minmax(0, 1fr);
  }
  .preview-col {
    position: static;
  }
  .scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 40;
    background: rgba(0, 0, 0, 0.4);
  }
  .sheet {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    max-height: 86vh;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 10px 20px 28px;
    border-radius: 28px 28px 0 0;
    background: var(--scl, var(--sc));
    transition: translate 0.3s var(--ease-decel);
  }
  .handle {
    display: block;
    width: 32px;
    height: 4px;
    margin: 0 auto 14px;
    border-radius: 2px;
    background: var(--on-sv);
    opacity: 0.5;
  }
  .sheet-enter-from,
  .sheet-leave-to {
    opacity: 0;
  }
  .sheet-enter-from .sheet,
  .sheet-leave-to .sheet {
    translate: 0 100%;
  }
  .sheet-enter-active,
  .sheet-leave-active {
    transition: opacity 0.25s;
  }
}
</style>
