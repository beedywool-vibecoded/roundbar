// After a deploy, a page left open still runs the old version. The code of a page it hasn't opened
// yet is loaded on the first visit, under a name the server no longer has: switching to that page
// failed with nothing on screen until a reload. Instead, that page is loaded in full (the new
// version). Once only: a second failure within a minute is the network, said in a snackbar.
import type { Router } from 'vue-router'
import { toast } from './ui'

const KEY = 'stale-chunk-reload'
const STALE = /dynamically imported module|importing a module script failed|error loading dynamically imported|unable to preload css/i

function reloadOnce(path: string): boolean {
  try {
    if (Date.now() - Number(sessionStorage.getItem(KEY)) < 60_000) return false
    sessionStorage.setItem(KEY, String(Date.now()))
  } catch {
    // no storage: reload anyway, the browser's own loop guard is the user
  }
  window.location.assign(path)
  return true
}

export function reloadOnStaleChunks(router: Router) {
  router.onError((err: unknown, to) => {
    const msg = err instanceof Error ? err.message : String(err)
    if (!STALE.test(msg)) return
    if (!reloadOnce(to.fullPath)) toast('This page could not open. Check your connection, then try again.', true)
  })
}
