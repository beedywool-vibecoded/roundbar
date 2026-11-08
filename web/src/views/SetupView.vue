<script setup lang="ts">
// First-run setup (docs/spec.md §4): opens by itself until it is finished or skipped, and
// from the Device page. Steps with nothing to do move on by themselves; `?step=<id>` opens one.
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '@/components/Icon.vue'
import ShapeLoader from '@/components/ShapeLoader.vue'
import AppsStep from '@/components/setup/AppsStep.vue'
import BoardStep from '@/components/setup/BoardStep.vue'
import DesktopStep from '@/components/setup/DesktopStep.vue'
import DoneStep from '@/components/setup/DoneStep.vue'
import ExtrasStep from '@/components/setup/ExtrasStep.vue'
import FirmwareStep from '@/components/setup/FirmwareStep.vue'
import KeyboardStep from '@/components/setup/KeyboardStep.vue'
import { STEP_LABELS, hasExtras, loadApps, wiz, type StepId } from '@/lib/setup'
import { dirty, save, saving, setSetupDone } from '@/lib/store'
import { toast } from '@/lib/ui'

const route = useRoute()
const router = useRouter()

const VIEWS: Record<StepId, Component> = {
  board: BoardStep,
  firmware: FirmwareStep,
  desktop: DesktopStep,
  keyboard: KeyboardStep,
  apps: AppsStep,
  extras: ExtrasStep,
  done: DoneStep,
}
// Extras only when something it knows is installed (shown while the app list loads)
const steps = computed<StepId[]>(() =>
  (['board', 'firmware', 'desktop', 'keyboard', 'apps', 'extras', 'done'] as StepId[]).filter((s) => s !== 'extras' || !wiz.apps || hasExtras.value),
)

const initial = route.query.step as StepId | undefined
const step = ref<StepId>(initial && initial in VIEWS ? initial : 'board')
const index = computed(() => Math.max(0, steps.value.indexOf(step.value)))
const forward = ref(true)
// Steps reached with Back, or opened directly, wait for the user instead of moving on
const hold = new Set<StepId>(initial ? [initial] : [])

const stepRef = ref<{ beforeNext?: () => void; nextLabel?: string } | null>(null)

function go(to: StepId, fwd: boolean) {
  clearAuto()
  forward.value = fwd
  if (!fwd) hold.add(to)
  step.value = to
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
function next() {
  if (wiz.busy) return
  stepRef.value?.beforeNext?.()
  const n = steps.value[index.value + 1]
  if (n) go(n, true)
}
function back() {
  const p = steps.value[index.value - 1]
  if (p && !wiz.busy) go(p, false)
}

// ---- moving on by itself ----
const AUTO_MS = 1600
const autoPending = ref(false)
let autoTimer: ReturnType<typeof setTimeout> | undefined
function clearAuto() {
  clearTimeout(autoTimer)
  autoTimer = undefined
  autoPending.value = false
}
watch(
  () => [step.value, !!wiz.auto[step.value]] as const,
  ([s, auto]) => {
    clearAuto()
    if (!auto || hold.has(s) || s === 'done') return
    autoPending.value = true
    autoTimer = setTimeout(() => {
      autoPending.value = false
      if (step.value === s && wiz.auto[s] && !hold.has(s)) next()
    }, AUTO_MS)
  },
  { immediate: true },
)
// Any click or key on the step means the user is looking: stay.
function stay() {
  if (autoPending.value) {
    hold.add(step.value)
    clearAuto()
  }
}

onMounted(() => {
  wiz.added = []
  wiz.auto = {}
  void loadApps()
})
onBeforeUnmount(clearAuto)

const nextLabel = computed(() => {
  if (step.value === 'done') return 'Finish'
  return stepRef.value?.nextLabel || 'Next'
})

async function finish() {
  if (dirty.value && !(await save())) return
  await setSetupDone(true)
  void router.push('/')
}
async function skip() {
  if (wiz.busy) return
  await setSetupDone(true)
  toast(dirty.value ? 'Setup skipped. Save to keep what you changed; run it again from Device.' : 'Setup skipped. Run it again any time from the Device page.')
  void router.push('/')
}
</script>

<template>
  <div class="page setup" @pointerdown="stay" @keydown="stay">
    <header class="setup-head">
      <ol class="stepper" aria-label="Setup steps">
        <li
          v-for="(s, i) in steps"
          :key="s"
          :class="{ done: i < index, current: i === index }"
          :aria-current="i === index ? 'step' : undefined"
        >
          <span class="num"><Icon v-if="i < index" name="check" :size="16" /><template v-else>{{ i + 1 }}</template></span>
          <span class="lbl">{{ STEP_LABELS[s] }}</span>
        </li>
      </ol>
      <div class="mobile-step muted small">Step {{ index + 1 }} of {{ steps.length }} · {{ STEP_LABELS[step] }}</div>
      <button class="btn btn-text st skip" type="button" :disabled="wiz.busy" @click="skip">Skip setup</button>
    </header>

    <div class="setup-body" :class="{ wide: step === 'extras' }">
      <Transition :name="forward ? 'step-fwd' : 'step-back'" mode="out-in">
        <div :key="step" class="card step-card" :class="{ bare: step === 'extras' }">
          <component :is="VIEWS[step]" ref="stepRef" @finish="go('done', true)" />
        </div>
      </Transition>

      <footer class="setup-nav">
        <button v-if="index > 0" class="btn btn-text st" type="button" :disabled="wiz.busy" @click="back">
          <Icon name="arrow_back" :size="18" /> Back
        </button>
        <span class="grow" />
        <span v-if="autoPending" class="auto small" role="status">
          <span class="auto-bar"><span :style="{ animationDuration: `${AUTO_MS}ms` }" /></span>
          All good, moving on
        </span>
        <button v-if="step === 'done'" class="btn btn-filled btn-lg st" type="button" :disabled="saving" @click="finish">
          <ShapeLoader v-if="saving" small label="Saving" />
          <Icon v-else name="check" :size="20" />
          {{ dirty ? 'Save and finish' : 'Finish' }}
        </button>
        <button v-else class="btn btn-filled btn-lg st" type="button" :disabled="wiz.busy" @click="next">
          {{ nextLabel }} <Icon name="arrow_forward" :size="20" />
        </button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.setup {
  display: grid;
  gap: 20px;
}
.setup-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.stepper {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0;
  margin: 0;
  padding: 0;
  list-style: none;
  min-width: 0;
}
.stepper li {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  color: var(--on-sv);
  font-size: 13px;
  font-weight: 500;
}
.stepper li:last-child {
  flex: 0 0 auto;
}
.stepper li:not(:last-child)::after {
  content: '';
  flex: 1;
  height: 2px;
  min-width: 12px;
  margin: 0 8px;
  border-radius: 1px;
  background: var(--sch);
}
.stepper li.done::after {
  background: var(--primary);
}
.num {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border-radius: 14px;
  background: var(--sch);
  color: var(--on-sv);
  font-size: 13px;
  font-weight: 600;
  transition:
    background 0.3s var(--ease),
    border-radius 0.4s var(--ease-decel);
}
.done .num {
  background: var(--primary);
  color: var(--on-primary);
}
.current {
  color: var(--on-surface);
}
.current .num {
  background: var(--pc);
  color: var(--on-pc);
  border-radius: 9px;
}
.lbl {
  white-space: nowrap;
}
.mobile-step {
  display: none;
  flex: 1;
}
.skip {
  flex-shrink: 0;
}
.setup-body {
  width: min(680px, 100%);
  justify-self: center;
  display: grid;
  gap: 16px;
}
.setup-body.wide {
  width: min(760px, 100%);
}
.step-card {
  padding: 28px 32px 32px;
}
.step-card.bare {
  padding: 0;
  background: transparent;
}
.setup-nav {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.auto {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--on-sv);
}
.auto-bar {
  width: 56px;
  height: 4px;
  border-radius: 2px;
  background: var(--sch);
  overflow: hidden;
}
.auto-bar span {
  display: block;
  height: 100%;
  background: var(--primary);
  animation: fill linear both;
}
@keyframes fill {
  from {
    width: 0;
  }
  to {
    width: 100%;
  }
}
.step-fwd-enter-active,
.step-back-enter-active {
  transition:
    opacity 0.3s var(--ease-decel),
    transform 0.4s var(--ease-decel);
}
.step-fwd-leave-active,
.step-back-leave-active {
  transition:
    opacity 0.15s var(--ease),
    transform 0.15s var(--ease);
}
.step-fwd-enter-from,
.step-back-leave-to {
  opacity: 0;
  transform: translateX(32px);
}
.step-fwd-leave-to,
.step-back-enter-from {
  opacity: 0;
  transform: translateX(-32px);
}
@media (max-width: 900px) {
  .lbl {
    display: none;
  }
  .current .lbl {
    display: inline;
  }
}
@media (max-width: 760px) {
  .stepper {
    display: none;
  }
  .mobile-step {
    display: block;
  }
  .step-card {
    padding: 20px 18px 24px;
  }
  .setup-nav .btn-lg {
    flex: 1;
  }
  .auto {
    order: -1;
    flex-basis: 100%;
    justify-content: center;
  }
}
@media (prefers-reduced-motion: reduce) {
  .step-fwd-enter-active,
  .step-back-enter-active,
  .step-fwd-leave-active,
  .step-back-leave-active {
    transition: opacity 0.15s;
  }
  .step-fwd-enter-from,
  .step-back-leave-to,
  .step-fwd-leave-to,
  .step-back-enter-from {
    transform: none;
  }
}
</style>

<style>
/* Shared by the step components (components/setup/*) */
.setup .step {
  display: grid;
  gap: 14px;
  text-align: center;
}
.setup .step h1 {
  font-size: 30px;
  line-height: 1.2;
  letter-spacing: -0.01em;
}
.setup .step .lead {
  margin: 0 auto;
  max-width: 520px;
  color: var(--on-sv);
  font-size: 16px;
}
.setup .step .center,
.setup .step .actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
}
.setup .step .card-inner {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 20px;
  background: var(--tc);
  color: var(--on-tc);
  text-align: left;
  flex-wrap: wrap;
}
.setup .step .card-inner .muted {
  color: inherit;
  opacity: 0.85;
}
.setup .step .card-inner .grow {
  min-width: 200px;
}
.setup .step .strong {
  font-weight: 600;
}
.setup .step .err {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin: 0;
  color: var(--error);
  flex-wrap: wrap;
}
.setup .step p {
  margin: 0;
}
@media (max-width: 760px) {
  .setup .step h1 {
    font-size: 24px;
  }
  .setup .step .lead {
    font-size: 15px;
  }
}
</style>
