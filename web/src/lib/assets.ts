// Uploaded images (docs/spec.md, Assets) as object URLs. `<img>` can't send the bearer
// token, so each asset is fetched once with it and kept as a blob URL, shared by every
// component through a reference count. A URL is revoked a while after its last user is
// gone, so moving between views doesn't refetch.
import { getCurrentInstance, onBeforeUnmount, reactive, watch } from 'vue'
import { api } from './api'

interface Entry {
  url: string
  error: boolean
  refs: number
  revoke?: ReturnType<typeof setTimeout>
}

const entries = reactive(new Map<string, Entry>())
const KEEP_MS = 30_000

function entry(id: string): Entry {
  let e = entries.get(id)
  if (!e) {
    entries.set(id, { url: '', error: false, refs: 0 })
    e = entries.get(id)!
  }
  return e
}

function acquire(id: string) {
  const e = entry(id)
  clearTimeout(e.revoke)
  e.revoke = undefined
  e.refs++
  if (e.url || e.error || e.refs > 1) return
  api
    .asset(id)
    .then((blob) => {
      if (entries.get(id) === e && !e.url) e.url = URL.createObjectURL(blob)
    })
    .catch(() => {
      if (entries.get(id) === e && !e.url) e.error = true
    })
}

function scheduleRevoke(id: string, e: Entry) {
  clearTimeout(e.revoke)
  e.revoke = setTimeout(() => {
    if (e.refs || entries.get(id) !== e) return
    if (e.url) URL.revokeObjectURL(e.url)
    entries.delete(id)
  }, KEEP_MS)
}

function release(id: string) {
  const e = entries.get(id)
  if (!e) return
  e.refs = Math.max(0, e.refs - 1)
  if (!e.refs) scheduleRevoke(id, e)
}

// A just-uploaded file is already in hand: show it without fetching it back.
export function seedAsset(id: string, file: Blob) {
  const e = entry(id)
  if (e.url) return
  e.url = URL.createObjectURL(file)
  e.error = false
  if (!e.refs) scheduleRevoke(id, e)
}

// In a component: `const assets = useAssets(() => ids)`, then `assets.src(id)` gives the
// object URL ('' while loading) and `assets.failed(id)` tells a missing file. The ids are
// followed as they change and everything is released on unmount.
export function useAssets(ids: () => readonly string[]) {
  const held = new Set<string>()
  const stop = watch(
    () => [...new Set(ids())],
    (want) => {
      const next = new Set(want)
      for (const id of held) if (!next.has(id)) release(id)
      for (const id of next) if (!held.has(id)) acquire(id)
      held.clear()
      next.forEach((id) => held.add(id))
    },
    { immediate: true },
  )
  if (getCurrentInstance()) {
    onBeforeUnmount(() => {
      stop()
      held.forEach(release)
      held.clear()
    })
  }
  return {
    src: (id: string) => entries.get(id)?.url ?? '',
    failed: (id: string) => entries.get(id)?.error ?? false,
  }
}
