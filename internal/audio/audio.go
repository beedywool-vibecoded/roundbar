// Package audio follows the default output and microphone (volume, mute,
// change notifications) and the per-app playback streams for mixer pages.
// Linux: PipeWire through its PulseAudio protocol server. Windows: Core Audio.
package audio

type State struct {
	Volume   int // 0..100 (can exceed 100 when boosted; clamped for the device)
	Muted    bool
	Mic      int
	MicMuted bool
}

type App struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Vol   int    `json:"vol"`
	Muted bool   `json:"muted"`
}

type Output struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
