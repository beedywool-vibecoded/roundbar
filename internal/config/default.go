package config

func ptr(v int) *int { return &v }

func keys(k string) *Action  { return &Action{Type: "keys", Keys: k} }
func media(c string) *Action { return &Action{Type: "media", Cmd: c} }
func btn(id, icon, label, color string, tap *Action) Button {
	return Button{ID: id, Icon: icon, Label: label, Color: color, Tap: tap}
}

func mediaPage() Page { return Page{ID: "media", Type: "media", Title: "Now playing"} }
func statsPage() Page {
	return Page{ID: "stats", Type: "stats", Title: "System", Items: []string{"cpu", "gpu", "ram", "cpuTemp"}}
}

// Default is written on first run. Palette: SchemeTonalSpot of #6750A4, dark.
func Default() *Config {
	return &Config{
		Version: 1,
		Theme: Theme{Seed: "#6750A4", Mode: "dark", Palette: map[string]string{
			"surface": "#141218", "surfaceContainer": "#211F26", "surfaceContainerHigh": "#2B2930",
			"surfaceContainerHighest": "#36343B", "onSurface": "#E6E0E9", "onSurfaceVariant": "#CAC4D0",
			"primary": "#D0BCFF", "onPrimary": "#381E72", "primaryContainer": "#4F378B", "onPrimaryContainer": "#EADDFF",
			"secondaryContainer": "#4A4458", "onSecondaryContainer": "#E8DEF8",
			"tertiary": "#EFB8C8", "tertiaryContainer": "#633B48", "onTertiaryContainer": "#FFD8E4",
			"error": "#F2B8B5", "errorContainer": "#8C1D18", "onErrorContainer": "#F9DEDC",
			"outline": "#938F99", "outlineVariant": "#49454F",
		}},
		Media:  Media{CoverBackground: true, CoverColors: true, Blur: ptr(60), CoverOpacity: ptr(50)},
		Device: Device{Brightness: 80, DimAfter: 30, SleepAfter: 300, LongPressMs: 450, KeyboardLayout: "fr"},
		Profiles: []Profile{
			{ID: "default", Name: "Default", Match: []string{}, Pages: []Page{
				mediaPage(),
				{ID: "desk", Type: "radial", Title: "Desktop",
					Center: &Button{ID: "overview", Icon: "apps", Label: "Overview", Color: "primary", Tap: keys("super")},
					Buttons: []Button{
						btn("copy", "content_copy", "Copy", "secondary", keys("ctrl+c")),
						btn("paste", "content_paste", "Paste", "secondary", keys("ctrl+v")),
						btn("undo", "undo", "Undo", "surface", keys("ctrl+z")),
						btn("shot", "screenshot", "Screenshot", "tertiary", keys("printscreen")),
						btn("files", "folder", "Files", "surface", &Action{Type: "open", Target: "org.gnome.Nautilus.desktop"}),
						btn("term", "terminal", "Terminal", "surface", keys("ctrl+alt+t")),
						btn("mic", "mic_off", "Mic", "error", &Action{Type: "mic", Cmd: "toggle"}),
						btn("lock", "lock", "Lock", "surface", keys("super+l")),
					}},
				{ID: "volume", Type: "dial", Icon: "volume_up", Label: "Volume", Source: "volume"},
				statsPage(),
			}},
			{ID: "browser", Name: "Browser", Match: []string{"firefox", "org.mozilla.firefox", "chromium", "google-chrome", "brave-browser", "zen"}, Pages: []Page{
				{ID: "tabs", Type: "radial", Title: "Browser",
					Center: &Button{ID: "newtab", Icon: "add", Label: "New tab", Color: "primary", Tap: keys("ctrl+t")},
					Buttons: []Button{
						btn("back", "arrow_back", "Back", "secondary", keys("alt+left")),
						btn("fwd", "arrow_forward", "Forward", "secondary", keys("alt+right")),
						btn("reload", "refresh", "Reload", "surface", keys("ctrl+r")),
						btn("close", "tab_close", "Close tab", "error", keys("ctrl+w")),
						btn("reopen", "history", "Reopen", "surface", keys("ctrl+shift+t")),
						btn("address", "search", "Address", "tertiary", keys("ctrl+l")),
						btn("prevtab", "chevron_left", "Prev tab", "surface", keys("ctrl+pageup")),
						btn("nexttab", "chevron_right", "Next tab", "surface", keys("ctrl+pagedown")),
					}},
				mediaPage(),
			}},
			{ID: "code", Name: "Code editor", Match: []string{"code", "code-oss", "vscodium", "dev.zed.zed", "jetbrains-idea"}, Pages: []Page{
				{ID: "edit", Type: "radial", Title: "Code",
					Center: &Button{ID: "palette", Icon: "terminal", Label: "Commands", Color: "primary", Tap: keys("ctrl+shift+p")},
					Buttons: []Button{
						btn("save", "save", "Save", "secondary", keys("ctrl+s")),
						btn("open", "file_open", "Quick open", "surface", keys("ctrl+p")),
						btn("find", "search", "Find", "surface", keys("ctrl+shift+f")),
						btn("sidebar", "dashboard", "Sidebar", "surface", keys("ctrl+b")),
						btn("debug", "bug_report", "Debug", "tertiary", keys("f5")),
						btn("format", "format_list_bulleted", "Format", "surface", keys("ctrl+shift+i")),
						btn("git", "commit", "Git", "surface", keys("ctrl+shift+g")),
						btn("rename", "edit", "Rename", "secondary", keys("f2")),
					}},
				statsPage(),
			}},
			{ID: "music", Name: "Music", Match: []string{"spotify", "com.spotify.client", "mpv", "rhythmbox", "org.gnome.music"}, Pages: []Page{
				mediaPage(),
				{ID: "volume", Type: "dial", Icon: "volume_up", Label: "Volume", Source: "volume"},
			}},
			{ID: "games", Name: "Discord & games", Match: []string{"discord", "vesktop", "steam", "steam_app_default"}, Pages: []Page{
				{ID: "voice", Type: "radial", Title: "Voice",
					Center: &Button{ID: "mute", Icon: "mic_off", Label: "Mute", Color: "error", Tap: keys("ctrl+shift+m")},
					Buttons: []Button{
						btn("deafen", "headphones", "Deafen", "tertiary", keys("ctrl+shift+d")),
						btn("overlay", "sports_esports", "Overlay", "primary", keys("shift+tab")),
						btn("shot", "photo_camera", "Screenshot", "secondary", keys("f12")),
						btn("micpc", "mic", "PC mic", "surface", &Action{Type: "mic", Cmd: "toggle"}),
					}},
				statsPage(),
				mediaPage(),
			}},
		},
	}
}
