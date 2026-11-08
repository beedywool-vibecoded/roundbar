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
import ShapePicker from '@/components/ShapePicker.vue'
import M3Slider from '@/components/M3Slider.vue'
import ImagePageEditor from '@/components/ImagePageEditor.vue'
import ClockPageEditor from '@/components/ClockPageEditor.vue'
import TimerPageEditor from '@/components/TimerPageEditor.vue'
import { api } from '@/lib/api'
import { DIAL_SOURCES, LIMITS, TOUCHPAD_DEFAULTS, MEDIA_BUTTONS, MEDIA_SHAPE_DEFAULTS, STAT_ITEMS, newButton, nextButtonId, pageTypeInfo } from '@/lib/model'
import { config, daemonUp, dirty, findProfile, issuesUnder, live, profileIndex, profilePalette } from '@/lib/store'
import { lockScroll, unlockScroll } from '@/lib/scrollLock'
import type { Action, Button, ButtonShape, DialPage, DialSource, MediaButton, Page, StatItem } from '@/lib/types'
import { run } from '@/lib/ui'

const props = defineProps<{ pid: string; page: string }>()

const profile = computed(() => findProfile(props.pid))
const pageIdx = computed(() => profile.value?.pages.findIndex((p) => p.id === props.page) ?? -1)
const page = computed<Page | undefined>(() => profile.value?.pages[pageIdx.value])
// The profile's own color when it has one
const palette = computed(() => profilePalette(profile.value))
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

// ---- touchpad ----
const touchSpeed = computed({
  get: () => (page.value?.type === 'touchpad' ? (page.value.speed ?? TOUCHPAD_DEFAULTS.speed) : TOUCHPAD_DEFAULTS.speed),
  set: (v: number) => page.value?.type === 'touchpad' && (page.value.speed = v),
})
const GESTURES: [string, string][] = [
  ['Drag', 'move pointer'],
  ['Tap', 'left click'],
  ['Hold', 'right click'],
  ['Double-tap + drag', 'drag & drop'],
  ['Slide along the ring', 'scroll'],
  ['Swipe sideways from the ring', 'change page'],
  ['Swipe down from the top of the ring', 'quick settings'],
]

// ---- media transport shapes: the default is stored as a missing key ----
function mediaShape(k: MediaButton): ButtonShape {
  const p = page.value
  return (p?.type === 'media' && p.shapes?.[k]) || MEDIA_SHAPE_DEFAULTS[k]
}
function setMediaShape(k: MediaButton, s: ButtonShape) {
  const p = page.value
  if (p?.type !== 'media') return
  const { [k]: _drop, ...rest } = p.shapes ?? {}
  const next = s === MEDIA_SHAPE_DEFAULTS[k] ? rest : { ...rest, [k]: s }
  if (Object.keys(next).length) p.shapes = next
  else delete p.shapes
}
const mediaFill = (k: MediaButton) => (k === 'play' || k === 'playing' ? palette.value.primaryContainer : palette.value.secondaryContainer)

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
              slideshow
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
          <p class="muted small hint">
            Drawn with the {{ profile.seed ? 'profile color' : 'device palette' }} ({{ config!.theme.mode }}), with sample values.
          </p>
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

            <template v-if="page.type === 'touchpad'">
              <div class="setting col" style="border-top: none; padding-bottom: 0">
                <div class="row-head">
                  <span>Pointer speed</span>
                  <span class="muted">{{ touchSpeed }}</span>
                </div>
                <M3Slider v-model="touchSpeed" label="Pointer speed" :min="1" :max="10" :step="1" />
              </div>
              <div>
                <p class="section-label" style="margin-top: 0">Gestures</p>
                <dl class="gestures">
                  <template v-for="[g, a] in GESTURES" :key="g">
                    <dt>{{ g }}</dt>
                    <dd class="muted">{{ a }}</dd>
                  </template>
                </dl>
              </div>
            </template>

            <p v-else-if="page.type === 'image'" class="muted small" style="margin: 0">
              Fills the screen with a picture. Several pictures cross-fade as a slideshow; a GIF plays on its own.
            </p>

            <p v-else-if="page.type === 'clock'" class="muted small" style="margin: 0">
              The time in large, from the PC clock. Optional weather and world clocks below it.
            </p>

            <p v-else-if="page.type === 'timer'" class="muted small" style="margin: 0">
              A countdown: turn the bezel ring to set the minutes, tap the center to start.
            </p>

            <p v-else-if="page.type === 'mixer'" class="muted small" style="margin: 0">
              The output devices as chips (tap one to switch), then one volume slider per app playing sound. Tap an app's icon to mute it. Nothing to set up.
            </p>

            <p v-else-if="page.type === 'windows'" class="muted small" style="margin: 0">
              The open windows as icons on a ring, most recent first (up to 8). Tap one to bring it to the front. Needs the GNOME extension. Nothing to set up.
            </p>

            <p v-else-if="page.type === 'clipboard'" class="muted small" style="margin: 0">
              The last 10 texts you copied on the PC, as a scrolling list. Tap one to paste it into the focused app. Nothing to set up.
            </p>

            <div v-else-if="page.type === 'discord'" class="form" style="gap: 8px">
              <p class="muted small" style="margin: 0">
                Your current Discord voice channel: members around the ring (a green ring while they talk, a badge when muted), big mute and deafen buttons, and leave.
                Shows "Not in a voice channel" otherwise. Nothing to set up on the page.
              </p>
              <p v-if="!live.status?.discord?.connected" class="warn-line small">
                <Icon name="link_off" :size="16" />
                <span>Discord isn't connected yet. <RouterLink to="/device#discord">Set it up on the Device page</RouterLink></span>
              </p>
            </div>

            <div v-else-if="page.type === 'obs'" class="form" style="gap: 8px">
              <p class="muted small" style="margin: 0">
                Live OBS: the center starts and stops recording (with the elapsed time), the ring shows your OBS scenes with the current one highlighted (tap to switch, up to 8),
                and a LIVE label in the record button while streaming. OBS is only contacted while this page is on screen or an OBS button is pressed. Nothing to set up on the page.
              </p>
              <p v-if="!live.status?.obs?.connected" class="warn-line small">
                <Icon name="link_off" :size="16" />
                <span>OBS isn't connected yet. <RouterLink to="/device#obs">Set it up on the Device page</RouterLink></span>
              </p>
            </div>

            <p v-else-if="page.type === 'game'" class="muted small" style="margin: 0">
              The game in focus (Steam games, <code>steam_app_*</code> windows): its library art blurred behind, the game name and how long you've been playing,
              with two buttons that press Steam's own shortcuts: <b>Shift+Tab</b> for the overlay and <b>F12</b> for a screenshot.
              Put it in a profile that matches <code>steam_app_*</code> so it shows up on its own. Nothing to set up.
            </p>

            <p v-else-if="page.type === 'mangohud'" class="muted small" style="margin: 0">
              Live numbers from <a href="https://github.com/flightlessmango/MangoHud" target="_blank" rel="noopener">MangoHud</a>: FPS, average and 1% low over 30 s,
              a frame-time graph, CPU and GPU load and temperature, GPU power and VRAM. MangoHud has no live API, so Roundbar turns its CSV logging on
              (through MangoHud's control socket) only while this page is on screen, and deletes those log files afterwards. The in-game HUD doesn't change.
              The first time, Roundbar adds <code>control=mangohud-%p</code> to <code>MangoHud.conf</code>: restart games that were already running.
            </p>

            <p v-else-if="page.type === 'media'" class="muted small" style="margin: 0">
              Shows the current player (MPRIS) with previous, play / pause and next, and the volume on the edge arc.
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

        <ImagePageEditor v-if="page.type === 'image'" :key="`${pid}/${page.id}`" :page="page" />
        <ClockPageEditor v-if="page.type === 'clock'" :key="`${pid}/${page.id}`" :page="page" />
        <TimerPageEditor v-if="page.type === 'timer'" :key="`${pid}/${page.id}`" :page="page" />

        <div v-if="page.type === 'media'" class="card">
          <div class="card-head"><h2>Button shapes</h2></div>
          <div class="form">
            <div v-for="b in MEDIA_BUTTONS" :key="b.key">
              <p class="section-label" style="margin-top: 0">{{ b.label }}</p>
              <ShapePicker
                :model-value="mediaShape(b.key)"
                :fallback="MEDIA_SHAPE_DEFAULTS[b.key]"
                :fill="mediaFill(b.key)"
                :label="`${b.label} shape`"
                @update:model-value="setMediaShape(b.key, $event)"
              />
            </div>
            <p class="tf-help" style="margin: 0">The play button takes its paused or playing shape with the player state. Click it in the preview to switch.</p>
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
.row-head {
  display: flex;
  justify-content: space-between;
}
.warn-line {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  color: var(--on-surface);
}
.warn-line a {
  color: var(--primary);
}
.gestures {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 6px 16px;
  margin: 8px 0 0;
  font-size: 14px;
}
.gestures dd {
  margin: 0;
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
