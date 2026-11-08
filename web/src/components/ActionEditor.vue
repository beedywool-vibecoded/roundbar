<script setup lang="ts">
// Edits one Action of any type in the spec, with a Test button that runs it for real.
import { computed, onBeforeUnmount, ref } from 'vue'
import Icon from './Icon.vue'
import M3Select from './M3Select.vue'
import TextField from './TextField.vue'
import KeyRecorder from './KeyRecorder.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api } from '@/lib/api'
import { onWindows } from '@/lib/store'
import { ACTION_TYPES, LIMITS, MEDIA_CMDS, MIC_CMDS, OBS_DEFAULT_INPUT, OBS_REQUESTS, checkAction, defaultAction, type Issue } from '@/lib/model'
import { effectiveLayout, layoutName, untypeable, validText } from '@/lib/layouts'
import { comboFromEvent, formatCombo, heldMods, modifierOf, type Modifier } from '@/lib/keys'
import { config, daemonUp, live } from '@/lib/store'
import type { Action, ActionType, MediaCmd, MicCmd, ObsRequest, Page, Step } from '@/lib/types'
import { run, toast } from '@/lib/ui'

const model = defineModel<Action>({ required: true })
const props = defineProps<{ pages: Page[] }>()

const type = computed({
  get: () => model.value.type,
  set: (t: ActionType) => t !== model.value.type && (model.value = defaultAction(t, props.pages)),
})
const typeOptions = ACTION_TYPES.map((a) => ({ value: a.value, label: a.label, hint: a.hint }))
const pageOptions = computed(() => props.pages.map((p) => ({ value: p.id, label: p.title || p.id, hint: `${p.type} · ${p.id}` })))

const problems = computed(() => {
  const out: Issue[] = []
  checkAction(model.value, props.pages, '', out)
  return out
})
// The layout the device will type with (configured, or detected for auto).
const typingLayout = computed(() => effectiveLayout(config.value?.device.keyboardLayout, live.status?.keyboard?.detected))
const textError = (t: string) => {
  if (t.length > LIMITS.text) return `At most ${LIMITS.text} characters`
  if (!validText(t)) return 'Has characters no keyboard layout can type'
  const l = typingLayout.value
  const bad = l ? untypeable(t, l) : []
  return bad.length ? `${layoutName(l)} can't type ${bad.join(' ')}` : null
}
const textHelp = computed(() => {
  const l = typingLayout.value
  const auto = (config.value?.device.keyboardLayout ?? 'auto') === 'auto'
  if (!l) return 'Typed with the PC keyboard layout (set on the Device page)'
  return `Typed with the ${layoutName(l)} layout${auto ? ' (detected)' : ''}, set on the Device page`
})

// Flat view of the union's fields for the template
const a = computed(() => model.value as unknown as Record<string, string>)
function set(field: string, v: unknown) {
  model.value = { ...model.value, [field]: v } as Action
}
// OBS: each request carries only its own field
function setObsRequest(r: ObsRequest) {
  const prev = model.value.type === 'obs' ? model.value : null
  const next: Action = { type: 'obs', request: r }
  if (r === 'SetScene') next.scene = prev?.scene ?? obsStatus.value?.current ?? ''
  if (r === 'ToggleMute') next.input = prev?.input || OBS_DEFAULT_INPUT
  model.value = next
}
const obsStatus = computed(() => live.status?.obs)
const sceneSuggestions = computed(() => (obsStatus.value?.connected ? (obsStatus.value.scenes ?? []).filter((s) => s !== a.value.scene) : []))
const inputSuggestions = computed(() => ['Mic/Aux', 'Desktop Audio'].filter((x) => x !== a.value.input))

const steps = computed(() => (model.value.type === 'macro' ? model.value.steps : []))
function setSteps(s: Step[]) {
  if (model.value.type === 'macro') model.value = { ...model.value, steps: s }
}

type StepKind = 'keys' | 'text' | 'delay' | 'command'
const STEP_KINDS: { value: StepKind; label: string; icon: string }[] = [
  { value: 'keys', label: 'Keys', icon: 'keyboard' },
  { value: 'text', label: 'Text', icon: 'text_fields' },
  { value: 'delay', label: 'Delay', icon: 'timer' },
  { value: 'command', label: 'Command', icon: 'terminal' },
]
const kindOf = (s: Step): StepKind => Object.keys(s)[0] as StepKind
const kindInfo = (s: Step) => STEP_KINDS.find((k) => k.value === kindOf(s)) ?? STEP_KINDS[0]!
const stepText = (s: Step) => ('text' in s ? s.text : '')
const stepKeys = (s: Step) => ('keys' in s ? s.keys : '')
const stepDelay = (s: Step) => ('delay' in s ? s.delay : 0)
function newStep(k: StepKind): Step {
  return k === 'keys' ? { keys: '' } : k === 'text' ? { text: '' } : k === 'delay' ? { delay: 250 } : { command: '' }
}
function addStep(k: StepKind) {
  setSteps([...steps.value, newStep(k)])
}
function updateStep(i: number, s: Step) {
  setSteps(steps.value.map((x, j) => (j === i ? s : x)))
}
function moveStep(i: number, d: number) {
  const s = [...steps.value]
  const j = i + d
  if (j < 0 || j >= s.length) return
  ;[s[i], s[j]] = [s[j]!, s[i]!]
  setSteps(s)
}
function removeStep(i: number) {
  setSteps(steps.value.filter((_, j) => j !== i))
}
function stepValue(s: Step): string {
  if ('keys' in s) return s.keys
  if ('text' in s) return s.text
  if ('command' in s) return s.command
  return ''
}
function setDelay(i: number, v: string) {
  const n = Number.parseInt(v, 10)
  updateStep(i, { delay: Number.isNaN(n) ? 0 : n })
}

const MAX_TEXT: number = LIMITS.text
const MAX_DELAY: number = LIMITS.delay
const MAX_STEPS: number = LIMITS.steps

// ---- macro recorder: key presses become `keys` steps, pauses become `delay` steps ----
const GAP_MS = 150
const IDLE_MS = 10_000
const recording = ref(false)
const recorded = ref(0)
let before: Step[] = []
let last: number | null = null
let lone: Modifier | null = null
let idle: ReturnType<typeof setTimeout> | undefined

function armIdle() {
  clearTimeout(idle)
  idle = setTimeout(stopRecording, IDLE_MS)
}
function record(keys: string) {
  const now = performance.now()
  const add: Step[] = []
  const room = MAX_STEPS - steps.value.length
  if (room <= 0) return stopRecording()
  if (last !== null && room >= 2) {
    const gap = now - last
    if (gap > GAP_MS) add.push({ delay: Math.min(MAX_DELAY, Math.round(gap / 50) * 50) })
  }
  add.push({ keys })
  last = now
  recorded.value += add.length
  const next = [...steps.value, ...add]
  setSteps(next)
  if (next.length >= MAX_STEPS) {
    toast(`A macro has at most ${MAX_STEPS} steps: recording stopped`)
    stopRecording()
  } else armIdle()
}
function onRecKey(e: KeyboardEvent) {
  e.preventDefault()
  e.stopPropagation()
  if (e.repeat) return
  if (e.key === 'Escape') return stopRecording()
  const m = modifierOf(e)
  if (m) {
    // Counts only if released without another key
    lone = heldMods(e).length === 1 ? m : null
    return
  }
  lone = null
  const c = comboFromEvent(e)
  if (c) record(formatCombo(c))
}
function onRecKeyUp(e: KeyboardEvent) {
  e.preventDefault()
  e.stopPropagation()
  if (lone && modifierOf(e) === lone && !heldMods(e).length) record(lone)
  lone = null
}
function startRecording() {
  if (recording.value || steps.value.length >= MAX_STEPS) return
  before = [...steps.value]
  recorded.value = 0
  last = null
  lone = null
  recording.value = true
  ;(document.activeElement as HTMLElement | null)?.blur()
  window.addEventListener('keydown', onRecKey, true)
  window.addEventListener('keyup', onRecKeyUp, true)
  window.addEventListener('blur', stopRecording)
  armIdle()
}
function stopRecording() {
  if (!recording.value) return
  recording.value = false
  clearTimeout(idle)
  window.removeEventListener('keydown', onRecKey, true)
  window.removeEventListener('keyup', onRecKeyUp, true)
  window.removeEventListener('blur', stopRecording)
  const n = recorded.value
  if (!n) return
  const restore = before
  toast(`Recorded ${n} step${n > 1 ? 's' : ''}`, false, { label: 'Undo', run: () => setSteps(restore) })
}
onBeforeUnmount(stopRecording)

const testing = ref(false)
function test() {
  run(testing, () => api.run(JSON.parse(JSON.stringify(model.value)) as Action), 'Sent to the daemon')
}
</script>

<template>
  <div class="ae">
    <div class="head">
      <M3Select v-model="type" class="type" :options="typeOptions" label="Action" />
      <button
        class="btn btn-tonal st"
        type="button"
        :disabled="testing || problems.length > 0 || !daemonUp"
        v-tip="!daemonUp ? 'Needs the daemon running' : problems[0]?.message ?? 'Run it now on this PC'"
        @click="test"
      >
        <ShapeLoader v-if="testing" small label="Testing" />
        <Icon v-else name="play_arrow" :size="18" />
        Test
      </button>
    </div>

    <KeyRecorder v-if="type === 'keys'" :model-value="a.keys ?? ''" @update:model-value="set('keys', $event)" />

    <TextField
      v-else-if="type === 'text'"
      :model-value="a.text ?? ''"
      label="Text to type"
      :max="MAX_TEXT"
      :error="a.text ? textError(a.text) : null"
      :help="textHelp"
      @update:model-value="set('text', $event)"
    />

    <M3Select
      v-else-if="type === 'media'"
      :model-value="a.cmd as MediaCmd"
      :options="[...MEDIA_CMDS]"
      label="Media key"
      @update:model-value="set('cmd', $event as MediaCmd)"
    />

    <template v-else-if="type === 'page'">
      <M3Select v-if="pageOptions.length" :model-value="a.page ?? ''" :options="pageOptions" label="Page" @update:model-value="set('page', $event)" />
      <p v-if="!pages.some((p) => p.id === a.page)" class="tf-help err">Pick a page of this profile</p>
    </template>

    <TextField
      v-else-if="type === 'command'"
      :model-value="a.cmd ?? ''"
      label="Shell command"
      placeholder="notify-send hello"
      mono
      :help="onWindows ? 'Run with cmd.exe /c, without a console window, as your user' : 'Run with sh -c, detached, as your user'"
      @update:model-value="set('cmd', $event)"
    />

    <TextField
      v-else-if="type === 'open'"
      :model-value="a.target ?? ''"
      label="URL, file or app"
      :placeholder="onWindows ? 'https://… or C:\\path\\to\\app.exe' : 'https://… or firefox.desktop'"
      mono
      :help="onWindows ? 'Opened by Windows: a URL, file, folder or program' : 'Opened with xdg-open, or gtk-launch for a .desktop id'"
      @update:model-value="set('target', $event)"
    />

    <M3Select
      v-else-if="type === 'mic'"
      :model-value="a.cmd as MicCmd"
      :options="[...MIC_CMDS]"
      label="Microphone"
      @update:model-value="set('cmd', $event as MicCmd)"
    />

    <template v-else-if="type === 'obs'">
      <M3Select :model-value="a.request as ObsRequest" :options="OBS_REQUESTS" label="OBS request" @update:model-value="setObsRequest($event as ObsRequest)" />
      <template v-if="a.request === 'SetScene'">
        <TextField
          :model-value="a.scene ?? ''"
          label="Scene"
          placeholder="Scene name, as in OBS"
          :error="!a.scene?.trim() ? 'Name the OBS scene to switch to' : null"
          @update:model-value="set('scene', $event)"
        />
        <div v-if="sceneSuggestions.length" class="obs-sugg">
          <span class="muted small">Scenes in OBS</span>
          <div class="chips">
            <button v-for="sc in sceneSuggestions" :key="sc" type="button" class="fchip st" @click="set('scene', sc)">
              {{ sc }}
            </button>
          </div>
        </div>
        <p v-else-if="!obsStatus?.connected" class="muted small" style="margin: 0">
          Connect OBS on the <RouterLink to="/device#obs">Device page</RouterLink> to pick from your scenes.
        </p>
      </template>
      <template v-else-if="a.request === 'ToggleMute'">
        <TextField
          :model-value="a.input ?? ''"
          label="Audio source"
          :placeholder="OBS_DEFAULT_INPUT"
          :error="!a.input?.trim() ? 'Name the OBS audio source to mute' : null"
          help="The source name as shown in the OBS audio mixer"
          @update:model-value="set('input', $event)"
        />
        <div class="chips">
          <button v-for="inp in inputSuggestions" :key="inp" type="button" class="fchip st" @click="set('input', inp)">
            {{ inp }}
          </button>
        </div>
      </template>
      <p class="muted small" style="margin: 0">
        Sent to OBS over its WebSocket, so it works even when OBS isn't focused.
        <template v-if="!obsStatus?.connected">
          <RouterLink to="/device#obs">Set up OBS</RouterLink> once on the Device page.
        </template>
      </p>
    </template>

    <div v-else-if="type === 'macro'" class="macro">
      <ol v-if="steps.length" class="steps">
        <li v-for="(s, i) in steps" :key="i" class="step">
          <span class="num">{{ i + 1 }}</span>
          <div class="step-body">
            <div class="step-kind">
              <Icon :name="kindInfo(s).icon" :size="18" />
              {{ kindInfo(s).label }}
            </div>
            <KeyRecorder v-if="kindOf(s) === 'keys'" :model-value="stepKeys(s)" @update:model-value="updateStep(i, { keys: $event })" />
            <TextField
              v-else-if="kindOf(s) === 'text'"
              :model-value="stepText(s)"
              label="Text"
              :max="MAX_TEXT"
              :error="stepText(s) ? textError(stepText(s)) : null"
              @update:model-value="updateStep(i, { text: $event })"
            />
            <label v-else-if="kindOf(s) === 'delay'" class="tf" :class="{ invalid: stepDelay(s) < 0 || stepDelay(s) > MAX_DELAY }">
              <span class="lbl">Wait</span>
              <span class="in">
                <input :value="stepDelay(s)" inputmode="numeric" @input="setDelay(i, ($event.target as HTMLInputElement).value)" />
                <span class="suffix">ms, max {{ MAX_DELAY }}</span>
              </span>
            </label>
            <TextField v-else :model-value="stepValue(s)" label="Command" mono @update:model-value="updateStep(i, { command: $event })" />
          </div>
          <div class="step-tools">
            <button class="icon-btn st" type="button" :disabled="i === 0" v-tip="'Move up'" @click="moveStep(i, -1)"><Icon name="arrow_upward" :size="18" /></button>
            <button class="icon-btn st" type="button" :disabled="i === steps.length - 1" v-tip="'Move down'" @click="moveStep(i, 1)"><Icon name="arrow_downward" :size="18" /></button>
            <button class="icon-btn st" type="button" v-tip="'Remove step'" @click="removeStep(i)"><Icon name="delete" :size="18" /></button>
          </div>
        </li>
      </ol>
      <p v-else class="muted small" style="margin: 0">No steps yet. Key and text steps are typed by the device, commands run on the PC.</p>
      <div v-if="recording" class="rec-bar" role="status">
        <span class="rec-dot" aria-hidden="true" />
        <span class="grow">
          <span class="rec-title">Recording · {{ recorded }} step{{ recorded === 1 ? '' : 's' }}</span>
          <span class="muted small">Press keys and combos; pauses become delays. Esc or 10 s without a key stops.</span>
        </span>
        <button class="btn btn-filled st stop" type="button" @click="stopRecording"><Icon name="stop" :size="18" fill /> Stop</button>
      </div>
      <div class="add-steps">
        <span class="muted small">{{ steps.length }} / {{ MAX_STEPS }}</span>
        <button
          class="fchip st rec-chip"
          type="button"
          :disabled="recording || steps.length >= MAX_STEPS"
          v-tip="'Record key presses as steps, added after the current ones'"
          @click="startRecording"
        >
          <Icon name="radio_button_checked" :size="16" /> Record
        </button>
        <button
          v-for="k in STEP_KINDS"
          :key="k.value"
          class="fchip st"
          type="button"
          :disabled="steps.length >= MAX_STEPS"
          @click="addStep(k.value)"
        >
          <Icon name="add" :size="16" /> {{ k.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ae {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.type {
  flex: 1;
  min-width: 0;
}
.macro {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.steps {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.step {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px;
  border-radius: 16px;
  background: var(--sch);
}
.num {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 14px;
  background: var(--secc);
  color: var(--on-secc);
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}
.step-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.step-kind {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--on-sv);
  min-height: 28px;
}
.step-tools {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.step-tools .icon-btn {
  width: 32px;
  height: 32px;
  border: none;
}
.rec-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 10px 10px 10px 16px;
  border-radius: 16px;
  background: var(--ec);
  color: var(--on-ec);
}
.rec-bar .muted {
  display: block;
  color: inherit;
  opacity: 0.8;
}
.rec-title {
  display: block;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.rec-dot {
  width: 14px;
  height: 14px;
  border-radius: 7px;
  background: var(--error);
  flex-shrink: 0;
  animation: rec-pulse 1.2s ease-in-out infinite;
}
@keyframes rec-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--error) 60%, transparent);
  }
  50% {
    box-shadow: 0 0 0 8px transparent;
    opacity: 0.6;
  }
}
@media (prefers-reduced-motion: reduce) {
  .rec-dot {
    animation: none;
  }
}
.stop {
  height: 36px;
  background: var(--error);
  color: var(--on-error);
}
.rec-chip {
  color: var(--error);
  border-color: color-mix(in srgb, var(--error) 50%, transparent);
}
.obs-sugg {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.ae a {
  color: var(--primary);
}
.add-steps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
</style>
