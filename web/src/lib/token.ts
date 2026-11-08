// The configurator is opened as `/#t=<token>`. The token is read once, kept for the tab in
// sessionStorage, and the hash is cleared before the router reads it. Imported first.

const KEY = 'roundbar.token'

function capture(): string {
  const m = /(?:^#|[#&])t=([^&]+)/.exec(location.hash)
  if (m?.[1]) {
    const t = decodeURIComponent(m[1])
    try {
      sessionStorage.setItem(KEY, t)
    } catch {
      // storage blocked: the token lasts until the tab reloads
    }
    history.replaceState(null, '', `${location.pathname}${location.search}#/`)
    return t
  }
  try {
    return sessionStorage.getItem(KEY) ?? ''
  } catch {
    return ''
  }
}

export const token = capture()

// `?mock` serves sample data from the page itself, for working on the UI without the Go side.
export const mockMode = new URLSearchParams(location.search).has('mock')
