<script setup lang="ts">
// Backup and restore: the server zips config.json with every asset file, and takes such a zip
// back. After a restore the config is loaded again from the server.
import { ref } from 'vue'
import Icon from './Icon.vue'
import M3Dialog from './M3Dialog.vue'
import ShapeLoader from './ShapeLoader.vue'
import { api } from '@/lib/api'
import { dirty, loadAll, loadError } from '@/lib/store'
import { toast, toastError } from '@/lib/ui'

const downloading = ref(false)
const restoring = ref(false)
const picker = ref<HTMLInputElement>()
const file = ref<File | null>(null)
const asking = ref(false)

function today(): string {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

async function download() {
  if (downloading.value) return
  downloading.value = true
  try {
    const blob = await api.backup()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `roundbar-backup-${today()}.zip`
    document.body.appendChild(a)
    a.click()
    a.remove()
    setTimeout(() => URL.revokeObjectURL(url), 10_000)
  } catch (e) {
    toastError(e)
  } finally {
    downloading.value = false
  }
}

function pick() {
  if (picker.value) {
    picker.value.value = ''
    picker.value.click()
  }
}
function picked(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  if (!/\.zip$/i.test(f.name)) {
    toast('Pick a Roundbar backup (.zip)', true)
    return
  }
  file.value = f
  asking.value = true
}

async function restore() {
  const f = file.value
  if (!f || restoring.value) return
  asking.value = false
  restoring.value = true
  try {
    await api.restore(f)
    // The existing load path: drops unsaved edits and the undo history
    await loadAll()
    if (loadError.value) toast(`Restored, but the config could not be reloaded: ${loadError.value}`, true)
    else toast('Backup restored. The daemon sends it to the device.')
  } catch (e) {
    toastError(e)
  } finally {
    restoring.value = false
    file.value = null
  }
}
</script>

<template>
  <section id="backup" class="card">
    <div class="card-head"><h2>Backup</h2></div>
    <p class="muted small" style="margin: 0">
      One file with your profiles, settings and images. Restore it after a reinstall or on another PC.
    </p>
    <div class="actions">
      <button class="btn btn-tonal st" type="button" :disabled="downloading" @click="download">
        <ShapeLoader v-if="downloading" small label="Preparing" />
        <Icon v-else name="download" :size="18" />
        Download backup
      </button>
      <button class="btn btn-outlined st" type="button" :disabled="restoring" @click="pick">
        <ShapeLoader v-if="restoring" small label="Restoring" />
        <Icon v-else name="upload" :size="18" />
        Restore…
      </button>
      <input ref="picker" type="file" accept=".zip,application/zip" hidden @change="picked" />
    </div>
    <p v-if="dirty" class="muted small" style="margin: 0">You have unsaved changes: a restore discards them.</p>

    <M3Dialog v-model="asking" title="Restore this backup?">
      <p class="muted" style="margin: 0">
        This replaces your current profiles and settings with the ones in <b>{{ file?.name }}</b>. Its images are added.
      </p>
      <p v-if="dirty" class="warn-line">
        <Icon name="warning" :size="18" />
        <span>Your unsaved changes are discarded too.</span>
      </p>
      <template #actions>
        <button class="btn btn-text st" type="button" @click="asking = false">Cancel</button>
        <button class="btn btn-danger st" type="button" @click="restore">{{ dirty ? 'Discard and restore' : 'Restore' }}</button>
      </template>
    </M3Dialog>
  </section>
</template>

<style scoped>
#backup {
  scroll-margin-top: 84px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.card-head {
  margin-bottom: 0;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.warn-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--warn);
}
</style>
