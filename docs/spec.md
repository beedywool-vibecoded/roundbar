# roundbar spec

The contract between the four parts. Change it here first, then in code.

```
┌──────────────┐  USB HID kbd + consumer   ┌──────────────┐
│   firmware   │ ────────────────────────▶ │   Linux PC   │
│ ESP32-S3     │  USB CDC: JSON lines      │              │
│ LVGL, 360²   │ ◀───────────────────────▶ │ roundbar     │◀── shell backend (focus, windows, see §5)
└──────────────┘                           │  daemon (Go) │◀── D-Bus ── MPRIS players, notifications
                                           │              │◀── pulse ── PipeWire (volume, mic)
                                           │              │◀── sysfs ── /proc, hwmon, amdgpu, i915/xe
                                           │              │◀── nvidia-smi (only while a GPU gauge shows)
                                           └──────┬───────┘
                                    unix socket   │
                                           ┌──────┴───────┐
                                           │ roundbar     │  http://127.0.0.1:<port>  ◀── browser (Vue)
                                           │  config (Go) │  lives only while a tab is open
                                           └──────────────┘
```

- `roundbar daemon` runs as a systemd user service (on Windows: inside the tray app, §6). It must stay
  cheap: event-driven, no busy loops, stats only collected while the device shows a page that needs them.
- `roundbar config` starts the configurator: an HTTP server on 127.0.0.1 that serves the
  embedded Vue app and exits ~10 s after the last browser tab disconnects.
- Files: config `~/.config/roundbar/config.json`, daemon socket `$XDG_RUNTIME_DIR/roundbar.sock`.
  Windows: config `%APPDATA%\roundbar\config.json`, socket `%LOCALAPPDATA%\roundbar\roundbar.sock` (AF_UNIX,
  Windows 10 1803+), log `%LOCALAPPDATA%\roundbar\roundbar.log`.

## 1. Config file (`config.json`)

The configurator writes it, the daemon watches it (inotify; ReadDirectoryChangesW on Windows) and pushes it to the device,
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
  "weather": {                 // optional; used by clock pages. Geocoded by the configurator (Open-Meteo)
    "city": "Paris", "lat": 48.85, "lon": 2.35, "units": "metric"   // "metric" | "imperial"
  },
  "device": {
    "brightness": 80,          // 5..100
    "dimAfter": 30,            // seconds idle before dimming to 20 %, 0 = never
    "sleepAfter": 300,         // seconds idle before screen off, 0 = never
    "longPressMs": 450,
    "keyboardLayout": "auto",  // host layout used to turn characters into HID keycodes: "auto" (the daemon
                               // detects the PC's active layout and sends it, see `layout` in §2) or an id from
                               // shared/keymaps.json: "us" | "us-intl" | "gb" | "fr" | "be" | "de" | "ch" |
                               // "ch-fr" | "es" | "it" | "se" | "fi" | "no" | "dk"; missing = "auto"
    "rotation": 0,             // screen rotation, degrees clockwise: 0 | 90 | 180 | 270 (touch follows)
    "edgeDoubleTap": "page",   // double tap near the left/right edge: "page" (prev/next, instant) |
                               // "quickSettings" | "screenOff" | "off"; missing = "page"
    "notifications": true,     // pull up from the bottom edge for the notification panel; missing = true
    "notifToast": true,        // show new notifications briefly on the device; missing = true
    "followPc": true           // screen goes black ~8 s after the PC side disappears (shutdown, daemon stopped,
                               // USB suspend) and wakes when it comes back; missing = true
  },
  "profiles": [
    {
      "id": "default",         // [a-z0-9-]{1,24}, unique. "default" must exist and is used when nothing matches.
      "name": "Default",
      "match": [],             // window classes / app ids, lowercase, e.g. ["firefox", "org.mozilla.firefox"];
                               // "*" is a wildcard: "steam_app_*" matches every Steam game
      "seed": "#386A20",       // optional: own color; the configurator also writes "palette" (same roles as theme.palette,
                               // SchemeTonalSpot for theme.mode). The device uses it for this profile's pages.
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
| `clock` | `style?: "analog"\|"digital"` (default analog), `weather?: bool` (default true, needs top-level `weather`), `zones?: [{ "label": "Tokyo", "tz": "Asia/Tokyo" }]` (0..3 world clocks, IANA names) | home time large, weather below, world clocks as small dials/rows |
| `timer` | `minutes?: number[]` (1..3 presets, 1..180, default [5, 15, 25]), `pomodoro?: bool`, `label?` | turn the bezel ring to set minutes, tap the center to start/pause, hold to reset; the ring empties as time runs out; pomodoro alternates 25 min work / 5 min break (4th break 15). At the end the screen pulses and the PC shows a notification. |
| `mixer` | – | one slider row per app playing sound (icon, name, volume; tap the icon to mute); scrolls vertically |
| `windows` | – | icons of the open windows on a ring (most recent first, up to 8); tap focuses the window |
| `clipboard` | – | the last 10 copied texts as a scrolling list; tap one to paste it into the focused app |
| `discord` | – | your Discord voice channel: members around the ring (avatar, speaking = green ring, muted badge), big mute / deafen buttons, leave; "Not in a voice channel" otherwise. Needs a Discord application set up in the configurator (Device view). |
| `obs` | – | live OBS (via obs-websocket): the center toggles recording (red rounded square + elapsed time while recording), the ring holds your OBS scenes (current one highlighted; tap to switch, up to 8), a "● LIVE" label inside the record button while streaming. The daemon connects to OBS only when this page is shown or an `obs` action runs, and holds the connection only while OBS is open. Needs OBS set up in the configurator (Device view). |
| `game` | – | the game in focus (Steam `steam_app_<id>` windows; on Windows a process started from a library's `steamapps\common\<installdir>`, reported with class `steam_app_<id>` too): blurred library art behind, large game name (two lines max) and session time, and two buttons the device types itself: Steam overlay (`shift+tab`) and Steam screenshot (`f12`) |
| `mangohud` | – | live MangoHud metrics: big FPS, average and 1% low over 30 s, frame-time graph (slowest frame per 250 ms, last 15 s), CPU (left edge arc) and GPU (right edge arc) load + temperature, GPU power and VRAM. The daemon adds `control=mangohud-%p` to MangoHud.conf once, turns CSV logging on over each game's control socket only while the page is visible (per-frame rows, MangoHud's default), reads the file of the process that is rendering, and deletes the log files it caused when the page goes away |
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
| `text` | `text: "hello"` (≤ 64 chars: newline, tab and any character at least one layout of `shared/keymaps.json` can type, e.g. printable ASCII, accented letters through dead keys, AltGr characters like `€`) | device (USB HID, uses `keyboardLayout`; characters the active layout can't type are skipped and the ack is `ok: false`) |
| `media` | `cmd: "play_pause"\|"next"\|"prev"\|"vol_up"\|"vol_down"\|"mute"` | device (USB HID consumer) |
| `page` | `page: "<page id>"` | device |
| `command` | `cmd: "notify-send hi"` (run with `sh -c`, detached; Windows: `cmd.exe /c`, no console window) | daemon |
| `open` | `target: "https://…" \| "firefox.desktop"` (xdg-open / gtk-launch; Windows: ShellExecute, so a URL, file, folder or program) | daemon |
| `mic` | `cmd: "toggle"\|"mute"\|"unmute"` | daemon |
| `macro` | `steps: Step[]` (≤ 32) | daemon (asks the device to type the key steps) |
| `obs` | `request: "ToggleRecord"\|"ToggleRecordPause"\|"ToggleStream"\|"SaveReplayBuffer"\|"ToggleVirtualCam"\|"StudioTransition"\|"SetScene"\|"ToggleMute"`, `scene?` (SetScene), `input?` (ToggleMute, e.g. "Mic/Aux") | daemon, through obs-websocket: works without OBS focused |

Step: `{ "keys": "ctrl+c" } | { "text": "…" } | { "delay": 250 } | { "command": "…" }`, delay ≤ 5000 ms.

Key names (lowercase): `a`–`z`, `0`–`9`, `f1`–`f24`, `enter esc tab space backspace delete insert home end
pageup pagedown up down left right minus equal leftbracket rightbracket backslash semicolon quote grave
comma period slash capslock printscreen`. Letter and digit keys name the **character**, not the physical key:
with `keyboardLayout: "fr"`, `ctrl+a` sends the physical key that types `a` on AZERTY (HID Q). Digits are sent
as the top row keys on every layout (GNOME shortcuts like `super+1` expect that, even where the digit itself needs
Shift), punctuation keys name the US physical key.

### Keyboard layouts (`shared/keymaps.json`)

An array of `{ "id", "name", "xkb": "de" | "ch(fr)", "keys": { "<XKB key name>": [base, shift, altGr, shift+altGr] } }`.
Keys are the 48 typing keys by XKB name (`TLDE`, `AE01`–`AE12`, `AD01`–`AD12`, `AC01`–`AC11`, `BKSL`, `LSGT`,
`AB01`–`AB10`), each sent as a fixed HID usage (its US position; `LSGT` is the ISO key, HID 0x64). A level is one
character, `"dead_<acute|grave|circumflex|tilde|diaeresis|abovering|cedilla|caron>"`, or missing.
`tools/keymap-from-xkb.py` prints an entry from the local xkeyboard-config; `go run ./tools/gen-keymaps`
turns the file into the firmware table (`firmware/main/keymaps.h`) and the configurator's (`web/src/lib/layouts.gen.ts`).
A character is typed with the fewest keystrokes the layout allows: a direct key (fewest modifiers first), else a
dead key followed by space or by itself (`^`, `~`, `´`…), else a dead key followed by the base letter (`é` on German
= dead acute, `e`), as in the standard Linux Compose table.

### Device-only controls (not in the config)

Pages wrap around: swiping past the last page goes to the first and vice versa.

Swiping down opens a quick settings panel on the device with a row of small numbered buttons (jump to a page of the
current profile) and two tiles:
- **Screen off**: backlight off plus panel display-off/sleep: no light at all. A double tap wakes it (single taps
  are ignored). Separate from the idle `sleepAfter`, which still wakes on a single tap.
- Pulling up from the **bottom** edge opens the notification panel (mirrored desktop notifications, newest first,
  scrollable): tap one to open it on the PC, swipe it sideways to dismiss, "Clear all" at the end.
- **Auto-switch**: when off, `profile` messages from the PC are ignored and a pause badge shows at the top. Stored
  on the device, so it stays off until turned back on.

### Assets (images for `image` pages)

Uploaded through the configurator and stored in `~/.config/roundbar/assets/<id>` (`%APPDATA%\roundbar\assets` on Windows), where `<id>` is 12 hex chars of
the SHA-256 of the stored file plus `.jpg` or `.gif`. Stills are re-encoded as JPEG with the longest side ≤ 1080 px;
GIFs are kept as uploaded (≤ 20 MB). When syncing, the daemon renders the device version of each image for the page's
`fit` (exactly 360×360 for cover, inside 360×360 for contain; baseline JPEG, or a re-encoded full-frame GIF ≤ 3 MB),
named `<stem>-c2.jpg` / `<stem>-f2.jpg` (`.gif` likewise; the digit is the device format version, JPEGs carry a
JFIF APP0 segment), and copies the ones the device lacks to its `assets`
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
| `activate` | `id` | focus a window from a `windows` page |
| `mixerSet` | `id`, `vol` (0..100) / `mixerMute`: `id` / `output`: `id` | per-app volume, mute, default output |
| `obsCmd` | `request`, `scene?`, `input?` | from an `obs` page (record button, scene buttons) |
| `discordToggle` | `what: "mute"\|"deaf"\|"leave"` | Discord voice controls |
| `paste` | `i` | paste clipboard entry i (the daemon puts it on the clipboard, then the device types ctrl+v) |
| `notifOpen` / `notifDismiss` | `id` / `notifClear` | act on notifications |
| `timerDone` | `label`, `phase?: "work"\|"break"` | a timer page finished: the daemon shows a desktop notification with a sound |
| `configDump` | `name: "config.json"`, `sha`, `n`, `i`, `data` (base64, ≤ 24 KiB raw per chunk) | reply to `configGet`: the stored config bytes; `n: 0` when the board has none |
| `assetDump` | `name`, `n`, `i`, `data` | reply to `assetGet`: one stored asset file; `n: 0` when missing |
| `gameClose` | – | the close button on a `game` page was confirmed: the daemon ends that game's processes |
| `log` | `msg` | debug |

### Config sync: the setup travels with the board

The board keeps the last config it was given. Each PC remembers in `~/.config/roundbar/sync.json` the sha both
sides had after their last push or pull (`synced`). On `hello`, with `pc` = this PC's config sha and `board` = `hello.cfg`:

| situation | action |
|---|---|
| `board` empty (never set up) | push |
| `board == pc` | nothing (record `synced`) |
| this PC still has the untouched default config (fresh install) | pull |
| `pc == synced` (the board was edited on another PC) | pull |
| `board == synced` (this PC was edited while unplugged) | push |
| both changed, or this PC has its own setup and never met a board | pull; the PC's config is first copied to `config.json.before-sync-<time>` |

Pull = `configGet`, validate, fetch missing images with `assetGet` (the board's screen-sized copies become the PC's
originals), save `config.json`, then push it back once so both sides share the new file's sha. If the board can't answer
(older firmware) the PC pushes instead. The daemon reports the outcome as `device.syncNote` in `status`.

### PC → Device

| t | fields | notes |
|---|---|---|
| `hello` | – | ask the device to resend `hello` |
| `layout` | `id` (a `shared/keymaps.json` id) | the PC's active keyboard layout as detected by the daemon, used when `device.keyboardLayout` is `"auto"`. Sent on connect and whenever the active layout changes. The device keeps the last one in flash; until one arrives `auto` means `us`. |
| `config` | `id`, `cfg: Config`, `sha: "<sha1>"` | sent when the sync rules below say the PC's config goes to the board, or when the file changes. Device stores it and acks. sha1 is over the exact bytes of the config file. |
| `configGet` | – | device answers with `configDump` chunks |
| `assetGet` | `name` (device file name) | device answers with `assetDump` chunks |
| `profile` | `id` | switch profile (focus changed). Device keeps one page index per profile. Unknown id → `default`. |
| `app` | `id`, `name` | the focused app, shown briefly as a toast |
| `media` | `title`, `artist`, `album`, `playing: bool`, `canNext`, `canPrev`, `player` | only while a `media` page is visible |
| `cover` | `seq`, `n`, `i`, `w`, `h`, `data` (base64 chunk of RGB565 little-endian pixels) | blurred album art, `w`×`h` (360×360), split in `n` chunks of ≤ 48 KiB raw. `{"t":"cover","seq":s,"n":0}` clears it. Sent on track change when `media.coverBackground` is on. |
| `mediaTheme` | `palette: {role: "#RRGGBB"}` (same roles as theme.palette) or `null` | cover palette for media pages when `media.coverColors` is on; `null` restores the theme |
| `volume` | `value: 0..100`, `muted: bool` | on change, and once when a page that shows volume becomes visible |
| `mic` | `muted: bool`, `value: 0..100` | same |
| `stats` | any of `cpu ram gpu vram` (0..100 %), `cpuTemp gpuTemp` (°C) | every 1.5 s while a `stats` page is visible and the screen is awake. GPU: a discrete one if any (most VRAM first), else the one driving a display. AMD: sysfs. NVIDIA: a long-lived `nvidia-smi -lms` child, running only while a page with a GPU gauge is shown. Intel (i915/xe): load from the DRM fdinfo of the user's own processes (no root needed), else current/max frequency; temperature from hwmon where exposed (discrete cards); no VRAM. Keys the GPU can't provide are left out. Windows: CPU from GetSystemTimes, RAM from GlobalMemoryStatusEx, no `cpuTemp`; the adapter with the most dedicated memory (DXGI); NVIDIA through the same `nvidia-smi` child, others through the performance counters `GPU Engine(*)\Utilization Percentage` (busiest engine) and `GPU Adapter Memory(*)\Dedicated Usage`, no `gpuTemp`. |
| `hid` | `keys?: string`, `text?: string`, `media?: string`, `hold?: ms` | macro steps and configurator tests, typed by the device. With an `id`, acked once typed. `hold` keeps a `keys` combo down that long (1..2000 ms; for programs that poll the keyboard, like MangoHud every 100 ms); older firmware ignores it and taps. |
| `show` | `profile`, `page` | configurator preview: jump to a page |
| `bootloader` | – | reboot into the ROM USB download mode (for flashing) |
| `time` | `epoch` (unix s), `offset` (local UTC offset, s), `zones: {tz: offset}` | on connect and every hour (DST); the device keeps time from it |
| `weather` | `city`, `temp`, `code` (WMO), `hi`, `lo`, `units`, `day: bool` | on connect and every 30 min while a clock page can show |
| `icon` | `name`, `w`, `h`, `data` (base64 RGB565A8: RGB565 LE plane then A8 plane) | app icons for mixer / windows / notifications, sent before the list that uses them; the device caches them by name until reboot |
| `obs` | `connected`, `recording`, `paused`, `recTime` (s), `streaming`, `scenes: [string]`, `current` | while an `obs` page is visible, on change and every second while recording |
| `game` | `name`, `appid`, `since` (unix s) | while a `game` page is visible; `{"t":"game","name":""}` when no game is focused |
| `gameArt` | same fields as `cover` | blurred library art for `game` pages |
| `mangohud` | `active`, `hint?` (`noGame`\|`waiting`\|`unsupported`: no MangoHud on this OS, i.e. Windows, sent once), `fps`, `ft` (ms), `avg`, `low` (1% low fps), `cpu`, `gpu` (%), `cpuTemp`, `gpuTemp` (°C), `power` (W), `vram` (GB), `graph: [ms]` (≤ 60, oldest first) | every 500 ms while a `mangohud` page is visible |
| `windows` | `list: [{id, app, title, icon}]` | while a `windows` page is visible, on change |
| `mixer` | `apps: [{id, name, icon, vol, muted}]` | while a `mixer` page is visible, on change |
| `clips` | `list: [string]` (newest first, ≤ 10, each ≤ 200 chars) | while a `clipboard` page is visible, on change |
| `notifs` | `list: [{id, app, title, body, icon}]` (newest first, ≤ 20) | on connect and on every change, if `device.notifications` |
| `notif` | `id, app, title, body, icon` | a new notification (for the toast), followed by `notifs` |
| `discord` | `state: {connected, user, channel, members: [{id, name, avatar, muted, speaking}], mute, deaf, error}` | while a `discord` page is visible, on change (avatars arrive first as `icon`s named by URL) |
| `assetsWant` | `id` | device replies `{"t":"assets","have":[ids],"free":bytes}` |
| `asset` | `name`, `n`, `i`, `data` (base64, ≤ 32 KiB raw per chunk), `id` (ack) | one file in `n` chunks; every chunk is acked, the file is renamed into place after the last |
| `assetsPrune` | `keep: [ids]` | delete every stored asset not listed |
| `assetsDone` | – | assets are in sync: rebuild pages that show images |

The daemon stops all periodic work when `view.awake` is false or when no device is attached.

## 3. Daemon socket (`$XDG_RUNTIME_DIR/roundbar.sock`, Windows `%LOCALAPPDATA%\roundbar\roundbar.sock`)

JSON lines, request/response, used by `roundbar config` (and `roundbar status`, `roundbar bootloader`, `roundbar firmware`).
`status.device.error` is `"permission"` when the board's port exists but can't be opened (no udev rule).

- `{"op":"status"}` → `{"os":"linux","device":{"connected":true,"port":"/dev/ttyACM1","fw":"0.1.0","cfg":"…","inSync":true},"keyboard":{"detected":"de","source":"gnome"},"focus":{"id":"firefox","name":"Firefox","class":"firefox"},"profile":"browser","recentApps":[{"id":"firefox","name":"Firefox","class":"firefox"},…],"view":{"profile":"…","page":"…","kind":"radial","awake":true},"extension":true,"desktop":{"backend":"gnome","focus":true,"windows":true,"notifications":true,"clipboard":true}}`
  - `os`: `linux` or `windows`; the configurator hides what the OS can't do (MangoHud on Windows).
  - `extension`: the focus source has reported a focused app (on GNOME: the extension runs).
  - `desktop`: the shell backend in use (§5) and what it provides. A feature that is `false` is unsupported on
    this desktop: its lists are always empty (`windows`, `clips`, `notifs` are sent as `[]`), so the configurator
    can say so instead of showing a page that never fills.
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
| GET | `/api/firmware` | `{repo, repoOk, idf, built, running}`: local facts only (source checkout, ESP-IDF, built version, update running) |
| GET | `/api/firmware/release` | the latest published release: `{version, status, appSize, fontsSize}` or `{error}`. `status` compares it with `device.fw`: `upToDate`, `updateAvailable`, `deviceNewer` (development build past the tag) or `unknown` (no device, or a version that isn't a release tag). Fetched only when asked (the Firmware card when shown, its refresh button adds `?refresh=1`), cached 10 min; nothing polls in the background |
| POST | `/api/firmware/update` | `{"source":"release"}` (default) installs the latest release (§6); `{"source":"build"}` builds the source checkout with ESP-IDF and flashes it with `tools/flash.sh` (developers). Progress comes as SSE `flash` events: `{line}`, `{progress:{done,total}}` (bytes) while downloading and writing, then `{done, ok}`. The configurator doesn't exit while an update runs, even with no tab open |
| POST | `/api/obs` | `{host, port, password}` → stored by the daemon in `obs.json` (0600), then it connects; state in status `.obs: {connected, error, recording, streaming, scenes, current}` |
| GET | `/api/backup` | a zip of `config.json` and every asset file (`assets/<id>`) |
| POST | `/api/restore` | that zip (body, `application/zip`) → validated, assets written, config saved; `400 {error}` otherwise |
| POST | `/api/discord` | `{clientId, clientSecret}` → stored by the daemon in `discord.json` (0600), then it connects; state in status `.discord` |
| GET | `/api/ui` | `{setupDone}` from `~/.config/roundbar/ui.json` (this PC's configurator state, not part of the config); missing file = all false |
| PUT | `/api/ui` | any of those fields → merged into `ui.json`, the whole state back |
| POST | `/api/udev` | installs the udev rule like `roundbar udev`, through `pkexec` (the desktop's password prompt; the configurator has no terminal), then `systemctl --user try-restart roundbar`. `{ok, already?}`, or `500 {error}` (prompt refused, no pkexec) |
| GET | `/api/apps` | installed apps for ready-made profiles: `{apps: [{id, name, icon?, categories?, keys?}], tools: {mangohud}}`. Linux: desktop entries (`Type=Application`, not `NoDisplay`/`Hidden`; user dirs override system ones); `id` is the lowercase desktop id (what focus reports), `keys` other names its windows may use (`StartupWMClass`, binary), `categories` the freedesktop ones, `icon` a URL of the endpoint below. Other platforms fill the same shape |
| GET | `/api/apps/<id>/icon` | that app's icon file from the icon theme (PNG, SVG, XPM), `404` otherwise |
| GET | `/api/vesktop` | `{installs: [{kind: "native"\|"flatpak", path, enabled}], port}`: Vesktop installs found and whether their `vesktop-flags.conf` has the DevTools port flag that live voice needs |
| POST | `/api/vesktop` | `{"enable": bool}` → appends `--remote-debugging-port=<port>` (with a `# Roundbar:` comment line above it) to each found install's flags file, or removes exactly those two lines; other lines are kept, the first version of an existing file is kept as `.roundbar.bak`. No install found: the native file. `{ok, changed, installs}`. Vesktop must be restarted |
| GET | `/api/events?t=<token>` | SSE: `status` events (same JSON as `/api/status`) whenever it changes. Token in the query because EventSource can't set headers. Also the liveness signal: the server exits 10 s after the last client is gone. |

**First-run setup** (`#/setup`): the configurator opens it by itself while `ui.json` has no `setupDone`, and the Device page
reopens it. Steps, each skipped when there is nothing to do: board (waits for it, fixes `device.error == "permission"`
with `POST /api/udev`), firmware (release check and update as on the Device page), desktop (backend and missing
features; GNOME extension needs one log out / in), keyboard layout, ready-made profiles for installed apps
(`GET /api/apps`, added to the config being edited so they can be undone), integrations found (Steam, OBS, Discord /
Vesktop, MangoHud), done (saves and sets `setupDone`). Skipping sets `setupDone` too.

## 5. Desktop backends

The daemon follows the desktop through one shell backend, picked at startup from the session environment
(`ROUNDBAR_DESKTOP=gnome|kde|wayland|x11|none` overrides it):

1. `XDG_CURRENT_DESKTOP` contains `KDE` → `kde`; contains `GNOME` (not Budgie, not GNOME Flashback) → `gnome`,
   on Wayland and X11 alike.
2. otherwise `WAYLAND_DISPLAY` is set, or the desktop is a Wayland-only one (Hyprland, sway, river, niri,
   labwc, Wayfire, COSMIC) → `wayland`.
3. otherwise `DISPLAY` is set → `x11` (XFCE, Cinnamon, MATE, i3, Openbox…).
4. nothing set at all (service started before the session exported its variables) → `gnome`, as before.

| backend | focus | windows | notifications | clipboard |
|---|---|---|---|---|
| `gnome` | extension `roundbar@beedy` (D-Bus `dev.roundbar.Focus`) | extension | extension | extension |
| `kde` | KWin script, loaded through `org.kde.KWin /Scripting`, calls the daemon's `dev.roundbar.Daemon` object | KWin script; activation by a one-shot script | monitor | Klipper (`org.kde.klipper`), else Wayland data control |
| `wayland` | `zwlr_foreign_toplevel_manager_v1` (activated state) | wlr toplevel manager (with activation), else `ext_foreign_toplevel_list_v1` (list only) | monitor | `ext_data_control_v1`, else `zwlr_data_control_v1` |
| `x11` | `_NET_ACTIVE_WINDOW` (PropertyNotify on the root) | `_NET_CLIENT_LIST_STACKING`; activation by the `_NET_ACTIVE_WINDOW` client message | monitor | `CLIPBOARD` selection via XFixes |
| `none` | – | – | monitor | – |
| `windows` | `SetWinEventHook(EVENT_SYSTEM_FOREGROUND)` | `EnumWindows` (Alt+Tab rules: visible, not cloaked, unowned, no tool windows), changes from a shell hook (`RegisterShellHookWindow`); activation by `SetForegroundWindow` | – (other apps' toasts are only readable by packaged apps) | `AddClipboardFormatListener` |

- "monitor": the daemon watches `Notify` calls to `org.freedesktop.Notifications` (whichever program serves it)
  with a D-Bus monitor connection and keeps the last 20. Open brings the sender's window forward (matched by the
  `desktop-entry` hint or app name) and closes the notification; dismiss calls `CloseNotification`.
- `windows` is the only backend on Windows. Focus values there: `id` and `class` are the lowercase executable name
  without `.exe` (`firefox`, `code`, `discord`; UWP apps: the app's process, not ApplicationFrameHost), `class` is
  `steam_app_<id>` for Steam games (see `game`), `name` the executable's FileDescription, else the window title.
  The desktop, taskbar, Start menu and task switcher don't change the focus. Clipboard entries marked by password
  managers (`ExcludeClipboardContentFromMonitorProcessing`, `Clipboard Viewer Ignore`,
  `CanIncludeInClipboardHistory` = 0) are never read. Media: the Global System Media Transport Controls current
  session (WinRT, change events), cover art from its thumbnail; volume and mixer: Core Audio with change callbacks.
  Keyboard layout: the focused window's (`GetKeyboardLayout`, KLID mapped to a keymap id), re-read on focus changes.
- Focus values follow the GNOME extension: `id` is the lowercase desktop file id without `.desktop` when one
  matches (by id, `StartupWMClass`, or the last part of a reverse-DNS id), else the lowercase class; `class` is
  the lowercase WM_CLASS class / Wayland app_id (Steam games: `steam_app_<id>`); `name` the desktop entry's
  name, else the window title. Windows are listed most recently focused first.
- Clipboard history: text only, ≤ 10 entries of ≤ 2000 characters; selections offering
  `x-kde-passwordManagerHint` (password managers) are never read.
- Everything is event-driven: one socket or bus connection per active backend, nothing started for the others,
  no polling and no helper processes.

## 6. Releases, firmware updates, installation

A tag `v*` makes the CI release job publish these release assets:

| asset | what |
|---|---|
| `roundbar-linux-amd64`, `roundbar-linux-arm64` | the static binary (daemon, configurator, GNOME extension embedded) |
| `roundbar_<ver>_<arch>.deb`, `roundbar-<ver>-1.<arch>.rpm` | packages: binary, udev rule, systemd user unit, launcher, extension |
| `roundbar-setup-windows-amd64.exe` | the Windows installer (NSIS, per user, §7) |
| `roundbar-windows-amd64.exe`, `roundbar-windows-arm64.exe` | the bare Windows app (tray + daemon + configurator) |
| `bootloader.bin` (0x0), `partition-table.bin` (0x8000), `roundbar.bin` (0x10000), `fonts.bin` (`fonts` partition, 0x250000) | the flash parts, written unchanged |
| `roundbar-full.bin` | every part merged at offset 0, gaps 0xFF (same bytes as `esptool merge-bin`), for first installs |
| `firmware.json` | the manifest below |
| `roundbar-firmware.tar.gz` | parts, `flash_args`, `partitions.csv` and the manifest, for `esptool write-flash @flash_args` |
| `SHA256SUMS` | SHA-256 of every asset above |

Release firmware reports the tag as `hello.fw` (CI writes `firmware/version.txt`); local builds report `git describe`.

```jsonc
// firmware.json, written by tools/fwrelease (internal/fwupdate.Manifest)
{
  "version": "v0.2.0", "chip": "esp32s3",
  "flashMode": "dio", "flashFreq": "80m", "flashSize": "16MB", "flashBytes": 16777216,
  "parts": [
    { "name": "app", "file": "roundbar.bin", "offset": 65536, "size": 1086256,
      "sha256": "<of the file>",
      // MD5 of the file padded with 0xFF to a multiple of 4: what SPI_FLASH_MD5 returns once it is written
      "flashMd5": "<hex>" }
    // and "bootloader", "partition-table", "fonts"
  ],
  "merged": { "file": "roundbar-full.bin", "size": 11408404, "sha256": "<hex>" }
}
```

**First install**: the web installer (`site/`, GitHub Pages, ESP Web Tools) flashes `roundbar-full.bin`. The Pages
workflow copies it next to the page because release downloads carry no CORS headers.

**Updates** (configurator Firmware card, or `roundbar firmware update`), no toolchain needed:
1. Fetch `releases/latest/download/firmware.json`, then the small parts from `releases/download/<version>/`, each
   checked against its SHA-256 and cached as `~/.cache/roundbar/firmware/<sha256>.bin`.
2. Send the daemon `{"op":"bootloader"}` (repeated for ~8 s) until a ROM download port appears: USB-OTG CDC `303a:0009`,
   or USB-Serial-JTAG `303a:1001` after holding BOOT while plugging in (asked for when the firmware doesn't answer;
   waits 2 min).
3. Talk to the ROM loader (internal/espflash): SYNC, SPI_ATTACH, SPI_SET_PARAMS (16 MB); on `1001` the RTC and super
   watchdogs are disabled first. Read SPI_FLASH_MD5 of every part's region; parts matching `flashMd5` are skipped, and
   the 9 MB font pack is only downloaded and written when it differs.
4. Write the other parts with FLASH_DEFL_BEGIN/DATA (zlib stream, 1 KiB blocks; plain FLASH_BEGIN/DATA if refused) and
   verify each with SPI_FLASH_MD5. No FLASH_END: on the ROM it leaves the loader.
5. Clear `RTC_CNTL_OPTION1.FORCE_DOWNLOAD_BOOT` and reset: watchdog on `0009` (it rejects DTR/RTS), RTS pulse on `1001`.
   After a failed write the chip stays in download mode so the update can be run again; a failure before anything was
   written resets it back into the old firmware.

**PC install**: `install.sh` at the repo root (`curl -fsSL …/master/install.sh | sh`) downloads the binary for the
machine to `~/.local/bin`, checks it against `SHA256SUMS`, explains and offers the udev rule (`roundbar udev --yes`),
runs `roundbar install --no-udev` and prints next steps. `roundbar install` itself offers the udev rule (sudo, or
pkexec without a terminal; `--no-udev` skips it), writes the systemd user unit, launcher and GNOME extension (skipping
any that a package installed under `/usr`) and enables the service. `roundbar uninstall` (or `install.sh --uninstall`)
reverses it, offers to remove `/etc/udev/rules.d/70-roundbar.rules`, deletes `~/.local/bin/roundbar` (unless
`--keep-binary`) and keeps `~/.config/roundbar`. `packaging/aur/PKGBUILD` (`roundbar-bin`) and `packaging/nfpm.yaml`
install the same files system-wide.

## 7. Windows app

One `roundbar.exe` (GUI subsystem, no console window), Windows 10 1803+ / 11, amd64 and arm64.

- `roundbar` or `roundbar tray`: the daemon plus a notification-area icon. One instance per user (named mutex
  `Local\roundbar`); starting it again opens the configurator instead. Menu: the device state (`Connected (COM5)` /
  `Not connected`, disabled), Open configurator (also a double click), Start at login (toggles the Run key), Quit.
  Timer notifications are the icon's balloon notifications.
- `roundbar daemon`: the daemon without the icon. `config`, `status`, `screenshot`, `bootloader` as on Linux; from a
  terminal they print there (the exe attaches to the parent console).
- `roundbar install`: Start at login (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, value `Roundbar` =
  `"<exe>" tray`) and a Start menu shortcut; `roundbar uninstall` removes both.
- Serial: the COM port whose `Enum\USB\VID_303A&PID_4011*\…\Device Parameters\PortName` is listed in
  `HARDWARE\DEVICEMAP\SERIALCOMM` (present), opened for overlapped I/O, DTR and RTS raised. Hotplug wakes the daemon
  through `CM_Register_Notification` (COM port and USB device interfaces); nothing polls.
- Installer `roundbar-setup-windows-amd64.exe` (NSIS, per user, no admin): `%LOCALAPPDATA%\Programs\Roundbar`,
  Start menu shortcut, Run key, starts the tray app; an uninstaller in Apps & features removes all of it (the
  configuration in `%APPDATA%\roundbar` is kept).
