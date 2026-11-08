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
  c.profiles.push({
    id: 'games',
    name: 'Games',
    match: ['steam_app_*'],
    pages: [
      { id: 'game', type: 'game', title: 'Game' },
      { id: 'obs', type: 'obs', title: 'OBS' },
      { id: 'mango', type: 'mangohud', title: 'MangoHud' },
      {
        id: 'stream',
        type: 'radial',
        title: 'Stream',
        center: { id: 'b9', icon: 'videocam', label: 'Record', color: 'error', tap: { type: 'obs', request: 'ToggleRecord' } },
        buttons: [
          { id: 'b1', icon: 'mic_off', label: 'Mute mic', color: 'error', tap: { type: 'obs', request: 'ToggleMute', input: 'Mic/Aux' } },
          { id: 'b2', icon: 'grid_view', label: 'BRB', color: 'secondary', tap: { type: 'obs', request: 'SetScene', scene: 'BRB' } },
        ],
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
    { id: 'steam_app_1145360', name: 'Hades', class: 'steam_app_1145360' },
    { id: 'org.gnome.console', name: 'Console', class: 'kgx' },
  ],
  view: { profile: 'default', page: 'main', kind: 'radial', awake: true },
  extension: true,
  keyboard: { detected: 'de', source: 'gnome' },
  desktop: { backend: 'gnome', focus: true, windows: true, notifications: true, clipboard: true },
  discord: { connected: false },
  obs: { connected: false },
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
  const key = `${method} ${path.split('?')[0]}`
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
    case 'GET /api/firmware/release':
      await wait(400)
      return { version: 'v0.2.0', status: status.device?.fw === '0.2.0' ? 'upToDate' : 'updateAvailable', appSize: 1086256, fontsSize: 8983572 }
    case 'POST /api/firmware/update':
      if (flashing) throw new ApiError('an update is already running', 409)
      flashing = true
      void ((body as { source?: string } | undefined)?.source === 'build' ? fakeFlash() : fakeInstall())
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
    case 'POST /api/obs': {
      const b = body as { host?: string; port?: number; password?: string }
      if (!b.host || !b.port) throw new ApiError('host and port are required', 400)
      if (b.password === 'wrong') {
        status.obs = { connected: false, error: 'authentication failed: wrong password' }
        listeners.forEach((l) => l({ ...status }))
        return { ok: true }
      }
      setTimeout(() => {
        status.obs = { connected: true, recording: false, streaming: true, scenes: ['Main', 'BRB', 'Game', 'Chat', 'Ending'], current: 'Main' }
        listeners.forEach((l) => l({ ...status }))
      }, 1200)
      return { ok: true }
    }
    case 'GET /api/backup':
      await wait(400)
      return fakeZip(JSON.stringify(cfg ?? starterConfig(), null, 2))
    case 'POST /api/restore': {
      if (!(body instanceof Blob) || body.size < 4) throw new ApiError('not a zip file', 400)
      const sig = new Uint8Array(await body.slice(0, 2).arrayBuffer())
      if (sig[0] !== 0x50 || sig[1] !== 0x4b) throw new ApiError('not a zip file', 400)
      await wait(600)
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

async function fakeInstall() {
  const lines = [
    'installing v0.2.0',
    'downloading bootloader.bin (20 KiB)',
    'downloading partition-table.bin (3 KiB)',
    'downloading roundbar.bin (1060 KiB)',
    'asking the device to restart in download mode',
    'bootloader on /dev/ttyACM1 (USB download port)',
    'bootloader: already up to date',
    'partition-table: already up to date',
    'fonts: already up to date',
    'writing app at 0x10000 (1060 KiB)',
  ]
  for (const line of lines) {
    await wait(250)
    emitFlash({ line })
  }
  const total = 1086256
  for (let done = 0; done <= total; done += total / 20) {
    await wait(150)
    emitFlash({ progress: { done: Math.round(done), total } })
  }
  for (const line of ['app written and verified', 'restarting the device', 'done: the device is restarting with v0.2.0']) {
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

// A real (stored, uncompressed) zip holding config.json, so the download opens in an archive tool.
function fakeZip(text: string): Blob {
  const data = new TextEncoder().encode(text)
  const name = new TextEncoder().encode('config.json')
  let crc = ~0
  for (const b of data) {
    crc ^= b
    for (let k = 0; k < 8; k++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1))
  }
  crc = ~crc >>> 0
  const local = new DataView(new ArrayBuffer(30))
  local.setUint32(0, 0x04034b50, true)
  local.setUint16(4, 20, true)
  local.setUint32(14, crc, true)
  local.setUint32(18, data.length, true)
  local.setUint32(22, data.length, true)
  local.setUint16(26, name.length, true)
  const central = new DataView(new ArrayBuffer(46))
  central.setUint32(0, 0x02014b50, true)
  central.setUint16(4, 20, true)
  central.setUint16(6, 20, true)
  central.setUint32(16, crc, true)
  central.setUint32(20, data.length, true)
  central.setUint32(24, data.length, true)
  central.setUint16(28, name.length, true)
  const centralAt = 30 + name.length + data.length
  const end = new DataView(new ArrayBuffer(22))
  end.setUint32(0, 0x06054b50, true)
  end.setUint16(8, 1, true)
  end.setUint16(10, 1, true)
  end.setUint32(12, 46 + name.length, true)
  end.setUint32(16, centralAt, true)
  return new Blob([local, name, data, central, name, end], { type: 'application/zip' })
}
