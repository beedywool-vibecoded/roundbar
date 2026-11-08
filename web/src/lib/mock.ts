// In-page stand-in for `roundbar config`, enabled with `?mock`. Development only: it keeps
// the config in memory and fakes a connected device.
import iconsJson from '../../../shared/icons.json'
import { ApiError, emitFlash } from './api'
import { starterConfig } from './model'
import { devicePalette } from './theme'
import type { AssetInfo, Config, Status } from './types'

// `?mock&new` starts without a config file; plain `?mock` with a sample that shows every page type.
let cfg: Config | null = new URLSearchParams(location.search).has('new') ? null : sample()

function sample(): Config {
  const c = starterConfig()
  c.weather = { city: 'Paris', lat: 48.85, lon: 2.35, units: 'metric' }
  c.profiles.push({
    id: 'desk',
    name: 'Desk',
    match: ['code'],
    seed: '#006A6A',
    palette: devicePalette('#006A6A', c.theme.mode),
    pages: [
      { id: 'clock', type: 'clock', title: 'Clock', zones: [{ label: 'Tokyo', tz: 'Asia/Tokyo' }, { label: 'New York', tz: 'America/New_York' }] },
      { id: 'digital', type: 'clock', style: 'digital', zones: [{ label: 'Tokyo', tz: 'Asia/Tokyo' }] },
      { id: 'timer', type: 'timer', title: 'Timer', pomodoro: true, label: 'Focus' },
      { id: 'mixer', type: 'mixer', title: 'Mixer' },
      { id: 'windows', type: 'windows', title: 'Windows' },
      { id: 'clipboard', type: 'clipboard', title: 'Clipboard' },
      { id: 'discord', type: 'discord', title: 'Discord' },
      {
        id: 'macros',
        type: 'radial',
        title: 'Macros',
        buttons: [{ id: 'b1', icon: 'content_copy', label: 'Copy line', color: 'primary', tap: { type: 'macro', steps: [{ keys: 'home' }, { keys: 'shift+end' }, { keys: 'ctrl+c' }] } }],
      },
    ],
  })
  return c
}
const status: Status = {
  daemon: true,
  device: { connected: true, port: '/dev/ttyACM0', fw: '0.1.0', cfg: '3f2a9c0d41b7', inSync: true },
  focus: { id: 'firefox', name: 'Firefox', class: 'firefox' },
  profile: 'default',
  recentApps: [
    { id: 'firefox', name: 'Firefox', class: 'firefox' },
    { id: 'code', name: 'Visual Studio Code', class: 'code' },
    { id: 'org.gnome.nautilus', name: 'Files', class: 'org.gnome.nautilus' },
    { id: 'spotify', name: 'Spotify', class: 'spotify' },
    { id: 'org.gnome.console', name: 'Console', class: 'kgx' },
  ],
  view: { profile: 'default', page: 'main', kind: 'radial', awake: true },
  extension: true,
  discord: { connected: false },
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))

// Uploaded files are kept as they are; the real server converts them to JPEG or GIF.
const assets = new Map<string, Blob>()

async function upload(body: unknown): Promise<AssetInfo> {
  if (!(body instanceof Blob) || !body.type.startsWith('image/')) throw new ApiError('This file is not an image', 415)
  if (body.size > 20 << 20) throw new ApiError('The file is too large (20 MB at most)', 413)
  await wait(600)
  const gif = body.type === 'image/gif'
  const id = `${Array.from({ length: 12 }, () => ((Math.random() * 16) | 0).toString(16)).join('')}.${gif ? 'gif' : 'jpg'}`
  assets.set(id, body)
  let w = 360
  let h = 360
  try {
    const bmp = await createImageBitmap(body)
    const k = Math.min(1, 360 / Math.min(bmp.width, bmp.height))
    w = Math.round(bmp.width * k)
    h = Math.round(bmp.height * k)
    bmp.close()
  } catch {
    // keep the square guess
  }
  return { id, kind: gif ? 'gif' : 'image', w, h, size: body.size }
}

export async function handle(method: string, path: string, body?: unknown): Promise<unknown> {
  await wait(200)
  const key = `${method} ${path}`
  if (key === 'POST /api/assets') return upload(body)
  if (method === 'GET' && path.startsWith('/api/assets/')) {
    const blob = assets.get(decodeURIComponent(path.slice('/api/assets/'.length)))
    if (!blob) throw new ApiError('No such asset', 404)
    return blob
  }
  switch (key) {
    case 'GET /api/config':
      return cfg ? JSON.parse(JSON.stringify(cfg)) : null
    case 'PUT /api/config':
      cfg = JSON.parse(JSON.stringify(body)) as Config
      return JSON.parse(JSON.stringify(cfg))
    case 'GET /api/status':
      return status
    case 'GET /api/icons':
      return iconsJson
    case 'POST /api/show': {
      const b = body as { profile: string; page: string }
      const page = (cfg ?? starterConfig()).profiles.find((p) => p.id === b.profile)?.pages.find((p) => p.id === b.page)
      status.view = { profile: b.profile, page: b.page, kind: page?.type ?? 'radial', awake: true }
      listeners.forEach((l) => l({ ...status }))
      return { ok: true }
    }
    case 'GET /api/firmware':
      return { repo: '/home/me/roundbar', repoOk: true, idf: '/home/me/esp/esp-idf-v5.4/export.sh', built: '0.2.0', running: flashing }
    case 'POST /api/firmware/update':
      if (flashing) throw new ApiError('an update is already running', 409)
      flashing = true
      void fakeFlash()
      return { ok: true }
    case 'POST /api/discord': {
      const b = body as { clientId?: string; clientSecret?: string }
      if (!b.clientId || !b.clientSecret) throw new ApiError('clientId and clientSecret are required', 400)
      setTimeout(() => {
        status.discord = { connected: true, user: 'alex', channel: 'General' }
        listeners.forEach((l) => l({ ...status }))
      }, 1500)
      return { ok: true }
    }
    case 'POST /api/run':
      console.info('run', body)
      return { ok: true }
  }
  throw new Error(`mock: no route for ${key}`)
}

let flashing = false
const FLASH_LOG = [
  'Executing action: all (aliases: build)',
  'Running ninja in directory /home/me/roundbar/firmware/build',
  '[1/4] Building C object esp-idf/main/CMakeFiles/__idf_main.dir/ui.c.obj',
  '[2/4] Linking CXX executable roundbar.elf',
  '[3/4] Generating binary image from built executable',
  'roundbar.bin binary size 0x1a2f40 bytes. Smallest app partition is 0x300000 bytes.',
  'Project build complete.',
  'Rebooting the device into download mode…',
  'esptool.py v4.8.1',
  'Serial port /dev/ttyACM0',
  'Connecting...',
  'Chip is ESP32-S3 (QFN56) (revision v0.2)',
  'Writing at 0x00010000... (10 %)',
  'Writing at 0x00080000... (40 %)',
  'Writing at 0x00120000... (75 %)',
  'Writing at 0x001a0000... (100 %)',
  'Wrote 1716032 bytes (1022431 compressed) at 0x00010000 in 14.2 seconds',
  'Hard resetting via RTS pin...',
  'Done',
]
async function fakeFlash() {
  for (const line of FLASH_LOG) {
    await wait(250)
    emitFlash({ line })
  }
  flashing = false
  status.device!.fw = '0.2.0'
  listeners.forEach((l) => l({ ...status }))
  emitFlash({ done: true, ok: true })
}

const listeners = new Set<(s: Status) => void>()
export function streamStatus(on: (s: Status) => void): () => void {
  listeners.add(on)
  on({ ...status })
  const apps = status.recentApps!
  let i = 0
  const timer = setInterval(() => {
    i = (i + 1) % apps.length
    status.focus = apps[i]
    on({ ...status })
  }, 8000)
  return () => {
    clearInterval(timer)
    listeners.delete(on)
  }
}
