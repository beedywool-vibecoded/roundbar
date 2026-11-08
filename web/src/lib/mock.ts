// In-page stand-in for `roundbar config`, enabled with `?mock`. Development only: it keeps
// the config in memory and fakes a connected device.
import iconsJson from '../../../shared/icons.json'
import { starterConfig } from './model'
import type { Config, Status } from './types'

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

export async function handle(method: string, path: string, body?: unknown): Promise<unknown> {
  await wait(200)
  const key = `${method} ${path}`
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
