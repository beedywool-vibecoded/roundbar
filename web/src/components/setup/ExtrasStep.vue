<script setup lang="ts">
// Step 6: integrations for what's installed: Steam, MangoHud, OBS, Discord / Vesktop.
import { computed } from 'vue'
import Icon from '../Icon.vue'
import M3Shape from '../M3Shape.vue'
import DiscordCard from '../DiscordCard.vue'
import ObsCard from '../ObsCard.vue'
import VesktopToggle from '../VesktopToggle.vue'
import SetupHero from './SetupHero.vue'
import { newPage } from '@/lib/model'
import { PRESETS } from '@/lib/presets'
import { addPreset, detected, gamesProfile } from '@/lib/setup'
import { toast } from '@/lib/ui'

defineEmits<{ finish: [] }>() // passed to every step; only the board step uses it

const games = computed(() => gamesProfile())
const hasMango = computed(() => !!games.value?.pages.some((p) => p.type === 'mangohud'))

function addMango() {
  let p = games.value
  if (!p) {
    const preset = PRESETS.find((x) => x.id === 'games')!
    p = addPreset(preset, preset.build().match)
  }
  if (p.pages.some((x) => x.type === 'mangohud')) return
  const page = newPage('mangohud', p.pages.map((x) => x.id))
  p.pages.splice(1, 0, page) // right after the game page
  toast(`Added the MangoHud page to ${p.name}`)
}
</script>

<template>
  <div class="step">
    <SetupHero icon="widgets" kind="clover" />
    <h1>Extras for your apps</h1>
    <p class="lead">Roundbar can talk to a few apps directly. Set up the ones you use, or skip them for now.</p>
  </div>

  <div class="extras">
    <section v-if="detected.steam" class="card extra">
      <div class="head">
        <M3Shape kind="sunny" :size="48" fill="var(--pc)">
          <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="46" class="sym" fill="var(--on-pc)">sports_esports</text>
        </M3Shape>
        <div class="grow">
          <h2>Steam games</h2>
          <p class="muted small">
            Nothing to set up. Start a game and the game page shows its art, how long you've played, and buttons for the
            Steam overlay and screenshots.
          </p>
          <p v-if="!games" class="small warn">Tick the Games profile in the previous step to get it.</p>
        </div>
        <Icon v-if="games" name="check" class="ok" />
      </div>
    </section>

    <section v-if="detected.mangohud" class="card extra">
      <div class="head">
        <M3Shape kind="sunny" :size="48" fill="var(--pc)">
          <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="46" class="sym" fill="var(--on-pc)">monitor_heart</text>
        </M3Shape>
        <div class="grow">
          <h2>MangoHud</h2>
          <p class="muted small">
            A page with live FPS, frame times, CPU and GPU while you play. The first time it shows, Roundbar adds one line to
            <code>MangoHud.conf</code> so it can read the numbers. Nothing changes on your game screen.
          </p>
        </div>
      </div>
      <div class="row-actions">
        <button v-if="!hasMango" class="btn btn-tonal st" type="button" @click="addMango">
          <Icon name="add" :size="18" /> {{ games ? `Add to ${games.name}` : 'Add a Games profile with it' }}
        </button>
        <span v-else class="ok small"><Icon name="check" :size="18" /> Added to {{ games?.name }}</span>
      </div>
    </section>

    <ObsCard v-if="detected.obs" />

    <DiscordCard v-if="detected.discord" />
    <section v-else-if="detected.vesktop" class="card extra">
      <div class="head">
        <M3Shape kind="sunny" :size="48" fill="var(--pc)">
          <text x="50" y="50" text-anchor="middle" dominant-baseline="central" font-size="46" class="sym" fill="var(--on-pc)">headset_mic</text>
        </M3Shape>
        <div class="grow">
          <h2>Discord (Vesktop)</h2>
          <p class="muted small">The Discord page shows your voice channel on the board.</p>
        </div>
      </div>
      <VesktopToggle />
    </section>
  </div>
</template>

<style scoped>
.extras {
  display: grid;
  gap: 12px;
  margin-top: 8px;
}
.extra {
  display: grid;
  gap: 12px;
}
.head {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}
.head h2 {
  font-size: 18px;
}
.head p {
  margin: 4px 0 0;
}
.row-actions {
  display: flex;
  justify-content: flex-end;
}
.ok {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--primary);
}
.warn {
  color: var(--warn);
}
.sym {
  font-family: 'Material Symbols Rounded';
  font-feature-settings: 'liga';
  font-variation-settings: 'FILL' 1;
}
</style>
