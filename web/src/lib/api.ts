// Client for the configurator API (docs/spec.md section 4). Every call carries the
// bearer token from the URL the configurator was opened with.
import { mockMode, token } from './token'
import type { Action, Config, Status } from './types'

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly path?: string,
  ) {
    super(message)
  }
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

type Handler = (method: string, path: string, body?: unknown) => Promise<unknown>
let mock: Handler | null = null
async function mockHandler(): Promise<Handler> {
  if (!mock) mock = (await import('./mock')).handle
  return mock
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  return (await requestFull<T>(method, path, body)).data
}

// Same, with the response headers.
export async function requestFull<T>(method: string, path: string, body?: unknown): Promise<{ data: T; headers: Headers }> {
  if (mockMode) return { data: (await (await mockHandler())(method, path, body)) as T, headers: new Headers() }
  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
      cache: 'no-store',
    })
  } catch {
    throw new ApiError('The configurator server is not reachable. Run `roundbar config` again.', 0)
  }
  const text = await res.text()
  let data: unknown = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = null
  }
  if (!res.ok) {
    if (res.status === 401) onUnauthorized()
    const d = (data ?? {}) as { error?: string; path?: string }
    throw new ApiError(d.error || text.trim() || `${res.status} ${res.statusText}`, res.status, d.path)
  }
  return { data: data as T, headers: res.headers }
}

export const api = {
  // X-Config-Error: the file on disk was invalid and the body holds the defaults instead.
  config: async () => {
    const r = await requestFull<Config | null>('GET', '/api/config')
    return { config: r.data, fileError: r.headers.get('X-Config-Error') ?? '' }
  },
  // 200 returns the config as saved
  saveConfig: (c: Config) => request<Config | null>('PUT', '/api/config', c),
  status: () => request<Status>('GET', '/api/status'),
  icons: () => request<Record<string, number>>('GET', '/api/icons'),
  show: (profile: string, page: string) => request<unknown>('POST', '/api/show', { profile, page }),
  run: (action: Action) => request<unknown>('POST', '/api/run', { action }),
}

// Status over server-sent events. The open stream is also what keeps `roundbar config`
// running, so it reconnects whenever it drops. EventSource can't send headers: the token
// goes in the query string. Returns a function that closes the stream for good.
export function streamStatus(on: { status: (s: Status) => void; connected: (up: boolean) => void }): () => void {
  if (mockMode) {
    let stop = () => {}
    import('./mock').then((m) => (stop = m.streamStatus(on.status)))
    on.connected(true)
    return () => stop()
  }
  let es: EventSource | null = null
  let retry: ReturnType<typeof setTimeout> | undefined
  let delay = 1000
  let closed = false

  const open = () => {
    es = new EventSource(`/api/events?t=${encodeURIComponent(token)}`)
    es.onopen = () => {
      delay = 1000
      on.connected(true)
    }
    const handle = (e: MessageEvent) => {
      try {
        on.status(JSON.parse(e.data) as Status)
      } catch {
        // ignore a malformed event
      }
    }
    es.addEventListener('status', handle as EventListener)
    es.onmessage = handle
    es.onerror = () => {
      on.connected(false)
      // The browser retries by itself while CONNECTING; after an HTTP error it gives up.
      // Reconnecting quickly matters: the server exits 10 s after the last client leaves.
      if (es?.readyState === EventSource.CLOSED && !closed) {
        es.close()
        clearTimeout(retry)
        retry = setTimeout(open, delay)
        delay = Math.min(delay * 2, 3000)
      }
    }
  }
  open()
  return () => {
    closed = true
    clearTimeout(retry)
    es?.close()
  }
}
