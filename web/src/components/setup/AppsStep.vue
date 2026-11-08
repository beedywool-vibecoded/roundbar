<script setup lang="ts">
// Step 5: ready-made profiles for the apps installed on this PC. Ticked ones are added to the
// config being edited when moving on, so the usual undo works.
import { computed, reactive, watch, watchEffect } from 'vue'
import Icon from '../Icon.vue'
import ShapeLoader from '../ShapeLoader.vue'
import SetupHero from './SetupHero.vue'
import { appIconUrl } from '@/lib/api'
import { addPreset, offers, wiz } from '@/lib/setup'
import { undo } from '@/lib/store'
import { toast } from '@/lib/ui'

const picked = reactive(new Set<string>())
let seeded = false
watch(
  offers,
  (list) => {
    if (seeded || !wiz.apps) return
    seeded = true
    for (const o of list) if (o.preset.suggested && !o.covered) picked.add(o.preset.id)
  },
  { immediate: true },
)
function toggle(id: string) {
  if (picked.has(id)) picked.delete(id)
  else picked.add(id)
}

const open = computed(() => offers.value.filter((o) => !o.covered))
const count = computed(() => open.value.filter((o) => picked.has(o.preset.id)).length)

watchEffect(() => (wiz.auto.apps = !!wiz.apps && open.value.length === 0))

const names = (o: (typeof offers.value)[number]) => o.found.map((a) => a.name).join(', ')

// Called by the setup when moving on
function beforeNext() {
  const chosen = open.value.filter((o) => picked.has(o.preset.id))
  if (!chosen.length) return
  for (const o of chosen) addPreset(o.preset, o.match)
  const n = chosen.length
  toast(`Added ${n} profile${n > 1 ? 's' : ''}: ${chosen.map((o) => o.preset.name).join(', ')}`, false, {
    label: 'Undo',
    run: () => {
      undo()
      wiz.added = wiz.added.slice(0, -n)
    },
  })
  picked.clear()
}
const nextLabel = computed(() => (count.value ? `Add ${count.value} profile${count.value > 1 ? 's' : ''}` : ''))
defineExpose({ beforeNext, nextLabel })
</script>

<template>
  <div class="step">
    <SetupHero icon="apps" kind="burst" />
    <h1>Profiles for your apps</h1>
    <template v-if="!wiz.apps">
      <p class="lead">Looking at what's installed…</p>
      <div class="center"><ShapeLoader /></div>
    </template>
    <template v-else-if="!offers.length">
      <p class="lead">
        No apps with a ready-made profile here. The Default profile works everywhere, and you can make your own in Profiles.
      </p>
    </template>
    <template v-else>
      <p class="lead">The board switches to a profile when you use its app. Pick the ones you want; you can change everything later.</p>
      <ul class="offers" role="group" aria-label="Ready-made profiles">
        <li v-for="o in offers" :key="o.preset.id">
          <button
            type="button"
            class="offer st"
            role="checkbox"
            :class="{ on: picked.has(o.preset.id), covered: o.covered }"
            :aria-checked="o.covered || picked.has(o.preset.id)"
            :disabled="o.covered"
            @click="toggle(o.preset.id)"
          >
            <span class="box"><Icon v-if="o.covered || picked.has(o.preset.id)" name="check" :size="18" /></span>
            <span class="pic"><Icon :name="o.preset.icon" :size="22" /></span>
            <span class="txt">
              <span class="name">
                {{ o.preset.name }}
                <span v-if="o.covered" class="chip chip-outline">Already set up</span>
              </span>
              <span class="muted small">{{ o.preset.hint }}</span>
              <span class="apps small">
                <template v-for="a in o.found.slice(0, 4)" :key="a.id">
                  <img v-if="appIconUrl(a.icon)" :src="appIconUrl(a.icon)" alt="" width="18" height="18" loading="lazy" />
                </template>
                <span class="ellipsis">{{ o.preset.fixedMatch ? `Games started from ${names(o)}` : `For ${names(o)}` }}</span>
              </span>
            </span>
          </button>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.offers {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
  text-align: left;
}
.offer {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px 12px 14px;
  border: 1px solid transparent;
  border-radius: 20px;
  background: var(--sch);
  color: var(--on-surface);
  text-align: left;
  transition:
    background 0.2s var(--ease),
    border-radius 0.3s var(--ease-decel);
}
.offer.on {
  background: var(--secc);
  color: var(--on-secc);
  border-radius: 28px;
}
.offer.covered {
  opacity: 0.7;
}
.box {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  border: 2px solid var(--on-sv);
  border-radius: 6px;
}
.on .box,
.covered .box {
  border-color: var(--primary);
  background: var(--primary);
  color: var(--on-primary);
}
.pic {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  border-radius: 14px;
  background: var(--pc);
  color: var(--on-pc);
}
.txt {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.on .muted {
  color: inherit;
  opacity: 0.8;
}
.name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}
.apps {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  opacity: 0.85;
}
.apps img {
  flex-shrink: 0;
  object-fit: contain;
}
</style>
