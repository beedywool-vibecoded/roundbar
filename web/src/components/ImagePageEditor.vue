<script setup lang="ts">
// Pictures of an `image` page: upload (drop or pick), thumbnails to reorder or remove,
// slideshow interval and fit. A GIF plays alone, stills make a slideshow of up to 20.
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import M3Slider from './M3Slider.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api } from '@/lib/api'
import { useAssets } from '@/lib/assets'
import { IMAGE_DEFAULTS, LIMITS, isGif } from '@/lib/model'
import type { ImageFit, ImagePage } from '@/lib/types'
import { confirm, toast, toastError } from '@/lib/ui'

const props = defineProps<{ page: ImagePage }>()

const assets = useAssets(() => props.page.images)
const hasGif = computed(() => props.page.images.some(isGif))
const stills = computed(() => (hasGif.value ? 0 : props.page.images.length))

// ---- upload ----
interface Slot {
  key: number
  name: string
  file: File
  done: boolean
  id: string | null
}
const pending = ref<Slot[]>([])
let seq = 0
const fileInput = ref<HTMLInputElement>()
const dropping = ref(false)

const gifFile = (f: File) => f.type === 'image/gif' || /\.gif$/i.test(f.name)
const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? '' : 's'}`

async function addFiles(list: FileList | File[] | null | undefined) {
  const files = [...(list ?? [])]
  if (!files.length) return
  const pg = props.page
  const gif = files.find(gifFile)
  let batch: File[]
  let replace = false
  if (gif) {
    batch = [gif]
    if (files.length > 1) toast(`A GIF plays alone on its page: only ${gif.name} is added`)
    if (pg.images.length || pending.value.length) {
      replace = await confirm({
        title: 'Replace the images?',
        body: hasGif.value
          ? 'This page already shows a GIF. A page shows a single GIF, so the new one takes its place.'
          : `A GIF plays alone on its page. The ${plural(pg.images.length, 'picture')} here will be removed.`,
        action: 'Replace',
      })
      if (!replace) return
    }
  } else {
    batch = files
    if (hasGif.value) {
      replace = await confirm({
        title: 'Replace the GIF?',
        body: 'A GIF plays alone on its page, so adding pictures removes it.',
        action: 'Replace',
      })
      if (!replace) return
    }
    const room = LIMITS.images - (replace ? 0 : pg.images.length + pending.value.length)
    if (room <= 0) {
      toast(`A page has at most ${LIMITS.images} images`, true)
      return
    }
    if (batch.length > room) {
      toast(`A page has at most ${LIMITS.images} images: only the first ${plural(room, 'file')} ${room === 1 ? 'is' : 'are'} added`, true)
      batch = batch.slice(0, room)
    }
  }
  if (replace) pg.images = []

  const slots: Slot[] = batch.map((file) => ({ key: ++seq, name: file.name, file, done: false, id: null }))
  pending.value.push(...slots)
  await Promise.all(
    slots.map(async (s) => {
      try {
        s.id = (await api.uploadAsset(s.file)).id
      } catch (e) {
        toastError(e instanceof Error ? new Error(`${s.name}: ${e.message}`) : e)
      }
      // Reactive copy, so the tile updates
      const live = pending.value.find((x) => x.key === s.key)
      if (live) {
        live.id = s.id
        live.done = true
      }
      flush(pg)
    }),
  )
}

// Results join the page in the order the files were picked.
function flush(pg: ImagePage) {
  while (pending.value[0]?.done) {
    const { id } = pending.value.shift()!
    if (!id) continue
    // The server decides what is a GIF; keep the page valid whatever it said.
    if (isGif(id)) pg.images = [id]
    else if (pg.images.some(isGif)) pg.images = [id]
    else if (pg.images.length >= LIMITS.images) toast(`A page has at most ${LIMITS.images} images`, true)
    else if (!pg.images.includes(id)) pg.images.push(id)
  }
}

function pick() {
  fileInput.value?.click()
}
function picked(e: Event) {
  const input = e.target as HTMLInputElement
  addFiles(input.files)
  input.value = ''
}
const isFileDrag = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
function dragOverZone(e: DragEvent) {
  if (!isFileDrag(e)) return
  e.preventDefault()
  e.dataTransfer!.dropEffect = 'copy'
  dropping.value = true
}
function dropZone(e: DragEvent) {
  dropping.value = false
  if (!isFileDrag(e)) return
  e.preventDefault()
  addFiles(e.dataTransfer!.files)
}

// ---- order ----
function move(i: number, d: number) {
  const arr = props.page.images
  const j = i + d
  if (j < 0 || j >= arr.length) return
  ;[arr[i], arr[j]] = [arr[j]!, arr[i]!]
}
function remove(i: number) {
  props.page.images.splice(i, 1)
}
const dragging = ref<number | null>(null)
function dragStart(i: number, e: DragEvent) {
  dragging.value = i
  e.dataTransfer!.effectAllowed = 'move'
  e.dataTransfer!.setData('text/plain', props.page.images[i]!)
}
function dragOverTile(j: number, e: DragEvent) {
  const i = dragging.value
  if (i === null) return
  e.preventDefault()
  e.dataTransfer!.dropEffect = 'move'
  if (i === j) return
  const arr = props.page.images
  arr.splice(j, 0, arr.splice(i, 1)[0]!)
  dragging.value = j
}

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
const full = computed(() => !hasGif.value && props.page.images.length + pending.value.length >= LIMITS.images)
</script>

<template>
  <div class="card">
    <div class="card-head">
      <h2>Images</h2>
      <span class="muted small">{{ hasGif ? 'GIF' : `${page.images.length} / ${LIMITS.images}` }}</span>
    </div>
    <div class="form">
      <div
        class="drop"
        :class="{ on: dropping }"
        @dragenter="dragOverZone"
        @dragover="dragOverZone"
        @dragleave.self="dropping = false"
        @drop="dropZone"
      >
        <Icon class="drop-icon" name="upload" :size="28" />
        <div class="grow">
          <div>Drop pictures or a GIF here</div>
          <div class="muted small">Pictures are scaled to fit 360 × 360. Several make a slideshow, a GIF plays alone.</div>
        </div>
        <button class="btn btn-tonal st" type="button" :disabled="full" v-tip="full ? `A page has at most ${LIMITS.images} images` : 'Pick image files'" @click="pick">
          <Icon name="image" :size="18" /> Choose files
        </button>
        <input ref="fileInput" type="file" accept="image/*" multiple hidden @change="picked" />
      </div>

      <ul v-if="page.images.length || pending.length" class="grid" aria-label="Images, in slideshow order">
        <li
          v-for="(id, i) in page.images"
          :key="id"
          class="tile"
          :class="{ dragging: dragging === i }"
          draggable="true"
          @dragstart="dragStart(i, $event)"
          @dragover="dragOverTile(i, $event)"
          @drop.prevent
          @dragend="dragging = null"
        >
          <img v-if="assets.src(id)" :src="assets.src(id)" :alt="`Image ${i + 1}`" draggable="false" />
          <div v-else-if="assets.failed(id)" class="ph" v-tip="`${id} is missing on the PC`"><Icon name="warning" :size="24" /></div>
          <div v-else class="ph"><ShapeLoader small label="Loading" /></div>
          <span v-if="page.images.length > 1" class="num">{{ i + 1 }}</span>
          <span v-if="isGif(id)" class="num gif">GIF</span>
          <div class="tools">
            <button v-if="page.images.length > 1" class="tbtn st" type="button" :disabled="i === 0" v-tip="'Move left'" :aria-label="`Move image ${i + 1} left`" @click="move(i, -1)">
              <Icon name="chevron_left" :size="18" />
            </button>
            <button class="tbtn st" type="button" v-tip="'Remove'" :aria-label="`Remove image ${i + 1}`" @click="remove(i)"><Icon name="delete" :size="18" /></button>
            <button v-if="page.images.length > 1" class="tbtn st" type="button" :disabled="i === page.images.length - 1" v-tip="'Move right'" :aria-label="`Move image ${i + 1} right`" @click="move(i, 1)">
              <Icon name="chevron_right" :size="18" />
            </button>
          </div>
        </li>
        <li v-for="s in pending" :key="`p${s.key}`" class="tile pending" v-tip="`Uploading ${s.name}`">
          <div class="ph"><ShapeLoader small :label="`Uploading ${s.name}`" /></div>
          <span class="name ellipsis small">{{ s.name }}</span>
        </li>
      </ul>
      <p v-if="page.images.length > 1" class="tf-help" style="margin: 0">Drag to reorder. The slideshow follows this order.</p>
      <p v-if="!page.images.length && !pending.length" class="tf-help err" style="margin: 0">Add an image</p>
    </div>

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
.drop {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
  padding: 16px 18px;
  border: 2px dashed var(--outline-v);
  border-radius: 20px;
  color: var(--on-surface);
  transition: border-color 0.2s var(--ease), background 0.2s var(--ease);
}
.drop.on {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 10%, transparent);
}
.drop-icon {
  color: var(--primary);
}
.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(104px, 1fr));
  gap: 10px;
}
.tile {
  position: relative;
  aspect-ratio: 1;
  border-radius: 50%;
  overflow: hidden;
  background: var(--sch);
  cursor: grab;
  transition: opacity 0.15s, scale 0.2s var(--ease);
}
.tile.dragging {
  opacity: 0.4;
  scale: 0.94;
}
.tile.pending {
  cursor: default;
}
.tile img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.ph {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  color: var(--on-sv);
}
.num {
  position: absolute;
  top: 10%;
  left: 50%;
  translate: -50% 0;
  min-width: 22px;
  padding: 1px 7px;
  border-radius: 11px;
  background: var(--pc);
  color: var(--on-pc);
  font-size: 12px;
  font-weight: 600;
  text-align: center;
}
.num.gif {
  background: var(--tc);
  color: var(--on-tc);
}
.name {
  position: absolute;
  left: 18%;
  right: 18%;
  bottom: 18%;
  text-align: center;
  color: var(--on-sv);
}
.tools {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 8%;
  display: flex;
  justify-content: center;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.15s;
}
.tile:hover .tools,
.tile:focus-within .tools {
  opacity: 1;
}
@media (hover: none) {
  .tools {
    opacity: 1;
  }
}
.tbtn {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 15px;
  display: grid;
  place-items: center;
  background: var(--schh);
  color: var(--on-surface);
}
.tbtn:disabled {
  opacity: 0.4;
}
.row-head {
  display: flex;
  justify-content: space-between;
}
</style>
