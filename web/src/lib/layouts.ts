// Host keyboard layouts (shared/keymaps.json, through the generated layouts.gen.ts).
import { LAYOUTS, LAYOUT_CHARS, type LayoutId } from './layouts.gen'
import type { KeyboardLayout } from './types'

export { LAYOUTS, type LayoutId }

export function layoutName(id: string | undefined | null): string {
  return LAYOUTS.find((l) => l.id === id)?.name ?? (id || '')
}

const ANY = new Set([...Object.values(LAYOUT_CHARS).join(''), '\n', '\t'])
const BY_ID = new Map(LAYOUTS.map((l) => [l.id as string, new Set([...LAYOUT_CHARS[l.id], '\n', '\t'])]))

// What the Go side accepts: characters at least one layout can type (with "auto" the
// layout is only known at run time and can change).
export function validText(s: string): boolean {
  for (const c of s) if (!ANY.has(c)) return false
  return true
}

// Characters of s this layout can't type, each once.
export function untypeable(s: string, id: string): string[] {
  const set = BY_ID.get(id)
  if (!set) return []
  return [...new Set([...s].filter((c) => !set.has(c)))]
}

// The layout the device types with: the configured one, or the detected one for auto.
export function effectiveLayout(cfg: KeyboardLayout | undefined, detected: string | undefined): string | null {
  if (cfg && cfg !== 'auto') return cfg
  return detected || null
}
