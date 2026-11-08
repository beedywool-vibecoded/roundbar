// Ready-made profiles offered in "New profile" and by the setup for the apps it finds.
import type { Action, Button, InstalledApp, ObsRequest, Page, Profile } from '@/lib/types'

export interface Preset {
  id: string
  name: string
  icon: string
  hint: string
  /** shown after creating, e.g. how to bind the keys in the app */
  setup?: string
  /** Which installed apps it is for (GET /api/apps): desktop ids, or any of these categories */
  detect?: { ids?: string[]; categories?: string[]; not?: string[] }
  /** Added by the setup unless the user unticks it */
  suggested?: boolean
  /** Match only these, not the apps found (games match steam_app_*, not Steam itself) */
  fixedMatch?: boolean
  build: () => Omit<Profile, 'id' | 'name'>
}

const obs = (request: ObsRequest, input?: string): Action => ({ type: 'obs', request, ...(input ? { input } : {}) })
const keys = (k: string): Action => ({ type: 'keys', keys: k })
const btn = (id: string, icon: string, label: string, color: Button['color'], tap: Action, hold?: Action): Button => ({
  id,
  icon,
  label,
  color,
  tap,
  ...(hold ? { hold } : {}),
})
const stats = (id = 'stats'): Page => ({ id, type: 'stats', title: 'System', items: ['cpu', 'gpu', 'ram', 'cpuTemp'] })

export const PRESETS: Preset[] = [
  {
    id: 'browser',
    name: 'Browser',
    icon: 'public',
    hint: 'Tabs, back and forward, zoom, and what is playing',
    detect: { categories: ['WebBrowser'] },
    suggested: true,
    build: () => ({
      match: [],
      pages: [
        {
          id: 'tabs',
          type: 'radial',
          title: 'Tabs',
          center: btn('reload', 'refresh', 'Reload', 'primary', keys('ctrl+r')),
          buttons: [
            btn('back', 'arrow_back', 'Back', 'secondary', keys('alt+left')),
            btn('forward', 'arrow_forward', 'Forward', 'secondary', keys('alt+right')),
            btn('newtab', 'add', 'New tab', 'primary', keys('ctrl+t')),
            btn('close', 'tab_close', 'Close tab', 'error', keys('ctrl+w')),
            btn('reopen', 'history', 'Reopen tab', 'tertiary', keys('ctrl+shift+t')),
            btn('next', 'keyboard_tab', 'Next tab', 'surface', keys('ctrl+tab'), keys('ctrl+shift+tab')),
            btn('zoomin', 'zoom_in', 'Zoom in', 'surface', keys('ctrl+equal')),
            btn('zoomout', 'zoom_out', 'Zoom out', 'surface', keys('ctrl+minus'), keys('ctrl+0')),
          ],
        } as Page,
        { id: 'media', type: 'media', title: 'Media' },
      ],
    }),
  },
  {
    id: 'code',
    name: 'Code',
    icon: 'code',
    hint: 'Command palette, search, run and the terminal of your code editor',
    detect: {
      ids: ['code', 'code-oss', 'codium', 'vscodium', 'com.visualstudio.code', 'com.vscodium.codium', 'dev.zed.zed', 'zed', 'cursor', 'sublime_text', 'com.sublimetext.three'],
      categories: ['IDE'],
    },
    suggested: true,
    build: () => ({
      match: [],
      pages: [
        {
          id: 'code',
          type: 'radial',
          title: 'Code',
          center: btn('palette', 'terminal', 'Commands', 'primary', keys('ctrl+shift+p')),
          buttons: [
            btn('save', 'save', 'Save', 'primary', keys('ctrl+s')),
            btn('file', 'file_open', 'Go to file', 'secondary', keys('ctrl+p')),
            btn('find', 'search', 'Find in files', 'secondary', keys('ctrl+shift+f')),
            btn('comment', 'edit_note', 'Comment', 'tertiary', keys('ctrl+slash')),
            btn('run', 'play_arrow', 'Run', 'tertiary', keys('f5'), keys('shift+f5')),
            btn('term', 'terminal', 'Terminal', 'surface', keys('ctrl+grave')),
            btn('undo', 'undo', 'Undo', 'surface', keys('ctrl+z'), keys('ctrl+shift+z')),
            btn('git', 'commit', 'Git', 'error', keys('ctrl+shift+g')),
          ],
        } as Page,
        { id: 'timer', type: 'timer', title: 'Focus', pomodoro: true, label: 'Focus' },
        stats(),
      ],
    }),
  },
  {
    id: 'music',
    name: 'Music',
    icon: 'music_note',
    hint: 'Now playing with the album art, volume and per-app volume',
    detect: {
      ids: [
        'spotify', 'com.spotify.client', 'rhythmbox', 'org.gnome.rhythmbox3', 'elisa', 'org.kde.elisa', 'io.bassi.amberol',
        'amberol', 'strawberry', 'org.strawberrymusicplayer.strawberry', 'lollypop', 'org.gnome.lollypop', 'audacious',
        'deadbeef', 'tidal-hifi', 'com.mastermindzh.tidal-hifi', 'youtube-music', 'com.github.th_ch.youtube_music', 'cider',
        'org.gnome.music', 'clementine', 'quodlibet', 'io.github.quodlibet.quodlibet', 'feishin',
      ],
    },
    suggested: true,
    build: () => ({
      match: [],
      pages: [
        { id: 'media', type: 'media', title: 'Now playing' },
        { id: 'volume', type: 'dial', title: 'Volume', icon: 'volume_up', label: 'Volume', source: 'volume' },
        { id: 'mixer', type: 'mixer', title: 'Mixer' },
      ],
    }),
  },
  {
    id: 'discord',
    name: 'Discord',
    icon: 'headset_mic',
    hint: "Your voice channel: who's talking, mute and deafen",
    setup: 'Voice pages need a one-time setup in Device → Discord.',
    detect: { ids: ['discord', 'com.discordapp.discord', 'vesktop', 'dev.vencord.vesktop', 'webcord', 'io.github.spacingbat3.webcord', 'legcord', 'armcord', 'discord-canary', 'discord-ptb'] },
    suggested: true,
    build: () => ({
      match: [],
      pages: [
        { id: 'voice', type: 'discord', title: 'Voice' },
        { id: 'mixer', type: 'mixer', title: 'Mixer' },
      ],
    }),
  },
  {
    id: 'games',
    name: 'Games',
    icon: 'sports_esports',
    hint: 'The game you are playing: art, session time, Steam overlay, screenshot, and system load',
    detect: { ids: ['steam', 'com.valvesoftware.steam'] },
    suggested: true,
    fixedMatch: true,
    build: () => ({
      match: ['steam_app_*'],
      pages: [{ id: 'game', type: 'game', title: 'Game' }, stats(), { id: 'mixer', type: 'mixer', title: 'Mixer' }],
    }),
  },
  {
    id: 'obs',
    name: 'OBS',
    icon: 'videocam',
    hint: 'Record, stream, live scenes and mutes for OBS Studio',
    setup: "Set up OBS in Device → OBS once (WebSocket password); then the buttons work even when OBS isn't focused.",
    detect: { ids: ['com.obsproject.studio', 'obs', 'obs-studio'] },
    suggested: true,
    build: () => ({
      match: ['com.obsproject.studio', 'obs'],
      pages: [
        {
          id: 'control',
          type: 'radial',
          title: 'OBS',
          center: btn('record', 'videocam', 'Record', 'error', obs('ToggleRecord')),
          buttons: [
            btn('stream', 'cast', 'Stream', 'primary', obs('ToggleStream')),
            btn('pause', 'pause', 'Pause rec.', 'secondary', obs('ToggleRecordPause')),
            btn('replay', 'history', 'Save replay', 'tertiary', obs('SaveReplayBuffer')),
            btn('mic', 'mic_off', 'Mute mic', 'error', obs('ToggleMute', 'Mic/Aux')),
            btn('desktop', 'volume_off', 'Mute desktop', 'surface', obs('ToggleMute', 'Desktop Audio')),
            btn('vcam', 'live_tv', 'Virtual cam', 'secondary', obs('ToggleVirtualCam')),
            btn('transition', 'sync', 'Transition', 'primary', obs('StudioTransition')),
          ],
        } as Page,
        { id: 'scenes', type: 'obs', title: 'Scenes' } as Page,
        stats(),
      ],
    }),
  },
  {
    id: 'terminal',
    name: 'Terminal',
    icon: 'terminal',
    hint: 'Copy, paste, tabs and stop for your terminal',
    detect: { categories: ['TerminalEmulator'] },
    build: () => ({
      match: [],
      pages: [
        {
          id: 'term',
          type: 'radial',
          title: 'Terminal',
          center: btn('stop', 'stop', 'Stop', 'error', keys('ctrl+c')),
          buttons: [
            btn('copy', 'content_copy', 'Copy', 'secondary', keys('ctrl+shift+c')),
            btn('paste', 'content_paste', 'Paste', 'secondary', keys('ctrl+shift+v')),
            btn('newtab', 'add', 'New tab', 'primary', keys('ctrl+shift+t')),
            btn('close', 'tab_close', 'Close tab', 'tertiary', keys('ctrl+shift+w')),
            btn('clear', 'clear_all', 'Clear', 'surface', keys('ctrl+l')),
            btn('search', 'search', 'History', 'surface', keys('ctrl+r')),
            btn('up', 'arrow_upward', 'Last command', 'primary', keys('up')),
            btn('enter', 'keyboard_return', 'Enter', 'primary', keys('enter')),
          ],
        } as Page,
        stats(),
      ],
    }),
  },
  {
    id: 'files',
    name: 'Files',
    icon: 'folder',
    hint: 'Back, up, new folder, rename and hidden files in your file manager',
    detect: { categories: ['FileManager'], not: ['org.gnome.baobab'] },
    build: () => ({
      match: [],
      pages: [
        {
          id: 'files',
          type: 'radial',
          title: 'Files',
          center: btn('open', 'folder_open', 'Open', 'primary', keys('enter')),
          buttons: [
            btn('back', 'arrow_back', 'Back', 'secondary', keys('alt+left')),
            btn('up', 'arrow_upward', 'Up', 'secondary', keys('alt+up')),
            btn('newfolder', 'add', 'New folder', 'primary', keys('ctrl+shift+n')),
            btn('rename', 'edit', 'Rename', 'tertiary', keys('f2')),
            btn('copy', 'content_copy', 'Copy', 'surface', keys('ctrl+c')),
            btn('paste', 'content_paste', 'Paste', 'surface', keys('ctrl+v')),
            btn('hidden', 'visibility', 'Hidden files', 'tertiary', keys('ctrl+h')),
            btn('search', 'search', 'Search', 'surface', keys('ctrl+f')),
          ],
        } as Page,
      ],
    }),
  },
]

/** The installed apps a preset is for. */
export function appsFor(p: Preset, apps: InstalledApp[]): InstalledApp[] {
  const d = p.detect
  if (!d) return []
  return apps.filter(
    (a) =>
      !d.not?.includes(a.id) &&
      (d.ids?.includes(a.id) || d.ids?.some((id) => a.keys?.includes(id)) || d.categories?.some((c) => a.categories?.includes(c))),
  )
}

/** What a profile for these apps matches: their ids and other window names, plus the preset's own. */
export function matchFor(p: Preset, found: InstalledApp[]): string[] {
  const own = p.build().match
  if (p.fixedMatch) return own
  const out = new Set<string>()
  for (const a of found) {
    out.add(a.id)
    for (const k of a.keys ?? []) if (/^[a-z0-9._*-]+$/.test(k)) out.add(k)
  }
  for (const m of own) out.add(m)
  return [...out]
}
