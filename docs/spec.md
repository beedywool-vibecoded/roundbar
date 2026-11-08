# roundbar spec

The contract between the four parts. Change it here first, then in code.

```
┌──────────────┐  USB HID kbd + consumer   ┌──────────────┐
│   firmware   │ ────────────────────────▶ │   Linux PC   │
│ ESP32-S3     │  USB CDC: JSON lines      │              │
│ LVGL, 360²   │ ◀───────────────────────▶ │ roundbar     │◀── D-Bus ── GNOME extension (focus)
└──────────────┘                           │  daemon (Go) │◀── D-Bus ── MPRIS players
                                           │              │◀── pulse ── PipeWire (volume, mic)
                                           │              │◀── sysfs ── /proc, hwmon, amdgpu
                                           └──────┬───────┘
                                    unix socket   │
                                           ┌──────┴───────┐
                                           │ roundbar     │  http://127.0.0.1:<port>  ◀── browser (Vue)
                                           │  config (Go) │  lives only while a tab is open
                                           └──────────────┘
```

- `roundbar daemon` runs as a systemd user service. It must stay cheap: event-driven,
  no busy loops, stats only collected while the device shows a page that needs them.
- `roundbar config` starts the configurator: an HTTP server on 127.0.0.1 that serves the
  embedded Vue app and exits ~10 s after the last browser tab disconnects.
- Files: config `~/.config/roundbar/config.json`, daemon socket `$XDG_RUNTIME_DIR/roundbar.sock`.

## 1. Config file (`config.json`)

The configurator writes it, the daemon watches it (inotify) and pushes it to the device,
the device keeps a copy in flash so it works without the daemon.

```jsonc
{
  "version": 1,
  "theme": {
    "seed": "#6750A4",
    "mode": "dark",                      // "dark" | "light"
    // Computed by the configurator with @material/material-color-utilities
    // (SchemeTonalSpot) for the chosen mode. The device only reads these.
    "palette": {
      "surface": "#141218", "surfaceContainer": "#211F26", "surfaceContainerHigh": "#2B2930",
      "surfaceContainerHighest": "#36343B", "onSurface": "#E6E0E9", "onSurfaceVariant": "#CAC4D0",
      "primary": "#D0BCFF", "onPrimary": "#381E72", "primaryContainer": "#4F378B", "onPrimaryContainer": "#EADDFF",
      "secondaryContainer": "#4A4458", "onSecondaryContainer": "#E8DEF8",
      "tertiary": "#EFB8C8", "tertiaryContainer": "#633B48", "onTertiaryContainer": "#FFD8E4",
      "error": "#F2B8B5", "errorContainer": "#8C1D18", "onErrorContainer": "#F9DEDC",
      "outline": "#938F99", "outlineVariant": "#49454F"
    }
  },
  "media": {
    "coverBackground": true,   // blurred album art behind media pages
    "coverColors": true,       // media pages take a palette derived from the album art
    "blur": 60,                // 0 (nearly sharp) … 100 (very soft); missing = 60
    "coverOpacity": 50         // how much the art shows through the surface-colored scrim, 0..100; missing = 50
  },
  "device": {
    "brightness": 80,          // 5..100
    "dimAfter": 30,            // seconds idle before dimming to 20 %, 0 = never
    "sleepAfter": 300,         // seconds idle before screen off, 0 = never
    "longPressMs": 450,
    "keyboardLayout": "fr",    // host layout used to turn characters into HID keycodes: "fr" | "us"
    "rotation": 0,             // screen rotation, degrees clockwise: 0 | 90 | 180 | 270 (touch follows)
    "followPc": true           // screen goes black ~8 s after the PC side disappears (shutdown, daemon stopped,
                               // USB suspend) and wakes when it comes back; missing = true
  },
  "profiles": [
    {
      "id": "default",         // [a-z0-9-]{1,24}, unique. "default" must exist and is used when nothing matches.
      "name": "Default",
      "match": [],             // window classes / app ids, lowercase, e.g. ["firefox", "org.mozilla.firefox"]
      "pages": [ /* Page, 1..8 */ ]
    }
  ]
}
```

### Page

Every page has `id` ([a-z0-9-]{1,24}, unique inside its profile), `type`, optional `title`.

| type | extra fields | what the device shows |
|---|---|---|
| `radial` | `center?: Button`, `buttons: Button[]` (1..8) | buttons on a ring, optional big center button |
| `media` | `shapes?: {prev?, next?, play?, playing?}` (shape names as for buttons) | now playing (title, artist, album art), prev / play-pause / next, volume on the edge arc. Defaults: prev/next/play `circle`, playing `square` (the play button switches shape with the playback state). |
| `stats` | `items: ("cpu"\|"cpuTemp"\|"ram"\|"gpu"\|"gpuTemp"\|"vram")[]` (1..4) | M3 expressive gauges |
| `dial` | `icon`, `label`, `source: "volume"\|"mic"\|"brightness"\|"custom"`, `step?: number` (custom only, 1..20), `onUp?: Action`, `onDown?: Action` (custom only) | one big value arc, drag along the edge |
| `touchpad` | `speed?: number` (1..10, default 5) | the device acts as a USB mouse: drag moves the cursor, tap = left click, long-press = right click, double-tap then drag = drag & drop, sliding along the outer ring scrolls. A sideways swipe starting on the ring changes page; a downward swipe from the top of the ring opens quick settings. |
| `image` | `images: string[]` (asset ids, 1..20; a `.gif` must be alone), `interval?: number` (seconds, 2..3600, default 8), `fit?: "cover"\|"contain"` (default cover) | full-screen picture; several stills cross-fade as a slideshow; a GIF plays |

### Button

```jsonc
{ "id": "b1", "icon": "play_arrow", "label": "Play", "color": "primary", "shape": "cookie", "tap": Action, "hold": Action /* optional */ }
```
- `shape` (optional): `"circle" | "square" | "cookie" | "clover" | "sunny" | "flower" | "burst"`. Missing: `circle` for ring
  buttons, `square` (rounded square) for the center button. Lobed shapes use the polar formula
  `r(t) = R·(1 + depth·cos(lobes·t))` with (lobes, depth): cookie (9, .07), clover (4, .17), sunny (8, .10),
  flower (6, .13), burst (12, .05). `square` is a squircle `|x|⁴+|y|⁴ = R⁴`. Pressing only scales the button
  to 91 % and lightens it; the shape stays.
- `icon`: a name from `shared/icons.json` (Material Symbols Rounded subset baked into the firmware).
- `label`: ≤ 16 chars, shown under the selected button.
- `color`: `"primary" | "secondary" | "tertiary" | "error" | "surface"` → container + on-container pair.

### Action

| type | fields | executed by |
|---|---|---|
| `keys` | `keys: "ctrl+shift+t"` (modifiers `ctrl shift alt super` + one key, or one modifier alone like `super`) | device (USB HID) |
| `text` | `text: "hello"` (≤ 64 chars, printable ASCII + `é è à ç ù`) | device (USB HID, uses `keyboardLayout`) |
| `media` | `cmd: "play_pause"\|"next"\|"prev"\|"vol_up"\|"vol_down"\|"mute"` | device (USB HID consumer) |
| `page` | `page: "<page id>"` | device |
| `command` | `cmd: "notify-send hi"` (run with `sh -c`, detached) | daemon |
| `open` | `target: "https://…" \| "firefox.desktop"` (xdg-open / gtk-launch) | daemon |
| `mic` | `cmd: "toggle"\|"mute"\|"unmute"` | daemon |
| `macro` | `steps: Step[]` (≤ 32) | daemon (asks the device to type the key steps) |

Step: `{ "keys": "ctrl+c" } | { "text": "…" } | { "delay": 250 } | { "command": "…" }`, delay ≤ 5000 ms.

Key names (lowercase): `a`–`z`, `0`–`9`, `f1`–`f24`, `enter esc tab space backspace delete insert home end
pageup pagedown up down left right minus equal leftbracket rightbracket backslash semicolon quote grave
comma period slash capslock printscreen`. Letter and digit keys name the **character**, not the physical key:
with `keyboardLayout: "fr"`, `ctrl+a` sends the physical key that types `a` on AZERTY (HID Q). Digits on `fr`
are sent with the top row as the US positions (GNOME shortcuts like `super+1` expect that), punctuation keys
name the US physical key.

### Device-only controls (not in the config)

Pages wrap around: swiping past the last page goes to the first and vice versa.

Swiping down opens a quick settings panel on the device with a row of small numbered buttons (jump to a page of the
current profile) and two tiles:
- **Screen off**: backlight off plus panel display-off/sleep: no light at all. A double tap wakes it (single taps
  are ignored). Separate from the idle `sleepAfter`, which still wakes on a single tap.
- **Auto-switch**: when off, `profile` messages from the PC are ignored and a pause badge shows at the top. Stored
  on the device, so it stays off until turned back on.

### Assets (images for `image` pages)

Uploaded through the configurator and stored in `~/.config/roundbar/assets/<id>`, where `<id>` is 12 hex chars of
the SHA-256 of the stored file plus `.jpg` or `.gif`. Stills are re-encoded as JPEG with the longest side ≤ 1080 px;
GIFs are kept as uploaded (≤ 20 MB). When syncing, the daemon renders the device version of each image for the page's
`fit` (exactly 360×360 for cover, inside 360×360 for contain; baseline JPEG, or a re-encoded full-frame GIF ≤ 3 MB),
named `<stem>-c.jpg` / `<stem>-f.jpg` (`.gif` likewise), and copies the ones the device lacks to its `assets`
partition (see §2).

## 2. Serial protocol (USB CDC, both directions)

- One JSON object per line, `\n` terminated, UTF-8, ≤ 64 KiB per line (the `config` message is the big one).
- Every message has `"t"` (type). Unknown types are ignored. The device is the source of truth for what is on screen.

### Device → PC

| t | fields | when |
|---|---|---|
| `hello` | `fw: "0.1.0"`, `proto: 1`, `cfg: "<sha1 of stored config, hex>"`, `w: 360`, `h: 360` | on CDC connect (DTR rising) and on `{"t":"hello"}` |
| `view` | `profile`, `page`, `kind: "radial"\|"media"\|"stats"\|"dial"`, `awake: bool` | page change, profile change, sleep / wake |
| `action` | `profile`, `page`, `btn`, `hold: bool`, `action: Action` | a daemon-executed action was triggered (`command open mic macro`) |
| `dial` | `source: "volume"\|"mic"`, `value: 0..100` | edge drag, throttled to ≤ 20 Hz. Brightness dials act on the device only; custom dials run their `onUp`/`onDown` actions on the device (daemon-side ones arrive as `action`). |
| `ack` | `id`, `ok: bool`, `err?` | reply to a PC message that carried an `id` |
| `assets` | `have: [ids]`, `free` | reply to `assetsWant` |
| `log` | `msg` | debug |

### PC → Device

| t | fields | notes |
|---|---|---|
| `hello` | – | ask the device to resend `hello` |
| `config` | `id`, `cfg: Config`, `sha: "<sha1>"` | sent when `hello.cfg` differs from the daemon's sha, or when the file changes. Device stores it and acks. sha1 is over the exact bytes of the config file. |
| `profile` | `id` | switch profile (focus changed). Device keeps one page index per profile. Unknown id → `default`. |
| `app` | `id`, `name` | the focused app, shown briefly as a toast |
| `media` | `title`, `artist`, `album`, `playing: bool`, `canNext`, `canPrev`, `player` | only while a `media` page is visible |
| `cover` | `seq`, `n`, `i`, `w`, `h`, `data` (base64 chunk of RGB565 little-endian pixels) | blurred album art, `w`×`h` (360×360), split in `n` chunks of ≤ 48 KiB raw. `{"t":"cover","seq":s,"n":0}` clears it. Sent on track change when `media.coverBackground` is on. |
| `mediaTheme` | `palette: {role: "#RRGGBB"}` (same roles as theme.palette) or `null` | cover palette for media pages when `media.coverColors` is on; `null` restores the theme |
| `volume` | `value: 0..100`, `muted: bool` | on change, and once when a page that shows volume becomes visible |
| `mic` | `muted: bool`, `value: 0..100` | same |
| `stats` | any of `cpu ram gpu vram` (0..100 %), `cpuTemp gpuTemp` (°C) | every 1.5 s while a `stats` page is visible and the screen is awake |
| `hid` | `keys?: string`, `text?: string`, `media?: string` | macro steps and configurator tests, typed by the device. With an `id`, acked once typed. |
| `show` | `profile`, `page` | configurator preview: jump to a page |
| `bootloader` | – | reboot into the ROM USB download mode (for flashing) |
| `assetsWant` | `id` | device replies `{"t":"assets","have":[ids],"free":bytes}` |
| `asset` | `name`, `n`, `i`, `data` (base64, ≤ 32 KiB raw per chunk), `id` (ack) | one file in `n` chunks; every chunk is acked, the file is renamed into place after the last |
| `assetsPrune` | `keep: [ids]` | delete every stored asset not listed |
| `assetsDone` | – | assets are in sync: rebuild pages that show images |

The daemon stops all periodic work when `view.awake` is false or when no device is attached.

## 3. Daemon socket (`$XDG_RUNTIME_DIR/roundbar.sock`)

JSON lines, request/response, used by `roundbar config` (and `roundbar status`, `roundbar flash`).

- `{"op":"status"}` → `{"device":{"connected":true,"port":"/dev/ttyACM1","fw":"0.1.0","cfg":"…","inSync":true},"focus":{"id":"firefox","name":"Firefox","class":"firefox"},"profile":"browser","recentApps":[{"id":"firefox","name":"Firefox","class":"firefox"},…],"view":{"profile":"…","page":"…","kind":"radial","awake":true},"extension":true}`
- `{"op":"show","profile":"…","page":"…"}` → `{"ok":true}`
- `{"op":"run","action":Action}` → `{"ok":true}` (test an action from the configurator)
- `{"op":"reload"}` → re-read config now
- `{"op":"bootloader"}` → `{"ok":true}`

## 4. Configurator HTTP API (`roundbar config`, 127.0.0.1 only)

All under `/api`, JSON. A random token is put in the URL opened in the browser (`/#t=<token>`) and must be
sent as `Authorization: Bearer <token>` on every API call.

| method | path | body / response |
|---|---|---|
| GET | `/api/config` | Config |
| PUT | `/api/config` | Config → validated, written atomically, daemon told to reload. `400 {"error":"…","path":"profiles[1].pages[0]"}` on invalid |
| GET | `/api/status` | daemon status (see socket) plus `"daemon":true`, or `{"daemon":false}` when the daemon isn't running |
| GET | `/api/icons` | `shared/icons.json` contents: `{"name": codepoint, …}` |
| POST | `/api/show` | `{"profile","page"}` → preview on the device |
| POST | `/api/run` | `{"action":Action}` → test it |
| POST | `/api/assets` | raw image body (`Content-Type: image/*`, ≤ 20 MB) → converted → `{"id","kind":"image"\|"gif","w","h","size"}`; `415 {"error"}` if it isn't an image |
| GET | `/api/assets/<id>` | the converted file (for thumbnails and previews) |
| GET | `/api/events?t=<token>` | SSE: `status` events (same JSON as `/api/status`) whenever it changes. Token in the query because EventSource can't set headers. Also the liveness signal: the server exits 10 s after the last client is gone. |
