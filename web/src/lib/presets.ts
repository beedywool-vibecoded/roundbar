// Ready-made profiles offered in "New profile".
import type { Action, Button, Page, Profile } from '@/lib/types'

export interface Preset {
  id: string
  name: string
  icon: string
  hint: string
  /** shown after creating, e.g. how to bind the keys in the app */
  setup?: string
  build: () => Omit<Profile, 'id' | 'name'>
}

const keys = (k: string): Action => ({ type: 'keys', keys: k })
const btn = (id: string, icon: string, label: string, color: Button['color'], k: string): Button => ({
  id,
  icon,
  label,
  color,
  tap: keys(k),
})
// OBS has no default hotkeys: Ctrl+Shift+Alt+F-keys are free everywhere, and each is
// bound once in OBS (Settings → Hotkeys: click a field, tap the button on Roundbar).
const obsKey = (n: number) => `ctrl+shift+alt+f${n}`

export const PRESETS: Preset[] = [
  {
    id: 'obs',
    name: 'OBS',
    icon: 'videocam',
    hint: 'Record, stream, scenes, mutes and screenshots for OBS Studio',
    setup:
      'In OBS, open Settings → Hotkeys, click the field of each action (e.g. "Start Recording"), then tap the matching button on Roundbar. The keys are Ctrl+Shift+Alt+F1 to F12.',
    build: () => ({
      match: ['com.obsproject.studio', 'obs'],
      pages: [
        {
          id: 'control',
          type: 'radial',
          title: 'OBS',
          center: btn('record', 'videocam', 'Record', 'error', obsKey(1)),
          buttons: [
            btn('stream', 'cast', 'Stream', 'primary', obsKey(2)),
            btn('pause', 'pause', 'Pause rec.', 'secondary', obsKey(3)),
            btn('replay', 'history', 'Save replay', 'tertiary', obsKey(4)),
            btn('shot', 'screenshot', 'Screenshot', 'surface', obsKey(5)),
            btn('mic', 'mic_off', 'Mute mic', 'error', obsKey(6)),
            btn('desktop', 'volume_off', 'Mute desktop', 'surface', obsKey(7)),
            btn('vcam', 'live_tv', 'Virtual cam', 'secondary', obsKey(8)),
            btn('transition', 'sync', 'Transition', 'primary', obsKey(9)),
          ],
        } as Page,
        {
          id: 'scenes',
          type: 'radial',
          title: 'Scenes',
          buttons: [
            btn('scene1', 'grid_view', 'Scene 1', 'primary', obsKey(10)),
            btn('scene2', 'dashboard', 'Scene 2', 'secondary', obsKey(11)),
            btn('scene3', 'layers', 'Scene 3', 'tertiary', obsKey(12)),
          ],
        } as Page,
        { id: 'stats', type: 'stats', title: 'System', items: ['cpu', 'gpu', 'ram', 'cpuTemp'] } as Page,
      ],
    }),
  },
]
