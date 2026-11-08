<script setup lang="ts">
// Pictures of an `image` page: the upload and thumbnails (ImageList), slideshow interval
// and fit. A GIF plays alone, stills make a slideshow of up to 20.
import { computed } from 'vue'
import Icon from './Icon.vue'
import M3Slider from './M3Slider.vue'
import ImageList from './ImageList.vue'
import type { ImageBinding } from '@/lib/assets'
import { IMAGE_DEFAULTS, LIMITS, isGif } from '@/lib/model'
import type { ImageFit, ImagePage } from '@/lib/types'

const props = defineProps<{ page: ImagePage }>()

// Bound to this page object, so an upload finishing later still lands here
const pg = props.page
const bind: ImageBinding = {
  get: () => pg.images,
  set: (ids) => (pg.images = ids),
}
const hasGif = computed(() => props.page.images.some(isGif))
const stills = computed(() => (hasGif.value ? 0 : props.page.images.length))

// ---- settings: the defaults are stored as missing keys ----
const interval = computed({
  get: () => Math.min(60, Math.max(2, props.page.interval ?? IMAGE_DEFAULTS.interval)),
  set: (v: number) => {
    if (v === IMAGE_DEFAULTS.interval) delete props.page.interval
    else props.page.interval = v
  },
})
const FITS: { value: ImageFit; label: string; hint: string }[] = [
  { value: 'cover', label: 'Fill circle', hint: 'Crop the picture to fill the round screen' },
  { value: 'contain', label: 'Show whole image', hint: 'Fit the whole picture inside, with the surface color around it' },
]
const fit = computed({
  get: () => props.page.fit ?? IMAGE_DEFAULTS.fit,
  set: (v: ImageFit) => {
    if (v === IMAGE_DEFAULTS.fit) delete props.page.fit
    else props.page.fit = v
  },
})
</script>

<template>
  <div class="card">
    <div class="card-head">
      <h2>Images</h2>
      <span class="muted small">{{ hasGif ? 'GIF' : `${page.images.length} / ${LIMITS.images}` }}</span>
    </div>
    <ImageList
      :bind="bind"
      gif
      required
      title="Drop pictures or a GIF here"
      hint="Pictures are scaled to fit 360 × 360. Several make a slideshow, a GIF plays alone."
    />

    <div v-if="stills >= 2" class="setting col" style="margin-top: 8px">
      <div class="row-head">
        <span>Slideshow interval</span>
        <span class="muted">{{ interval }} s</span>
      </div>
      <M3Slider v-model="interval" label="Slideshow interval" :min="2" :max="60" :step="1" :format="(v) => `${v} s`" />
      <p v-if="(page.interval ?? 0) > 60" class="muted small" style="margin: 0">The config sets {{ page.interval }} s; moving the slider replaces it.</p>
    </div>
    <div class="setting">
      <div class="grow">
        <div>Fit</div>
        <div class="muted small">{{ FITS.find((f) => f.value === fit)!.hint }}</div>
      </div>
      <div class="seg" role="radiogroup" aria-label="Fit">
        <button v-for="f in FITS" :key="f.value" type="button" role="radio" class="st" :class="{ on: fit === f.value }" :aria-checked="fit === f.value" @click="fit = f.value">
          <Icon v-if="fit === f.value" name="check" :size="18" />
          {{ f.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.row-head {
  display: flex;
  justify-content: space-between;
}
</style>
