// Ready-made profiles offered in "New profile".
import type { Action, Button, ObsRequest, Page, Profile } from '@/lib/types'

export interface Preset {
  id: string
  name: string
  icon: string
  hint: string
  /** shown after creating, e.g. how to bind the keys in the app */
  setup?: string
  build: () => Omit<Profile, 'id' | 'name'>
}

const obs = (request: ObsRequest, input?: string): Action => ({ type: 'obs', request, ...(input ? { input } : {}) })
const btn = (id: string, icon: string, label: string, color: Button['color'], tap: Action): Button => ({
  id,
  icon,
  label,
  color,
  tap,
})

export const PRESETS: Preset[] = [
  {
    id: 'obs',
    name: 'OBS',
    icon: 'videocam',
    hint: 'Record, stream, live scenes and mutes for OBS Studio',
    setup: "Set up OBS in Device → OBS once (WebSocket password); then the buttons work even when OBS isn't focused.",
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
        { id: 'stats', type: 'stats', title: 'System', items: ['cpu', 'gpu', 'ram', 'cpuTemp'] } as Page,
      ],
    }),
  },
]
