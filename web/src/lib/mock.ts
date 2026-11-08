// In-page stand-in for `roundbar config`, enabled with `?mock`. Development only: it keeps
// the config in memory and fakes a connected device.
import iconsJson from '../../../shared/icons.json'
import { ApiError } from './api'
import { starterConfig } from './model'
import type { AssetInfo, Config, Status } from './types'

let cfg: Config | null = null
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
    case 'POST /api/run':
      console.info('run', body)
      return { ok: true }
  }
  throw new Error(`mock: no route for ${key}`)
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
