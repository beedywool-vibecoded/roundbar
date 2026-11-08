<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/Icon.vue'
import RoundPreview from '@/components/RoundPreview.vue'
import M3Switch from '@/components/M3Switch.vue'
import M3Slider from '@/components/M3Slider.vue'
import SeedPicker from '@/components/SeedPicker.vue'
import WeatherCard from '@/components/WeatherCard.vue'
import { devicePalette } from '@/lib/theme'
import { config } from '@/lib/store'
import { pageTypeInfo } from '@/lib/model'
import { PALETTE_ROLES, type Page, type ThemeMode } from '@/lib/types'

const theme = computed(() => config.value!.theme)
const media = computed(() => config.value!.media)
const palette = computed(() => devicePalette(theme.value.seed, theme.value.mode))

// Linked from the clock page editor: /appearance#weather
const route = useRoute()
onMounted(async () => {
  if (route.hash !== '#weather') return
  await nextTick()
  setTimeout(() => document.getElementById('weather')?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 250)
})

const blurText = (v: number) => (v === 0 ? 'Sharp' : `${v}`)

const MODES: { id: ThemeMode; label: string; icon: string }[] = [
  { id: 'dark', label: 'Dark', icon: 'dark_mode' },
  { id: 'light', label: 'Light', icon: 'light_mode' },
]

// One page of each kind from the config, so the preview shows the user's own buttons
const samples = computed<Page[]>(() => {
  const all = config.value!.profiles.flatMap((p) => p.pages)
  const out: Page[] = []
  for (const t of ['radial', 'media', 'stats', 'dial', 'clock', 'timer'] as const) {
    const p = all.find((x) => x.type === t)
    if (p) out.push(p)
  }
  return out.length ? out : all.slice(0, 1)
})
const sample = ref(0)
const shown = computed(() => samples.value[Math.min(sample.value, samples.value.length - 1)])
</script>

<template>
  <div class="page">
    <div class="page-title"><h1>Appearance</h1></div>

    <div class="cols">
      <div class="left">
        <section class="card">
          <div class="card-head"><h2>Device theme</h2></div>
          <div class="setting col">
            <div>
              <div>Seed color</div>
              <div class="muted small">The device palette is generated from it with Material You's tonal spot scheme. This configurator follows it too.</div>
            </div>
            <SeedPicker v-model="theme.seed" label="Seed color" />
          </div>
          <div class="setting">
            <div class="grow">
              <div>Mode</div>
              <div class="muted small">For the device only. This page follows your system's dark or light setting. Profile colors follow this mode too.</div>
            </div>
            <div class="seg" role="radiogroup" aria-label="Device mode">
              <button
                v-for="m in MODES"
                :key="m.id"
                type="button"
                role="radio"
                class="st"
                :class="{ on: theme.mode === m.id }"
                :aria-checked="theme.mode === m.id"
                @click="theme.mode = m.id"
              >
                <Icon :name="m.icon" :size="18" /> {{ m.label }}
              </button>
            </div>
          </div>
          <div class="setting col">
            <div>
              <div>Palette sent to the device</div>
              <div class="muted small">Computed again on every save.</div>
            </div>
            <div class="roles">
              <div v-for="r in PALETTE_ROLES" :key="r" class="role">
                <span class="sw" :style="{ background: palette[r] }" />
                <span class="small ellipsis">{{ r }}</span>
                <code class="plain small muted">{{ palette[r] }}</code>
              </div>
            </div>
          </div>
          <div class="card-head" style="margin-top: 16px"><h2>Now playing</h2></div>
          <div class="setting">
            <div class="grow">
              <div>Album art background</div>
              <div class="muted small">A blurred copy of the cover behind media pages</div>
            </div>
            <M3Switch v-model="media.coverBackground" label="Album art background" />
          </div>
          <template v-if="media.coverBackground">
            <div class="setting col">
              <div class="row-head">
                <span>Blur</span>
                <span class="muted">{{ blurText(media.blur) }}</span>
              </div>
              <M3Slider v-model="media.blur" label="Blur" :min="0" :max="100" :step="5" :format="blurText" />
            </div>
            <div class="setting col">
              <div class="row-head">
                <span>Album art opacity</span>
                <span class="muted">{{ media.coverOpacity }} %</span>
              </div>
              <M3Slider v-model="media.coverOpacity" label="Album art opacity" :min="0" :max="100" :step="5" :format="(v) => `${v} %`" />
              <p class="muted small" style="margin: 0">How much of the cover shows through the dimmed layer behind the text.</p>
            </div>
          </template>
          <div class="setting">
            <div class="grow">
              <div>Colors from album art</div>
              <div class="muted small">Media pages take a palette from the cover; other pages keep the theme</div>
            </div>
            <M3Switch v-model="media.coverColors" label="Colors from album art" />
          </div>
        </section>
        <WeatherCard />
      </div>

      <section class="card preview-card">
        <div class="card-head"><h2>Preview</h2></div>
        <div class="preview-wrap">
          <RoundPreview v-if="shown" :page="shown" :palette="palette" />
        </div>
        <div v-if="samples.length > 1" class="chips samples" role="tablist" aria-label="Sample page">
          <button v-for="(p, i) in samples" :key="i" type="button" role="tab" class="fchip st" :class="{ on: sample === i }" :aria-selected="sample === i" @click="sample = i">
            <Icon :name="sample === i ? 'check' : pageTypeInfo(p.type).icon" :size="16" /> {{ pageTypeInfo(p.type).label }}
          </button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(300px, 1fr);
  gap: 16px;
  align-items: start;
}
.left {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
#weather {
  scroll-margin-top: 84px;
}
.row-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.sw {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  flex-shrink: 0;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--on-surface) 20%, transparent);
}
.roles {
  display: grid;
  grid-auto-rows: min-content;
  align-content: start;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 20px;
}
.role {
  display: grid;
  grid-template-columns: 20px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
}
.preview-card {
  position: sticky;
  top: 84px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}
.preview-card .card-head {
  align-self: stretch;
}
.samples {
  justify-content: center;
}
.preview-wrap {
  width: min(100%, 340px);
  padding: 11px;
}
@media (max-width: 560px) {
  .roles {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (max-width: 1000px) {
  .cols {
    grid-template-columns: minmax(0, 1fr);
  }
  .preview-card {
    position: static;
    order: -1;
  }
}
</style>
