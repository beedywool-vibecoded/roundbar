// App-level page clipboard for copying pages across profiles. Mirrored to sessionStorage so
// it survives a reload of the tab.
import { ref, watch } from 'vue'
import type { Page } from './types'

const KEY = 'roundbar.clipboard.page'

function load(): Page | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    const p = raw ? (JSON.parse(raw) as Page) : null
    return p && typeof p === 'object' && typeof p.id === 'string' && typeof p.type === 'string' ? p : null
  } catch {
    return null
  }
}

export const clipboardPage = ref<Page | null>(load())

watch(clipboardPage, (p) => {
  try {
    if (p) sessionStorage.setItem(KEY, JSON.stringify(p))
    else sessionStorage.removeItem(KEY)
  } catch {
    // Storage blocked: the clipboard still works until the tab reloads
  }
})

export function copyPage(p: Page) {
  clipboardPage.value = JSON.parse(JSON.stringify(p)) as Page
}
