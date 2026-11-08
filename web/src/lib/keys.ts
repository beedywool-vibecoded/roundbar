// Key combos in the spec's syntax: lowercase modifiers `ctrl shift alt super` joined with
// `+`, then one key name, or one modifier alone (`super` opens the GNOME overview). Letters and digits name the character (the device maps them
// through keyboardLayout), punctuation names the US physical key.

export const MODIFIERS = ['ctrl', 'shift', 'alt', 'super'] as const
export type Modifier = (typeof MODIFIERS)[number]

const LETTERS = 'abcdefghijklmnopqrstuvwxyz'.split('')
const DIGITS = '0123456789'.split('')
const FKEYS = Array.from({ length: 24 }, (_, i) => `f${i + 1}`)
const NAMED = [
  'enter',
  'esc',
  'tab',
  'space',
  'backspace',
  'delete',
  'insert',
  'home',
  'end',
  'pageup',
  'pagedown',
  'up',
  'down',
  'left',
  'right',
  'minus',
  'equal',
  'leftbracket',
  'rightbracket',
  'backslash',
  'semicolon',
  'quote',
  'grave',
  'comma',
  'period',
  'slash',
  'capslock',
  'printscreen',
]
export const KEY_NAMES: string[] = [...LETTERS, ...DIGITS, ...FKEYS, ...NAMED]
const KEY_SET = new Set(KEY_NAMES)

// Shown in key caps
const PRETTY: Record<string, string> = {
  ctrl: 'Ctrl',
  shift: 'Shift',
  alt: 'Alt',
  super: 'Super',
  enter: 'Enter',
  esc: 'Esc',
  tab: 'Tab',
  space: 'Space',
  backspace: 'Backspace',
  delete: 'Delete',
  insert: 'Insert',
  home: 'Home',
  end: 'End',
  pageup: 'Page Up',
  pagedown: 'Page Down',
  up: '↑',
  down: '↓',
  left: '←',
  right: '→',
  minus: '-',
  equal: '=',
  leftbracket: '[',
  rightbracket: ']',
  backslash: '\\',
  semicolon: ';',
  quote: "'",
  grave: '`',
  comma: ',',
  period: '.',
  slash: '/',
  capslock: 'Caps Lock',
  printscreen: 'Print Screen',
}
export function prettyKey(k: string): string {
  return PRETTY[k] ?? k.toUpperCase()
}

export interface Combo {
  mods: Modifier[]
  key: string
}

export function parseCombo(s: string): Combo {
  const parts = s
    .toLowerCase()
    .split('+')
    .map((p) => p.trim())
    .filter(Boolean)
  const mods: Modifier[] = []
  let key = ''
  for (const p of parts) {
    if ((MODIFIERS as readonly string[]).includes(p)) {
      if (!mods.includes(p as Modifier)) mods.push(p as Modifier)
    } else key = p
  }
  return { mods, key }
}

export function formatCombo(c: Combo): string {
  const mods = MODIFIERS.filter((m) => c.mods.includes(m))
  return [...mods, c.key].filter(Boolean).join('+')
}

// null when valid, else why not
export function comboError(s: string): string | null {
  if (!s) return 'Record or pick a key'
  const parts = s.split('+')
  if (parts.length === 1 && (MODIFIERS as readonly string[]).includes(s)) return null
  const key = parts[parts.length - 1] ?? ''
  const mods = parts.slice(0, -1)
  if (!KEY_SET.has(key)) return `"${key}" is not a key name`
  for (const m of mods) if (!(MODIFIERS as readonly string[]).includes(m)) return `"${m}" is not a modifier`
  if (new Set(mods).size !== mods.length) return 'A modifier is repeated'
  return null
}

const BY_KEY: Record<string, string> = {
  Enter: 'enter',
  Escape: 'esc',
  Tab: 'tab',
  ' ': 'space',
  Backspace: 'backspace',
  Delete: 'delete',
  Insert: 'insert',
  Home: 'home',
  End: 'end',
  PageUp: 'pageup',
  PageDown: 'pagedown',
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
  CapsLock: 'capslock',
  PrintScreen: 'printscreen',
}
// Punctuation by physical position (US names)
const BY_CODE: Record<string, string> = {
  Minus: 'minus',
  Equal: 'equal',
  BracketLeft: 'leftbracket',
  BracketRight: 'rightbracket',
  Backslash: 'backslash',
  IntlBackslash: 'backslash',
  Semicolon: 'semicolon',
  Quote: 'quote',
  Backquote: 'grave',
  Comma: 'comma',
  Period: 'period',
  Slash: 'slash',
  Space: 'space',
  Enter: 'enter',
  NumpadEnter: 'enter',
  Tab: 'tab',
  Escape: 'esc',
  Backspace: 'backspace',
}

const MOD_KEYS = new Set(['Control', 'Shift', 'Alt', 'AltGraph', 'Meta', 'OS', 'Super', 'Hyper', 'ContextMenu'])

export function isModifierKey(e: KeyboardEvent): boolean {
  return MOD_KEYS.has(e.key)
}

// The modifier a lone modifier key press stands for
export function modifierOf(e: KeyboardEvent): Modifier | null {
  switch (e.key) {
    case 'Control':
      return 'ctrl'
    case 'Shift':
      return 'shift'
    case 'Alt':
      return 'alt'
    case 'Meta':
    case 'OS':
    case 'Super':
      return 'super'
  }
  return null
}

export function heldMods(e: KeyboardEvent): Modifier[] {
  const m: Modifier[] = []
  if (e.ctrlKey) m.push('ctrl')
  if (e.shiftKey) m.push('shift')
  if (e.altKey) m.push('alt')
  if (e.metaKey) m.push('super')
  return m
}

// A keydown turned into a combo, or null for a lone modifier or a key the device can't send.
export function comboFromEvent(e: KeyboardEvent): Combo | null {
  if (isModifierKey(e)) return null
  let mods = heldMods(e)
  const k = e.key
  let key = ''
  // Letters and digits: the character typed, whatever the layout.
  if (k.length === 1 && /[a-z]/i.test(k)) key = k.toLowerCase()
  else if (k.length === 1 && /[0-9]/.test(k)) {
    key = k
    // Some layouts (AZERTY) need Shift to type a top-row digit: the browser then reports
    // the digit itself for Shift+DigitN. The device sends digits as the top row without
    // it, so that Shift was only there to produce the character.
    if (e.shiftKey && e.code === `Digit${k}`) mods = mods.filter((m) => m !== 'shift')
  } else if (BY_KEY[k]) key = BY_KEY[k]
  else if (/^F([1-9]|1[0-9]|2[0-4])$/.test(k)) key = k.toLowerCase()
  else if (BY_CODE[e.code]) key = BY_CODE[e.code]!
  else if (/^Digit[0-9]$/.test(e.code)) key = e.code.slice(5)
  else if (/^Numpad[0-9]$/.test(e.code)) key = e.code.slice(6)
  else if (/^Key[A-Z]$/.test(e.code)) key = e.code.slice(3).toLowerCase()
  if (!key || !KEY_SET.has(key)) return null
  return { mods, key }
}
